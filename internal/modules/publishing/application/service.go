package application

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	customrepo "cixing/internal/modules/customkeywords/infra/db/repo"
	officialapp "cixing/internal/modules/official/application"
	officialdb "cixing/internal/modules/official/infra/db/gen"
	officialrepo "cixing/internal/modules/official/infra/db/repo"
	dpub "cixing/internal/modules/publishing/domain"
	publishingdb "cixing/internal/modules/publishing/infra/db/gen"
	pgrepo "cixing/internal/modules/publishing/infra/db/repo"
	"cixing/internal/platform/postgres"
	"cixing/internal/shared/common"
)

const publishSessionTTL = 48 * time.Hour

type Service struct {
	db              *pgxpool.Pool
	officialCatalog *officialapp.CatalogService
	customKeywords  *customrepo.Repository
	now             func() time.Time
}

func NewService(db *pgxpool.Pool, officialCatalog *officialapp.CatalogService, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{db: db, officialCatalog: officialCatalog, now: now}
}

func (s *Service) WithCustomKeywords(repo *customrepo.Repository) *Service {
	s.customKeywords = repo
	return s
}

type CreateOfficialSessionInput struct {
	OwnerUserID       uuid.UUID
	OfficialKeywordID uuid.UUID
	BizDate           time.Time
}

type CreateCustomSessionInput struct {
	OwnerUserID     uuid.UUID
	CustomKeywordID uuid.UUID
}

type CommitSessionInput struct {
	OwnerUserID uuid.UUID
	SessionID   uuid.UUID
}

type CommitSessionOutput struct {
	SessionID uuid.UUID
	UploadID  uuid.UUID
}

