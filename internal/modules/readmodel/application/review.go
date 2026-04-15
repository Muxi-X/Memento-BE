package application

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	dofficial "cixing/internal/modules/official/domain"
	"cixing/internal/shared/common"
)

type ReviewDateItemOutput struct {
	BizDate       time.Time
	Keyword       dofficial.OfficialKeyword
	MyUploadCount *int32
	MyImageCount  *int32
}

type ReviewDatesOutput struct {
	TotalParticipationDays int64
	TotalImageCount        int64
	Items                  []ReviewDateItemOutput
}

type ReviewKeywordCountItemOutput struct {
	Keyword       dofficial.OfficialKeyword
	MyUploadCount int32
	MyImageCount  int32
}

type ReviewKeywordCountsOutput struct {
	Items []ReviewKeywordCountItemOutput
}

type ReviewUploadListOutput struct {
	Items []ReviewUploadCardOutput
}

func (s *Service) ListReviewDates(ctx context.Context, userID uuid.UUID, limit int) (*ReviewDatesOutput, error) {
	limit32 := clampLimit(limit, 30, 100)

	items, err := s.repo.ListMyReviewDateStats(ctx, userID, limit32)
	if err != nil {
		return nil, err
	}
	totalDays, err := s.repo.CountMyReviewParticipationDays(ctx, userID)
	if err != nil {
		return nil, err
	}
	totalImages, err := s.repo.CountMyReviewImageTotal(ctx, userID)
	if err != nil {
		return nil, err
	}

	outItems := make([]ReviewDateItemOutput, 0, len(items))
	for _, item := range items {
		uploadCount := item.MyUploadCount
		imageCount := item.MyImageCount
		outItems = append(outItems, ReviewDateItemOutput{
			BizDate:       item.BizDate,
			Keyword:       item.Keyword,
			MyUploadCount: &uploadCount,
			MyImageCount:  &imageCount,
		})
	}

	return &ReviewDatesOutput{
		TotalParticipationDays: totalDays,
		TotalImageCount:        totalImages,
		Items:                  outItems,
	}, nil
}

func (s *Service) ListReviewKeywords(ctx context.Context, userID uuid.UUID) (*ReviewKeywordCountsOutput, error) {
	items, err := s.repo.ListMyReviewKeywordCounts(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]ReviewKeywordCountItemOutput, 0, len(items))
	for _, item := range items {
		out = append(out, ReviewKeywordCountItemOutput{
			Keyword:       item.Keyword,
			MyUploadCount: item.MyUploadCount,
			MyImageCount:  item.MyImageCount,
		})
	}
	return &ReviewKeywordCountsOutput{Items: out}, nil
}

func (s *Service) ListReviewMyUploadsByDate(ctx context.Context, userID uuid.UUID, bizDate time.Time, limit int) (*ReviewUploadListOutput, error) {
	rows, err := s.repo.ListMyReviewUploadsByDate(ctx, userID, bizDate, clampLimit(limit, 20, 50))
	if err != nil {
		return nil, err
	}
	items := make([]ReviewUploadCardOutput, 0, len(rows))
	for _, row := range rows {
		items = append(items, s.reviewCardOutput(row))
	}
	return &ReviewUploadListOutput{Items: items}, nil
}

func (s *Service) ListReviewMyUploadsByKeyword(ctx context.Context, userID, keywordID uuid.UUID, limit int) (*ReviewUploadListOutput, error) {
	if _, err := s.repo.GetOfficialKeyword(ctx, keywordID); err != nil {
		return nil, err
	}
	rows, err := s.repo.ListMyReviewUploadsByKeyword(ctx, userID, keywordID, clampLimit(limit, 20, 50))
	if err != nil {
		return nil, err
	}
	items := make([]ReviewUploadCardOutput, 0, len(rows))
	for _, row := range rows {
		items = append(items, s.reviewCardOutput(row))
	}
	return &ReviewUploadListOutput{Items: items}, nil
}

func (s *Service) GetReviewUpload(ctx context.Context, userID, uploadID uuid.UUID) (*ReviewUploadDetailOutput, error) {
	card, err := s.repo.GetPublicUploadCard(ctx, uploadID)
	if err != nil {
		if !errors.Is(err, common.ErrNotFound) {
			return nil, err
		}
		card, err = s.repo.GetMyReviewUploadCard(ctx, uploadID, userID)
		if err != nil {
			return nil, err
		}
	}
	images, err := s.repo.ListUploadImages(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	return &ReviewUploadDetailOutput{
		ReviewUploadCardOutput: s.reviewCardOutput(card),
		Images:                 s.workImagesOutput(images),
	}, nil
}
