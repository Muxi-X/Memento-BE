package media

import (
	"time"

	"github.com/google/uuid"
)

type Variant struct {
	ID        uuid.UUID
	AssetID   uuid.UUID
	Name      VariantName
	ObjectKey string
	Width     *int32
	Height    *int32
	Status    VariantStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (v *Variant) StartProcessing() error {
	if v.Status != VariantStatusPending {
		return ErrInvalidVariantStatusTransition
	}
	v.Status = VariantStatusProcessing
	return nil
}

func (v *Variant) MarkReady(width, height *int32) error {
	if v.Status != VariantStatusProcessing {
		return ErrInvalidVariantStatusTransition
	}
	if err := validateMediaDimensions(width, height); err != nil {
		return err
	}
	v.Width = width
	v.Height = height
	v.Status = VariantStatusReady
	return nil
}

func (v *Variant) MarkFailed() error {
	if v.Status != VariantStatusProcessing {
		return ErrInvalidVariantStatusTransition
	}
	v.Status = VariantStatusFailed
	return nil
}
