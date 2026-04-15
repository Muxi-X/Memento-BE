package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	GetEmailIdentityByEmail(ctx context.Context, email string) (EmailIdentity, error)
	GetUserEmailIdentityByUserID(ctx context.Context, userID uuid.UUID) (EmailIdentity, error)
	CreateUserEmailIdentity(ctx context.Context, userID uuid.UUID, email string, verified bool, passwordHash *string) error
	MarkEmailVerified(ctx context.Context, userID uuid.UUID) error
	SetPasswordHashByUserID(ctx context.Context, userID uuid.UUID, passwordHash string) error
	ResetPasswordWithResetToken(ctx context.Context, tokenHash string, passwordHash string) (uuid.UUID, error)

	InvalidateEmailActionSessionsByEmailPurpose(ctx context.Context, email string, purpose EmailActionPurpose) error
	CreateEmailActionSession(ctx context.Context, purpose EmailActionPurpose, email string, tokenHash string, expiresAt time.Time) error
	GetEmailActionSessionByHash(ctx context.Context, tokenHash string) (ActionSession, error)
	MarkEmailActionSessionUsed(ctx context.Context, id uuid.UUID) error
}

type RegistrationRepository interface {
	RegisterUser(ctx context.Context, email string, passwordHash string) (uuid.UUID, error)
	RegisterUserWithSignupToken(ctx context.Context, tokenHash string, passwordHash string) (uuid.UUID, error)
}
