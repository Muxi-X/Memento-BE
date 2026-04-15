package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	dcustom "cixing/internal/modules/customkeywords/domain"
	"cixing/internal/modules/customkeywords/infra/db/repo"
	platformoss "cixing/internal/platform/oss"
	"cixing/internal/shared/common"
)

type Service struct {
	repo     *repo.Repository
	resolver *platformoss.URLResolver
}

func NewService(repo *repo.Repository, resolver *platformoss.URLResolver) *Service {
	return &Service{repo: repo, resolver: resolver}
}

type ImageVariantOutput struct {
	URL    string
	Width  int32
	Height int32
}

type SquareSmallImageRefOutput struct {
	ID          uuid.UUID
	SquareSmall ImageVariantOutput
}

type SquareMediumImageRefOutput struct {
	ID           uuid.UUID
	SquareMedium ImageVariantOutput
}

type DetailLargeImageRefOutput struct {
	ID          uuid.UUID
	DetailLarge ImageVariantOutput
}

type KeywordItemOutput struct {
	ID               uuid.UUID
	Text             string
	TargetImageCount *int32
	IsActive         bool
	TotalImageCount  int32
	MyImageCount     int32
	CoverImage       *SquareSmallImageRefOutput
	CreatedAt        time.Time
}

type MeHomeOutput struct {
	TotalImageCount int32
	Items           []KeywordItemOutput
}

type ImageCardOutput struct {
	ID           uuid.UUID
	Image        SquareMediumImageRefOutput
	DisplayOrder int32
	CreatedAt    time.Time
}

type GalleryOutput struct {
	CoverSource dcustom.CoverSource
	CoverImage  *DetailLargeImageRefOutput
	Items       []ImageCardOutput
}

type ImageDetailOutput struct {
	ID              uuid.UUID
	CustomKeywordID uuid.UUID
	Image           DetailLargeImageRefOutput
	DisplayOrder    int32
	Title           *string
	Note            *string
	HasAudio        bool
	AudioDurationMs *int32
	AudioPlayURL    *string
	CreatedAt       time.Time
}

type SetCoverOutput struct {
	KeywordID   uuid.UUID
	CoverSource dcustom.CoverSource
	CoverImage  *DetailLargeImageRefOutput
}

type UpdateKeywordInput struct {
	Text             *string
	TargetImageCount *int
	IsActive         *bool
}

func (s *Service) EnsureOwnedActiveKeyword(ctx context.Context, ownerUserID, keywordID uuid.UUID) error {
	if ownerUserID == uuid.Nil || keywordID == uuid.Nil {
		return ErrInvalidInput
	}
	keyword, err := s.repo.GetKeywordByIDForUser(ctx, keywordID, ownerUserID)
	if err != nil {
		return err
	}
	if keyword.Status != dcustom.StatusActive {
		return common.ErrConflict
	}
	return nil
}

