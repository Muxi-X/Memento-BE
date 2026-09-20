package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dofficial "cixing/internal/modules/official/domain"
	officialdb "cixing/internal/modules/official/infra/db/gen"
	officialrepo "cixing/internal/modules/official/infra/db/repo"
	"cixing/internal/platform/postgres"
	"cixing/internal/shared/common"
)

var (
	ErrCatalogNotConfigured     = errors.New("official catalog service is not configured")
	ErrKeywordChangeNotFeasible = errors.New("official keyword change is not feasible")
)

type CatalogService struct {
	db   *pgxpool.Pool
	repo dofficial.Repository
	now  func() time.Time
}

func NewCatalogService(db *pgxpool.Pool, repo dofficial.Repository, now func() time.Time) *CatalogService {
	if now == nil {
		now = time.Now
	}
	return &CatalogService{db: db, repo: repo, now: now}
}

func (s *CatalogService) GetDailyKeyword(ctx context.Context, bizDate time.Time) (dofficial.KeywordWithStats, error) {
	if _, err := s.EnsureDailyKeywordAssignment(ctx, bizDate); err != nil {
		return dofficial.KeywordWithStats{}, err
	}
	return s.repo.GetKeywordForDateWithStats(ctx, common.NormalizeBizDate(bizDate))
}

// EnsureRotationInitialized persists S = the next Shanghai business date exactly
// once. It is intended to be called during application startup before serving
// requests.
func (s *CatalogService) EnsureRotationInitialized(ctx context.Context) error {
	if s == nil || s.db == nil || s.repo == nil {
		return ErrCatalogNotConfigured
	}
	return postgres.WithTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		repo := officialrepo.NewRepository(officialdb.New(tx))
		state, err := repo.LockRotationState(ctx)
		if err != nil {
			if errors.Is(err, common.ErrNotFound) {
				return ErrRotationStateInconsistent
			}
			return err
		}
		if state.Initialized {
			return nil
		}
		effectiveDate := common.NormalizeBizDate(s.now()).AddDate(0, 0, 1)
		return s.initializeRotationLocked(ctx, repo, effectiveDate)
	})
}

// InitializeRotation is an explicit initialization entry for controlled
// migration/testing workflows. It is idempotent after S has been persisted.
func (s *CatalogService) InitializeRotation(ctx context.Context, effectiveDate time.Time) error {
	if s == nil || s.db == nil || s.repo == nil {
		return ErrCatalogNotConfigured
	}
	effectiveDate = common.NormalizeBizDate(effectiveDate)
	if effectiveDate.IsZero() {
		return errors.New("effective business date is required")
	}
	return postgres.WithTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		repo := officialrepo.NewRepository(officialdb.New(tx))
		return s.initializeRotationLocked(ctx, repo, effectiveDate)
	})
}

func (s *CatalogService) initializeRotationLocked(ctx context.Context, repo dofficial.Repository, effectiveDate time.Time) error {
	state, err := repo.LockRotationState(ctx)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return ErrRotationStateInconsistent
		}
		return err
	}
	if state.Initialized {
		return nil
	}

	hasFuture, err := repo.HasDailyKeywordAssignmentOnOrAfter(ctx, effectiveDate)
	if err != nil {
		return err
	}
	if hasFuture {
		return fmt.Errorf("%w: official assignments exist on or after %s", ErrRotationStateInconsistent, effectiveDate.Format(time.DateOnly))
	}

	keywords, err := repo.ListOfficialKeywords(ctx)
	if err != nil {
		return err
	}
	lastAssignment, err := repo.GetLastDailyKeywordAssignmentBefore(ctx, effectiveDate)
	if err != nil && !errors.Is(err, common.ErrNotFound) {
		return err
	}

	queue := initialRotationQueue(keywords, lastAssignment, err == nil)
	if _, err := repo.InitializeRotation(ctx, effectiveDate); err != nil {
		return err
	}
	if err := repo.SnapshotOfficialKeywordCatalog(ctx); err != nil {
		return err
	}
	for i, keywordID := range queue {
		if err := repo.InsertRotationQueueItem(ctx, keywordID, int64(i+1)); err != nil {
			return err
		}
	}
	_, err = repo.SyncRotationQueueTail(ctx)
	return err
}

