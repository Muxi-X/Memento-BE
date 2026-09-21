package application

import (
	"context"
	"time"

	"github.com/google/uuid"

	"cixing/internal/modules/readmodel/infra/db/repo"
	"cixing/internal/shared/common"
)

const (
	sortLatest = "latest"
	sortRandom = "random"
)

type ReactionCounts struct {
	Inspired  int32
	Resonated int32
}

type ImageVariantOutput struct {
	URL    string
	Width  int32
	Height int32
}

type CardImageRefOutput struct {
	ID      uuid.UUID
	Card4x3 ImageVariantOutput
}

type DetailLargeImageRefOutput struct {
	ID          uuid.UUID
	DetailLarge ImageVariantOutput
}

type WorkImageOutput struct {
	ID              uuid.UUID
	Image           DetailLargeImageRefOutput
	DisplayOrder    int32
	Title           *string
	Note            *string
	HasAudio        bool
	AudioDurationMs *int32
	AudioPlayURL    *string
	CreatedAt       time.Time
}

type PublicUploadCardOutput struct {
	ID                 uuid.UUID
	BizDate            time.Time
	KeywordID          uuid.UUID
	CoverImage         CardImageRefOutput
	DisplayText        *string
	CoverHasAudio      *bool
	CoverAudioDuration *int32
	ImageCount         int32
	ReactionCounts     *ReactionCounts
	MyReactions        []string
	CreatedAt          time.Time
}

type PublicUploadDetailOutput struct {
	PublicUploadCardOutput
	Images []WorkImageOutput
}

type PublicUploadListOutput struct {
	Items      []PublicUploadCardOutput
	Seed       *float64
	NextCursor *string
	HasMore    bool
}

type ReviewUploadCardOutput struct {
	ID                 uuid.UUID
	BizDate            time.Time
	KeywordID          uuid.UUID
	CoverImage         CardImageRefOutput
	DisplayText        *string
	CoverHasAudio      *bool
	CoverAudioDuration *int32
	ImageCount         int32
	CreatedAt          time.Time
}

type ReviewUploadDetailOutput struct {
	ReviewUploadCardOutput
	Images []WorkImageOutput
}

func (s *Service) ListOfficialDateUploads(ctx context.Context, bizDate time.Time, options PublicUploadListOptions, viewerID *uuid.UUID) (*PublicUploadListOutput, error) {
	bizDate = common.NormalizeBizDate(bizDate)
	page, err := resolvePublicUploadPage(dateScope, bizDate.Format("2006-01-02"), options, s.now)
	if err != nil {
		return nil, err
	}
	if s.officialCatalog != nil && s.shouldLazyAssignOfficialDate(bizDate) {
		if _, err := s.officialCatalog.EnsureDailyKeywordAssignment(ctx, bizDate); err != nil {
			return nil, err
		}
	} else {
		if _, err := s.repo.GetOfficialHomeDay(ctx, bizDate); err != nil {
			return nil, err
		}
	}
	return s.loadPublicUploadPage(ctx, page, options.IncludeReactionCounts, viewerID, func(p repo.PublicUploadPageParams) ([]repo.PublicUploadPageRow, error) {
		return s.repo.ListPublicUploadsByDatePage(ctx, bizDate, p)
	})
}

func (s *Service) GetOfficialUpload(ctx context.Context, uploadID uuid.UUID, includeReactionCounts bool, viewerID *uuid.UUID) (*PublicUploadDetailOutput, error) {
	card, err := s.repo.GetPublicUploadCard(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	images, err := s.repo.ListUploadImages(ctx, uploadID)
	if err != nil {
		return nil, err
	}

	var reactionMap map[uuid.UUID][]string
	if viewerID != nil {
		reactionMap, err = s.myReactionsByUploadIDs(ctx, *viewerID, []uuid.UUID{uploadID})
		if err != nil {
			return nil, err
		}
	}

	out := s.publicCardOutput(card, includeReactionCounts, reactionMap[uploadID])
	detail := &PublicUploadDetailOutput{
		PublicUploadCardOutput: out,
		Images:                 s.workImagesOutput(images),
	}
	return detail, nil
}

func (s *Service) ListReviewAllUploadsByKeyword(ctx context.Context, userID, keywordID uuid.UUID, options PublicUploadListOptions) (*PublicUploadListOutput, error) {
	page, err := resolvePublicUploadPage(keywordScope, keywordID.String(), options, s.now)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.GetOfficialKeyword(ctx, keywordID); err != nil {
		return nil, err
	}
	return s.loadPublicUploadPage(ctx, page, options.IncludeReactionCounts, &userID, func(p repo.PublicUploadPageParams) ([]repo.PublicUploadPageRow, error) {
		return s.repo.ListPublicUploadsByKeywordPage(ctx, keywordID, p)
	})
}

func (s *Service) loadPublicUploadPage(ctx context.Context, page publicUploadPage, counts bool, viewer *uuid.UUID, read func(repo.PublicUploadPageParams) ([]repo.PublicUploadPageRow, error)) (*PublicUploadListOutput, error) {
	if page.query.CutoffAt.IsZero() {
		cutoff, err := s.repo.PublicUploadsCutoff(ctx)
		if err != nil {
			return nil, err
		}
		page.query.CutoffAt = cutoff
	}
	rows, err := read(page.query)
	if err != nil {
		return nil, err
	}
	cards, next, more, err := page.finish(rows)
	if err != nil {
		return nil, err
	}
	var seed *float64
	if page.query.Sort == sortRandom {
		seed = &page.query.Seed
	}
	out, err := s.publicUploadList(ctx, cards, counts, viewer, seed)
	if err != nil {
		return nil, err
	}
	out.NextCursor, out.HasMore = next, more
	return out, nil
}

func (s *Service) publicUploadList(ctx context.Context, rows []repo.UploadCard, includeReactionCounts bool, viewerID *uuid.UUID, seed *float64) (*PublicUploadListOutput, error) {
	var (
		reactionMap map[uuid.UUID][]string
		err         error
	)
	if viewerID != nil && len(rows) > 0 {
		reactionMap, err = s.myReactionsByUploadIDs(ctx, *viewerID, uploadIDs(rows))
		if err != nil {
			return nil, err
		}
	}

	items := make([]PublicUploadCardOutput, 0, len(rows))
	for _, row := range rows {
		items = append(items, s.publicCardOutput(row, includeReactionCounts, reactionMap[row.ID]))
	}
	return &PublicUploadListOutput{
		Items: items,
		Seed:  seed,
	}, nil
}

func (s *Service) myReactionsByUploadIDs(ctx context.Context, userID uuid.UUID, uploadIDs []uuid.UUID) (map[uuid.UUID][]string, error) {
	rows, err := s.repo.ListMyReactionTypesByUploadIDs(ctx, userID, uploadIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID][]string, len(uploadIDs))
	for _, row := range rows {
		out[row.UploadID] = append(out[row.UploadID], row.Type)
	}
	return out, nil
}

