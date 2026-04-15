package repo

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	dcustom "cixing/internal/modules/customkeywords/domain"
	customkeywordsdb "cixing/internal/modules/customkeywords/infra/db/gen"
	"cixing/internal/shared/common"
)

type Repository struct {
	q customkeywordsdb.Querier
}

func NewRepository(q customkeywordsdb.Querier) *Repository {
	return &Repository{q: q}
}

type Keyword struct {
	ID               uuid.UUID
	OwnerUserID      uuid.UUID
	Text             string
	TargetImageCount *int32
	CoverAssetID     *uuid.UUID
	CoverSource      dcustom.CoverSource
	Status           dcustom.Status
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type KeywordSummary struct {
	Keyword
	TotalImageCount int32
}

type CoverAsset struct {
	ID                uuid.UUID
	OriginalObjectKey string
	OriginalWidth     *int32
	OriginalHeight    *int32
}

type KeywordImageCard struct {
	ID                uuid.UUID
	ImageAssetID      uuid.UUID
	OriginalObjectKey string
	OriginalWidth     *int32
	OriginalHeight    *int32
	DisplayOrder      int32
	CreatedAt         time.Time
}

type KeywordImageDetail struct {
	ID                uuid.UUID
	CustomKeywordID   uuid.UUID
	ImageAssetID      uuid.UUID
	OriginalObjectKey string
	OriginalWidth     *int32
	OriginalHeight    *int32
	DisplayOrder      int32
	Title             *string
	Note              *string
	HasAudio          bool
	AudioDurationMs   *int32
	AudioObjectKey    *string
	CreatedAt         time.Time
}

type UpdateKeywordInput struct {
	Text             *string
	TargetImageCount *int32
	Status           *dcustom.Status
}

func (r *Repository) GetKeywordByIDForUser(ctx context.Context, keywordID, ownerUserID uuid.UUID) (Keyword, error) {
	row, err := r.q.GetCustomKeywordByIDForUser(ctx, customkeywordsdb.GetCustomKeywordByIDForUserParams{
		ID:          keywordID,
		OwnerUserID: ownerUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Keyword{}, common.ErrNotFound
		}
		return Keyword{}, err
	}
	return mapKeyword(row), nil
}

func (r *Repository) CreateKeyword(ctx context.Context, ownerUserID uuid.UUID, text string, targetImageCount *int32) (Keyword, error) {
	row, err := r.q.CreateCustomKeyword(ctx, customkeywordsdb.CreateCustomKeywordParams{
		OwnerUserID:      ownerUserID,
		Text:             strings.TrimSpace(text),
		TargetImageCount: int4OrNull(targetImageCount),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return Keyword{}, common.ErrConflict
		}
		return Keyword{}, err
	}
	return mapKeyword(row), nil
}

func (r *Repository) UpdateKeyword(ctx context.Context, ownerUserID, keywordID uuid.UUID, in UpdateKeywordInput) (Keyword, error) {
	row, err := r.q.UpdateCustomKeywordForUser(ctx, customkeywordsdb.UpdateCustomKeywordForUserParams{
		Text:             textOrNull(in.Text),
		TargetImageCount: int4OrNull(in.TargetImageCount),
		Status:           statusOrNull(in.Status),
		ID:               keywordID,
		OwnerUserID:      ownerUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Keyword{}, common.ErrNotFound
		}
		if isUniqueViolation(err) {
			return Keyword{}, common.ErrConflict
		}
		return Keyword{}, err
	}
	return mapKeyword(row), nil
}

func (r *Repository) DeleteKeyword(ctx context.Context, ownerUserID, keywordID uuid.UUID) error {
	affected, err := r.q.SoftDeleteCustomKeywordForUser(ctx, customkeywordsdb.SoftDeleteCustomKeywordForUserParams{
		ID:          keywordID,
		OwnerUserID: ownerUserID,
	})
	if err != nil {
		return err
	}
	if affected == 0 {
		return common.ErrNotFound
	}
	return nil
}

func (r *Repository) ListKeywordSummaries(ctx context.Context, ownerUserID uuid.UUID) ([]KeywordSummary, error) {
	rows, err := r.q.ListCustomKeywordSummariesForUser(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	out := make([]KeywordSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, KeywordSummary{
			Keyword:         mapKeywordSummaryRow(row),
			TotalImageCount: row.TotalImageCount,
		})
	}
	return out, nil
}

func (r *Repository) CountVisibleImagesForKeyword(ctx context.Context, ownerUserID, keywordID uuid.UUID) (int32, error) {
	return r.q.CountVisibleCustomKeywordImages(ctx, customkeywordsdb.CountVisibleCustomKeywordImagesParams{
		OwnerUserID: ownerUserID,
		KeywordID:   keywordID,
	})
}

func (r *Repository) ResolveImageAssetForKeywordImage(ctx context.Context, ownerUserID, keywordID, imageID uuid.UUID) (uuid.UUID, error) {
	assetID, err := r.q.ResolveCustomKeywordImageAsset(ctx, customkeywordsdb.ResolveCustomKeywordImageAssetParams{
		ImageID:     imageID,
		OwnerUserID: ownerUserID,
		KeywordID:   keywordID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, common.ErrNotFound
		}
		return uuid.Nil, err
	}
	return assetID, nil
}

func (r *Repository) SetKeywordManualCover(ctx context.Context, ownerUserID, keywordID, assetID uuid.UUID) (Keyword, error) {
	row, err := r.q.SetCustomKeywordManualCover(ctx, customkeywordsdb.SetCustomKeywordManualCoverParams{
		AssetID:     assetID,
		ID:          keywordID,
		OwnerUserID: ownerUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Keyword{}, common.ErrNotFound
		}
		return Keyword{}, err
	}
	return mapKeyword(row), nil
}

func (r *Repository) ClearKeywordCover(ctx context.Context, ownerUserID, keywordID uuid.UUID) (Keyword, error) {
	row, err := r.q.ClearCustomKeywordCover(ctx, customkeywordsdb.ClearCustomKeywordCoverParams{
		ID:          keywordID,
		OwnerUserID: ownerUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Keyword{}, common.ErrNotFound
		}
		return Keyword{}, err
	}
	return mapKeyword(row), nil
}

func (r *Repository) GetCoverAsset(ctx context.Context, assetID uuid.UUID) (*CoverAsset, error) {
	row, err := r.q.GetCustomKeywordCoverAsset(ctx, assetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, common.ErrNotFound
		}
		return nil, err
	}
	return &CoverAsset{
		ID:                row.ID,
		OriginalObjectKey: row.OriginalObjectKey,
		OriginalWidth:     int4Ptr(row.Width),
		OriginalHeight:    int4Ptr(row.Height),
	}, nil
}

