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

func (r *Repository) ListOfficialKeywords(ctx context.Context) ([]dofficial.OfficialKeyword, error) {
	rows, err := r.q.ListOfficialKeywords(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dofficial.OfficialKeyword, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapKeyword(row))
	}
	return out, nil
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

func (r *Repository) InsertOfficialKeyword(ctx context.Context, params dofficial.UpsertKeywordParams) (dofficial.OfficialKeyword, error) {
	row, err := r.q.InsertOfficialKeyword(ctx, officialdb.InsertOfficialKeywordParams{
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

func (r *Repository) UpdateOfficialKeyword(ctx context.Context, params dofficial.UpsertKeywordParams) (dofficial.OfficialKeyword, error) {
	displayOrder := int32(0)
	if params.DisplayOrder != nil {
		displayOrder = *params.DisplayOrder
	}
	row, err := r.q.UpdateOfficialKeyword(ctx, officialdb.UpdateOfficialKeywordParams{
		ID:           params.ID,
		Text:         params.Text,
		Category:     string(params.Category),
		DisplayOrder: displayOrder,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dofficial.OfficialKeyword{}, common.ErrNotFound
		}
		if isUniqueViolation(err) {
			return dofficial.OfficialKeyword{}, common.ErrConflict
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

func (r *Repository) LockUser(ctx context.Context, userID uuid.UUID) error {
	if _, err := r.q.LockUser(ctx, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return common.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) GetDailyPrompt(ctx context.Context, userID uuid.UUID, bizDate time.Time) (dofficial.DailyPrompt, error) {
	row, err := r.q.GetDailyPrompt(ctx, officialdb.GetDailyPromptParams{
		UserID:  userID,
		BizDate: dateOnlyArg(bizDate),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dofficial.DailyPrompt{}, common.ErrNotFound
		}
		return dofficial.DailyPrompt{}, err
	}
	return mapDailyPrompt(row), nil
}

func (r *Repository) InsertDailyPrompt(ctx context.Context, params dofficial.InsertDailyPromptParams) (dofficial.DailyPrompt, error) {
	row, err := r.q.InsertDailyPrompt(ctx, officialdb.InsertDailyPromptParams{
		UserID:          params.UserID,
		BizDate:         dateOnlyArg(params.BizDate),
		KeywordID:       params.KeywordID,
		PromptID:        params.PromptID,
		Kind:            string(params.Kind),
		ContentSnapshot: params.ContentSnapshot,
		SelectedAt:      pgtype.Timestamptz{Time: params.SelectedAt, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dofficial.DailyPrompt{}, common.ErrConflict
		}
		return dofficial.DailyPrompt{}, err
	}
	return mapDailyPrompt(row), nil
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

func (r *Repository) GetLastDailyKeywordAssignmentBefore(ctx context.Context, bizDate time.Time) (dofficial.DailyKeywordAssignment, error) {
	row, err := r.q.GetLastDailyKeywordAssignmentBefore(ctx, dateOnlyArg(bizDate))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dofficial.DailyKeywordAssignment{}, common.ErrNotFound
		}
		return dofficial.DailyKeywordAssignment{}, err
	}
	return mapAssignment(row), nil
}

func (r *Repository) HasDailyKeywordAssignmentOnOrAfter(ctx context.Context, bizDate time.Time) (bool, error) {
	return r.q.HasDailyKeywordAssignmentOnOrAfter(ctx, dateOnlyArg(bizDate))
}

func (r *Repository) InsertDailyKeywordAssignment(ctx context.Context, bizDate time.Time, keywordID uuid.UUID) (dofficial.DailyKeywordAssignment, bool, error) {
	row, err := r.q.InsertDailyKeywordAssignment(ctx, officialdb.InsertDailyKeywordAssignmentParams{
		BizDate:   dateOnlyArg(bizDate),
		KeywordID: keywordID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dofficial.DailyKeywordAssignment{}, false, nil
		}
		return dofficial.DailyKeywordAssignment{}, false, err
	}
	return mapAssignment(row), true, nil
}

func (r *Repository) ListDailyKeywordAssignmentsBetween(ctx context.Context, startDate time.Time, endDate time.Time) ([]dofficial.DailyKeywordAssignment, error) {
	rows, err := r.q.ListDailyKeywordAssignmentsBetween(ctx, officialdb.ListDailyKeywordAssignmentsBetweenParams{
		StartDate: dateOnlyArg(startDate),
		EndDate:   dateOnlyArg(endDate),
	})
	if err != nil {
		return nil, err
	}
	out := make([]dofficial.DailyKeywordAssignment, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapAssignment(row))
	}
	return out, nil
}

func (r *Repository) LockRotationState(ctx context.Context) (dofficial.RotationState, error) {
	row, err := r.q.LockOfficialKeywordRotationState(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dofficial.RotationState{}, common.ErrNotFound
		}
		return dofficial.RotationState{}, err
	}
	return mapRotationState(row), nil
}

func (r *Repository) InitializeRotation(ctx context.Context, effectiveDate time.Time) (dofficial.RotationState, error) {
	row, err := r.q.InitializeOfficialKeywordRotation(ctx, dateOnlyArg(effectiveDate))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dofficial.RotationState{}, common.ErrConflict
		}
		return dofficial.RotationState{}, err
	}
	return mapRotationState(row), nil
}

func (r *Repository) InsertRotationQueueItem(ctx context.Context, keywordID uuid.UUID, queuePosition int64) error {
	return r.q.InsertOfficialKeywordRotationQueueItem(ctx, officialdb.InsertOfficialKeywordRotationQueueItemParams{
		KeywordID:     keywordID,
		QueuePosition: queuePosition,
	})
}

func (r *Repository) SyncRotationQueueTail(ctx context.Context) (dofficial.RotationState, error) {
	row, err := r.q.SyncOfficialKeywordRotationQueueTail(ctx)
	if err != nil {
		return dofficial.RotationState{}, err
	}
	return mapRotationState(row), nil
}

func (r *Repository) ListRotationQueue(ctx context.Context) ([]dofficial.RotationQueueKeyword, error) {
	rows, err := r.q.ListOfficialKeywordRotationQueue(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dofficial.RotationQueueKeyword, 0, len(rows))
	for _, row := range rows {
		out = append(out, dofficial.RotationQueueKeyword{
			KeywordID:     row.KeywordID,
			QueuePosition: row.QueuePosition,
			IsActive:      row.IsActive,
		})
	}
	return out, nil
}

func (r *Repository) MoveRotationKeywordToTail(ctx context.Context, keywordID uuid.UUID) (int64, error) {
	row, err := r.q.MoveOfficialKeywordToRotationTail(ctx, keywordID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, common.ErrNotFound
		}
		return 0, err
	}
	return row.QueuePosition, nil
}

func (r *Repository) MoveOrInsertRotationKeywordAtTail(ctx context.Context, keywordID uuid.UUID) error {
	_, err := r.q.MoveOrInsertOfficialKeywordAtRotationTail(ctx, keywordID)
	return err
}

func (r *Repository) AdvanceRotationDate(ctx context.Context, bizDate time.Time) (dofficial.RotationState, error) {
	row, err := r.q.AdvanceOfficialKeywordRotationDate(ctx, dateOnlyArg(bizDate))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dofficial.RotationState{}, common.ErrNotFound
		}
		return dofficial.RotationState{}, err
	}
	return mapRotationState(row), nil
}

