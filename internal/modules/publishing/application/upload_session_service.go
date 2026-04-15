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
	"github.com/jackc/pgx/v5/pgxpool"

	mediaapp "cixing/internal/modules/media/application"
	dmedia "cixing/internal/modules/media/domain"
	mediadb "cixing/internal/modules/media/infra/db/gen"
	mediarepo "cixing/internal/modules/media/infra/db/repo"
	dpub "cixing/internal/modules/publishing/domain"
	publishingdb "cixing/internal/modules/publishing/infra/db/gen"
	pgrepo "cixing/internal/modules/publishing/infra/db/repo"
	"cixing/internal/platform/postgres"
	"cixing/internal/shared/common"
)

var (
	ErrInvalidUploadPublishInput = errors.New("invalid upload publish input")
	ErrUploadPublishExpired      = errors.New("upload publish session expired")
)

const uploadPublishSessionPutExpireDefault = 15 * time.Minute

type UploadSessionServiceConfig struct {
	Bucket           string
	UploadPrefix     string
	PutPresignExpire time.Duration
	Now              func() time.Time
}

type UploadSessionService struct {
	db               *pgxpool.Pool
	publishing       *Service
	storage          common.ObjectStorage
	bucket           string
	uploadPrefix     string
	putPresignExpire time.Duration
	now              func() time.Time
}

type CreateUploadSessionInput struct {
	OwnerUserID       uuid.UUID
	ContextType       dpub.PublishContextType
	OfficialKeywordID *uuid.UUID
	CustomKeywordID   *uuid.UUID
	BizDate           *time.Time
}

type CreateUploadSessionOutput struct {
	SessionID uuid.UUID
	Status    string
	ExpiresAt time.Time
}

type UploadPresignedTarget struct {
	Method    string
	URL       string
	Headers   map[string]string
	ObjectKey string
	ExpiresAt time.Time
}

type PresignBatchItemInput struct {
	ClientImageID      string
	ImageContentType   string
	ImageContentLength int64
	ImageSHA256        *string
	AudioContentType   *string
	AudioContentLength *int64
	AudioSHA256        *string
}

type PresignBatchInput struct {
	OwnerUserID uuid.UUID
	SessionID   uuid.UUID
	Items       []PresignBatchItemInput
}

type PresignBatchItemOutput struct {
	ItemID        uuid.UUID
	ClientImageID string
	ImageID       uuid.UUID
	ImageUpload   UploadPresignedTarget
	AudioUpload   *UploadPresignedTarget
}

type PresignBatchOutput struct {
	SessionID uuid.UUID
	Items     []PresignBatchItemOutput
}

type CompleteBatchItemInput struct {
	ItemID          uuid.UUID
	ImageEtag       string
	ImageWidth      int
	ImageHeight     int
	AudioEtag       *string
	AudioDurationMS *int
	DisplayOrder    int
	IsCover         bool
	Title           *string
	Note            *string
}

type CompleteBatchInput struct {
	OwnerUserID uuid.UUID
	SessionID   uuid.UUID
	Items       []CompleteBatchItemInput
}

type CompleteBatchItemOutput struct {
	ItemID       uuid.UUID
	DisplayOrder int
	IsCover      bool
	Status       string
}

type CompleteBatchOutput struct {
	SessionID uuid.UUID
	Status    string
	Items     []CompleteBatchItemOutput
}

type CommitUploadSessionInput struct {
	OwnerUserID uuid.UUID
	SessionID   uuid.UUID
}

type CommitUploadSessionOutput struct {
	SessionID        uuid.UUID
	Status           string
	UploadID         uuid.UUID
	UploadVisibility dpub.WorkUploadVisibilityStatus
}

