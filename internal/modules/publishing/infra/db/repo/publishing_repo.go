package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	dpub "cixing/internal/modules/publishing/domain"
	publishingdb "cixing/internal/modules/publishing/infra/db/gen"
	"cixing/internal/shared/common"
)

type Repository struct {
	q publishingdb.Querier
}

var _ dpub.Repository = (*Repository)(nil)

func NewRepository(q publishingdb.Querier) *Repository {
	return &Repository{q: q}
}

func (r *Repository) CreateSession(ctx context.Context, params dpub.CreateSessionParams) (dpub.PublishSession, error) {
	row, err := r.q.CreatePublishSession(ctx, publishingdb.CreatePublishSessionParams{
		OwnerUserID:       params.OwnerUserID,
		ContextType:       string(params.ContextType),
		OfficialKeywordID: uuidOrNull(params.OfficialKeywordID),
		CustomKeywordID:   uuidOrNull(params.CustomKeywordID),
		BizDate:           dateOrNull(params.BizDate),
		Status:            string(params.Status),
		ExpiresAt:         timestamptz(params.ExpiresAt),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return dpub.PublishSession{}, common.ErrConflict
		}
		return dpub.PublishSession{}, err
	}
	return mapSession(row), nil
}

func (r *Repository) GetAggregateForUpdate(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID) (dpub.Aggregate, error) {
	return r.loadAggregateForUpdate(ctx, sessionID, ownerUserID)
}

func (r *Repository) UpdateSession(ctx context.Context, params dpub.UpdateSessionParams) (dpub.PublishSession, error) {
	row, err := r.q.UpdatePublishSessionStateForOwner(ctx, publishingdb.UpdatePublishSessionStateForOwnerParams{
		ID:                params.ID,
		OwnerUserID:       params.OwnerUserID,
		Status:            string(params.Status),
		PublishedUploadID: uuidOrNull(params.PublishedUpload),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dpub.PublishSession{}, common.ErrNotFound
		}
		return dpub.PublishSession{}, err
	}
	return mapSession(row), nil
}

func (r *Repository) UpsertSessionItem(ctx context.Context, params dpub.UpsertSessionItemParams) (dpub.PublishSessionItem, error) {
	row, err := r.q.UpsertPublishSessionItem(ctx, publishingdb.UpsertPublishSessionItemParams{
		SessionID:     params.SessionID,
		OwnerUserID:   params.OwnerUserID,
		ClientImageID: params.ClientImageID,
		ImageAssetID:  params.ImageAssetID,
		AudioAssetID:  uuidOrNull(params.AudioAssetID),
		DisplayOrder:  int4OrNull(params.DisplayOrder),
		IsCover:       params.IsCover,
		Title:         textOrNull(params.Title),
		Note:          textOrNull(params.Note),
		Status:        string(params.Status),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return dpub.PublishSessionItem{}, common.ErrConflict
		}
		return dpub.PublishSessionItem{}, err
	}
	return mapSessionItem(row), nil
}

func (r *Repository) ClearSessionItemCover(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID) error {
	_, err := r.q.ClearPublishSessionItemCoverForOwner(ctx, publishingdb.ClearPublishSessionItemCoverForOwnerParams{
		SessionID:   sessionID,
		OwnerUserID: ownerUserID,
	})
	return err
}

func (r *Repository) GetDailyKeywordAssignment(ctx context.Context, bizDate time.Time) (dpub.DailyKeywordAssignment, error) {
	row, err := r.q.GetDailyKeywordAssignmentByBizDate(ctx, dateOrNull(&bizDate))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dpub.DailyKeywordAssignment{}, common.ErrNotFound
		}
		return dpub.DailyKeywordAssignment{}, err
	}
	return dpub.DailyKeywordAssignment{
		BizDate:   row.BizDate.Time,
		KeywordID: row.KeywordID,
	}, nil
}