func (s *CatalogService) EnsureDailyKeywordAssignment(ctx context.Context, bizDate time.Time) (dofficial.DailyKeywordAssignment, error) {
	if s == nil || s.db == nil || s.repo == nil {
		return dofficial.DailyKeywordAssignment{}, ErrCatalogNotConfigured
	}

	var assignment dofficial.DailyKeywordAssignment
	err := postgres.WithTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		repo := officialrepo.NewRepository(officialdb.New(tx))
		var err error
		assignment, err = s.ensureDailyKeywordAssignment(ctx, repo, bizDate, nil)
		return err
	})
	if err != nil {
		return dofficial.DailyKeywordAssignment{}, err
	}
	return assignment, nil
}

// EnsureDailyKeywordAssignmentWithRepository performs the scheduling operation on
// the caller's existing transaction. Callers must keep the repository transaction
// open until this method returns.
func (s *CatalogService) EnsureDailyKeywordAssignmentWithRepository(
	ctx context.Context,
	repo dofficial.Repository,
	bizDate time.Time,
) (dofficial.DailyKeywordAssignment, error) {
	if s == nil || repo == nil {
		return dofficial.DailyKeywordAssignment{}, ErrCatalogNotConfigured
	}
	return s.ensureDailyKeywordAssignment(ctx, repo, bizDate, nil)
}

// LockDailyKeywordRotation acquires the database-wide rotation lock in the
// caller's transaction. It allows callers that also hold another lock to resample
// business time after all waits have completed.
func (s *CatalogService) LockDailyKeywordRotation(ctx context.Context, repo dofficial.Repository) error {
	if s == nil || repo == nil {
		return ErrCatalogNotConfigured
	}
	if _, err := repo.LockRotationState(ctx); err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return ErrRotationStateInconsistent
		}
		return err
	}
	return nil
}

// ScheduleOfficialKeyword persists keyword metadata and records the activation
// state change for the next Shanghai business day. It does not expose an HTTP
// management endpoint.
func (s *CatalogService) ScheduleOfficialKeyword(ctx context.Context, params dofficial.UpsertKeywordParams) (dofficial.OfficialKeyword, error) {
	if s == nil || s.db == nil || s.repo == nil {
		return dofficial.OfficialKeyword{}, ErrCatalogNotConfigured
	}

	var result dofficial.OfficialKeyword
	err := postgres.WithTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		repo := officialrepo.NewRepository(officialdb.New(tx))
		state, err := repo.LockRotationState(ctx)
		if err != nil {
			return err
		}
		if !state.Initialized {
			return ErrRotationNotInitialized
		}
		if err := validateRecordedCatalog(ctx, repo); err != nil {
			return err
		}

		effectiveDate := common.NormalizeBizDate(s.now()).AddDate(0, 0, 1)
		existing, err := repo.GetOfficialKeywordByID(ctx, params.ID)
		switch {
		case errors.Is(err, common.ErrNotFound):
			insertParams := params
			insertParams.IsActive = false
			result, err = repo.InsertOfficialKeyword(ctx, insertParams)
			if err != nil {
				return err
			}
			if err := repo.RegisterOfficialKeywordBaseline(ctx, result.ID); err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			updateParams := params
			order := existing.DisplayOrder
			updateParams.DisplayOrder = &order
			result, err = repo.UpdateOfficialKeyword(ctx, updateParams)
			if err != nil {
				return err
			}
		}

		// The catalog reflects the last generated day, which may lag behind
		// accepted changes. Compare with the projected state on the new change's
		// effective date, including earlier requests for that same date.
		projectedActive := result.IsActive
		pending, err := repo.ListPendingOfficialKeywordChangesThrough(ctx, effectiveDate)
		if err != nil {
			return err
		}
		for _, change := range pending {
			if change.KeywordID == params.ID {
				projectedActive = change.Action == dofficial.KeywordChangeActivate
			}
		}
		if params.IsActive == projectedActive {
			return nil
		}
		if !params.IsActive {
			if err := s.validateDeactivationLocked(ctx, repo, params.ID, effectiveDate); err != nil {
				return err
			}
		}
		action := dofficial.KeywordChangeActivate
		if !params.IsActive {
			action = dofficial.KeywordChangeDeactivate
		}
		_, err = repo.InsertOfficialKeywordChange(ctx, dofficial.InsertOfficialKeywordChangeParams{
			KeywordID:     params.ID,
			Action:        action,
			EffectiveDate: effectiveDate,
		})
		return err
	})
	if err != nil {
		return dofficial.OfficialKeyword{}, err
	}
	return result, nil
}

