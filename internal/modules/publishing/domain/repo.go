package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type DailyKeywordAssignment struct {
	BizDate   time.Time
	KeywordID uuid.UUID
}

type MediaAssetLite struct {
	ID          uuid.UUID
	OwnerUserID uuid.UUID
	Kind        MediaKind
	Status      MediaAssetStatus
	DurationMS  *int32
}

type CreateSessionParams struct {
	OwnerUserID       uuid.UUID
	ContextType       PublishContextType
	OfficialKeywordID *uuid.UUID
	CustomKeywordID   *uuid.UUID
	BizDate           *time.Time
	Status            SessionStatus
	ExpiresAt         time.Time
}

type UpdateSessionParams struct {
	ID              uuid.UUID
	OwnerUserID     uuid.UUID
	Status          SessionStatus
	PublishedUpload *uuid.UUID
}

type UpsertSessionItemParams struct {
	SessionID     uuid.UUID
	OwnerUserID   uuid.UUID
	ClientImageID string
	ImageAssetID  uuid.UUID
	AudioAssetID  *uuid.UUID
	DisplayOrder  *int32
	IsCover       bool
	Title         *string
	Note          *string
	Status        ItemStatus
}

type CreateWorkUploadParams struct {
	AuthorUserID      uuid.UUID
	ContextType       PublishContextType
	OfficialKeywordID *uuid.UUID
	CustomKeywordID   *uuid.UUID
	BizDate           *time.Time
	VisibilityStatus  WorkUploadVisibilityStatus
	CoverAssetID      uuid.UUID
	ImageCount        int32
	PublishedAt       time.Time
}

type CreateWorkUploadImageParams struct {
	UploadID     uuid.UUID
	ImageAssetID uuid.UUID
	DisplayOrder int32
}

type UpsertWorkUploadImageContentParams struct {
	WorkUploadImageID uuid.UUID
	Title             *string
	Note              *string
	AudioAssetID      *uuid.UUID
	AudioDurationMS   *int32
}

type Repository interface {
	CreateSession(ctx context.Context, params CreateSessionParams) (PublishSession, error)
	GetAggregateForUpdate(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID) (Aggregate, error)
	UpdateSession(ctx context.Context, params UpdateSessionParams) (PublishSession, error)
	UpsertSessionItem(ctx context.Context, params UpsertSessionItemParams) (PublishSessionItem, error)
	ClearSessionItemCover(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID) error
	GetDailyKeywordAssignment(ctx context.Context, bizDate time.Time) (DailyKeywordAssignment, error)
	GetMediaAssetLite(ctx context.Context, assetID uuid.UUID) (MediaAssetLite, error)
	CreateWorkUpload(ctx context.Context, params CreateWorkUploadParams) (WorkUpload, error)
	CreateWorkUploadImage(ctx context.Context, params CreateWorkUploadImageParams) (WorkUploadImage, error)
	UpsertWorkUploadImageContent(ctx context.Context, params UpsertWorkUploadImageContentParams) (WorkUploadImageContent, error)
}