func (s *Service) Create(ctx context.Context, ownerUserID uuid.UUID, text string, targetImageCount *int) (*KeywordItemOutput, error) {
	if ownerUserID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, ErrInvalidKeywordText
	}
	target, err := normalizeTargetImageCount(targetImageCount)
	if err != nil {
		return nil, err
	}

	keyword, err := s.repo.CreateKeyword(ctx, ownerUserID, text, target)
	if err != nil {
		return nil, err
	}
	out, err := s.keywordItemOutput(ctx, keyword, 0)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Service) List(ctx context.Context, ownerUserID uuid.UUID) ([]KeywordItemOutput, error) {
	if ownerUserID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	rows, err := s.repo.ListKeywordSummaries(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	out := make([]KeywordItemOutput, 0, len(rows))
	for _, row := range rows {
		item, err := s.keywordItemOutput(ctx, row.Keyword, row.TotalImageCount)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Service) ListForMeHome(ctx context.Context, ownerUserID uuid.UUID) (*MeHomeOutput, error) {
	items, err := s.List(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	var total int32
	for _, item := range items {
		total += item.MyImageCount
	}
	return &MeHomeOutput{
		TotalImageCount: total,
		Items:           items,
	}, nil
}

func (s *Service) Update(ctx context.Context, ownerUserID, keywordID uuid.UUID, in UpdateKeywordInput) (*KeywordItemOutput, error) {
	if ownerUserID == uuid.Nil || keywordID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	var text *string
	if in.Text != nil {
		trimmed := strings.TrimSpace(*in.Text)
		if trimmed == "" {
			return nil, ErrInvalidKeywordText
		}
		text = &trimmed
	}
	target, err := normalizeTargetImageCount(in.TargetImageCount)
	if err != nil {
		return nil, err
	}
	status := (*dcustom.Status)(nil)
	if in.IsActive != nil {
		next := dcustom.StatusInactive
		if *in.IsActive {
			next = dcustom.StatusActive
		}
		status = &next
	}

	keyword, err := s.repo.UpdateKeyword(ctx, ownerUserID, keywordID, repo.UpdateKeywordInput{
		Text:             text,
		TargetImageCount: target,
		Status:           status,
	})
	if err != nil {
		return nil, err
	}
	count, err := s.repo.CountVisibleImagesForKeyword(ctx, ownerUserID, keywordID)
	if err != nil {
		return nil, err
	}
	out, err := s.keywordItemOutput(ctx, keyword, count)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Service) Delete(ctx context.Context, ownerUserID, keywordID uuid.UUID) error {
	if ownerUserID == uuid.Nil || keywordID == uuid.Nil {
		return ErrInvalidInput
	}
	return s.repo.DeleteKeyword(ctx, ownerUserID, keywordID)
}

func (s *Service) SetCover(ctx context.Context, ownerUserID, keywordID, imageID uuid.UUID) (*SetCoverOutput, error) {
	if ownerUserID == uuid.Nil || keywordID == uuid.Nil || imageID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	assetID, err := s.repo.ResolveImageAssetForKeywordImage(ctx, ownerUserID, keywordID, imageID)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.SetKeywordManualCover(ctx, ownerUserID, keywordID, assetID); err != nil {
		return nil, err
	}
	cover, err := s.resolveCover(ctx, ownerUserID, keywordID)
	if err != nil {
		return nil, err
	}
	return &SetCoverOutput{
		KeywordID:   keywordID,
		CoverSource: dcustom.CoverSourceManual,
		CoverImage:  cover,
	}, nil
}

func (s *Service) ClearCover(ctx context.Context, ownerUserID, keywordID uuid.UUID) (*SetCoverOutput, error) {
	if ownerUserID == uuid.Nil || keywordID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	if _, err := s.repo.ClearKeywordCover(ctx, ownerUserID, keywordID); err != nil {
		return nil, err
	}
	cover, err := s.resolveCover(ctx, ownerUserID, keywordID)
	if err != nil {
		return nil, err
	}
	return &SetCoverOutput{
		KeywordID:   keywordID,
		CoverSource: dcustom.CoverSourceAutoLatest,
		CoverImage:  cover,
	}, nil
}

func (s *Service) ListImages(ctx context.Context, ownerUserID, keywordID uuid.UUID, limit int) (*GalleryOutput, error) {
	if ownerUserID == uuid.Nil || keywordID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	keyword, err := s.repo.GetKeywordByIDForUser(ctx, keywordID, ownerUserID)
	if err != nil {
		return nil, err
	}
	cards, err := s.repo.ListKeywordImages(ctx, ownerUserID, keywordID, clampLimit(limit, 50, 200))
	if err != nil {
		return nil, err
	}
	items := make([]ImageCardOutput, 0, len(cards))
	for _, card := range cards {
		variant := s.resolveSquareMedium(card.OriginalObjectKey)
		if variant == nil {
			continue
		}
		items = append(items, ImageCardOutput{
			ID: card.ID,
			Image: SquareMediumImageRefOutput{
				ID: card.ImageAssetID,
				SquareMedium: ImageVariantOutput{
					URL:    variant.URL,
					Width:  variant.Width,
					Height: variant.Height,
				},
			},
			DisplayOrder: card.DisplayOrder,
			CreatedAt:    card.CreatedAt,
		})
	}
	cover, coverSource, err := s.resolveCoverWithSource(ctx, ownerUserID, keyword)
	if err != nil {
		return nil, err
	}
	return &GalleryOutput{
		CoverSource: coverSource,
		CoverImage:  cover,
		Items:       items,
	}, nil
}

func (s *Service) GetImage(ctx context.Context, ownerUserID, imageID uuid.UUID) (*ImageDetailOutput, error) {
	if ownerUserID == uuid.Nil || imageID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	row, err := s.repo.GetKeywordImageDetail(ctx, ownerUserID, imageID)
	if err != nil {
		return nil, err
	}
	variant := s.resolveDetailLarge(row.OriginalObjectKey, row.OriginalWidth, row.OriginalHeight)
	if variant == nil {
		return nil, common.ErrNotFound
	}
	return &ImageDetailOutput{
		ID:              row.ID,
		CustomKeywordID: row.CustomKeywordID,
		Image: DetailLargeImageRefOutput{
			ID: row.ImageAssetID,
			DetailLarge: ImageVariantOutput{
				URL:    variant.URL,
				Width:  variant.Width,
				Height: variant.Height,
			},
		},
		DisplayOrder:    row.DisplayOrder,
		Title:           row.Title,
		Note:            row.Note,
		HasAudio:        row.HasAudio,
		AudioDurationMs: row.AudioDurationMs,
		AudioPlayURL:    s.resolvePtr(row.AudioObjectKey),
		CreatedAt:       row.CreatedAt,
	}, nil
}

func (s *Service) keywordItemOutput(ctx context.Context, keyword repo.Keyword, imageCount int32) (KeywordItemOutput, error) {
	cover, err := s.resolveSquareSmallCover(ctx, keyword.OwnerUserID, keyword)
	if err != nil {
		return KeywordItemOutput{}, err
	}
	return KeywordItemOutput{
		ID:               keyword.ID,
		Text:             keyword.Text,
		TargetImageCount: keyword.TargetImageCount,
		IsActive:         keyword.Status == dcustom.StatusActive,
		TotalImageCount:  imageCount,
		MyImageCount:     imageCount,
		CoverImage:       cover,
		CreatedAt:        keyword.CreatedAt,
	}, nil
}

func (s *Service) resolveCover(ctx context.Context, ownerUserID, keywordID uuid.UUID) (*DetailLargeImageRefOutput, error) {
	keyword, err := s.repo.GetKeywordByIDForUser(ctx, keywordID, ownerUserID)
	if err != nil {
		return nil, err
	}
	cover, _, err := s.resolveCoverWithSource(ctx, ownerUserID, keyword)
	return cover, err
}

func (s *Service) resolveSquareSmallCover(ctx context.Context, ownerUserID uuid.UUID, keyword repo.Keyword) (*SquareSmallImageRefOutput, error) {
	asset, _, err := s.resolveCoverAsset(ctx, ownerUserID, keyword)
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, nil
	}
	variant := s.resolveSquareSmall(asset.OriginalObjectKey)
	if variant == nil {
		return nil, nil
	}
	return &SquareSmallImageRefOutput{
		ID: asset.ID,
		SquareSmall: ImageVariantOutput{
			URL:    variant.URL,
			Width:  variant.Width,
			Height: variant.Height,
		},
	}, nil
}

func (s *Service) resolveCoverAsset(ctx context.Context, ownerUserID uuid.UUID, keyword repo.Keyword) (*repo.CoverAsset, dcustom.CoverSource, error) {
	if keyword.CoverSource == dcustom.CoverSourceManual && keyword.CoverAssetID != nil {
		asset, err := s.repo.GetCoverAsset(ctx, *keyword.CoverAssetID)
		if err == nil {
			return asset, dcustom.CoverSourceManual, nil
		}
		if err != nil && !errors.Is(err, common.ErrNotFound) {
			return nil, "", err
		}
	}
	asset, err := s.repo.GetLatestCoverAssetForKeyword(ctx, ownerUserID, keyword.ID)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return nil, dcustom.CoverSourceAutoLatest, nil
		}
		return nil, "", err
	}
	return asset, dcustom.CoverSourceAutoLatest, nil
}