func (s *Service) CreateOfficialSession(ctx context.Context, in CreateOfficialSessionInput) (*dpub.PublishSession, error) {
	if s == nil || s.db == nil {
		return nil, dpub.ErrInvalidState
	}
	if in.OwnerUserID == uuid.Nil || in.OfficialKeywordID == uuid.Nil {
		return nil, ErrInvalidUploadPublishInput
	}

	repo := s.repo(publishingdb.New(s.db))
	bizDate := common.NormalizeBizDate(in.BizDate)
	assignment, err := s.dailyKeywordAssignment(ctx, repo, bizDate)
	if err != nil {
		return nil, err
	}
	if assignment.KeywordID != in.OfficialKeywordID {
		return nil, dpub.ErrKeywordMismatch
	}

	now := s.now().UTC()
	session := dpub.NewOfficialSession(in.OwnerUserID, in.OfficialKeywordID, bizDate, now, now.Add(publishSessionTTL))
	created, err := repo.CreateSession(ctx, dpub.CreateSessionParams{
		OwnerUserID:       session.OwnerUserID,
		ContextType:       session.ContextType,
		OfficialKeywordID: session.OfficialKeywordID,
		BizDate:           session.BizDate,
		Status:            session.Status,
		ExpiresAt:         session.ExpiresAt,
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (s *Service) CreateCustomSession(ctx context.Context, in CreateCustomSessionInput) (*dpub.PublishSession, error) {
	if s == nil || s.db == nil {
		return nil, dpub.ErrInvalidState
	}
	if in.OwnerUserID == uuid.Nil || in.CustomKeywordID == uuid.Nil {
		return nil, ErrInvalidUploadPublishInput
	}
	if err := s.ensureCustomKeywordActive(ctx, in.OwnerUserID, in.CustomKeywordID); err != nil {
		return nil, err
	}

	repo := s.repo(publishingdb.New(s.db))
	now := s.now().UTC()
	session := dpub.NewCustomSession(in.OwnerUserID, in.CustomKeywordID, now, now.Add(publishSessionTTL))
	created, err := repo.CreateSession(ctx, dpub.CreateSessionParams{
		OwnerUserID:     session.OwnerUserID,
		ContextType:     session.ContextType,
		CustomKeywordID: session.CustomKeywordID,
		Status:          session.Status,
		ExpiresAt:       session.ExpiresAt,
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (s *Service) CommitSession(ctx context.Context, in CommitSessionInput) (*CommitSessionOutput, error) {
	if s == nil || s.db == nil {
		return nil, dpub.ErrInvalidState
	}
	if in.OwnerUserID == uuid.Nil || in.SessionID == uuid.Nil {
		return nil, ErrInvalidUploadPublishInput
	}

	out := &CommitSessionOutput{SessionID: in.SessionID}
	err := postgres.WithTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		repo := s.repo(publishingdb.New(tx))
		agg, err := repo.GetAggregateForUpdate(ctx, in.SessionID, in.OwnerUserID)
		if err != nil {
			return err
		}

		switch agg.Session.ContextType {
		case dpub.PublishContextOfficialToday:
			if agg.Session.BizDate == nil {
				return dpub.ErrInvalidContext
			}
			assignment, err := s.dailyKeywordAssignment(ctx, repo, *agg.Session.BizDate)
			if err != nil {
				return err
			}
			ordered, cover, err := agg.BeginOfficialCommit(s.now(), assignment.KeywordID)
			if err != nil {
				return err
			}
			return s.persistCommittedUpload(ctx, tx, repo, agg, ordered, cover, out)

		case dpub.PublishContextCustomKeyword:
			if agg.Session.CustomKeywordID == nil {
				return dpub.ErrInvalidContext
			}
			if err := s.ensureCustomKeywordActive(ctx, in.OwnerUserID, *agg.Session.CustomKeywordID); err != nil {
				return err
			}
			ordered, cover, err := agg.BeginCustomCommit(s.now(), *agg.Session.CustomKeywordID)
			if err != nil {
				return err
			}
			return s.persistCommittedUpload(ctx, tx, repo, agg, ordered, cover, out)

		default:
			return dpub.ErrInvalidContext
		}
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) persistCommittedUpload(ctx context.Context, tx pgx.Tx, repo *pgrepo.Repository, agg dpub.Aggregate, ordered []dpub.PublishSessionItem, cover dpub.PublishSessionItem, out *CommitSessionOutput) error {
	publishedAt := s.now().UTC()
	workUpload, err := repo.CreateWorkUpload(ctx, dpub.CreateWorkUploadParams{
		AuthorUserID:      agg.Session.OwnerUserID,
		ContextType:       agg.Session.ContextType,
		OfficialKeywordID: agg.Session.OfficialKeywordID,
		CustomKeywordID:   agg.Session.CustomKeywordID,
		BizDate:           agg.Session.BizDate,
		VisibilityStatus:  dpub.WorkUploadVisibilityVisible,
		CoverAssetID:      cover.ImageAssetID,
		ImageCount:        int32(len(ordered)),
		PublishedAt:       publishedAt,
	})
	if err != nil {
		return err
	}

	for _, item := range ordered {
		image, err := repo.CreateWorkUploadImage(ctx, dpub.CreateWorkUploadImageParams{
			UploadID:     workUpload.ID,
			ImageAssetID: item.ImageAssetID,
			DisplayOrder: *item.DisplayOrder,
		})
		if err != nil {
			return err
		}

		if item.Title == nil && item.Note == nil && item.AudioAssetID == nil {
			continue
		}

		var audioDuration *int32
		if item.AudioAssetID != nil {
			audioAsset, err := repo.GetMediaAssetLite(ctx, *item.AudioAssetID)
			if err != nil {
				return err
			}
			if err := validateAssetOwnership(audioAsset, agg.Session.OwnerUserID, dpub.MediaKindAudio); err != nil {
				return err
			}
			audioDuration = audioAsset.DurationMS
		}
		if _, err := repo.UpsertWorkUploadImageContent(ctx, dpub.UpsertWorkUploadImageContentParams{
			WorkUploadImageID: image.ID,
			Title:             item.Title,
			Note:              item.Note,
			AudioAssetID:      item.AudioAssetID,
			AudioDurationMS:   audioDuration,
		}); err != nil {
			return err
		}
	}

	if err := agg.MarkCommitted(workUpload.ID); err != nil {
		return err
	}
	if _, err := repo.UpdateSession(ctx, dpub.UpdateSessionParams{
		ID:              agg.Session.ID,
		OwnerUserID:     agg.Session.OwnerUserID,
		Status:          agg.Session.Status,
		PublishedUpload: agg.Session.PublishedUploadID,
	}); err != nil {
		return err
	}
	if agg.Session.ContextType == dpub.PublishContextOfficialToday && agg.Session.BizDate != nil {
		if _, err := officialrepo.NewRepository(officialdb.New(tx)).RecomputeDailyKeywordStatsFromUploads(ctx, *agg.Session.BizDate); err != nil {
			return err
		}
	}
	out.UploadID = workUpload.ID
	return nil
}

func (s *Service) ensureCustomKeywordActive(ctx context.Context, ownerUserID, keywordID uuid.UUID) error {
	if s.customKeywords == nil {
		return dpub.ErrInvalidContext
	}
	keyword, err := s.customKeywords.GetKeywordByIDForUser(ctx, keywordID, ownerUserID)
	if err != nil {
		return err
	}
	if keyword.Status != "active" {
		return common.ErrConflict
	}
	return nil
}

func (s *Service) repo(q publishingdb.Querier) *pgrepo.Repository {
	return pgrepo.NewRepository(q)
}

func validateAssetOwnership(asset dpub.MediaAssetLite, ownerUserID uuid.UUID, expectedKind dpub.MediaKind) error {
	if asset.OwnerUserID != ownerUserID || asset.Kind != expectedKind {
		return dpub.ErrInvalidAsset
	}
	switch asset.Status {
	case dpub.MediaAssetStatusUploaded, dpub.MediaAssetStatusProcessing, dpub.MediaAssetStatusReady:
		return nil
	default:
		return dpub.ErrInvalidAsset
	}
}

func normalizeOptionalText(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}

func (s *Service) dailyKeywordAssignment(ctx context.Context, repo *pgrepo.Repository, bizDate time.Time) (dpub.DailyKeywordAssignment, error) {
	normalized := common.NormalizeBizDate(bizDate)
	if s.officialCatalog != nil {
		assignment, err := s.officialCatalog.EnsureDailyKeywordAssignment(ctx, normalized)
		if err != nil {
			return dpub.DailyKeywordAssignment{}, err
		}
		return dpub.DailyKeywordAssignment{
			BizDate:   assignment.BizDate,
			KeywordID: assignment.KeywordID,
		}, nil
	}
	return repo.GetDailyKeywordAssignment(ctx, normalized)
}
