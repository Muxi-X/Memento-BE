package application

import (
	"context"
	"time"

	"github.com/google/uuid"

	customapp "cixing/internal/modules/customkeywords/application"
	officialapp "cixing/internal/modules/official/application"
	"cixing/internal/modules/readmodel/infra/db/repo"
	platformoss "cixing/internal/platform/oss"
	"cixing/internal/shared/common"
)

type Service struct {
	repo            *repo.Repository
	resolver        *platformoss.URLResolver
	officialCatalog *officialapp.CatalogService
	customKeywords  *customapp.Service
	now             func() time.Time
}

func NewService(repo *repo.Repository, resolver *platformoss.URLResolver, officialCatalog *officialapp.CatalogService, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{
		repo:            repo,
		resolver:        resolver,
		officialCatalog: officialCatalog,
		now:             now,
	}
}

func (s *Service) WithCustomKeywords(customKeywords *customapp.Service) *Service {
	s.customKeywords = customKeywords
	return s
}

type OfficialHomeOutput struct {
	Today     OfficialHomeDay
	Yesterday OfficialHomeDay
}

type OfficialHomeDay = repo.OfficialHomeDay

type MeHomeOutput struct {
	Nickname                string
	AvatarURL               *string
	OfficialImageCount      int32
	CustomImageCount        int32
	CustomKeywords          []MeHomeCustomKeywordOutput
	UnreadNotificationCount int32
}

type SquareSmallImageRefOutput struct {
	ID          uuid.UUID
	SquareSmall ImageVariantOutput
}

type MeHomeCustomKeywordOutput struct {
	ID               uuid.UUID
	Text             string
	TargetImageCount *int32
	TotalImageCount  int32
	MyImageCount     int32
	CoverImage       *SquareSmallImageRefOutput
}

func (s *Service) GetOfficialHome(ctx context.Context, baseDate *time.Time) (*OfficialHomeOutput, error) {
	target := common.NormalizeBizDate(s.now())
	if baseDate != nil {
		target = common.NormalizeBizDate(*baseDate)
	}
	if s.officialCatalog != nil {
		for _, day := range []time.Time{target, target.AddDate(0, 0, -1)} {
			if !s.shouldLazyAssignOfficialDate(day) {
				continue
			}
			if _, err := s.officialCatalog.EnsureDailyKeywordAssignment(ctx, day); err != nil {
				return nil, err
			}
		}
	}

	today, err := s.repo.GetOfficialHomeDay(ctx, target)
	if err != nil {
		return nil, err
	}
	yesterday, err := s.repo.GetOfficialHomeDay(ctx, target.AddDate(0, 0, -1))
	if err != nil {
		return nil, err
	}

	return &OfficialHomeOutput{
		Today:     today,
		Yesterday: yesterday,
	}, nil
}

func (s *Service) GetMeHome(ctx context.Context, userID uuid.UUID) (*MeHomeOutput, error) {
	row, err := s.repo.GetMeHomeSummary(ctx, userID)
	if err != nil {
		return nil, err
	}

	customImageCount := row.CustomImageCount
	outKeywords := make([]MeHomeCustomKeywordOutput, 0)
	if s.customKeywords != nil {
		customHome, err := s.customKeywords.ListForMeHome(ctx, userID)
		if err != nil {
			return nil, err
		}
		customImageCount = customHome.TotalImageCount
		outKeywords = make([]MeHomeCustomKeywordOutput, 0, len(customHome.Items))
		for _, item := range customHome.Items {
			outKeywords = append(outKeywords, MeHomeCustomKeywordOutput{
				ID:               item.ID,
				Text:             item.Text,
				TargetImageCount: item.TargetImageCount,
				TotalImageCount:  item.TotalImageCount,
				MyImageCount:     item.MyImageCount,
				CoverImage:       squareSmallCoverFromCustomKeywords(item.CoverImage),
			})
		}
	} else {
		customKeywords, err := s.repo.ListMeHomeCustomKeywords(ctx, userID)
		if err != nil {
			return nil, err
		}
		outKeywords = make([]MeHomeCustomKeywordOutput, 0, len(customKeywords))
		for _, item := range customKeywords {
			outKeywords = append(outKeywords, s.meHomeCustomKeywordOutput(item))
		}
	}

	return &MeHomeOutput{
		Nickname:                row.Nickname,
		AvatarURL:               resolveSquareSmallURL(s.resolver, row.AvatarObjectKey),
		OfficialImageCount:      row.OfficialImageCount,
		CustomImageCount:        customImageCount,
		CustomKeywords:          outKeywords,
		UnreadNotificationCount: row.UnreadNotificationCount,
	}, nil
}

func (s *Service) meHomeCustomKeywordOutput(item repo.MeHomeCustomKeyword) MeHomeCustomKeywordOutput {
	out := MeHomeCustomKeywordOutput{
		ID:               item.ID,
		Text:             item.Text,
		TargetImageCount: item.TargetImageCount,
		TotalImageCount:  item.TotalImageCount,
		MyImageCount:     item.MyImageCount,
	}
	if item.CoverImage == nil || s.resolver == nil {
		return out
	}
	variant := s.resolver.ResolveSquareSmallVariant(item.CoverImage.ObjectKey)
	if variant == nil {
		return out
	}
	out.CoverImage = &SquareSmallImageRefOutput{
		ID: item.CoverImage.ID,
		SquareSmall: ImageVariantOutput{
			URL:    variant.URL,
			Width:  variant.Width,
			Height: variant.Height,
		},
	}
	return out
}

func resolveURL(resolver *platformoss.URLResolver, objectKey *string) *string {
	if resolver == nil || objectKey == nil || *objectKey == "" {
		return nil
	}
	url := resolver.ResolveObjectKey(*objectKey)
	if url == "" {
		return nil
	}
	return &url
}

func resolveSquareSmallURL(resolver *platformoss.URLResolver, objectKey *string) *string {
	if resolver == nil || objectKey == nil || *objectKey == "" {
		return nil
	}
	url := resolver.ResolveSquareSmallObjectKey(*objectKey)
	if url == "" {
		return nil
	}
	return &url
}

func squareSmallCoverFromCustomKeywords(in *customapp.SquareSmallImageRefOutput) *SquareSmallImageRefOutput {
	if in == nil {
		return nil
	}
	return &SquareSmallImageRefOutput{
		ID: in.ID,
		SquareSmall: ImageVariantOutput{
			URL:    in.SquareSmall.URL,
			Width:  in.SquareSmall.Width,
			Height: in.SquareSmall.Height,
		},
	}
}

func (s *Service) shouldLazyAssignOfficialDate(bizDate time.Time) bool {
	today := common.NormalizeBizDate(s.now())
	target := common.NormalizeBizDate(bizDate)
	return target.Equal(today) || target.Equal(today.AddDate(0, 0, -1))
}
