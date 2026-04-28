package application

import (
	"context"
	"errors"
	"mime"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	mediaapp "cixing/internal/modules/media/application"
	dmedia "cixing/internal/modules/media/domain"
	mediadb "cixing/internal/modules/media/infra/db/gen"
	mediarepo "cixing/internal/modules/media/infra/db/repo"
	profiledb "cixing/internal/modules/profile/infra/db/gen"
	profilerepo "cixing/internal/modules/profile/infra/db/repo"
	platformoss "cixing/internal/platform/oss"
	"cixing/internal/platform/postgres"
	"cixing/internal/shared/common"
)

const (
	avatarUploadSessionTTL              = 48 * time.Hour
	avatarUploadSessionPutExpireDefault = 15 * time.Minute

	avatarUploadStatusCreated   = "created"
	avatarUploadStatusPresigned = "presigned"
	avatarUploadStatusCompleted = "completed"
	avatarUploadStatusExpired   = "expired"
)

type AvatarUploadServiceConfig struct {
	Bucket           string
	UploadPrefix     string
	PutPresignExpire time.Duration
	Now              func() time.Time
}

type AvatarUploadService struct {
	db               *pgxpool.Pool
	storage          common.ObjectStorage
	resolver         *platformoss.URLResolver
	bucket           string
	uploadPrefix     string
	putPresignExpire time.Duration
	now              func() time.Time
}

type CreateAvatarUploadSessionOutput struct {
	SessionID uuid.UUID
	Status    string
	ExpiresAt time.Time
}

type PresignAvatarImageInput struct {
	UserID             uuid.UUID
	SessionID          uuid.UUID
	ImageContentType   string
	ImageContentLength int64
	ImageSHA256        *string
}

type PresignedUploadTarget struct {
	Method    string
	URL       string
	Headers   map[string]string
	ObjectKey string
	ExpiresAt time.Time
}

type PresignAvatarImageOutput struct {
	SessionID   uuid.UUID
	Status      string
	ImageID     uuid.UUID
	ImageUpload PresignedUploadTarget
}

type CompleteAvatarUploadInput struct {
	UserID      uuid.UUID
	SessionID   uuid.UUID
	ImageETag   string
	ImageWidth  int
	ImageHeight int
}

type CompleteAvatarUploadOutput struct {
	SessionID uuid.UUID
	Status    string
	Profile   ProfileOutput
}

