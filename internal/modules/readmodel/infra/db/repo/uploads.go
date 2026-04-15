package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	dofficial "cixing/internal/modules/official/domain"
	readmodeldb "cixing/internal/modules/readmodel/infra/db/gen"
	"cixing/internal/shared/common"
)

type UploadCard struct {
	ID                     uuid.UUID
	BizDate                time.Time
	KeywordID              uuid.UUID
	CoverImageID           uuid.UUID
	DisplayText            *string
	CoverHasAudio          bool
	CoverAudioDurationMs   *int32
	CoverObjectKey         string
	ImageCount             int32
	ReactionInspiredCount  int32
	ReactionResonatedCount int32
	CreatedAt              time.Time
}

type UploadImage struct {
	ID                uuid.UUID
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

type UploadReaction struct {
	UploadID uuid.UUID
	Type     string
}

func (r *Repository) GetOfficialKeyword(ctx context.Context, keywordID uuid.UUID) (dofficial.OfficialKeyword, error) {
	row, err := r.q.GetOfficialKeyword(ctx, keywordID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dofficial.OfficialKeyword{}, common.ErrNotFound
		}
		return dofficial.OfficialKeyword{}, err
	}
	return dofficial.OfficialKeyword{
		ID:           row.ID,
		Text:         row.Text,
		Category:     dofficial.KeywordCategory(enumString(row.Category)),
		IsActive:     row.IsActive,
		DisplayOrder: row.DisplayOrder,
	}, nil
}

func (r *Repository) ListPublicUploadsByDateLatest(ctx context.Context, bizDate time.Time, limit int32) ([]UploadCard, error) {
	rows, err := r.q.ListPublicUploadsByDateLatest(ctx, readmodeldb.ListPublicUploadsByDateLatestParams{
		BizDate: dateOnlyArg(bizDate),
		Limit:   limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]UploadCard, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPublicUploadCard(
			row.ID,
			row.BizDate,
			row.KeywordID,
			row.CoverImageID,
			row.DisplayText,
			row.CoverHasAudio,
			row.CoverAudioDurationMs,
			row.CoverObjectKey,
			row.ImageCount,
			row.ReactionInspiredCount,
			row.ReactionResonatedCount,
			row.CreatedAt,
		))
	}
	return out, nil
}

func (r *Repository) ListPublicUploadsByDateRandom(ctx context.Context, bizDate time.Time, seed float64, limit int32) ([]UploadCard, error) {
	rows, err := r.q.ListPublicUploadsByDateRandom(ctx, readmodeldb.ListPublicUploadsByDateRandomParams{
		BizDate:    dateOnlyArg(bizDate),
		Seed:       seed,
		LimitCount: limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]UploadCard, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPublicUploadCard(
			row.ID,
			row.BizDate,
			row.KeywordID,
			row.CoverImageID,
			row.DisplayText,
			row.CoverHasAudio,
			row.CoverAudioDurationMs,
			row.CoverObjectKey,
			row.ImageCount,
			row.ReactionInspiredCount,
			row.ReactionResonatedCount,
			row.CreatedAt,
		))
	}
	return out, nil
}

func (r *Repository) ListPublicUploadsByKeywordLatest(ctx context.Context, keywordID uuid.UUID, limit int32) ([]UploadCard, error) {
	rows, err := r.q.ListPublicUploadsByKeywordLatest(ctx, readmodeldb.ListPublicUploadsByKeywordLatestParams{
		OfficialKeywordID: pgtype.UUID{Bytes: keywordID, Valid: true},
		Limit:             limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]UploadCard, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPublicUploadCard(
			row.ID,
			row.BizDate,
			row.KeywordID,
			row.CoverImageID,
			row.DisplayText,
			row.CoverHasAudio,
			row.CoverAudioDurationMs,
			row.CoverObjectKey,
			row.ImageCount,
			row.ReactionInspiredCount,
			row.ReactionResonatedCount,
			row.CreatedAt,
		))
	}
	return out, nil
}