func (s *CatalogService) DeactivateOfficialKeyword(ctx context.Context, keywordID uuid.UUID) error {
	if s == nil || s.repo == nil {
		return ErrCatalogNotConfigured
	}
	keyword, err := s.repo.GetOfficialKeywordByID(ctx, keywordID)
	if err != nil {
		return err
	}
	_, err = s.ScheduleOfficialKeyword(ctx, dofficial.UpsertKeywordParams{
		ID:           keyword.ID,
		Text:         keyword.Text,
		Category:     keyword.Category,
		IsActive:     false,
		DisplayOrder: &keyword.DisplayOrder,
	})
	return err
}

func (s *CatalogService) EnsureDailyKeywordAssignmentWithValidator(ctx context.Context, bizDate time.Time, validate func(time.Time) error) (dofficial.DailyKeywordAssignment, error) {
	if s == nil || s.db == nil || s.repo == nil {
		return dofficial.DailyKeywordAssignment{}, ErrCatalogNotConfigured
	}
	var assignment dofficial.DailyKeywordAssignment
	err := postgres.WithTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		repo := officialrepo.NewRepository(officialdb.New(tx))
		var err error
		assignment, err = s.ensureDailyKeywordAssignment(ctx, repo, bizDate, validate)
		return err
	})
	if err != nil {
		return dofficial.DailyKeywordAssignment{}, err
	}
	return assignment, nil
}

