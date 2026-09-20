package application

import (
	"context"
	"errors"
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
	ErrInvalidPromptKind   = errors.New("invalid prompt kind")
	ErrPromptDateChanged   = errors.New("prompt date changed")
	ErrKeywordMismatch     = errors.New("prompt keyword mismatch")
	ErrPromptNotAvailable  = errors.New("prompt not available")
	ErrUnauthenticatedUser = errors.New("unauthenticated user")
)

type PromptService struct {
	db      *pgxpool.Pool
	repo    dofficial.Repository
	catalog *CatalogService
	now     func() time.Time
}

func NewPromptService(db *pgxpool.Pool, repo dofficial.Repository, catalog *CatalogService, now func() time.Time) *PromptService {
	if now == nil {
		now = time.Now
	}
	return &PromptService{db: db, repo: repo, catalog: catalog, now: now}
}

type PromptOutput struct {
	ID         uuid.UUID
	Kind       dofficial.PromptKind
	Content    string
	BizDate    time.Time
	KeywordID  uuid.UUID
	SelectedAt time.Time
	ResetsAt   time.Time
}

type DailyPromptSelectionOutput struct {
	ID         uuid.UUID
	Kind       dofficial.PromptKind
	Content    string
	SelectedAt time.Time
}

type DailyPromptOutput struct {
	BizDate   time.Time
	ResetsAt  time.Time
	KeywordID *uuid.UUID
	Selection *DailyPromptSelectionOutput
}

func (s *PromptService) GetDailyPrompt(ctx context.Context, userID uuid.UUID) (*DailyPromptOutput, error) {
	if userID == uuid.Nil {
		return nil, ErrUnauthenticatedUser
	}

	now := s.now()
	bizDate, resetsAt := businessDateBounds(now)
	selection, err := s.repo.GetDailyPrompt(ctx, userID, bizDate)
	if err == nil {
		return dailyPromptOutput(selection, resetsAt), nil
	}
	if !errors.Is(err, common.ErrNotFound) {
		return nil, err
	}

	var keywordID *uuid.UUID
	if s.catalog != nil {
		assignment, err := s.catalog.EnsureDailyKeywordAssignment(ctx, bizDate)
		switch {
		case err == nil:
			keyword, keywordErr := s.repo.GetOfficialKeywordByID(ctx, assignment.KeywordID)
			if keywordErr == nil && keyword.IsActive {
				id := assignment.KeywordID
				keywordID = &id
			} else if keywordErr != nil && !errors.Is(keywordErr, common.ErrNotFound) {
				return nil, keywordErr
			}
		case errors.Is(err, common.ErrNotFound):
			// An empty official catalog is a successful unselected state.
		default:
			return nil, err
		}
	}

	return &DailyPromptOutput{
		BizDate:   bizDate,
		ResetsAt:  resetsAt,
		KeywordID: keywordID,
		Selection: nil,
	}, nil
}