func (s *Service) resolveCoverWithSource(ctx context.Context, ownerUserID uuid.UUID, keyword repo.Keyword) (*DetailLargeImageRefOutput, dcustom.CoverSource, error) {
	asset, source, err := s.resolveCoverAsset(ctx, ownerUserID, keyword)
	if err != nil {
		return nil, "", err
	}
	if asset == nil {
		return nil, source, nil
	}
	variant := s.resolveDetailLarge(asset.OriginalObjectKey, asset.OriginalWidth, asset.OriginalHeight)
	if variant == nil {
		return nil, source, nil
	}
	return &DetailLargeImageRefOutput{
		ID: asset.ID,
		DetailLarge: ImageVariantOutput{
			URL:    variant.URL,
			Width:  variant.Width,
			Height: variant.Height,
		},
	}, source, nil
}

func normalizeTargetImageCount(v *int) (*int32, error) {
	if v == nil {
		return nil, nil
	}
	if *v < 1 {
		return nil, ErrInvalidTargetImageCount
	}
	n := int32(*v)
	return &n, nil
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

func int32Value(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

func (s *Service) resolve(key string) string {
	if s.resolver == nil || strings.TrimSpace(key) == "" {
		return ""
	}
	return s.resolver.ResolveObjectKey(key)
}

func (s *Service) resolveSquareSmall(key string) *platformoss.ResolvedImageVariant {
	if s.resolver == nil || strings.TrimSpace(key) == "" {
		return nil
	}
	return s.resolver.ResolveSquareSmallVariant(key)
}

func (s *Service) resolveSquareMedium(key string) *platformoss.ResolvedImageVariant {
	if s.resolver == nil || strings.TrimSpace(key) == "" {
		return nil
	}
	return s.resolver.ResolveSquareMediumVariant(key)
}

func (s *Service) resolveDetailLarge(key string, width, height *int32) *platformoss.ResolvedImageVariant {
	if s.resolver == nil || strings.TrimSpace(key) == "" {
		return nil
	}
	return s.resolver.ResolveDetailLargeVariant(key, width, height)
}

func (s *Service) resolvePtr(key *string) *string {
	if key == nil {
		return nil
	}
	url := s.resolve(*key)
	if url == "" {
		return nil
	}
	return &url
}
