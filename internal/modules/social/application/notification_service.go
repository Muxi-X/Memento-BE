package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	dsocial "cixing/internal/modules/social/domain"
	socialdb "cixing/internal/modules/social/infra/db/gen"
	"cixing/internal/modules/social/infra/db/repo"
	platformoss "cixing/internal/platform/oss"
)

const maxNotificationListLimit = 200

type NotificationService struct {
	db       *pgxpool.Pool
	resolver *platformoss.URLResolver
	now      func() time.Time
}

type NotificationImageOutput struct {
	URL    string
	Width  int32
	Height int32
}

type NotificationCoverOutput struct {
	SquareSmall NotificationImageOutput
}

type NotificationOutput struct {
	ID             uuid.UUID
	Type           dsocial.NotificationType
	ReactionType   *dsocial.ReactionType
	UploadID       uuid.UUID
	ActorAvatarURL *string
	CoverImage     *NotificationCoverOutput
	CreatedAt      time.Time
	ReadAt         *time.Time
}

type ListNotificationsOutput struct {
	Items []NotificationOutput
}

func NewNotificationService(db *pgxpool.Pool, resolver *platformoss.URLResolver, now func() time.Time) *NotificationService {
	if now == nil {
		now = time.Now
	}
	return &NotificationService{db: db, resolver: resolver, now: now}
}

func (s *NotificationService) List(ctx context.Context, recipientUserID uuid.UUID) (*ListNotificationsOutput, error) {
	rows, err := repo.NewRepository(socialdb.New(s.db)).ListNotifications(ctx, recipientUserID, maxNotificationListLimit)
	if err != nil {
		return nil, err
	}
	items := make([]NotificationOutput, 0, len(rows))
	for _, row := range rows {
		item := NotificationOutput{
			ID:             row.ID,
			Type:           row.Type,
			ReactionType:   row.ReactionType,
			UploadID:       row.TargetUploadID,
			ActorAvatarURL: resolveNotificationAvatarURL(s.resolver, row.ActorAvatarObjectKey),
			CreatedAt:      row.CreatedAt,
			ReadAt:         row.ReadAt,
		}
		if row.TargetCoverObjectKey != nil {
			variant := s.resolver.ResolveSquareSmallVariant(*row.TargetCoverObjectKey)
			if variant != nil && variant.URL != "" {
				item.CoverImage = &NotificationCoverOutput{
					SquareSmall: NotificationImageOutput{
						URL:    variant.URL,
						Width:  variant.Width,
						Height: variant.Height,
					},
				}
			}
		}
		items = append(items, item)
	}
	return &ListNotificationsOutput{Items: items}, nil
}

func (s *NotificationService) MarkAllRead(ctx context.Context, recipientUserID uuid.UUID) error {
	return repo.NewRepository(socialdb.New(s.db)).MarkAllNotificationsRead(ctx, recipientUserID, s.now().UTC())
}

func resolveNotificationAvatarURL(resolver *platformoss.URLResolver, objectKey *string) *string {
	if resolver == nil || objectKey == nil || *objectKey == "" {
		return nil
	}
	url := resolver.ResolveSquareSmallObjectKey(*objectKey)
	if url == "" {
		return nil
	}
	return &url
}