func (r *Repository) GetMediaAssetLite(ctx context.Context, assetID uuid.UUID) (dpub.MediaAssetLite, error) {
	row, err := r.q.GetMediaAssetLiteByID(ctx, assetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dpub.MediaAssetLite{}, common.ErrNotFound
		}
		return dpub.MediaAssetLite{}, err
	}
	return dpub.MediaAssetLite{
		ID:          row.ID,
		OwnerUserID: row.OwnerUserID,
		Kind:        dpub.MediaKind(enumString(row.MediaKind)),
		Status:      dpub.MediaAssetStatus(enumString(row.Status)),
		DurationMS:  int4Ptr(row.DurationMs),
	}, nil
}

func (r *Repository) CreateWorkUpload(ctx context.Context, params dpub.CreateWorkUploadParams) (dpub.WorkUpload, error) {
	row, err := r.q.CreateWorkUpload(ctx, publishingdb.CreateWorkUploadParams{
		AuthorUserID:      params.AuthorUserID,
		ContextType:       string(params.ContextType),
		OfficialKeywordID: uuidOrNull(params.OfficialKeywordID),
		CustomKeywordID:   uuidOrNull(params.CustomKeywordID),
		BizDate:           dateOrNull(params.BizDate),
		VisibilityStatus:  string(params.VisibilityStatus),
		CoverAssetID:      params.CoverAssetID,
		ImageCount:        params.ImageCount,
		PublishedAt:       timestamptz(params.PublishedAt),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return dpub.WorkUpload{}, common.ErrConflict
		}
		return dpub.WorkUpload{}, err
	}
	return mapWorkUpload(row), nil
}

func (r *Repository) CreateWorkUploadImage(ctx context.Context, params dpub.CreateWorkUploadImageParams) (dpub.WorkUploadImage, error) {
	row, err := r.q.CreateWorkUploadImage(ctx, publishingdb.CreateWorkUploadImageParams{
		UploadID:     params.UploadID,
		ImageAssetID: params.ImageAssetID,
		DisplayOrder: params.DisplayOrder,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return dpub.WorkUploadImage{}, common.ErrConflict
		}
		return dpub.WorkUploadImage{}, err
	}
	return mapWorkUploadImage(row), nil
}

func (r *Repository) UpsertWorkUploadImageContent(ctx context.Context, params dpub.UpsertWorkUploadImageContentParams) (dpub.WorkUploadImageContent, error) {
	row, err := r.q.UpsertWorkUploadImageContent(ctx, publishingdb.UpsertWorkUploadImageContentParams{
		WorkUploadImageID: params.WorkUploadImageID,
		Title:             textOrNull(params.Title),
		Note:              textOrNull(params.Note),
		AudioAssetID:      uuidOrNull(params.AudioAssetID),
		AudioDurationMs:   int4OrNull(params.AudioDurationMS),
	})
	if err != nil {
		return dpub.WorkUploadImageContent{}, err
	}
	return mapWorkUploadImageContent(row), nil
}

func (r *Repository) loadAggregateForUpdate(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID) (dpub.Aggregate, error) {
	row, err := r.q.GetPublishSessionByIDForOwnerForUpdate(ctx, publishingdb.GetPublishSessionByIDForOwnerForUpdateParams{
		ID:          sessionID,
		OwnerUserID: ownerUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dpub.Aggregate{}, common.ErrNotFound
		}
		return dpub.Aggregate{}, err
	}

	rows, err := r.q.ListPublishSessionItemsBySessionForOwnerForUpdate(ctx, publishingdb.ListPublishSessionItemsBySessionForOwnerForUpdateParams{
		SessionID:   sessionID,
		OwnerUserID: ownerUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dpub.Aggregate{}, common.ErrNotFound
		}
		return dpub.Aggregate{}, err
	}
	return dpub.Aggregate{
		Session: mapSession(row),
		Items:   mapSessionItems(rows),
	}, nil
}

func mapSession(row publishingdb.PublishSession) dpub.PublishSession {
	return dpub.PublishSession{
		ID:                row.ID,
		OwnerUserID:       row.OwnerUserID,
		ContextType:       dpub.PublishContextType(enumString(row.ContextType)),
		OfficialKeywordID: uuidPtr(row.OfficialKeywordID),
		CustomKeywordID:   uuidPtr(row.CustomKeywordID),
		BizDate:           datePtr(row.BizDate),
		Status:            dpub.SessionStatus(enumString(row.Status)),
		ExpiresAt:         row.ExpiresAt.Time,
		PublishedUploadID: uuidPtr(row.PublishedUploadID),
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
	}
}

