package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	profiledb "cixing/internal/modules/profile/infra/db/gen"
	"cixing/internal/shared/common"
)

type Repository struct {
	q profiledb.Querier
}

type SettingsRow struct {
	Nickname                    string
	Email                       *string
	AvatarObjectKey             *string
	ReactionNotificationEnabled bool
	CreationReminderEnabled     bool
}

func NewRepository(q profiledb.Querier) *Repository {
	return &Repository{q: q}
}

func (r *Repository) GetSettings(ctx context.Context, userID uuid.UUID) (SettingsRow, error) {
	row, err := r.q.GetProfileSettings(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SettingsRow{}, common.ErrNotFound
		}
		return SettingsRow{}, err
	}
	return SettingsRow{
		Nickname:                    row.Nickname,
		Email:                       textPtr(row.Email),
		AvatarObjectKey:             textPtr(row.AvatarObjectKey),
		ReactionNotificationEnabled: row.ReactionNotificationEnabled,
		CreationReminderEnabled:     row.CreationReminderEnabled,
	}, nil
}

func (r *Repository) UpdateNotificationSettings(ctx context.Context, userID uuid.UUID, reactionEnabled, creationReminderEnabled *bool) (SettingsRow, error) {
	affected, err := r.q.UpdateUserNotificationSettings(ctx, profiledb.UpdateUserNotificationSettingsParams{
		UserID:                      userID,
		ReactionNotificationEnabled: optionalBool(reactionEnabled),
		CreationReminderEnabled:     optionalBool(creationReminderEnabled),
	})
	if err != nil {
		return SettingsRow{}, err
	}
	if affected == 0 {
		return SettingsRow{}, common.ErrNotFound
	}
	return r.GetSettings(ctx, userID)
}

func (r *Repository) UpdateNickname(ctx context.Context, userID uuid.UUID, nickname string) (SettingsRow, error) {
	affected, err := r.q.UpdateUserNickname(ctx, profiledb.UpdateUserNicknameParams{
		UserID:   userID,
		Nickname: nickname,
	})
	if err != nil {
		return SettingsRow{}, err
	}
	if affected == 0 {
		return SettingsRow{}, common.ErrNotFound
	}
	return r.GetSettings(ctx, userID)
}

func textPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func optionalBool(v *bool) pgtype.Bool {
	if v == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *v, Valid: true}
}
