package domain

import (
	"sort"
	"time"

	"github.com/google/uuid"

	"cixing/internal/shared/common"
)

type PublishSession struct {
	ID                uuid.UUID
	OwnerUserID       uuid.UUID
	ContextType       PublishContextType
	OfficialKeywordID *uuid.UUID
	CustomKeywordID   *uuid.UUID
	BizDate           *time.Time
	Status            SessionStatus
	ExpiresAt         time.Time
	PublishedUploadID *uuid.UUID
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type PublishSessionItem struct {
	ID            uuid.UUID
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
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Aggregate struct {
	Session PublishSession
	Items   []PublishSessionItem
}

func NewOfficialSession(ownerUserID uuid.UUID, keywordID uuid.UUID, bizDate time.Time, now time.Time, expiresAt time.Time) PublishSession {
	bizDate = dateOnly(bizDate)
	return PublishSession{
		OwnerUserID:       ownerUserID,
		ContextType:       PublishContextOfficialToday,
		OfficialKeywordID: &keywordID,
		BizDate:           &bizDate,
		Status:            SessionStatusCreated,
		ExpiresAt:         expiresAt,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func NewCustomSession(ownerUserID uuid.UUID, keywordID uuid.UUID, now time.Time, expiresAt time.Time) PublishSession {
	return PublishSession{
		OwnerUserID:     ownerUserID,
		ContextType:     PublishContextCustomKeyword,
		CustomKeywordID: &keywordID,
		Status:          SessionStatusCreated,
		ExpiresAt:       expiresAt,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func (a *Aggregate) EnsureMutable(now time.Time) error {
	if !a.Session.ExpiresAt.After(now) {
		return ErrExpired
	}
	switch a.Session.Status {
	case SessionStatusCommitted, SessionStatusCanceled, SessionStatusExpired:
		return ErrInvalidState
	default:
		return nil
	}
}

func (a *Aggregate) BeginOfficialCommit(now time.Time, expectedKeywordID uuid.UUID) ([]PublishSessionItem, PublishSessionItem, error) {
	if err := a.EnsureMutable(now); err != nil {
		return nil, PublishSessionItem{}, err
	}
	if a.Session.Status != SessionStatusCreated {
		return nil, PublishSessionItem{}, ErrInvalidState
	}
	if a.Session.ContextType != PublishContextOfficialToday || a.Session.OfficialKeywordID == nil || a.Session.BizDate == nil {
		return nil, PublishSessionItem{}, ErrInvalidContext
	}
	if *a.Session.OfficialKeywordID != expectedKeywordID {
		return nil, PublishSessionItem{}, ErrKeywordMismatch
	}
	return a.commitItems()
}

func (a *Aggregate) BeginCustomCommit(now time.Time, expectedKeywordID uuid.UUID) ([]PublishSessionItem, PublishSessionItem, error) {
	if err := a.EnsureMutable(now); err != nil {
		return nil, PublishSessionItem{}, err
	}
	if a.Session.Status != SessionStatusCreated {
		return nil, PublishSessionItem{}, ErrInvalidState
	}
	if a.Session.ContextType != PublishContextCustomKeyword || a.Session.CustomKeywordID == nil {
		return nil, PublishSessionItem{}, ErrInvalidContext
	}
	if *a.Session.CustomKeywordID != expectedKeywordID {
		return nil, PublishSessionItem{}, ErrKeywordMismatch
	}
	return a.commitItems()
}

func (a *Aggregate) commitItems() ([]PublishSessionItem, PublishSessionItem, error) {
	if len(a.Items) == 0 {
		return nil, PublishSessionItem{}, common.ErrConflict
	}

	ordered := make([]PublishSessionItem, 0, len(a.Items))
	seenOrders := make(map[int32]struct{}, len(a.Items))
	coverCount := 0
	var cover PublishSessionItem

	for _, item := range a.Items {
		if item.Status != ItemStatusUploaded || item.DisplayOrder == nil || *item.DisplayOrder <= 0 {
			return nil, PublishSessionItem{}, common.ErrConflict
		}
		if _, exists := seenOrders[*item.DisplayOrder]; exists {
			return nil, PublishSessionItem{}, common.ErrConflict
		}
		seenOrders[*item.DisplayOrder] = struct{}{}
		if item.IsCover {
			coverCount++
			cover = item
		}
		ordered = append(ordered, item)
	}
	if coverCount != 1 {
		return nil, PublishSessionItem{}, common.ErrConflict
	}
	sort.Slice(ordered, func(i, j int) bool {
		return *ordered[i].DisplayOrder < *ordered[j].DisplayOrder
	})
	for i := range ordered {
		if *ordered[i].DisplayOrder != int32(i+1) {
			return nil, PublishSessionItem{}, common.ErrConflict
		}
	}
	return ordered, cover, nil
}

func (a *Aggregate) MarkCommitted(uploadID uuid.UUID) error {
	if a.Session.Status != SessionStatusCreated {
		return ErrInvalidState
	}
	a.Session.Status = SessionStatusCommitted
	a.Session.PublishedUploadID = &uploadID
	return nil
}

func dateOnly(t time.Time) time.Time {
	return common.NormalizeBizDate(t)
}

func int32Ptr(v int32) *int32 {
	n := v
	return &n
}