func (s *PromptService) Draw(ctx context.Context, userID uuid.UUID, keywordID uuid.UUID, kind string, requestedBizDate *time.Time) (*PromptOutput, error) {
	promptKind, err := parsePromptKind(kind)
	if err != nil {
		return nil, err
	}
	if userID == uuid.Nil {
		return nil, ErrUnauthenticatedUser
	}
	if s.db == nil || s.catalog == nil {
		return nil, errors.New("prompt service is not configured")
	}

	var out *PromptOutput
	err = postgres.WithTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		repo := officialrepo.NewRepository(officialdb.New(tx))
		if err := repo.LockUser(ctx, userID); err != nil {
			if errors.Is(err, common.ErrNotFound) {
				return ErrUnauthenticatedUser
			}
			return err
		}

		now := s.now()
		bizDate, resetsAt := businessDateBounds(now)
		if requestedBizDate != nil && !sameBusinessDate(*requestedBizDate, now) {
			return ErrPromptDateChanged
		}

		saved, err := repo.GetDailyPrompt(ctx, userID, bizDate)
		if err == nil {
			out = promptOutput(saved, resetsAt)
			return nil
		}
		if !errors.Is(err, common.ErrNotFound) {
			return err
		}

		// Keep lock order user -> rotation. The date is sampled again after any
		// wait for the global rotation lock so a request cannot cross midnight
		// silently.
		if err := s.catalog.LockDailyKeywordRotation(ctx, repo); err != nil {
			return err
		}
		now = s.now()
		postLockBizDate, postLockResetsAt := businessDateBounds(now)
		if requestedBizDate != nil && !sameBusinessDate(*requestedBizDate, now) {
			return ErrPromptDateChanged
		}
		if !postLockBizDate.Equal(bizDate) {
			bizDate, resetsAt = postLockBizDate, postLockResetsAt
			saved, err = repo.GetDailyPrompt(ctx, userID, bizDate)
			if err == nil {
				out = promptOutput(saved, resetsAt)
				return nil
			}
			if !errors.Is(err, common.ErrNotFound) {
				return err
			}
		}

		assignment, err := s.catalog.EnsureDailyKeywordAssignmentWithRepository(ctx, repo, bizDate)
		if err != nil {
			if errors.Is(err, common.ErrNotFound) {
				return ErrPromptNotAvailable
			}
			return err
		}

		keyword, err := repo.GetOfficialKeywordByID(ctx, assignment.KeywordID)
		if err != nil {
			if errors.Is(err, common.ErrNotFound) {
				return ErrPromptNotAvailable
			}
			return err
		}
		if !keyword.IsActive {
			return ErrPromptNotAvailable
		}
		if assignment.KeywordID != keywordID {
			return ErrKeywordMismatch
		}

		prompt, err := repo.DrawRandomPrompt(ctx, assignment.KeywordID, promptKind)
		if err != nil {
			if errors.Is(err, common.ErrNotFound) {
				return ErrPromptNotAvailable
			}
			return err
		}

		created, err := repo.InsertDailyPrompt(ctx, dofficial.InsertDailyPromptParams{
			UserID:          userID,
			BizDate:         bizDate,
			KeywordID:       assignment.KeywordID,
			PromptID:        prompt.ID,
			Kind:            prompt.Kind,
			ContentSnapshot: prompt.Content,
			SelectedAt:      now.UTC(),
		})
		if err == nil {
			out = promptOutput(created, resetsAt)
			return nil
		}
		if !errors.Is(err, common.ErrConflict) {
			return err
		}

		saved, err = repo.GetDailyPrompt(ctx, userID, bizDate)
		if err != nil {
			return err
		}
		out = promptOutput(saved, resetsAt)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func parsePromptKind(kind string) (dofficial.PromptKind, error) {
	switch dofficial.PromptKind(kind) {
	case dofficial.PromptKindIntuition, dofficial.PromptKindStructure, dofficial.PromptKindConcept:
		return dofficial.PromptKind(kind), nil
	default:
		return "", ErrInvalidPromptKind
	}
}

func businessDateBounds(now time.Time) (time.Time, time.Time) {
	loc := common.BusinessLocation()
	local := now.In(loc)
	nextMidnight := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, loc)
	return common.NormalizeBizDate(now), nextMidnight
}

func sameBusinessDate(value, reference time.Time) bool {
	loc := common.BusinessLocation()
	value = value.In(loc)
	reference = reference.In(loc)
	return value.Year() == reference.Year() && value.Month() == reference.Month() && value.Day() == reference.Day()
}

func promptOutput(saved dofficial.DailyPrompt, resetsAt time.Time) *PromptOutput {
	return &PromptOutput{
		ID:         saved.PromptID,
		Kind:       saved.Kind,
		Content:    saved.ContentSnapshot,
		BizDate:    common.NormalizeBizDate(saved.BizDate),
		KeywordID:  saved.KeywordID,
		SelectedAt: saved.SelectedAt,
		ResetsAt:   resetsAt,
	}
}

func dailyPromptOutput(saved dofficial.DailyPrompt, resetsAt time.Time) *DailyPromptOutput {
	keywordID := saved.KeywordID
	return &DailyPromptOutput{
		BizDate:   common.NormalizeBizDate(saved.BizDate),
		ResetsAt:  resetsAt,
		KeywordID: &keywordID,
		Selection: &DailyPromptSelectionOutput{
			ID:         saved.PromptID,
			Kind:       saved.Kind,
			Content:    saved.ContentSnapshot,
			SelectedAt: saved.SelectedAt,
		},
	}
}