func (r *Repository) ListPublicUploadsByKeywordRandom(ctx context.Context, keywordID uuid.UUID, seed float64, limit int32) ([]UploadCard, error) {
	rows, err := r.q.ListPublicUploadsByKeywordRandom(ctx, readmodeldb.ListPublicUploadsByKeywordRandomParams{
		KeywordID:  keywordID,
		Seed:       seed,
		LimitCount: limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]UploadCard, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPublicUploadCard(
			row.ID,
			row.BizDate,
			row.KeywordID,
			row.CoverImageID,
			row.DisplayText,
			row.CoverHasAudio,
			row.CoverAudioDurationMs,
			row.CoverObjectKey,
			row.ImageCount,
			row.ReactionInspiredCount,
			row.ReactionResonatedCount,
			row.CreatedAt,
		))
	}
	return out, nil
}

func (r *Repository) GetPublicUploadCard(ctx context.Context, uploadID uuid.UUID) (UploadCard, error) {
	row, err := r.q.GetPublicUploadCard(ctx, uploadID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UploadCard{}, common.ErrNotFound
		}
		return UploadCard{}, err
	}
	return mapPublicUploadCard(
		row.ID,
		row.BizDate,
		row.KeywordID,
		row.CoverImageID,
		row.DisplayText,
		row.CoverHasAudio,
		row.CoverAudioDurationMs,
		row.CoverObjectKey,
		row.ImageCount,
		row.ReactionInspiredCount,
		row.ReactionResonatedCount,
		row.CreatedAt,
	), nil
}

func (r *Repository) GetMyReviewUploadCard(ctx context.Context, uploadID, userID uuid.UUID) (UploadCard, error) {
	row, err := r.q.GetMyReviewUploadCard(ctx, readmodeldb.GetMyReviewUploadCardParams{
		ID:           uploadID,
		AuthorUserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UploadCard{}, common.ErrNotFound
		}
		return UploadCard{}, err
	}
	return mapReviewUploadCard(
		row.ID,
		row.BizDate,
		row.KeywordID,
		row.CoverImageID,
		row.DisplayText,
		row.CoverHasAudio,
		row.CoverAudioDurationMs,
		row.CoverObjectKey,
		row.ImageCount,
		row.CreatedAt,
	), nil
}

func (r *Repository) ListMyReviewUploadsByDate(ctx context.Context, userID uuid.UUID, bizDate time.Time, limit int32) ([]UploadCard, error) {
	rows, err := r.q.ListMyReviewUploadsByDate(ctx, readmodeldb.ListMyReviewUploadsByDateParams{
		AuthorUserID: userID,
		BizDate:      dateOnlyArg(bizDate),
		Limit:        limit,
	})
	if err != nil {
		return nil, err
	}
	return mapReviewUploadCardsByDate(rows), nil
}

func (r *Repository) ListMyReviewUploadsByKeyword(ctx context.Context, userID, keywordID uuid.UUID, limit int32) ([]UploadCard, error) {
	rows, err := r.q.ListMyReviewUploadsByKeyword(ctx, readmodeldb.ListMyReviewUploadsByKeywordParams{
		AuthorUserID:      userID,
		OfficialKeywordID: pgtype.UUID{Bytes: keywordID, Valid: true},
		Limit:             limit,
	})
	if err != nil {
		return nil, err
	}
	return mapReviewUploadCardsByKeyword(rows), nil
}

