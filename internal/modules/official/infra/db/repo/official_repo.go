package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	dofficial "cixing/internal/modules/official/domain"
	officialdb "cixing/internal/modules/official/infra/db/gen"
	"cixing/internal/shared/common"
)

type Repository struct {
	q officialdb.Querier
}

var _ dofficial.Repository = (*Repository)(nil)

func NewRepository(q officialdb.Querier) *Repository {
	return &Repository{q: q}
}

func (r *Repository) GetOfficialKeywordByID(ctx context.Context, id uuid.UUID) (dofficial.OfficialKeyword, error) {
	row, err := r.q.GetOfficialKeywordByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dofficial.OfficialKeyword{}, common.ErrNotFound
		}
		return dofficial.OfficialKeyword{}, err
	}
	return mapKeyword(row), nil
}

func (r *Repository) GetOfficialKeywordByText(ctx context.Context, text string) (dofficial.OfficialKeyword, error) {
	row, err := r.q.GetOfficialKeywordByText(ctx, text)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dofficial.OfficialKeyword{}, common.ErrNotFound
		}
		return dofficial.OfficialKeyword{}, err
	}
	return mapKeyword(row), nil
}

func (r *Repository) ListActiveOfficialKeywords(ctx context.Context) ([]dofficial.OfficialKeyword, error) {
	rows, err := r.q.ListActiveOfficialKeywords(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dofficial.OfficialKeyword, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapKeyword(row))
	}
	return out, nil
}

func (r *Repository) UpsertOfficialKeyword(ctx context.Context, params dofficial.UpsertKeywordParams) (dofficial.OfficialKeyword, error) {
	row, err := r.q.UpsertOfficialKeyword(ctx, officialdb.UpsertOfficialKeywordParams{
		ID:           params.ID,
		Text:         params.Text,
		Category:     string(params.Category),
		IsActive:     params.IsActive,
		DisplayOrder: int4OrNull(params.DisplayOrder),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return dofficial.OfficialKeyword{}, common.ErrConflict
		}
		return dofficial.OfficialKeyword{}, err
	}
	return mapKeyword(row), nil
}

func (r *Repository) DeactivateOfficialKeyword(ctx context.Context, id uuid.UUID) (dofficial.OfficialKeyword, error) {
	row, err := r.q.DeactivateOfficialKeyword(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dofficial.OfficialKeyword{}, common.ErrNotFound
		}
		return dofficial.OfficialKeyword{}, err
	}
	return mapKeyword(row), nil
}

func (r *Repository) DrawRandomPrompt(ctx context.Context, keywordID uuid.UUID, kind dofficial.PromptKind) (dofficial.OfficialPrompt, error) {
	row, err := r.q.DrawRandomPrompt(ctx, officialdb.DrawRandomPromptParams{
		KeywordID: keywordID,
		Kind:      string(kind),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dofficial.OfficialPrompt{}, common.ErrNotFound
		}
		return dofficial.OfficialPrompt{}, err
	}
	return mapPrompt(row), nil
}

func (r *Repository) ListPromptsByKeyword(ctx context.Context, keywordID uuid.UUID) ([]dofficial.OfficialPrompt, error) {
	rows, err := r.q.ListPromptsByKeyword(ctx, keywordID)
	if err != nil {
		return nil, err
	}
	out := make([]dofficial.OfficialPrompt, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPrompt(row))
	}
	return out, nil
}

func (r *Repository) UpsertPrompt(ctx context.Context, params dofficial.UpsertPromptParams) (dofficial.OfficialPrompt, error) {
	row, err := r.q.UpsertPrompt(ctx, officialdb.UpsertPromptParams{
		ID:           params.ID,
		KeywordID:    params.KeywordID,
		Kind:         string(params.Kind),
		Content:      params.Content,
		DisplayOrder: int4OrNull(params.DisplayOrder),
		IsActive:     params.IsActive,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return dofficial.OfficialPrompt{}, common.ErrConflict
		}
		return dofficial.OfficialPrompt{}, err
	}
	return mapPrompt(row), nil
}

func (r *Repository) GetDailyKeywordAssignment(ctx context.Context, bizDate time.Time) (dofficial.DailyKeywordAssignment, error) {
	row, err := r.q.GetDailyKeywordAssignment(ctx, dateOnlyArg(bizDate))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dofficial.DailyKeywordAssignment{}, common.ErrNotFound
		}
		return dofficial.DailyKeywordAssignment{}, err
	}
	return mapAssignment(row), nil
}

