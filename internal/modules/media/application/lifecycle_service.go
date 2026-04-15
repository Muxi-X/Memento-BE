package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	dmedia "cixing/internal/modules/media/domain"
)

type LifecycleService struct {
	repo dmedia.Repository
	now  func() time.Time
}

func NewLifecycleService(repo dmedia.Repository, now func() time.Time) (*LifecycleService, error) {
	if repo == nil {
		return nil, errors.New("media: repo is required")
	}
	if now == nil {
		now = time.Now
	}
	return &LifecycleService{
		repo: repo,
		now:  now,
	}, nil
}

type CreatePendingAssetInput struct {
	OwnerUserID       uuid.UUID
	Kind              dmedia.Kind
	MimeType          string
	OriginalObjectKey string
	ByteSize          int64
}

type MarkAssetUploadedInput struct {
	AssetID    uuid.UUID
	Width      *int32
	Height     *int32
	DurationMS *int32
}

type MarkVariantReadyInput struct {
	AssetID uuid.UUID
	Name    dmedia.VariantName
	Width   *int32
	Height  *int32
}

func (s *LifecycleService) CreatePendingAsset(ctx context.Context, in CreatePendingAssetInput) (dmedia.Asset, error) {
	if err := validateCreatePendingAssetInput(in); err != nil {
		return dmedia.Asset{}, err
	}

	return s.repo.CreateAsset(ctx, dmedia.CreateAssetParams{
		OwnerUserID:       in.OwnerUserID,
		Kind:              in.Kind,
		MimeType:          strings.TrimSpace(in.MimeType),
		OriginalObjectKey: strings.TrimSpace(in.OriginalObjectKey),
		ByteSize:          in.ByteSize,
		Status:            dmedia.AssetStatusPendingUpload,
	})
}

func (s *LifecycleService) MarkAssetUploaded(ctx context.Context, in MarkAssetUploadedInput) (dmedia.Asset, error) {
	asset, err := s.repo.GetAssetByIDForUpdate(ctx, in.AssetID)
	if err != nil {
		return dmedia.Asset{}, err
	}
	if err := asset.MarkUploaded(in.Width, in.Height, in.DurationMS); err != nil {
		return dmedia.Asset{}, err
	}
	return s.repo.TransitionAssetStatus(ctx, dmedia.TransitionAssetStatusParams{
		ID:            asset.ID,
		CurrentStatus: dmedia.AssetStatusPendingUpload,
		NextStatus:    asset.Status,
		Width:         asset.Width,
		Height:        asset.Height,
		DurationMS:    asset.DurationMS,
	})
}

func (s *LifecycleService) StartAssetProcessing(ctx context.Context, assetID uuid.UUID) (dmedia.Asset, error) {
	asset, err := s.repo.GetAssetByIDForUpdate(ctx, assetID)
	if err != nil {
		return dmedia.Asset{}, err
	}
	if err := asset.StartProcessing(); err != nil {
		return dmedia.Asset{}, err
	}
	return s.repo.TransitionAssetStatus(ctx, dmedia.TransitionAssetStatusParams{
		ID:            asset.ID,
		CurrentStatus: dmedia.AssetStatusUploaded,
		NextStatus:    asset.Status,
		Width:         asset.Width,
		Height:        asset.Height,
		DurationMS:    asset.DurationMS,
	})
}

func (s *LifecycleService) MarkAssetReady(ctx context.Context, assetID uuid.UUID) (dmedia.Asset, error) {
	asset, err := s.repo.GetAssetByIDForUpdate(ctx, assetID)
	if err != nil {
		return dmedia.Asset{}, err
	}
	if err := asset.MarkReady(); err != nil {
		return dmedia.Asset{}, err
	}
	return s.repo.TransitionAssetStatus(ctx, dmedia.TransitionAssetStatusParams{
		ID:            asset.ID,
		CurrentStatus: dmedia.AssetStatusProcessing,
		NextStatus:    asset.Status,
		Width:         asset.Width,
		Height:        asset.Height,
		DurationMS:    asset.DurationMS,
	})
}