func (s *CatalogService) ensureDailyKeywordAssignment(
	ctx context.Context,
	repo dofficial.Repository,
	bizDate time.Time,
	validate func(time.Time) error,
) (dofficial.DailyKeywordAssignment, error) {
	normalized := common.NormalizeBizDate(bizDate)
	assignment, err := repo.GetDailyKeywordAssignment(ctx, normalized)
	if err == nil {
		return assignment, nil
	}
	if !errors.Is(err, common.ErrNotFound) {
		return dofficial.DailyKeywordAssignment{}, err
	}

	state, err := repo.LockRotationState(ctx)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return dofficial.DailyKeywordAssignment{}, ErrRotationStateInconsistent
		}
		return dofficial.DailyKeywordAssignment{}, err
	}
	assignment, err = repo.GetDailyKeywordAssignment(ctx, normalized)
	if err == nil {
		return assignment, nil
	}
	if !errors.Is(err, common.ErrNotFound) {
		return dofficial.DailyKeywordAssignment{}, err
	}
	if !state.Initialized {
		activeKeywords, err := repo.ListActiveOfficialKeywords(ctx)
		if err != nil {
			return dofficial.DailyKeywordAssignment{}, err
		}
		if len(activeKeywords) == 0 {
			return dofficial.DailyKeywordAssignment{}, common.ErrNotFound
		}
		return dofficial.DailyKeywordAssignment{}, ErrRotationNotInitialized
	}
	if normalized.Before(state.EffectiveDate) {
		return dofficial.DailyKeywordAssignment{}, common.ErrNotFound
	}

	now := s.now()
	if validate != nil {
		if err := validate(now); err != nil {
			return dofficial.DailyKeywordAssignment{}, err
		}
	}
	today := common.NormalizeBizDate(now)
	if normalized.After(today) {
		return dofficial.DailyKeywordAssignment{}, ErrRotationDateNotAllowed
	}

	startDate := state.EffectiveDate
	if !state.LastPlannedDate.IsZero() {
		startDate = state.LastPlannedDate.AddDate(0, 0, 1)
	}
	if normalized.Before(startDate) {
		return dofficial.DailyKeywordAssignment{}, fmt.Errorf(
			"%w: missing date %s is before next schedulable date %s",
			ErrRotationStateInconsistent,
			normalized.Format(time.DateOnly),
			startDate.Format(time.DateOnly),
		)
	}

	// Validate before applying changes: an overdue action must not conceal an
	// unrecorded edit to the catalog by overwriting it during catch-up.
	if err := validateRecordedCatalog(ctx, repo); err != nil {
		return dofficial.DailyKeywordAssignment{}, err
	}
	for day := startDate; !day.After(normalized); day = day.AddDate(0, 0, 1) {
		changes, err := repo.ListPendingOfficialKeywordChangesThrough(ctx, day)
		if err != nil {
			return dofficial.DailyKeywordAssignment{}, err
		}
		for _, change := range changes {
			if err := applyOfficialKeywordChange(ctx, repo, change); err != nil {
				return dofficial.DailyKeywordAssignment{}, err
			}
		}

		queue, err := repo.ListRotationQueue(ctx)
		if err != nil {
			return dofficial.DailyKeywordAssignment{}, err
		}
		if err := validateActiveQueueConsistency(ctx, repo, queue); err != nil {
			return dofficial.DailyKeywordAssignment{}, err
		}
		historyRows, err := repo.ListDailyKeywordAssignmentsBetween(
			ctx,
			day.AddDate(0, 0, -29),
			day.AddDate(0, 0, -1),
		)
		if err != nil {
			return dofficial.DailyKeywordAssignment{}, err
		}
		history := make(map[time.Time]uuid.UUID, len(historyRows))
		for _, row := range historyRows {
			history[common.NormalizeBizDate(row.BizDate)] = row.KeywordID
		}

		decision, err := planNextDay(day, state.EffectiveDate, queue, history)
		if err != nil {
			if errors.Is(err, ErrNoEligibleRotationKeyword) && !hasActiveRotationKeyword(queue) {
				return dofficial.DailyKeywordAssignment{}, common.ErrNotFound
			}
			return dofficial.DailyKeywordAssignment{}, fmt.Errorf(
				"%w: plan %s: %v",
				ErrRotationStateInconsistent,
				day.Format(time.DateOnly),
				err,
			)
		}

		_, inserted, err := repo.InsertDailyKeywordAssignment(ctx, day, decision.KeywordID)
		if err != nil {
			return dofficial.DailyKeywordAssignment{}, err
		}
		if !inserted {
			actual, getErr := repo.GetDailyKeywordAssignment(ctx, day)
			if getErr != nil {
				return dofficial.DailyKeywordAssignment{}, getErr
			}
			return dofficial.DailyKeywordAssignment{}, fmt.Errorf(
				"%w: date %s already contains keyword %s while rotation is at %s",
				ErrRotationStateInconsistent,
				day.Format(time.DateOnly),
				actual.KeywordID,
				state.LastPlannedDate.Format(time.DateOnly),
			)
		}

		if decision.Mode == rotationModeNormal {
			if _, err := repo.MoveRotationKeywordToTail(ctx, decision.QueueKeywordID); err != nil {
				return dofficial.DailyKeywordAssignment{}, err
			}
		}
		if _, err := repo.AdvanceRotationDate(ctx, day); err != nil {
			return dofficial.DailyKeywordAssignment{}, err
		}
	}

	result, err := repo.GetDailyKeywordAssignment(ctx, normalized)
	if err != nil {
		return dofficial.DailyKeywordAssignment{}, err
	}
	return result, nil
}

func applyOfficialKeywordChange(ctx context.Context, repo dofficial.Repository, change dofficial.OfficialKeywordChange) error {
	switch change.Action {
	case dofficial.KeywordChangeActivate:
		if err := repo.SetOfficialKeywordActive(ctx, change.KeywordID, true); err != nil {
			return err
		}
		if err := repo.MoveOrInsertRotationKeywordAtTail(ctx, change.KeywordID); err != nil {
			return err
		}
	case dofficial.KeywordChangeDeactivate:
		if err := repo.SetOfficialKeywordActive(ctx, change.KeywordID, false); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%w: unknown change action %q", ErrRotationStateInconsistent, change.Action)
	}
	return repo.MarkOfficialKeywordChangeApplied(ctx, change.ID)
}

