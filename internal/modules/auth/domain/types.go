package domain

import (
	"time"

	"github.com/google/uuid"
)

type EmailActionPurpose int16

const (
	PurposeSignup        EmailActionPurpose = 1
	PurposeResetPassword EmailActionPurpose = 2
)

type EmailIdentity struct {
	UserID        uuid.UUID
	Email         string
	EmailVerified bool
	PasswordHash  *string
}

type ActionSession struct {
	ID        uuid.UUID
	Purpose   EmailActionPurpose
	Email     string
	ExpiresAt time.Time
	UsedAt    *time.Time
}
