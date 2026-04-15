package media

import (
	"time"

	"github.com/google/uuid"
)

type Asset struct {
	ID                uuid.UUID
	OwnerUserID       uuid.UUID
	Kind              Kind
	MimeType          string
	OriginalObjectKey string
	ByteSize          int64
	Width             *int32
	Height            *int32
	DurationMS        *int32
	Status            AssetStatus
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

func (a *Asset) MarkUploaded(width, height, durationMS *int32) error {
	if a.Status != AssetStatusPendingUpload {
		return ErrInvalidAssetStatusTransition
	}
	if err := validateMediaDimensions(width, height); err != nil {
		return err
	}
	if err := validateDuration(durationMS); err != nil {
		return err
	}
	a.Width = width
	a.Height = height
	a.DurationMS = durationMS
	a.Status = AssetStatusUploaded
	return nil
}

func (a *Asset) StartProcessing() error {
	if a.Status != AssetStatusUploaded {
		return ErrInvalidAssetStatusTransition
	}
	a.Status = AssetStatusProcessing
	return nil
}

func (a *Asset) MarkReady() error {
	if a.Status != AssetStatusProcessing {
		return ErrInvalidAssetStatusTransition
	}
	a.Status = AssetStatusReady
	return nil
}

func (a *Asset) MarkFailed() error {
	if a.Status != AssetStatusProcessing {
		return ErrInvalidAssetStatusTransition
	}
	a.Status = AssetStatusFailed
	return nil
}

func (a *Asset) SoftDelete(now time.Time) error {
	if a.Status == AssetStatusDeleted {
		return nil
	}
	a.Status = AssetStatusDeleted
	a.DeletedAt = &now
	return nil
}

func validateMediaDimensions(width, height *int32) error {
	if width != nil && *width <= 0 {
		return ErrInvalidMediaMetadata
	}
	if height != nil && *height <= 0 {
		return ErrInvalidMediaMetadata
	}
	return nil
}

func validateDuration(durationMS *int32) error {
	if durationMS != nil && *durationMS < 0 {
		return ErrInvalidMediaMetadata
	}
	return nil
}