func (s *CatalogService) validateDeactivationLocked(ctx context.Context, repo dofficial.Repository, keywordID uuid.UUID, effectiveDate time.Time) error {
	state, err := repo.LockRotationState(ctx)
	if err != nil {
		return err
	}
	if !state.Initialized {
		return ErrRotationNotInitialized
	}

	keywords, err := repo.ListOfficialKeywords(ctx)
	if err != nil {
		return err
	}
	active := make(map[uuid.UUID]bool, len(keywords))
	for _, keyword := range keywords {
		active[keyword.ID] = keyword.IsActive
	}
	queueRows, err := repo.ListRotationQueue(ctx)
	if err != nil {
		return err
	}
	order := make([]uuid.UUID, 0, len(queueRows)+1)
	for _, row := range queueRows {
		order = append(order, row.KeywordID)
	}

	changes, err := repo.ListPendingOfficialKeywordChangesThrough(ctx, effectiveDate.AddDate(0, 0, 29))
	if err != nil {
		return err
	}
	newChangeID := int64(1)
	for _, change := range changes {
		if change.ID >= newChangeID {
			newChangeID = change.ID + 1
		}
	}
	changes = append(changes, dofficial.OfficialKeywordChange{
		ID:            newChangeID,
		KeywordID:     keywordID,
		Action:        dofficial.KeywordChangeDeactivate,
		EffectiveDate: effectiveDate,
	})
	sort.SliceStable(changes, func(i, j int) bool {
		if !changes[i].EffectiveDate.Equal(changes[j].EffectiveDate) {
			return changes[i].EffectiveDate.Before(changes[j].EffectiveDate)
		}
		return changes[i].ID < changes[j].ID
	})

	simulationStart := state.LastPlannedDate.AddDate(0, 0, 1)
	if simulationStart.Before(state.EffectiveDate) {
		simulationStart = state.EffectiveDate
	}
	historyRows, err := repo.ListDailyKeywordAssignmentsBetween(ctx, simulationStart.AddDate(0, 0, -29), simulationStart.AddDate(0, 0, -1))
	if err != nil {
		return err
	}
	history := make(map[time.Time]uuid.UUID, len(historyRows)+30)
	for _, row := range historyRows {
		history[common.NormalizeBizDate(row.BizDate)] = row.KeywordID
	}

	changeIndex := 0
	for day := simulationStart; !day.After(effectiveDate.AddDate(0, 0, 29)); day = day.AddDate(0, 0, 1) {
		for changeIndex < len(changes) && !changes[changeIndex].EffectiveDate.After(day) {
			change := changes[changeIndex]
			changeIndex++
			if err := applyVirtualKeywordChange(active, &order, change); err != nil {
				return err
			}
		}
		if !day.Before(effectiveDate) && countActiveKeywords(active) < 7 {
			return fmt.Errorf("%w: fewer than seven active keywords on %s", ErrKeywordChangeNotFeasible, day.Format(time.DateOnly))
		}
		queue, err := virtualRotationQueue(order, active)
		if err != nil {
			return err
		}
		decision, err := planNextDay(day, state.EffectiveDate, queue, history)
		if err != nil {
			return fmt.Errorf("%w: simulation failed on %s: %v", ErrKeywordChangeNotFeasible, day.Format(time.DateOnly), err)
		}
		history[day] = decision.KeywordID
		if decision.Mode == rotationModeNormal {
			moveVirtualKeyword(&order, decision.QueueKeywordID)
		}
	}
	// Earlier dates only reconstruct the queue and history. Old windows that
	// end before this change takes effect must not reject an otherwise feasible
	// change. Validate each affected window once after the simulation.
	return validateSimulationWindows(history, state.EffectiveDate, effectiveDate, effectiveDate.AddDate(0, 0, 29))
}

func initialRotationQueue(keywords []dofficial.OfficialKeyword, last dofficial.DailyKeywordAssignment, hasLast bool) []uuid.UUID {
	if len(keywords) == 0 {
		return nil
	}
	start := 0
	if hasLast {
		for i, keyword := range keywords {
			if keyword.ID == last.KeywordID {
				start = (i + 1) % len(keywords)
				break
			}
		}
	}
	queue := make([]uuid.UUID, 0, len(keywords))
	for offset := 0; offset < len(keywords); offset++ {
		keyword := keywords[(start+offset)%len(keywords)]
		if keyword.IsActive {
			queue = append(queue, keyword.ID)
		}
	}
	return queue
}

func applyVirtualKeywordChange(active map[uuid.UUID]bool, order *[]uuid.UUID, change dofficial.OfficialKeywordChange) error {
	switch change.Action {
	case dofficial.KeywordChangeActivate:
		active[change.KeywordID] = true
		moveVirtualKeyword(order, change.KeywordID)
	case dofficial.KeywordChangeDeactivate:
		active[change.KeywordID] = false
	default:
		return fmt.Errorf("%w: unknown change action %q", ErrRotationStateInconsistent, change.Action)
	}
	return nil
}

