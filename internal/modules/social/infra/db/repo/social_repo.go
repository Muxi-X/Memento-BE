package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	dsocial "cixing/internal/modules/social/domain"
	socialdb "cixing/internal/modules/social/infra/db/gen"
	"cixing/internal/shared/common"
)

type Repository struct {
	q socialdb.Querier
}

func NewRepository(q socialdb.Querier) *Repository {
	return &Repository{q: q}
}

type ReactionTarget struct {
	UploadID                    uuid.UUID
	AuthorUserID                uuid.UUID
	ReactionNotificationEnabled bool
	TargetCoverObjectKey        *string
}

type ActorSnapshot struct {
	UserID          uuid.UUID
	Nickname        string
	AvatarObjectKey *string
}

type CreateNotificationInput struct {
	RecipientUserID              uuid.UUID
	ActorUserID                  uuid.UUID
	ActorNicknameSnapshot        string
	ActorAvatarObjectKeySnapshot *string
	TargetUploadID               uuid.UUID
	TargetUploadCoverObjectKey   *string
	Type                         dsocial.NotificationType
	ReactionType                 *dsocial.ReactionType
}

type NotificationRow struct {
	ID                   uuid.UUID
	ActorAvatarObjectKey *string
	TargetUploadID       uuid.UUID
	TargetCoverObjectKey *string
	Type                 dsocial.NotificationType
	ReactionType         *dsocial.ReactionType
	ReadAt               *time.Time
	CreatedAt            time.Time
}

func (r *Repository) GetVisibleReactionTarget(ctx context.Context, uploadID uuid.UUID) (ReactionTarget, error) {
	row, err := r.q.GetVisibleReactionTarget(ctx, uploadID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ReactionTarget{}, common.ErrNotFound
		}
		return ReactionTarget{}, err
	}
	return ReactionTarget{
		UploadID:                    row.UploadID,
		AuthorUserID:                row.AuthorUserID,
		ReactionNotificationEnabled: row.ReactionNotificationEnabled,
		TargetCoverObjectKey:        textPtr(row.TargetCoverObjectKey),
	}, nil
}

func (r *Repository) GetActorSnapshot(ctx context.Context, userID uuid.UUID) (ActorSnapshot, error) {
	row, err := r.q.GetActorSnapshot(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ActorSnapshot{}, common.ErrNotFound
		}
		return ActorSnapshot{}, err
	}
	return ActorSnapshot{
		UserID:          row.UserID,
		Nickname:        row.Nickname,
		AvatarObjectKey: textPtr(row.AvatarObjectKey),
	}, nil
}

func (r *Repository) InsertReaction(ctx context.Context, uploadID uuid.UUID, userID uuid.UUID, reactionType dsocial.ReactionType) (bool, error) {
	row, err := r.q.InsertReaction(ctx, socialdb.InsertReactionParams{
		UploadID: uploadID,
		UserID:   userID,
		Type:     reactionType.String(),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return row.UploadID != uuid.Nil, nil
}

func (r *Repository) DeleteReaction(ctx context.Context, uploadID uuid.UUID, userID uuid.UUID, reactionType dsocial.ReactionType) (bool, error) {
	row, err := r.q.DeleteReaction(ctx, socialdb.DeleteReactionParams{
		UploadID: uploadID,
		UserID:   userID,
		Type:     reactionType.String(),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return row.UploadID != uuid.Nil, nil
}

func (r *Repository) RecomputeReactionCounts(ctx context.Context, uploadID uuid.UUID) error {
	_, err := r.q.RecomputeReactionCounts(ctx, uploadID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return common.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) CreateNotification(ctx context.Context, in CreateNotificationInput) error {
	return r.q.CreateNotification(ctx, socialdb.CreateNotificationParams{
		RecipientUserID:                     in.RecipientUserID,
		ActorUserID:                         in.ActorUserID,
		ActorNicknameSnapshot:               in.ActorNicknameSnapshot,
		ActorAvatarVariantKeySnapshot:       textOrNull(in.ActorAvatarObjectKeySnapshot),
		TargetUploadID:                      in.TargetUploadID,
		TargetUploadCoverVariantKeySnapshot: textOrNull(in.TargetUploadCoverObjectKey),
		Type:                                in.Type.String(),
		ReactionType:                        reactionTypeOrNull(in.ReactionType),
	})
}

func (r *Repository) ListNotifications(ctx context.Context, recipientUserID uuid.UUID, limit int) ([]NotificationRow, error) {
	rows, err := r.q.ListNotifications(ctx, socialdb.ListNotificationsParams{
		RecipientUserID: recipientUserID,
		RowLimit:        int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]NotificationRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, NotificationRow{
			ID:                   row.ID,
			ActorAvatarObjectKey: textPtr(row.ActorAvatarObjectKey),
			TargetUploadID:       row.TargetUploadID,
			TargetCoverObjectKey: textPtr(row.TargetCoverObjectKey),
			Type:                 dsocial.NotificationType(row.Type),
			ReactionType:         nullableReactionType(row.ReactionType),
			ReadAt:               timestamptzPtr(row.ReadAt),
			CreatedAt:            row.CreatedAt.Time,
		})
	}
	return out, nil
}

func (r *Repository) MarkAllNotificationsRead(ctx context.Context, recipientUserID uuid.UUID, readAt time.Time) error {
	return r.q.MarkAllNotificationsRead(ctx, socialdb.MarkAllNotificationsReadParams{
		RecipientUserID: recipientUserID,
		ReadAt:          pgtype.Timestamptz{Time: readAt, Valid: true},
	})
}

func textPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func textOrNull(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}

func reactionTypeOrNull(v *dsocial.ReactionType) interface{} {
	if v == nil {
		return nil
	}
	return v.String()
}

func nullableReactionType(v interface{}) *dsocial.ReactionType {
	switch vv := v.(type) {
	case nil:
		return nil
	case string:
		if vv == "" {
			return nil
		}
		t := dsocial.ReactionType(vv)
		return &t
	case []byte:
		if len(vv) == 0 {
			return nil
		}
		t := dsocial.ReactionType(string(vv))
		return &t
	case pgtype.Text:
		if !vv.Valid || vv.String == "" {
			return nil
		}
		t := dsocial.ReactionType(vv.String)
		return &t
	default:
		return nil
	}
}

func timestamptzPtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}