func (r *Repository) UpsertDailyKeywordAssignment(ctx context.Context, bizDate time.Time, keywordID uuid.UUID) (dofficial.DailyKeywordAssignment, error) {
	row, err := r.q.UpsertDailyKeywordAssignment(ctx, officialdb.UpsertDailyKeywordAssignmentParams{
		BizDate:   dateOnlyArg(bizDate),
		KeywordID: keywordID,
	})
	if err != nil {
		return dofficial.DailyKeywordAssignment{}, err
	}
	return mapAssignment(row), nil
}

func (r *Repository) GetKeywordForDateWithStats(ctx context.Context, bizDate time.Time) (dofficial.KeywordWithStats, error) {
	row, err := r.q.GetKeywordForDateWithStats(ctx, dateOnlyArg(bizDate))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dofficial.KeywordWithStats{}, common.ErrNotFound
		}
		return dofficial.KeywordWithStats{}, err
	}
	return dofficial.KeywordWithStats{
		BizDate: row.BizDate.Time,
		Keyword: dofficial.OfficialKeyword{
			ID:           row.KeywordID,
			Text:         row.Text,
			Category:     dofficial.KeywordCategory(enumString(row.Category)),
			IsActive:     row.IsActive,
			DisplayOrder: row.DisplayOrder,
		},
		ParticipantUserCount: row.ParticipantUserCount,
		UploadCount:          row.UploadCount,
		ImageCount:           row.ImageCount,
	}, nil
}

func (r *Repository) GetDailyKeywordStat(ctx context.Context, bizDate time.Time) (dofficial.DailyKeywordStat, error) {
	row, err := r.q.GetDailyKeywordStat(ctx, dateOnlyArg(bizDate))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dofficial.DailyKeywordStat{}, common.ErrNotFound
		}
		return dofficial.DailyKeywordStat{}, err
	}
	return mapStat(row), nil
}

func (r *Repository) UpsertDailyKeywordStat(ctx context.Context, params dofficial.UpsertDailyKeywordStatParams) (dofficial.DailyKeywordStat, error) {
	row, err := r.q.UpsertDailyKeywordStat(ctx, officialdb.UpsertDailyKeywordStatParams{
		BizDate:              dateOnlyArg(params.BizDate),
		ParticipantUserCount: params.ParticipantUserCount,
		UploadCount:          params.UploadCount,
		ImageCount:           params.ImageCount,
	})
	if err != nil {
		return dofficial.DailyKeywordStat{}, err
	}
	return mapStat(row), nil
}

func (r *Repository) RecomputeDailyKeywordStatsFromUploads(ctx context.Context, bizDate time.Time) (dofficial.DailyKeywordStat, error) {
	row, err := r.q.RecomputeDailyKeywordStatsFromUploads(ctx, dateOnlyArg(bizDate))
	if err != nil {
		return dofficial.DailyKeywordStat{}, err
	}
	return mapStat(row), nil
}

func mapKeyword(row officialdb.OfficialKeyword) dofficial.OfficialKeyword {
	return dofficial.OfficialKeyword{
		ID:           row.ID,
		Text:         row.Text,
		Category:     dofficial.KeywordCategory(enumString(row.Category)),
		IsActive:     row.IsActive,
		DisplayOrder: row.DisplayOrder,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}

func mapPrompt(row officialdb.OfficialKeywordPrompt) dofficial.OfficialPrompt {
	return dofficial.OfficialPrompt{
		ID:           row.ID,
		KeywordID:    row.KeywordID,
		Kind:         dofficial.PromptKind(enumString(row.Kind)),
		Content:      row.Content,
		DisplayOrder: row.DisplayOrder,
		IsActive:     row.IsActive,
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
	}
}

func mapAssignment(row officialdb.DailyKeywordAssignment) dofficial.DailyKeywordAssignment {
	return dofficial.DailyKeywordAssignment{
		BizDate:   row.BizDate.Time,
		KeywordID: row.KeywordID,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

func mapStat(row officialdb.DailyKeywordStat) dofficial.DailyKeywordStat {
	return dofficial.DailyKeywordStat{
		BizDate:              row.BizDate.Time,
		ParticipantUserCount: row.ParticipantUserCount,
		UploadCount:          row.UploadCount,
		ImageCount:           row.ImageCount,
		CreatedAt:            row.CreatedAt.Time,
		UpdatedAt:            row.UpdatedAt.Time,
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

func dateOnlyArg(t time.Time) pgtype.Date {
	return pgtype.Date{Time: common.NormalizeBizDate(t), Valid: true}
}