func (r *Repository) InsertOfficialKeywordChange(ctx context.Context, params dofficial.InsertOfficialKeywordChangeParams) (dofficial.OfficialKeywordChange, error) {
	row, err := r.q.InsertOfficialKeywordChange(ctx, officialdb.InsertOfficialKeywordChangeParams{
		KeywordID:     params.KeywordID,
		Action:        string(params.Action),
		EffectiveDate: dateOnlyArg(params.EffectiveDate),
	})
	if err != nil {
		return dofficial.OfficialKeywordChange{}, err
	}
	return mapKeywordChange(row), nil
}

func (r *Repository) ListPendingOfficialKeywordChangesThrough(ctx context.Context, bizDate time.Time) ([]dofficial.OfficialKeywordChange, error) {
	rows, err := r.q.ListPendingOfficialKeywordChangesThrough(ctx, dateOnlyArg(bizDate))
	if err != nil {
		return nil, err
	}
	out := make([]dofficial.OfficialKeywordChange, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapKeywordChange(row))
	}
	return out, nil
}

func (r *Repository) SnapshotOfficialKeywordCatalog(ctx context.Context) error {
	return r.q.SnapshotOfficialKeywordCatalog(ctx)
}

func (r *Repository) RegisterOfficialKeywordBaseline(ctx context.Context, keywordID uuid.UUID) error {
	return r.q.RegisterOfficialKeywordBaseline(ctx, keywordID)
}

func (r *Repository) HasOfficialKeywordCatalogMismatch(ctx context.Context) (bool, error) {
	return r.q.HasOfficialKeywordCatalogMismatch(ctx)
}

func (r *Repository) SetOfficialKeywordActive(ctx context.Context, keywordID uuid.UUID, isActive bool) error {
	return r.q.SetOfficialKeywordActive(ctx, officialdb.SetOfficialKeywordActiveParams{
		KeywordID: keywordID,
		IsActive:  isActive,
	})
}

func (r *Repository) MarkOfficialKeywordChangeApplied(ctx context.Context, changeID int64) error {
	return r.q.MarkOfficialKeywordChangeApplied(ctx, changeID)
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

func mapRotationState(row officialdb.OfficialKeywordRotationState) dofficial.RotationState {
	state := dofficial.RotationState{
		Initialized:       row.Initialized,
		NextQueuePosition: row.NextQueuePosition,
	}
	if row.EffectiveDate.Valid {
		state.EffectiveDate = row.EffectiveDate.Time
	}
	if row.LastPlannedDate.Valid {
		state.LastPlannedDate = row.LastPlannedDate.Time
	}
	return state
}

func mapKeywordChange(row officialdb.OfficialKeywordChange) dofficial.OfficialKeywordChange {
	change := dofficial.OfficialKeywordChange{
		ID:            row.ID,
		KeywordID:     row.KeywordID,
		Action:        dofficial.KeywordChangeAction(row.Action),
		EffectiveDate: row.EffectiveDate.Time,
		CreatedAt:     row.CreatedAt.Time,
	}
	if row.AppliedAt.Valid {
		change.AppliedAt = row.AppliedAt.Time
	}
	return change
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

func mapDailyPrompt(row officialdb.UserDailyPrompt) dofficial.DailyPrompt {
	return dofficial.DailyPrompt{
		UserID:          row.UserID,
		BizDate:         row.BizDate.Time,
		KeywordID:       row.KeywordID,
		PromptID:        row.PromptID,
		Kind:            dofficial.PromptKind(enumString(row.Kind)),
		ContentSnapshot: row.ContentSnapshot,
		SelectedAt:      row.SelectedAt.Time,
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
