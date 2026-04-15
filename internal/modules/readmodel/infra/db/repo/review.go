package repo

import (
	"context"
	"time"

	"github.com/google/uuid"

	dofficial "cixing/internal/modules/official/domain"
	readmodeldb "cixing/internal/modules/readmodel/infra/db/gen"
)

type ReviewDateStat struct {
	BizDate       time.Time
	Keyword       dofficial.OfficialKeyword
	MyUploadCount int32
	MyImageCount  int32
}

type ReviewKeywordCount struct {
	Keyword       dofficial.OfficialKeyword
	MyUploadCount int32
	MyImageCount  int32
}

func (r *Repository) ListMyReviewDateStats(ctx context.Context, userID uuid.UUID, limit int32) ([]ReviewDateStat, error) {
	rows, err := r.q.ListMyReviewDateStats(ctx, readmodeldb.ListMyReviewDateStatsParams{
		AuthorUserID: userID,
		Limit:        limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ReviewDateStat, 0, len(rows))
	for _, row := range rows {
		out = append(out, ReviewDateStat{
			BizDate: row.BizDate.Time,
			Keyword: dofficial.OfficialKeyword{
				ID:           row.KeywordID,
				Text:         row.Text,
				Category:     dofficial.KeywordCategory(enumString(row.Category)),
				IsActive:     row.IsActive,
				DisplayOrder: row.DisplayOrder,
			},
			MyUploadCount: row.MyUploadCount,
			MyImageCount:  row.MyImageCount,
		})
	}
	return out, nil
}

func (r *Repository) CountMyReviewParticipationDays(ctx context.Context, userID uuid.UUID) (int64, error) {
	return r.q.CountMyReviewParticipationDays(ctx, userID)
}

func (r *Repository) CountMyReviewImageTotal(ctx context.Context, userID uuid.UUID) (int64, error) {
	return r.q.CountMyReviewImageTotal(ctx, userID)
}

func (r *Repository) ListMyReviewKeywordCounts(ctx context.Context, userID uuid.UUID) ([]ReviewKeywordCount, error) {
	rows, err := r.q.ListMyReviewKeywordCounts(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]ReviewKeywordCount, 0, len(rows))
	for _, row := range rows {
		out = append(out, ReviewKeywordCount{
			Keyword: dofficial.OfficialKeyword{
				ID:           row.KeywordID,
				Text:         row.Text,
				Category:     dofficial.KeywordCategory(enumString(row.Category)),
				IsActive:     row.IsActive,
				DisplayOrder: row.DisplayOrder,
			},
			MyUploadCount: row.MyUploadCount,
			MyImageCount:  row.MyImageCount,
		})
	}
	return out, nil
}
