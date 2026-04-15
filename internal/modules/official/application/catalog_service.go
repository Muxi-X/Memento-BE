package application

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"

	dofficial "cixing/internal/modules/official/domain"
	"cixing/internal/shared/common"
)

var rotationAnchorBizDate = time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)

type CatalogService struct {
	repo dofficial.Repository
	now  func() time.Time
}

func NewCatalogService(repo dofficial.Repository, now func() time.Time) *CatalogService {
	if now == nil {
		now = time.Now
	}
	return &CatalogService{repo: repo, now: now}
}

func (s *CatalogService) GetDailyKeyword(ctx context.Context, bizDate time.Time) (dofficial.KeywordWithStats, error) {
	if _, err := s.EnsureDailyKeywordAssignment(ctx, bizDate); err != nil {
		return dofficial.KeywordWithStats{}, err
	}
	return s.repo.GetKeywordForDateWithStats(ctx, common.NormalizeBizDate(bizDate))
}

func (s *CatalogService) AssignDailyKeyword(ctx context.Context, bizDate time.Time, keywordID uuid.UUID) (dofficial.DailyKeywordAssignment, error) {
	return s.repo.UpsertDailyKeywordAssignment(ctx, common.NormalizeBizDate(bizDate), keywordID)
}

func (s *CatalogService) EnsureDailyKeywordAssignment(ctx context.Context, bizDate time.Time) (dofficial.DailyKeywordAssignment, error) {
	normalized := common.NormalizeBizDate(bizDate)
	assignment, err := s.repo.GetDailyKeywordAssignment(ctx, normalized)
	if err == nil {
		return assignment, nil
	}
	if !errors.Is(err, common.ErrNotFound) {
		return dofficial.DailyKeywordAssignment{}, err
	}

	active, err := s.repo.ListActiveOfficialKeywords(ctx)
	if err != nil {
		return dofficial.DailyKeywordAssignment{}, err
	}
	if len(active) == 0 {
		return dofficial.DailyKeywordAssignment{}, common.ErrNotFound
	}

	keyword := rotatingKeywordForDate(active, normalized)
	return s.repo.UpsertDailyKeywordAssignment(ctx, normalized, keyword.ID)
}

func rotatingKeywordForDate(keywords []dofficial.OfficialKeyword, bizDate time.Time) dofficial.OfficialKeyword {
	ordered := make([]dofficial.OfficialKeyword, 0, len(keywords))
	for _, keyword := range keywords {
		if keyword.IsActive {
			ordered = append(ordered, keyword)
		}
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].DisplayOrder != ordered[j].DisplayOrder {
			return ordered[i].DisplayOrder < ordered[j].DisplayOrder
		}
		return ordered[i].ID.String() < ordered[j].ID.String()
	})

	if len(ordered) == 0 {
		return dofficial.OfficialKeyword{}
	}

	days := int(common.NormalizeBizDate(bizDate).Sub(rotationAnchorBizDate) / (24 * time.Hour))
	idx := days % len(ordered)
	if idx < 0 {
		idx += len(ordered)
	}
	return ordered[idx]
}
