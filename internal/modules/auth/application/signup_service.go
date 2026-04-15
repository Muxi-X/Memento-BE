package application

import (
	"context"
	"errors"
	"time"

	dauth "cixing/internal/modules/auth/domain"
	"cixing/internal/shared/common"
)

type SignupService struct {
	CodeSvc           *CodeService
	AuthRepo          dauth.Repository
	RegistrationRepo  dauth.RegistrationRepository
	TokenSvc          *TokenService
	SignupTokenTTL    time.Duration
	HashPepper        string
	PasswordMinLength int
}

func (s *SignupService) SendCode(ctx context.Context, email string) error {
	return s.CodeSvc.SendCode(ctx, CodePurposeSignup, email)
}

func (s *SignupService) VerifyCode(ctx context.Context, email, code string) (string, error) {
	valid, err := s.CodeSvc.VerifyCode(ctx, CodePurposeSignup, email, code)
	if err != nil {
		return "", err
	}
	if !valid {
		return "", ErrCodeVerificationFailed
	}

	token, err := generateToken(32)
	if err != nil {
		return "", err
	}
	tokenHash := hashWithPepper(s.HashPepper, token)
	expiresAt := time.Now().Add(s.SignupTokenTTL)

	email = normalizeEmail(email)
	if err := s.AuthRepo.InvalidateEmailActionSessionsByEmailPurpose(ctx, email, dauth.PurposeSignup); err != nil {
		return "", err
	}
	if err := s.AuthRepo.CreateEmailActionSession(ctx, dauth.PurposeSignup, email, tokenHash, expiresAt); err != nil {
		return "", err
	}

	return token, nil
}

func (s *SignupService) Complete(ctx context.Context, signupToken, password string) (*AuthToken, error) {
	if len(password) < s.PasswordMinLength {
		return nil, ErrPasswordTooShort
	}
	if signupToken == "" {
		return nil, ErrInvalidSignupToken
	}

	pwHash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}

	tokenHash := hashWithPepper(s.HashPepper, signupToken)
	userID, err := s.RegistrationRepo.RegisterUserWithSignupToken(ctx, tokenHash, pwHash)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return nil, ErrInvalidSignupToken
		}
		return nil, err
	}

	return s.TokenSvc.IssueToken(userID)
}
