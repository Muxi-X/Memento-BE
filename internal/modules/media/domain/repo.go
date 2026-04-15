package media

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	CreateAsset(ctx context.Context, params CreateAssetParams) (Asset, error)
	GetAssetByID(ctx context.Context, id uuid.UUID) (Asset, error)
	GetAssetByIDForUpdate(ctx context.Context, id uuid.UUID) (Asset, error)
	UpdateAsset(ctx context.Context, params UpdateAssetParams) (Asset, error)
	TransitionAssetStatus(ctx context.Context, params TransitionAssetStatusParams) (Asset, error)
	SoftDeleteAsset(ctx context.Context, id uuid.UUID, deletedAt time.Time) (Asset, error)

	UpsertVariant(ctx context.Context, params UpsertVariantParams) (Variant, error)
	GetVariant(ctx context.Context, assetID uuid.UUID, name VariantName) (Variant, error)
	GetVariantForUpdate(ctx context.Context, assetID uuid.UUID, name VariantName) (Variant, error)
	ListVariantsByAssetID(ctx context.Context, assetID uuid.UUID) ([]Variant, error)
	UpdateVariant(ctx context.Context, params UpdateVariantParams) (Variant, error)
	TransitionVariantStatus(ctx context.Context, params TransitionVariantStatusParams) (Variant, error)
	CountAssetLiveReferences(ctx context.Context, assetID uuid.UUID) (int64, error)
}

type CreateAssetParams struct {
	OwnerUserID       uuid.UUID
	Kind              Kind
	MimeType          string
	OriginalObjectKey string
	ByteSize          int64
	Width             *int32
	Height            *int32
	DurationMS        *int32
	Status            AssetStatus
}

type UpdateAssetParams struct {
	ID         uuid.UUID
	Status     AssetStatus
	Width      *int32
	Height     *int32
	DurationMS *int32
	DeletedAt  *time.Time
}

type TransitionAssetStatusParams struct {
	ID            uuid.UUID
	CurrentStatus AssetStatus
	NextStatus    AssetStatus
	Width         *int32
	Height        *int32
	DurationMS    *int32
}

type UpsertVariantParams struct {
	AssetID   uuid.UUID
	Name      VariantName
	ObjectKey string
	Width     *int32
	Height    *int32
	Status    VariantStatus
}

type UpdateVariantParams struct {
	AssetID   uuid.UUID
	Name      VariantName
	ObjectKey *string
	Width     *int32
	Height    *int32
	Status    VariantStatus
}

type TransitionVariantStatusParams struct {
	AssetID       uuid.UUID
	Name          VariantName
	CurrentStatus VariantStatus
	NextStatus    VariantStatus
	Width         *int32
	Height        *int32
}