func mapSessionItems(rows []publishingdb.PublishSessionItem) []dpub.PublishSessionItem {
	out := make([]dpub.PublishSessionItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapSessionItem(row))
	}
	return out
}

func mapSessionItem(row publishingdb.PublishSessionItem) dpub.PublishSessionItem {
	return dpub.PublishSessionItem{
		ID:            row.ID,
		SessionID:     row.SessionID,
		OwnerUserID:   row.OwnerUserID,
		ClientImageID: row.ClientImageID,
		ImageAssetID:  row.ImageAssetID,
		AudioAssetID:  uuidPtr(row.AudioAssetID),
		DisplayOrder:  int4Ptr(row.DisplayOrder),
		IsCover:       row.IsCover,
		Title:         textPtr(row.Title),
		Note:          textPtr(row.Note),
		Status:        dpub.ItemStatus(enumString(row.Status)),
		CreatedAt:     row.CreatedAt.Time,
		UpdatedAt:     row.UpdatedAt.Time,
	}
}

func mapWorkUpload(row publishingdb.WorkUpload) dpub.WorkUpload {
	return dpub.WorkUpload{
		ID:                     row.ID,
		AuthorUserID:           row.AuthorUserID,
		ContextType:            dpub.PublishContextType(enumString(row.ContextType)),
		OfficialKeywordID:      uuidPtr(row.OfficialKeywordID),
		CustomKeywordID:        uuidPtr(row.CustomKeywordID),
		BizDate:                datePtr(row.BizDate),
		VisibilityStatus:       dpub.WorkUploadVisibilityStatus(enumString(row.VisibilityStatus)),
		CoverAssetID:           row.CoverAssetID,
		ImageCount:             row.ImageCount,
		ReactionInspiredCount:  row.ReactionInspiredCount,
		ReactionResonatedCount: row.ReactionResonatedCount,
		RandKey:                row.RandKey,
		PublishedAt:            row.PublishedAt.Time,
		CreatedAt:              row.CreatedAt.Time,
		UpdatedAt:              row.UpdatedAt.Time,
		DeletedAt:              timePtr(row.DeletedAt),
	}
}

func mapWorkUploadImage(row publishingdb.WorkUploadImage) dpub.WorkUploadImage {
	return dpub.WorkUploadImage{
		ID:           row.ID,
		UploadID:     row.UploadID,
		ImageAssetID: row.ImageAssetID,
		DisplayOrder: row.DisplayOrder,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
		DeletedAt:    timePtr(row.DeletedAt),
	}
}

func mapWorkUploadImageContent(row publishingdb.WorkUploadImageContent) dpub.WorkUploadImageContent {
	return dpub.WorkUploadImageContent{
		WorkUploadImageID: row.WorkUploadImageID,
		Title:             textPtr(row.Title),
		Note:              textPtr(row.Note),
		AudioAssetID:      uuidPtr(row.AudioAssetID),
		AudioDurationMS:   int4Ptr(row.AudioDurationMs),
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
	}
}

func enumString(v interface{}) string {
	switch e := v.(type) {
	case string:
		return e
	case []byte:
		return string(e)
	case fmt.Stringer:
		return e.String()
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	}
}

func int4OrNull(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}

func int4Ptr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	n := v.Int32
	return &n
}

func textOrNull(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}

func textPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func uuidOrNull(v *uuid.UUID) pgtype.UUID {
	if v == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *v, Valid: true}
}

func uuidPtr(v pgtype.UUID) *uuid.UUID {
	if !v.Valid {
		return nil
	}
	id := uuid.UUID(v.Bytes)
	return &id
}

func dateOrNull(v *time.Time) pgtype.Date {
	if v == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: dateOnly(*v), Valid: true}
}

func datePtr(v pgtype.Date) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func timePtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func dateOnly(t time.Time) time.Time {
	return common.NormalizeBizDate(t)
}
