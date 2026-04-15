package application

import (
	"context"
	"errors"
	"time"

	dauth "cixing/internal/modules/auth/domain"
	"cixing/internal/shared/common"
)

type PasswordResetService struct {
	CodeSvc           *CodeService
	AuthRepo          dauth.Repository
	TokenSvc          *TokenService
	ResetTokenTTL     time.Duration
	HashPepper        string
	PasswordMinLength int
}

func (s *PasswordResetService) SendCode(ctx context.Context, email string) error {
	return s.CodeSvc.SendCode(ctx, CodePurposeReset, email)
}

func (s *PasswordResetService) VerifyCode(ctx context.Context, email, code string) (string, error) {
	valid, err := s.CodeSvc.VerifyCode(ctx, CodePurposeReset, email, code)
	if err != nil {
		return "", err
	}
	if !valid {
		return "", ErrCodeVerificationFailed
	}

	if _, err := s.AuthRepo.GetEmailIdentityByEmail(ctx, normalizeEmail(email)); err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return "", ErrCodeVerificationFailed
		}
		return "", err
	}

	token, err := generateToken(32)
	if err != nil {
		return "", err
	}
	tokenHash := hashWithPepper(s.HashPepper, token)
	expiresAt := time.Now().Add(s.ResetTokenTTL)

	email = normalizeEmail(email)
	if err := s.AuthRepo.InvalidateEmailActionSessionsByEmailPurpose(ctx, email, dauth.PurposeResetPassword); err != nil {
		return "", err
	}
	if err := s.AuthRepo.CreateEmailActionSession(ctx, dauth.PurposeResetPassword, email, tokenHash, expiresAt); err != nil {
		return "", err
	}

	return token, nil
}

func (s *PasswordResetService) Complete(ctx context.Context, resetToken, newPassword string) (*AuthToken, error) {
	if len(newPassword) < s.PasswordMinLength {
		return nil, ErrPasswordTooShort
	}
	if resetToken == "" {
		return nil, ErrInvalidResetToken
	}

	pwHash, err := hashPassword(newPassword)
	if err != nil {
		return nil, err
	}

	tokenHash := hashWithPepper(s.HashPepper, resetToken)
	userID, err := s.AuthRepo.ResetPasswordWithResetToken(ctx, tokenHash, pwHash)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return nil, ErrInvalidResetToken
		}
		return nil, err
	}

	return s.TokenSvc.IssueToken(userID)
}
