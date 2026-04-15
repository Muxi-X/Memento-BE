package application

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"cixing/internal/modules/profile/infra/db/repo"
	platformoss "cixing/internal/platform/oss"
)

type Service struct {
	repo     *repo.Repository
	resolver *platformoss.URLResolver
}

func NewService(repo *repo.Repository, resolver *platformoss.URLResolver) *Service {
	return &Service{repo: repo, resolver: resolver}
}

type ProfileOutput struct {
	Nickname  string
	AvatarURL *string
	Email     *string
}

type NotificationSettingsOutput struct {
	ReactionEnabled bool
}

type SettingsOutput struct {
	Profile       ProfileOutput
	Notifications NotificationSettingsOutput
}

func (s *Service) GetSettings(ctx context.Context, userID uuid.UUID) (*SettingsOutput, error) {
	if userID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	row, err := s.repo.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := s.settingsOutput(row)
	return &out, nil
}

func (s *Service) UpdateReactionNotifications(ctx context.Context, userID uuid.UUID, enabled *bool) (*SettingsOutput, error) {
	if userID == uuid.Nil || enabled == nil {
		return nil, ErrInvalidInput
	}
	row, err := s.repo.UpdateReactionNotifications(ctx, userID, *enabled)
	if err != nil {
		return nil, err
	}
	out := s.settingsOutput(row)
	return &out, nil
}

func (s *Service) UpdateNickname(ctx context.Context, userID uuid.UUID, nickname string) (*ProfileOutput, error) {
	if userID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	nickname = strings.TrimSpace(nickname)
	if nickname == "" || len(nickname) > 40 {
		return nil, ErrInvalidNickname
	}
	row, err := s.repo.UpdateNickname(ctx, userID, nickname)
	if err != nil {
		return nil, err
	}
	profile := s.profileOutput(row)
	return &profile, nil
}

func (s *Service) settingsOutput(row repo.SettingsRow) SettingsOutput {
	return SettingsOutput{
		Profile:       s.profileOutput(row),
		Notifications: NotificationSettingsOutput{ReactionEnabled: row.ReactionNotificationEnabled},
	}
}

func (s *Service) profileOutput(row repo.SettingsRow) ProfileOutput {
	return ProfileOutput{
		Nickname:  row.Nickname,
		AvatarURL: resolveSquareSmallURL(s.resolver, row.AvatarObjectKey),
		Email:     row.Email,
	}
}

func resolveSquareSmallURL(resolver *platformoss.URLResolver, objectKey *string) *string {
	if resolver == nil || objectKey == nil || *objectKey == "" {
		return nil
	}
	variant := resolver.ResolveSquareSmallVariant(*objectKey)
	if variant.URL == "" {
		return nil
	}
	return &variant.URL
}