func (s *Service) publicCardOutput(row repo.UploadCard, includeReactionCounts bool, myReactions []string) PublicUploadCardOutput {
	out := PublicUploadCardOutput{
		ID:        row.ID,
		BizDate:   row.BizDate,
		KeywordID: row.KeywordID,
		CoverImage: CardImageRefOutput{
			ID: row.CoverImageID,
		},
		DisplayText:        row.DisplayText,
		CoverHasAudio:      boolPtr(row.CoverHasAudio),
		CoverAudioDuration: row.CoverAudioDurationMs,
		ImageCount:         row.ImageCount,
		MyReactions:        copyStrings(myReactions),
		CreatedAt:          row.CreatedAt,
	}
	if variant := s.resolver.ResolveCard4x3Variant(row.CoverObjectKey); variant != nil {
		out.CoverImage.Card4x3 = ImageVariantOutput{
			URL:    variant.URL,
			Width:  variant.Width,
			Height: variant.Height,
		}
	}
	if includeReactionCounts {
		out.ReactionCounts = &ReactionCounts{
			Inspired:  row.ReactionInspiredCount,
			Resonated: row.ReactionResonatedCount,
		}
	}
	return out
}

func (s *Service) reviewCardOutput(row repo.UploadCard) ReviewUploadCardOutput {
	out := ReviewUploadCardOutput{
		ID:        row.ID,
		BizDate:   row.BizDate,
		KeywordID: row.KeywordID,
		CoverImage: CardImageRefOutput{
			ID: row.CoverImageID,
		},
		DisplayText:        row.DisplayText,
		CoverHasAudio:      boolPtr(row.CoverHasAudio),
		CoverAudioDuration: row.CoverAudioDurationMs,
		ImageCount:         row.ImageCount,
		CreatedAt:          row.CreatedAt,
	}
	if variant := s.resolver.ResolveCard4x3Variant(row.CoverObjectKey); variant != nil {
		out.CoverImage.Card4x3 = ImageVariantOutput{
			URL:    variant.URL,
			Width:  variant.Width,
			Height: variant.Height,
		}
	}
	return out
}

func (s *Service) workImagesOutput(rows []repo.UploadImage) []WorkImageOutput {
	out := make([]WorkImageOutput, 0, len(rows))
	for _, row := range rows {
		var detail ImageVariantOutput
		if variant := s.resolver.ResolveDetailLargeVariant(row.OriginalObjectKey, row.OriginalWidth, row.OriginalHeight); variant != nil {
			detail = ImageVariantOutput{
				URL:    variant.URL,
				Width:  variant.Width,
				Height: variant.Height,
			}
		}
		out = append(out, WorkImageOutput{
			ID: row.ID,
			Image: DetailLargeImageRefOutput{
				ID:          row.ImageAssetID,
				DetailLarge: detail,
			},
			DisplayOrder:    row.DisplayOrder,
			Title:           row.Title,
			Note:            row.Note,
			HasAudio:        row.HasAudio,
			AudioDurationMs: row.AudioDurationMs,
			AudioPlayURL:    resolveURL(s.resolver, row.AudioObjectKey),
			CreatedAt:       row.CreatedAt,
		})
	}
	return out
}

func uploadIDs(rows []repo.UploadCard) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.ID)
	}
	return out
}

func clampLimit(v, defaultValue, maxValue int) int32 {
	if v <= 0 {
		return int32(defaultValue)
	}
	if v > maxValue {
		return int32(maxValue)
	}
	return int32(v)
}

func normalizeSeed(now func() time.Time, seed *float64) float64 {
	if seed != nil {
		return *seed
	}
	if now == nil {
		now = time.Now
	}
	n := now().UnixNano()
	if n < 0 {
		n = -n
	}
	return float64(n%1_000_000) / 1_000_000.0
}

func boolPtr(v bool) *bool {
	b := v
	return &b
}

func copyStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