func virtualRotationQueue(order []uuid.UUID, active map[uuid.UUID]bool) ([]dofficial.RotationQueueKeyword, error) {
	queue := make([]dofficial.RotationQueueKeyword, 0, len(order))
	seen := make(map[uuid.UUID]struct{}, len(order))
	for i, keywordID := range order {
		queue = append(queue, dofficial.RotationQueueKeyword{
			KeywordID:     keywordID,
			QueuePosition: int64(i + 1),
			IsActive:      active[keywordID],
		})
		seen[keywordID] = struct{}{}
	}
	for keywordID, isActive := range active {
		if isActive {
			if _, ok := seen[keywordID]; !ok {
				return nil, fmt.Errorf("%w: active keyword %s is missing from queue", ErrRotationStateInconsistent, keywordID)
			}
		}
	}
	return queue, nil
}

func moveVirtualKeyword(order *[]uuid.UUID, keywordID uuid.UUID) {
	for i, current := range *order {
		if current != keywordID {
			continue
		}
		copy((*order)[i:], (*order)[i+1:])
		(*order)[len(*order)-1] = keywordID
		return
	}
	*order = append(*order, keywordID)
}

func countActiveKeywords(active map[uuid.UUID]bool) int {
	count := 0
	for _, isActive := range active {
		if isActive {
			count++
		}
	}
	return count
}

func validateSimulationWindows(history map[time.Time]uuid.UUID, effectiveDate time.Time, simulationStart time.Time, simulationEnd time.Time) error {
	for end := simulationStart; !end.After(simulationEnd); end = end.AddDate(0, 0, 1) {
		if window, ok := completeWindow(history, end.AddDate(0, 0, -6), end); ok {
			seen := make(map[uuid.UUID]struct{}, len(window))
			for _, keywordID := range window {
				seen[keywordID] = struct{}{}
			}
			if len(seen) != 7 {
				return fmt.Errorf("%w: seven-day window ending %s repeats a keyword", ErrKeywordChangeNotFeasible, end.Format(time.DateOnly))
			}
		}
		start30 := end.AddDate(0, 0, -29)
		if start30.Before(effectiveDate) {
			continue
		}
		if window, ok := completeWindow(history, start30, end); ok {
			seen := make(map[uuid.UUID]struct{}, len(window))
			for _, keywordID := range window {
				seen[keywordID] = struct{}{}
			}
			if len(seen) >= 30 {
				return fmt.Errorf("%w: thirty-day window ending %s has no repeat", ErrKeywordChangeNotFeasible, end.Format(time.DateOnly))
			}
		}
	}
	return nil
}

func completeWindow(history map[time.Time]uuid.UUID, start time.Time, end time.Time) ([]uuid.UUID, bool) {
	out := make([]uuid.UUID, 0, 30)
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		keywordID, ok := history[day]
		if !ok {
			return nil, false
		}
		out = append(out, keywordID)
	}
	return out, true
}

func hasActiveRotationKeyword(queue []dofficial.RotationQueueKeyword) bool {
	for _, item := range queue {
		if item.IsActive {
			return true
		}
	}
	return false
}

func validateRecordedCatalog(ctx context.Context, repo dofficial.Repository) error {
	mismatch, err := repo.HasOfficialKeywordCatalogMismatch(ctx)
	if err != nil {
		return err
	}
	if mismatch {
		return fmt.Errorf("%w: official catalog does not match its baseline and applied changes", ErrRotationStateInconsistent)
	}
	return nil
}

func validateActiveQueueConsistency(ctx context.Context, repo dofficial.Repository, queue []dofficial.RotationQueueKeyword) error {
	active, err := repo.ListActiveOfficialKeywords(ctx)
	if err != nil {
		return err
	}
	queued := make(map[uuid.UUID]struct{}, len(queue))
	for _, item := range queue {
		queued[item.KeywordID] = struct{}{}
	}
	for _, keyword := range active {
		if _, ok := queued[keyword.ID]; !ok {
			return fmt.Errorf("%w: active keyword %s is missing from the rotation queue", ErrRotationStateInconsistent, keyword.ID)
		}
	}
	return nil
}