func (r *Repository) ListUploadImages(ctx context.Context, uploadID uuid.UUID) ([]UploadImage, error) {
	rows, err := r.q.ListUploadImages(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	out := make([]UploadImage, 0, len(rows))
	for _, row := range rows {
		out = append(out, UploadImage{
			ID:                row.ID,
			ImageAssetID:      row.ImageAssetID,
			OriginalObjectKey: row.OriginalObjectKey,
			OriginalWidth:     int4Ptr(row.OriginalWidth),
			OriginalHeight:    int4Ptr(row.OriginalHeight),
			DisplayOrder:      row.DisplayOrder,
			Title:             textPtr(row.Title),
			Note:              textPtr(row.Note),
			HasAudio:          row.HasAudio,
			AudioDurationMs:   int4Ptr(row.AudioDurationMs),
			AudioObjectKey:    textPtr(row.AudioObjectKey),
			CreatedAt:         row.CreatedAt.Time,
		})
	}
	return out, nil
}

func (r *Repository) ListMyReactionTypesByUploadIDs(ctx context.Context, userID uuid.UUID, uploadIDs []uuid.UUID) ([]UploadReaction, error) {
	if len(uploadIDs) == 0 {
		return []UploadReaction{}, nil
	}
	rows, err := r.q.ListMyReactionTypesByUploadIDs(ctx, readmodeldb.ListMyReactionTypesByUploadIDsParams{
		UserID:  userID,
		Column2: uploadIDs,
	})
	if err != nil {
		return nil, err
	}
	out := make([]UploadReaction, 0, len(rows))
	for _, row := range rows {
		out = append(out, UploadReaction{
			UploadID: row.UploadID,
			Type:     enumString(row.Type),
		})
	}
	return out, nil
}

func mapReviewUploadCardsByDate(rows []readmodeldb.ListMyReviewUploadsByDateRow) []UploadCard {
	out := make([]UploadCard, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapReviewUploadCard(
			row.ID,
			row.BizDate,
			row.KeywordID,
			row.CoverImageID,
			row.DisplayText,
			row.CoverHasAudio,
			row.CoverAudioDurationMs,
			row.CoverObjectKey,
			row.ImageCount,
			row.CreatedAt,
		))
	}
	return out
}

func mapReviewUploadCardsByKeyword(rows []readmodeldb.ListMyReviewUploadsByKeywordRow) []UploadCard {
	out := make([]UploadCard, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapReviewUploadCard(
			row.ID,
			row.BizDate,
			row.KeywordID,
			row.CoverImageID,
			row.DisplayText,
			row.CoverHasAudio,
			row.CoverAudioDurationMs,
			row.CoverObjectKey,
			row.ImageCount,
			row.CreatedAt,
		))
	}
	return out
}

func mapPublicUploadCard(
	id uuid.UUID,
	bizDate pgtype.Date,
	keywordID pgtype.UUID,
	coverImageID uuid.UUID,
	displayText interface{},
	coverHasAudio bool,
	coverAudioDurationMs pgtype.Int4,
	coverObjectKey string,
	imageCount int32,
	reactionInspiredCount int32,
	reactionResonatedCount int32,
	createdAt pgtype.Timestamptz,
) UploadCard {
	return UploadCard{
		ID:                     id,
		BizDate:                zeroIfInvalidDate(bizDate),
		KeywordID:              zeroIfInvalidUUID(keywordID),
		CoverImageID:           coverImageID,
		DisplayText:            anyTextPtr(displayText),
		CoverHasAudio:          coverHasAudio,
		CoverAudioDurationMs:   int4Ptr(coverAudioDurationMs),
		CoverObjectKey:         coverObjectKey,
		ImageCount:             imageCount,
		ReactionInspiredCount:  reactionInspiredCount,
		ReactionResonatedCount: reactionResonatedCount,
		CreatedAt:              createdAt.Time,
	}
}

func mapReviewUploadCard(
	id uuid.UUID,
	bizDate pgtype.Date,
	keywordID pgtype.UUID,
	coverImageID uuid.UUID,
	displayText interface{},
	coverHasAudio bool,
	coverAudioDurationMs pgtype.Int4,
	coverObjectKey string,
	imageCount int32,
	createdAt pgtype.Timestamptz,
) UploadCard {
	card := mapPublicUploadCard(
		id,
		bizDate,
		keywordID,
		coverImageID,
		displayText,
		coverHasAudio,
		coverAudioDurationMs,
		coverObjectKey,
		imageCount,
		0,
		0,
		createdAt,
	)
	card.ReactionInspiredCount = 0
	card.ReactionResonatedCount = 0
	return card
}
