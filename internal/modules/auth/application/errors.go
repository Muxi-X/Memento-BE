package application

import "errors"

var (
	ErrInvalidEmail            = errors.New("invalid email")
	ErrInvalidVerificationCode = errors.New("invalid verification code")
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrCodeVerificationFailed  = errors.New("code verification failed")
	ErrInvalidToken            = errors.New("invalid token")
	ErrInvalidSignupToken      = errors.New("invalid signup token")
	ErrInvalidResetToken       = errors.New("invalid reset token")
	ErrPasswordTooShort        = errors.New("password too short")
)