func NewAvatarUploadService(db *pgxpool.Pool, storage common.ObjectStorage, resolver *platformoss.URLResolver, cfg AvatarUploadServiceConfig) (*AvatarUploadService, error) {
	if db == nil || storage == nil {
		return nil, errors.New("avatar upload service dependencies are required")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("avatar upload bucket is required")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.PutPresignExpire <= 0 {
		cfg.PutPresignExpire = avatarUploadSessionPutExpireDefault
	}
	return &AvatarUploadService{
		db:               db,
		storage:          storage,
		resolver:         resolver,
		bucket:           strings.TrimSpace(cfg.Bucket),
		uploadPrefix:     strings.TrimSpace(cfg.UploadPrefix),
		putPresignExpire: cfg.PutPresignExpire,
		now:              cfg.Now,
	}, nil
}

func (s *AvatarUploadService) Create(ctx context.Context, userID uuid.UUID) (*CreateAvatarUploadSessionOutput, error) {
	if userID == uuid.Nil {
		return nil, ErrInvalidAvatarUploadInput
	}
	expiresAt := s.now().UTC().Add(avatarUploadSessionTTL)
	row, err := profiledb.New(s.db).CreateAvatarUploadSession(ctx, profiledb.CreateAvatarUploadSessionParams{
		UserID:    userID,
		ExpiresAt: timestamptz(expiresAt),
	})
	if err != nil {
		return nil, err
	}
	return &CreateAvatarUploadSessionOutput{
		SessionID: row.ID,
		Status:    row.Status,
		ExpiresAt: row.ExpiresAt.Time,
	}, nil
}

func (s *AvatarUploadService) PresignImage(ctx context.Context, in PresignAvatarImageInput) (*PresignAvatarImageOutput, error) {
	if in.UserID == uuid.Nil || in.SessionID == uuid.Nil || !isImageContentType(in.ImageContentType) || in.ImageContentLength <= 0 {
		return nil, ErrInvalidAvatarUploadInput
	}

	var out *PresignAvatarImageOutput
	now := s.now().UTC()
	err := postgres.WithTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		profileQ := profiledb.New(tx)
		mediaRepo := mediarepo.NewRepository(mediadb.New(tx))
		mediaLifecycle, err := mediaapp.NewLifecycleService(mediaRepo, s.now)
		if err != nil {
			return err
		}
		session, err := loadAvatarSessionForUpdate(ctx, profileQ, in.SessionID, in.UserID)
		if err != nil {
			return err
		}
		if err := ensureAvatarSessionMutable(ctx, profileQ, session, now); err != nil {
			return err
		}

		var asset dmedia.Asset
		switch session.Status {
		case avatarUploadStatusCreated:
			if session.ImageAssetID.Valid {
				return common.ErrConflict
			}
			objectKey := buildAvatarObjectKey(s.uploadPrefix, in.UserID, in.SessionID, in.ImageContentType, now)
			asset, err = mediaLifecycle.CreatePendingAsset(ctx, mediaapp.CreatePendingAssetInput{
				OwnerUserID:       in.UserID,
				Kind:              dmedia.KindImage,
				MimeType:          strings.TrimSpace(in.ImageContentType),
				OriginalObjectKey: objectKey,
				ByteSize:          in.ImageContentLength,
			})
			if err != nil {
				return err
			}
			session, err = profileQ.SetAvatarUploadSessionImage(ctx, profiledb.SetAvatarUploadSessionImageParams{
				ID:           in.SessionID,
				UserID:       in.UserID,
				ImageAssetID: uuidToPgtype(asset.ID),
			})
			if err != nil {
				return err
			}
		case avatarUploadStatusPresigned:
			if !session.ImageAssetID.Valid {
				return common.ErrConflict
			}
			asset, err = mediaRepo.GetAssetByID(ctx, session.ImageAssetID.Bytes)
			if err != nil {
				return err
			}
			if err := validatePendingAvatarAssetForReuse(asset, in.UserID, in.ImageContentType, in.ImageContentLength); err != nil {
				return err
			}
		default:
			return common.ErrConflict
		}

		presigned, err := s.storage.PresignPut(ctx, s.bucket, asset.OriginalObjectKey, asset.MimeType, asset.ByteSize, s.putPresignExpire)
		if err != nil {
			return err
		}
		out = &PresignAvatarImageOutput{
			SessionID: session.ID,
			Status:    session.Status,
			ImageID:   asset.ID,
			ImageUpload: PresignedUploadTarget{
				Method:    presigned.Method,
				URL:       presigned.URL,
				Headers:   cloneStringMap(presigned.Headers),
				ObjectKey: asset.OriginalObjectKey,
				ExpiresAt: presigned.ExpiresAt,
			},
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *AvatarUploadService) Complete(ctx context.Context, in CompleteAvatarUploadInput) (*CompleteAvatarUploadOutput, error) {
	if in.UserID == uuid.Nil || in.SessionID == uuid.Nil || strings.TrimSpace(in.ImageETag) == "" || in.ImageWidth <= 0 || in.ImageHeight <= 0 {
		return nil, ErrInvalidAvatarUploadInput
	}

	var out *CompleteAvatarUploadOutput
	now := s.now().UTC()
	err := postgres.WithTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		profileQ := profiledb.New(tx)
		session, err := loadAvatarSessionForUpdate(ctx, profileQ, in.SessionID, in.UserID)
		if err != nil {
			return err
		}
		if session.Status == avatarUploadStatusCompleted {
			profile, err := s.loadAvatarProfile(ctx, profileQ, in.UserID)
			if err != nil {
				return err
			}
			out = &CompleteAvatarUploadOutput{
				SessionID: session.ID,
				Status:    session.Status,
				Profile:   profile,
			}
			return nil
		}
		if err := ensureAvatarSessionCompletable(ctx, profileQ, session, now); err != nil {
			return err
		}
		if !session.ImageAssetID.Valid {
			return common.ErrConflict
		}

		mediaRepo := mediarepo.NewRepository(mediadb.New(tx))
		mediaLifecycle, err := mediaapp.NewLifecycleService(mediaRepo, s.now)
		if err != nil {
			return err
		}
		asset, err := mediaRepo.GetAssetByID(ctx, session.ImageAssetID.Bytes)
		if err != nil {
			return err
		}
		if err := s.ensureUploadedAvatarAsset(ctx, mediaLifecycle, asset, in); err != nil {
			return err
		}

		affected, err := profileQ.UpdateUserAvatarAsset(ctx, profiledb.UpdateUserAvatarAssetParams{
			UserID:               in.UserID,
			CurrentAvatarAssetID: uuidToPgtype(asset.ID),
		})
		if err != nil {
			return err
		}
		if affected == 0 {
			return common.ErrNotFound
		}
		completed, err := profileQ.CompleteAvatarUploadSession(ctx, profiledb.CompleteAvatarUploadSessionParams{
			ID:     in.SessionID,
			UserID: in.UserID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return common.ErrNotFound
			}
			return err
		}
		profile, err := s.loadAvatarProfile(ctx, profileQ, in.UserID)
		if err != nil {
			return err
		}
		out = &CompleteAvatarUploadOutput{
			SessionID: completed.ID,
			Status:    completed.Status,
			Profile:   profile,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *AvatarUploadService) ensureUploadedAvatarAsset(ctx context.Context, lifecycle *mediaapp.LifecycleService, asset dmedia.Asset, in CompleteAvatarUploadInput) error {
	if asset.OwnerUserID != in.UserID || asset.Kind != dmedia.KindImage {
		return common.ErrConflict
	}
	exists, size, etag, _, err := s.storage.Head(ctx, s.bucket, asset.OriginalObjectKey)
	if err != nil {
		return err
	}
	if !exists || size != asset.ByteSize {
		return common.ErrConflict
	}
	if normalizeETag(in.ImageETag) != normalizeETag(etag) {
		return common.ErrConflict
	}
	switch asset.Status {
	case dmedia.AssetStatusPendingUpload:
		width := int32(in.ImageWidth)
		height := int32(in.ImageHeight)
		_, err = lifecycle.MarkAssetUploaded(ctx, mediaapp.MarkAssetUploadedInput{
			AssetID: asset.ID,
			Width:   &width,
			Height:  &height,
		})
		return err
	case dmedia.AssetStatusUploaded, dmedia.AssetStatusProcessing, dmedia.AssetStatusReady:
		return nil
	default:
		return common.ErrConflict
	}
}

func (s *AvatarUploadService) loadAvatarProfile(ctx context.Context, q profiledb.Querier, userID uuid.UUID) (ProfileOutput, error) {
	settings, err := profilerepo.NewRepository(q).GetSettings(ctx, userID)
	if err != nil {
		return ProfileOutput{}, err
	}
	return ProfileOutput{
		Nickname:  settings.Nickname,
		AvatarURL: resolveSquareSmallURL(s.resolver, settings.AvatarObjectKey),
		Email:     settings.Email,
	}, nil
}

func loadAvatarSessionForUpdate(ctx context.Context, q profiledb.Querier, sessionID uuid.UUID, userID uuid.UUID) (profiledb.AvatarUploadSession, error) {
	session, err := q.GetAvatarUploadSessionForUpdate(ctx, profiledb.GetAvatarUploadSessionForUpdateParams{
		ID:     sessionID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return profiledb.AvatarUploadSession{}, common.ErrNotFound
		}
		return profiledb.AvatarUploadSession{}, err
	}
	return session, nil
}

func ensureAvatarSessionMutable(ctx context.Context, q profiledb.Querier, session profiledb.AvatarUploadSession, now time.Time) error {
	if !session.ExpiresAt.Time.After(now) {
		if session.Status != avatarUploadStatusCompleted && session.Status != avatarUploadStatusExpired {
			if _, err := q.ExpireAvatarUploadSession(ctx, profiledb.ExpireAvatarUploadSessionParams{
				ID:     session.ID,
				UserID: session.UserID,
			}); err != nil {
				return err
			}
		}
		return ErrAvatarUploadExpired
	}
	switch session.Status {
	case avatarUploadStatusCreated, avatarUploadStatusPresigned:
		return nil
	default:
		return common.ErrConflict
	}
}

func ensureAvatarSessionCompletable(ctx context.Context, q profiledb.Querier, session profiledb.AvatarUploadSession, now time.Time) error {
	if !session.ExpiresAt.Time.After(now) {
		if session.Status != avatarUploadStatusExpired {
			if _, err := q.ExpireAvatarUploadSession(ctx, profiledb.ExpireAvatarUploadSessionParams{
				ID:     session.ID,
				UserID: session.UserID,
			}); err != nil {
				return err
			}
		}
		return ErrAvatarUploadExpired
	}
	if session.Status != avatarUploadStatusPresigned {
		return common.ErrConflict
	}
	return nil
}

func validatePendingAvatarAssetForReuse(asset dmedia.Asset, ownerUserID uuid.UUID, contentType string, contentLength int64) error {
	if asset.OwnerUserID != ownerUserID || asset.Kind != dmedia.KindImage {
		return common.ErrConflict
	}
	if asset.Status != dmedia.AssetStatusPendingUpload {
		return common.ErrConflict
	}
	if !strings.EqualFold(strings.TrimSpace(asset.MimeType), strings.TrimSpace(contentType)) || asset.ByteSize != contentLength {
		return common.ErrConflict
	}
	if strings.TrimSpace(asset.OriginalObjectKey) == "" {
		return common.ErrConflict
	}
	return nil
}

func buildAvatarObjectKey(prefix string, userID uuid.UUID, sessionID uuid.UUID, contentType string, now time.Time) string {
	datePath := now.UTC().Format("2006/01/02")
	name := path.Join(datePath, "avatars", userID.String(), sessionID.String(), "image"+extByContentType(contentType))
	pfx := strings.Trim(prefix, "/")
	if pfx == "" {
		return strings.TrimLeft(name, "/")
	}
	return strings.TrimLeft(path.Join(pfx, name), "/")
}

func extByContentType(contentType string) string {
	exts, _ := mime.ExtensionsByType(contentType)
	for _, ext := range exts {
		ext = strings.TrimSpace(strings.ToLower(ext))
		if ext == ".jpeg" {
			return ".jpg"
		}
		if ext != "" {
			return ext
		}
	}
	if slash := strings.LastIndex(contentType, "/"); slash >= 0 && slash < len(contentType)-1 {
		ext := "." + strings.TrimSpace(strings.ToLower(contentType[slash+1:]))
		if ext == ".jpeg" {
			return ".jpg"
		}
		if ext != "." {
			return ext
		}
	}
	return ".bin"
}

func isImageContentType(contentType string) bool {
	return strings.HasPrefix(strings.TrimSpace(strings.ToLower(contentType)), "image/")
}

func normalizeETag(v string) string {
	normalized := strings.TrimSpace(v)
	if strings.HasPrefix(strings.ToLower(normalized), "w/") {
		normalized = normalized[2:]
	}
	normalized = strings.TrimSpace(normalized)
	normalized = strings.Trim(normalized, "\"")
	return strings.ToLower(strings.TrimSpace(normalized))
}

func cloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func uuidToPgtype(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func timestamptz(v time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: v, Valid: true}
}
