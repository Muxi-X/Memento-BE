package domain

import (
	"time"

	"github.com/google/uuid"
)

type WorkUpload struct {
	ID                     uuid.UUID
	AuthorUserID           uuid.UUID
	ContextType            PublishContextType
	OfficialKeywordID      *uuid.UUID
	CustomKeywordID        *uuid.UUID
	BizDate                *time.Time
	VisibilityStatus       WorkUploadVisibilityStatus
	CoverAssetID           uuid.UUID
	ImageCount             int32
	ReactionInspiredCount  int32
	ReactionResonatedCount int32
	RandKey                float64
	PublishedAt            time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
	DeletedAt              *time.Time
}

type WorkUploadImage struct {
	ID           uuid.UUID
	UploadID     uuid.UUID
	ImageAssetID uuid.UUID
	DisplayOrder int32
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

type WorkUploadImageContent struct {
	WorkUploadImageID uuid.UUID
	Title             *string
	Note              *string
	AudioAssetID      *uuid.UUID
	AudioDurationMS   *int32
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func NewProcessingWorkUpload(authorUserID uuid.UUID, keywordID uuid.UUID, bizDate time.Time, coverAssetID uuid.UUID, imageCount int32, publishedAt time.Time) WorkUpload {
	bizDate = dateOnly(bizDate)
	return WorkUpload{
		AuthorUserID:      authorUserID,
		ContextType:       PublishContextOfficialToday,
		OfficialKeywordID: &keywordID,
		BizDate:           &bizDate,
		VisibilityStatus:  WorkUploadVisibilityProcessing,
		CoverAssetID:      coverAssetID,
		ImageCount:        imageCount,
		PublishedAt:       publishedAt,
		CreatedAt:         publishedAt,
		UpdatedAt:         publishedAt,
	}
}