func (s *LifecycleService) MarkAssetFailed(ctx context.Context, assetID uuid.UUID) (dmedia.Asset, error) {
	asset, err := s.repo.GetAssetByIDForUpdate(ctx, assetID)
	if err != nil {
		return dmedia.Asset{}, err
	}
	if err := asset.MarkFailed(); err != nil {
		return dmedia.Asset{}, err
	}
	return s.repo.TransitionAssetStatus(ctx, dmedia.TransitionAssetStatusParams{
		ID:            asset.ID,
		CurrentStatus: dmedia.AssetStatusProcessing,
		NextStatus:    asset.Status,
		Width:         asset.Width,
		Height:        asset.Height,
		DurationMS:    asset.DurationMS,
	})
}

func (s *LifecycleService) SoftDeleteAsset(ctx context.Context, assetID uuid.UUID) (dmedia.Asset, error) {
	asset, err := s.repo.GetAssetByIDForUpdate(ctx, assetID)
	if err != nil {
		return dmedia.Asset{}, err
	}
	if err := asset.SoftDelete(s.now().UTC()); err != nil {
		return dmedia.Asset{}, err
	}
	return s.repo.SoftDeleteAsset(ctx, asset.ID, *asset.DeletedAt)
}

func (s *LifecycleService) StartVariantProcessing(ctx context.Context, assetID uuid.UUID, name dmedia.VariantName) (dmedia.Variant, error) {
	variant, err := s.repo.GetVariantForUpdate(ctx, assetID, name)
	if err != nil {
		return dmedia.Variant{}, err
	}
	if err := variant.StartProcessing(); err != nil {
		return dmedia.Variant{}, err
	}
	return s.repo.TransitionVariantStatus(ctx, dmedia.TransitionVariantStatusParams{
		AssetID:       assetID,
		Name:          name,
		CurrentStatus: dmedia.VariantStatusPending,
		NextStatus:    variant.Status,
		Width:         variant.Width,
		Height:        variant.Height,
	})
}

func (s *LifecycleService) MarkVariantReady(ctx context.Context, in MarkVariantReadyInput) (dmedia.Variant, error) {
	variant, err := s.repo.GetVariantForUpdate(ctx, in.AssetID, in.Name)
	if err != nil {
		return dmedia.Variant{}, err
	}
	if err := variant.MarkReady(in.Width, in.Height); err != nil {
		return dmedia.Variant{}, err
	}
	return s.repo.TransitionVariantStatus(ctx, dmedia.TransitionVariantStatusParams{
		AssetID:       in.AssetID,
		Name:          in.Name,
		CurrentStatus: dmedia.VariantStatusProcessing,
		NextStatus:    variant.Status,
		Width:         variant.Width,
		Height:        variant.Height,
	})
}

func (s *LifecycleService) MarkVariantFailed(ctx context.Context, assetID uuid.UUID, name dmedia.VariantName) (dmedia.Variant, error) {
	variant, err := s.repo.GetVariantForUpdate(ctx, assetID, name)
	if err != nil {
		return dmedia.Variant{}, err
	}
	if err := variant.MarkFailed(); err != nil {
		return dmedia.Variant{}, err
	}
	return s.repo.TransitionVariantStatus(ctx, dmedia.TransitionVariantStatusParams{
		AssetID:       assetID,
		Name:          name,
		CurrentStatus: dmedia.VariantStatusProcessing,
		NextStatus:    variant.Status,
		Width:         variant.Width,
		Height:        variant.Height,
	})
}

func validateCreatePendingAssetInput(in CreatePendingAssetInput) error {
	if in.OwnerUserID == uuid.Nil {
		return dmedia.ErrInvalidMediaMetadata
	}
	if in.Kind != dmedia.KindImage && in.Kind != dmedia.KindAudio {
		return dmedia.ErrInvalidMediaMetadata
	}
	if strings.TrimSpace(in.MimeType) == "" || strings.TrimSpace(in.OriginalObjectKey) == "" {
		return dmedia.ErrInvalidMediaMetadata
	}
	if in.ByteSize <= 0 {
		return dmedia.ErrInvalidMediaMetadata
	}
	return nil
}