func NewUploadSessionService(db *pgxpool.Pool, publishing *Service, storage common.ObjectStorage, cfg UploadSessionServiceConfig) (*UploadSessionService, error) {
	if db == nil || publishing == nil || storage == nil {
		return nil, errors.New("upload publish session service dependencies are required")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("upload publish session bucket is required")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.PutPresignExpire <= 0 {
		cfg.PutPresignExpire = uploadPublishSessionPutExpireDefault
	}
	return &UploadSessionService{
		db:               db,
		publishing:       publishing,
		storage:          storage,
		bucket:           strings.TrimSpace(cfg.Bucket),
		uploadPrefix:     strings.TrimSpace(cfg.UploadPrefix),
		putPresignExpire: cfg.PutPresignExpire,
		now:              cfg.Now,
	}, nil
}

func (s *UploadSessionService) Create(ctx context.Context, in CreateUploadSessionInput) (*CreateUploadSessionOutput, error) {
	if in.OwnerUserID == uuid.Nil || in.ContextType == "" {
		return nil, ErrInvalidUploadPublishInput
	}
	var (
		session *dpub.PublishSession
		err     error
	)
	switch in.ContextType {
	case dpub.PublishContextOfficialToday:
		if in.OfficialKeywordID == nil || in.BizDate == nil {
			return nil, ErrInvalidUploadPublishInput
		}
		session, err = s.publishing.CreateOfficialSession(ctx, CreateOfficialSessionInput{
			OwnerUserID:       in.OwnerUserID,
			OfficialKeywordID: *in.OfficialKeywordID,
			BizDate:           *in.BizDate,
		})
	case dpub.PublishContextCustomKeyword:
		if in.CustomKeywordID == nil {
			return nil, ErrInvalidUploadPublishInput
		}
		session, err = s.publishing.CreateCustomSession(ctx, CreateCustomSessionInput{
			OwnerUserID:     in.OwnerUserID,
			CustomKeywordID: *in.CustomKeywordID,
		})
	default:
		return nil, ErrInvalidUploadPublishInput
	}
	if err != nil {
		return nil, err
	}
	return &CreateUploadSessionOutput{
		SessionID: session.ID,
		Status:    mapSessionStatus(session.Status),
		ExpiresAt: session.ExpiresAt,
	}, nil
}

func (s *UploadSessionService) PresignBatch(ctx context.Context, in PresignBatchInput) (*PresignBatchOutput, error) {
	if in.OwnerUserID == uuid.Nil || in.SessionID == uuid.Nil || len(in.Items) == 0 {
		return nil, ErrInvalidUploadPublishInput
	}
	now := s.now().UTC()
	out := &PresignBatchOutput{SessionID: in.SessionID}

	err := postgres.WithTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		publishingRepo := pgrepo.NewRepository(publishingdb.New(tx))
		mediaRepo := mediarepo.NewRepository(mediadb.New(tx))
		mediaLifecycle, err := mediaapp.NewLifecycleService(mediaRepo, s.now)
		if err != nil {
			return err
		}

		agg, err := publishingRepo.GetAggregateForUpdate(ctx, in.SessionID, in.OwnerUserID)
		if err != nil {
			return err
		}
		if !agg.Session.ExpiresAt.After(now) {
			if _, err := publishingRepo.UpdateSession(ctx, dpub.UpdateSessionParams{
				ID:          agg.Session.ID,
				OwnerUserID: agg.Session.OwnerUserID,
				Status:      dpub.SessionStatusExpired,
			}); err != nil {
				return err
			}
			return ErrUploadPublishExpired
		}
		if err := agg.EnsureMutable(now); err != nil {
			return err
		}

		existingItemsByClientID := make(map[string]dpub.PublishSessionItem, len(agg.Items))
		for _, existingItem := range agg.Items {
			existingItemsByClientID[existingItem.ClientImageID] = existingItem
		}

		clientIDs := make(map[string]struct{}, len(in.Items))
		items := make([]PresignBatchItemOutput, 0, len(in.Items))
		for _, item := range in.Items {
			if err := validatePresignBatchItem(item); err != nil {
				return err
			}
			if _, exists := clientIDs[item.ClientImageID]; exists {
				return common.ErrConflict
			}
			clientIDs[item.ClientImageID] = struct{}{}

			if existingItem, ok := existingItemsByClientID[item.ClientImageID]; ok {
				reused, err := s.presignExistingPendingItem(ctx, mediaRepo, in.OwnerUserID, existingItem, item)
				if err != nil {
					return err
				}
				items = append(items, reused)
				continue
			}

			imageObjectKey := buildPublishObjectKey(s.uploadPrefix, in.OwnerUserID, in.SessionID, item.ClientImageID, "image", item.ImageContentType, now)
			imageAsset, err := mediaLifecycle.CreatePendingAsset(ctx, mediaapp.CreatePendingAssetInput{
				OwnerUserID:       in.OwnerUserID,
				Kind:              dmedia.KindImage,
				MimeType:          item.ImageContentType,
				OriginalObjectKey: imageObjectKey,
				ByteSize:          item.ImageContentLength,
			})
			if err != nil {
				return err
			}
			imagePresign, err := s.storage.PresignPut(ctx, s.bucket, imageObjectKey, item.ImageContentType, item.ImageContentLength, s.putPresignExpire)
			if err != nil {
				return err
			}

			var audioAssetID *uuid.UUID
			var audioUpload *UploadPresignedTarget
			if item.AudioContentType != nil && item.AudioContentLength != nil {
				audioObjectKey := buildPublishObjectKey(s.uploadPrefix, in.OwnerUserID, in.SessionID, item.ClientImageID, "audio", *item.AudioContentType, now)
				audioAsset, err := mediaLifecycle.CreatePendingAsset(ctx, mediaapp.CreatePendingAssetInput{
					OwnerUserID:       in.OwnerUserID,
					Kind:              dmedia.KindAudio,
					MimeType:          *item.AudioContentType,
					OriginalObjectKey: audioObjectKey,
					ByteSize:          *item.AudioContentLength,
				})
				if err != nil {
					return err
				}
				audioAssetID = &audioAsset.ID
				audioPresign, err := s.storage.PresignPut(ctx, s.bucket, audioObjectKey, *item.AudioContentType, *item.AudioContentLength, s.putPresignExpire)
				if err != nil {
					return err
				}
				audioUpload = &UploadPresignedTarget{
					Method:    audioPresign.Method,
					URL:       audioPresign.URL,
					Headers:   cloneStringMap(audioPresign.Headers),
					ObjectKey: audioObjectKey,
					ExpiresAt: audioPresign.ExpiresAt,
				}
			}

			sessionItem, err := publishingRepo.UpsertSessionItem(ctx, dpub.UpsertSessionItemParams{
				SessionID:     in.SessionID,
				OwnerUserID:   in.OwnerUserID,
				ClientImageID: item.ClientImageID,
				ImageAssetID:  imageAsset.ID,
				AudioAssetID:  audioAssetID,
				Status:        dpub.ItemStatusPendingUpload,
			})
			if err != nil {
				return err
			}

			items = append(items, PresignBatchItemOutput{
				ItemID:        sessionItem.ID,
				ClientImageID: item.ClientImageID,
				ImageID:       imageAsset.ID,
				ImageUpload: UploadPresignedTarget{
					Method:    imagePresign.Method,
					URL:       imagePresign.URL,
					Headers:   cloneStringMap(imagePresign.Headers),
					ObjectKey: imageObjectKey,
					ExpiresAt: imagePresign.ExpiresAt,
				},
				AudioUpload: audioUpload,
			})
		}

		out.Items = items
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *UploadSessionService) presignExistingPendingItem(ctx context.Context, mediaRepo *mediarepo.Repository, ownerUserID uuid.UUID, sessionItem dpub.PublishSessionItem, item PresignBatchItemInput) (PresignBatchItemOutput, error) {
	if sessionItem.Status != dpub.ItemStatusPendingUpload {
		return PresignBatchItemOutput{}, common.ErrConflict
	}

	imageAsset, err := mediaRepo.GetAssetByID(ctx, sessionItem.ImageAssetID)
	if err != nil {
		return PresignBatchItemOutput{}, err
	}
	if err := validatePendingAssetForPresignReuse(imageAsset, ownerUserID, dmedia.KindImage, item.ImageContentType, item.ImageContentLength); err != nil {
		return PresignBatchItemOutput{}, err
	}
	imagePresign, err := s.storage.PresignPut(ctx, s.bucket, imageAsset.OriginalObjectKey, imageAsset.MimeType, imageAsset.ByteSize, s.putPresignExpire)
	if err != nil {
		return PresignBatchItemOutput{}, err
	}

	var audioUpload *UploadPresignedTarget
	if sessionItem.AudioAssetID != nil {
		if item.AudioContentType == nil || item.AudioContentLength == nil {
			return PresignBatchItemOutput{}, common.ErrConflict
		}
		audioAsset, err := mediaRepo.GetAssetByID(ctx, *sessionItem.AudioAssetID)
		if err != nil {
			return PresignBatchItemOutput{}, err
		}
		if err := validatePendingAssetForPresignReuse(audioAsset, ownerUserID, dmedia.KindAudio, *item.AudioContentType, *item.AudioContentLength); err != nil {
			return PresignBatchItemOutput{}, err
		}
		audioPresign, err := s.storage.PresignPut(ctx, s.bucket, audioAsset.OriginalObjectKey, audioAsset.MimeType, audioAsset.ByteSize, s.putPresignExpire)
		if err != nil {
			return PresignBatchItemOutput{}, err
		}
		audioUpload = &UploadPresignedTarget{
			Method:    audioPresign.Method,
			URL:       audioPresign.URL,
			Headers:   cloneStringMap(audioPresign.Headers),
			ObjectKey: audioAsset.OriginalObjectKey,
			ExpiresAt: audioPresign.ExpiresAt,
		}
	} else if item.AudioContentType != nil || item.AudioContentLength != nil {
		return PresignBatchItemOutput{}, common.ErrConflict
	}

	return PresignBatchItemOutput{
		ItemID:        sessionItem.ID,
		ClientImageID: sessionItem.ClientImageID,
		ImageID:       imageAsset.ID,
		ImageUpload: UploadPresignedTarget{
			Method:    imagePresign.Method,
			URL:       imagePresign.URL,
			Headers:   cloneStringMap(imagePresign.Headers),
			ObjectKey: imageAsset.OriginalObjectKey,
			ExpiresAt: imagePresign.ExpiresAt,
		},
		AudioUpload: audioUpload,
	}, nil
}

func (s *UploadSessionService) CompleteBatch(ctx context.Context, in CompleteBatchInput) (*CompleteBatchOutput, error) {
	if in.OwnerUserID == uuid.Nil || in.SessionID == uuid.Nil || len(in.Items) == 0 {
		return nil, ErrInvalidUploadPublishInput
	}
	now := s.now().UTC()
	out := &CompleteBatchOutput{SessionID: in.SessionID}

	err := postgres.WithTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		publishingRepo := pgrepo.NewRepository(publishingdb.New(tx))
		mediaRepo := mediarepo.NewRepository(mediadb.New(tx))
		mediaLifecycle, err := mediaapp.NewLifecycleService(mediaRepo, s.now)
		if err != nil {
			return err
		}

		agg, err := publishingRepo.GetAggregateForUpdate(ctx, in.SessionID, in.OwnerUserID)
		if err != nil {
			return err
		}
		if !agg.Session.ExpiresAt.After(now) {
			if _, err := publishingRepo.UpdateSession(ctx, dpub.UpdateSessionParams{
				ID:          agg.Session.ID,
				OwnerUserID: agg.Session.OwnerUserID,
				Status:      dpub.SessionStatusExpired,
			}); err != nil {
				return err
			}
			return ErrUploadPublishExpired
		}
		if err := agg.EnsureMutable(now); err != nil {
			return err
		}

		itemByID := make(map[uuid.UUID]dpub.PublishSessionItem, len(agg.Items))
		for _, item := range agg.Items {
			itemByID[item.ID] = item
		}

		seenItemIDs := make(map[uuid.UUID]struct{}, len(in.Items))
		seenOrders := make(map[int]struct{}, len(in.Items))
		coverCount := 0
		for _, item := range in.Items {
			if err := validateCompleteBatchItem(item); err != nil {
				return err
			}
			if _, ok := itemByID[item.ItemID]; !ok {
				return common.ErrNotFound
			}
			if _, exists := seenItemIDs[item.ItemID]; exists {
				return common.ErrConflict
			}
			if _, exists := seenOrders[item.DisplayOrder]; exists {
				return common.ErrConflict
			}
			seenItemIDs[item.ItemID] = struct{}{}
			seenOrders[item.DisplayOrder] = struct{}{}
			if item.IsCover {
				coverCount++
			}
		}
		if coverCount > 1 {
			return common.ErrConflict
		}
		if coverCount == 1 {
			if err := publishingRepo.ClearSessionItemCover(ctx, in.SessionID, in.OwnerUserID); err != nil {
				return err
			}
		}

		itemsOut := make([]CompleteBatchItemOutput, 0, len(in.Items))
		for _, reqItem := range in.Items {
			sessionItem := itemByID[reqItem.ItemID]

			imageAsset, err := mediaRepo.GetAssetByID(ctx, sessionItem.ImageAssetID)
			if err != nil {
				return err
			}
			if err := ensureOwnedAssetUploaded(ctx, mediaLifecycle, imageAsset, in.OwnerUserID, dmedia.KindImage, s.storage, s.bucket, reqItem.ImageEtag, reqItem.ImageWidth, reqItem.ImageHeight, nil); err != nil {
				return err
			}

			var audioAssetID *uuid.UUID
			if sessionItem.AudioAssetID != nil {
				if reqItem.AudioEtag == nil {
					return ErrInvalidUploadPublishInput
				}
				audioAsset, err := mediaRepo.GetAssetByID(ctx, *sessionItem.AudioAssetID)
				if err != nil {
					return err
				}
				if err := ensureOwnedAssetUploaded(ctx, mediaLifecycle, audioAsset, in.OwnerUserID, dmedia.KindAudio, s.storage, s.bucket, *reqItem.AudioEtag, 0, 0, reqItem.AudioDurationMS); err != nil {
					return err
				}
				audioAssetID = sessionItem.AudioAssetID
			} else if reqItem.AudioEtag != nil || reqItem.AudioDurationMS != nil {
				return ErrInvalidUploadPublishInput
			}

			displayOrder := int32(reqItem.DisplayOrder)
			title := normalizeOptionalText(reqItem.Title)
			note := normalizeOptionalText(reqItem.Note)
			sessionItem, err = publishingRepo.UpsertSessionItem(ctx, dpub.UpsertSessionItemParams{
				SessionID:     in.SessionID,
				OwnerUserID:   in.OwnerUserID,
				ClientImageID: sessionItem.ClientImageID,
				ImageAssetID:  sessionItem.ImageAssetID,
				AudioAssetID:  audioAssetID,
				DisplayOrder:  &displayOrder,
				IsCover:       reqItem.IsCover,
				Title:         title,
				Note:          note,
				Status:        dpub.ItemStatusUploaded,
			})
			if err != nil {
				return err
			}

			itemsOut = append(itemsOut, CompleteBatchItemOutput{
				ItemID:       sessionItem.ID,
				DisplayOrder: reqItem.DisplayOrder,
				IsCover:      reqItem.IsCover,
				Status:       mapItemStatus(sessionItem.Status),
			})
		}

		out.Status = mapSessionStatus(agg.Session.Status)
		out.Items = itemsOut
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *UploadSessionService) Commit(ctx context.Context, in CommitUploadSessionInput) (*CommitUploadSessionOutput, error) {
	if in.OwnerUserID == uuid.Nil || in.SessionID == uuid.Nil {
		return nil, ErrInvalidUploadPublishInput
	}
	out, err := s.publishing.CommitSession(ctx, CommitSessionInput{
		OwnerUserID: in.OwnerUserID,
		SessionID:   in.SessionID,
	})
	if err != nil {
		return nil, err
	}
	return &CommitUploadSessionOutput{
		SessionID:        out.SessionID,
		Status:           mapSessionStatus(dpub.SessionStatusCommitted),
		UploadID:         out.UploadID,
		UploadVisibility: dpub.WorkUploadVisibilityVisible,
	}, nil
}

func validatePresignBatchItem(item PresignBatchItemInput) error {
	if strings.TrimSpace(item.ClientImageID) == "" {
		return ErrInvalidUploadPublishInput
	}
	if !strings.HasPrefix(strings.TrimSpace(strings.ToLower(item.ImageContentType)), "image/") || item.ImageContentLength <= 0 {
		return ErrInvalidUploadPublishInput
	}
	if item.AudioContentType != nil || item.AudioContentLength != nil {
		if item.AudioContentType == nil || item.AudioContentLength == nil {
			return ErrInvalidUploadPublishInput
		}
		if !strings.HasPrefix(strings.TrimSpace(strings.ToLower(*item.AudioContentType)), "audio/") || *item.AudioContentLength <= 0 {
			return ErrInvalidUploadPublishInput
		}
	}
	return nil
}

func validatePendingAssetForPresignReuse(asset dmedia.Asset, ownerUserID uuid.UUID, expectedKind dmedia.Kind, contentType string, contentLength int64) error {
	if asset.OwnerUserID != ownerUserID || asset.Kind != expectedKind {
		return dpub.ErrInvalidAsset
	}
	if asset.Status != dmedia.AssetStatusPendingUpload {
		return common.ErrConflict
	}
	if !equalContentTypes(asset.MimeType, contentType) || asset.ByteSize != contentLength {
		return common.ErrConflict
	}
	if strings.TrimSpace(asset.OriginalObjectKey) == "" {
		return common.ErrConflict
	}
	return nil
}

func equalContentTypes(left string, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func validateCompleteBatchItem(item CompleteBatchItemInput) error {
	if item.ItemID == uuid.Nil || strings.TrimSpace(item.ImageEtag) == "" || item.ImageWidth <= 0 || item.ImageHeight <= 0 || item.DisplayOrder <= 0 {
		return ErrInvalidUploadPublishInput
	}
	if item.AudioDurationMS != nil && *item.AudioDurationMS < 0 {
		return ErrInvalidUploadPublishInput
	}
	if item.AudioEtag != nil && strings.TrimSpace(*item.AudioEtag) == "" {
		return ErrInvalidUploadPublishInput
	}
	return nil
}

func ensureOwnedAssetUploaded(ctx context.Context, lifecycle *mediaapp.LifecycleService, asset dmedia.Asset, ownerUserID uuid.UUID, expectedKind dmedia.Kind, storage common.ObjectStorage, bucket string, expectedETag string, width int, height int, durationMS *int) error {
	if asset.OwnerUserID != ownerUserID || asset.Kind != expectedKind {
		return dpub.ErrInvalidAsset
	}
	exists, size, etag, _, err := storage.Head(ctx, bucket, asset.OriginalObjectKey)
	if err != nil {
		return err
	}
	if !exists || size != asset.ByteSize {
		return common.ErrConflict
	}
	if normalizeETag(expectedETag) != normalizeETag(etag) {
		return common.ErrConflict
	}

	switch asset.Status {
	case dmedia.AssetStatusPendingUpload:
		var widthPtr *int32
		var heightPtr *int32
		if width > 0 {
			w := int32(width)
			widthPtr = &w
		}
		if height > 0 {
			h := int32(height)
			heightPtr = &h
		}
		var durationPtr *int32
		if durationMS != nil {
			d := int32(*durationMS)
			durationPtr = &d
		}
		_, err := lifecycle.MarkAssetUploaded(ctx, mediaapp.MarkAssetUploadedInput{
			AssetID:    asset.ID,
			Width:      widthPtr,
			Height:     heightPtr,
			DurationMS: durationPtr,
		})
		return err
	case dmedia.AssetStatusUploaded, dmedia.AssetStatusProcessing, dmedia.AssetStatusReady:
		return nil
	default:
		return common.ErrConflict
	}
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

func mapSessionStatus(status dpub.SessionStatus) string {
	switch status {
	case dpub.SessionStatusCreated:
		return "created"
	case dpub.SessionStatusCommitted:
		return "committed"
	case dpub.SessionStatusCanceled:
		return "canceled"
	case dpub.SessionStatusExpired:
		return "expired"
	default:
		return string(status)
	}
}

func mapItemStatus(status dpub.ItemStatus) string {
	switch status {
	case dpub.ItemStatusPendingUpload:
		return "pending_upload"
	case dpub.ItemStatusUploaded:
		return "uploaded"
	default:
		return string(status)
	}
}

func buildPublishObjectKey(prefix string, userID uuid.UUID, sessionID uuid.UUID, clientImageID string, kind string, contentType string, now time.Time) string {
	name := strings.TrimLeft(path.Join("uploads", userID.String(), sessionID.String(), clientImageID, kind+extByContentType(contentType)), "/")
	pfx := strings.Trim(prefix, "/")
	datePath := now.UTC().Format("2006/01/02")
	if pfx == "" {
		return path.Join(datePath, name)
	}
	return path.Join(pfx, datePath, name)
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

func cloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