func (r *Repository) GetLatestCoverAssetForKeyword(ctx context.Context, ownerUserID, keywordID uuid.UUID) (*CoverAsset, error) {
	row, err := r.q.GetLatestCustomKeywordCoverAsset(ctx, customkeywordsdb.GetLatestCustomKeywordCoverAssetParams{
		OwnerUserID: ownerUserID,
		KeywordID:   keywordID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, common.ErrNotFound
		}
		return nil, err
	}
	return &CoverAsset{
		ID:                row.ID,
		OriginalObjectKey: row.OriginalObjectKey,
		OriginalWidth:     int4Ptr(row.Width),
		OriginalHeight:    int4Ptr(row.Height),
	}, nil
}

func (r *Repository) ListKeywordImages(ctx context.Context, ownerUserID, keywordID uuid.UUID, limit int32) ([]KeywordImageCard, error) {
	rows, err := r.q.ListCustomKeywordImages(ctx, customkeywordsdb.ListCustomKeywordImagesParams{
		OwnerUserID: ownerUserID,
		KeywordID:   keywordID,
		RowLimit:    limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]KeywordImageCard, 0, len(rows))
	for _, row := range rows {
		out = append(out, KeywordImageCard{
			ID:                row.ID,
			ImageAssetID:      row.ImageAssetID,
			OriginalObjectKey: row.OriginalObjectKey,
			OriginalWidth:     int4Ptr(row.Width),
			OriginalHeight:    int4Ptr(row.Height),
			DisplayOrder:      row.DisplayOrder,
			CreatedAt:         row.CreatedAt.Time,
		})
	}
	return out, nil
}

func (r *Repository) GetKeywordImageDetail(ctx context.Context, ownerUserID, imageID uuid.UUID) (KeywordImageDetail, error) {
	row, err := r.q.GetCustomKeywordImageDetail(ctx, customkeywordsdb.GetCustomKeywordImageDetailParams{
		ImageID:     imageID,
		OwnerUserID: ownerUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return KeywordImageDetail{}, common.ErrNotFound
		}
		return KeywordImageDetail{}, err
	}
	return KeywordImageDetail{
		ID:                row.ID,
		CustomKeywordID:   uuidValue(row.CustomKeywordID),
		ImageAssetID:      row.ImageAssetID,
		OriginalObjectKey: row.OriginalObjectKey,
		OriginalWidth:     int4Ptr(row.Width),
		OriginalHeight:    int4Ptr(row.Height),
		DisplayOrder:      row.DisplayOrder,
		Title:             textPtr(row.Title),
		Note:              textPtr(row.Note),
		HasAudio:          boolValue(row.HasAudio),
		AudioDurationMs:   int4Ptr(row.AudioDurationMs),
		AudioObjectKey:    textPtr(row.AudioObjectKey),
		CreatedAt:         row.CreatedAt.Time,
	}, nil
}

func mapKeyword(row customkeywordsdb.CustomKeyword) Keyword {
	return Keyword{
		ID:               row.ID,
		OwnerUserID:      row.OwnerUserID,
		Text:             row.Text,
		TargetImageCount: zeroInt32Ptr(row.TargetImageCount),
		CoverAssetID:     uuidPtr(row.CoverAssetID),
		CoverSource:      dcustom.CoverSource(row.CoverMode),
		Status:           dcustom.Status(row.Status),
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
	}
}

func mapKeywordSummaryRow(row customkeywordsdb.ListCustomKeywordSummariesForUserRow) Keyword {
	return Keyword{
		ID:               row.ID,
		OwnerUserID:      row.OwnerUserID,
		Text:             row.Text,
		TargetImageCount: zeroInt32Ptr(row.TargetImageCount),
		CoverAssetID:     uuidPtr(row.CoverAssetID),
		CoverSource:      dcustom.CoverSource(row.CoverMode),
		Status:           dcustom.Status(row.Status),
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
	}
}

func textOrNull(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}

func statusOrNull(v *dcustom.Status) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	s := string(*v)
	return pgtype.Text{String: s, Valid: true}
}

func textPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
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

func uuidPtr(v pgtype.UUID) *uuid.UUID {
	if !v.Valid {
		return nil
	}
	id := uuid.UUID(v.Bytes)
	return &id
}

func uuidValue(v pgtype.UUID) uuid.UUID {
	if !v.Valid {
		return uuid.Nil
	}
	return uuid.UUID(v.Bytes)
}

func zeroInt32Ptr(v int32) *int32 {
	if v == 0 {
		return nil
	}
	n := v
	return &n
}

func boolValue(v interface{}) bool {
	switch vv := v.(type) {
	case bool:
		return vv
	default:
		return false
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
