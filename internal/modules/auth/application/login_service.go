package application

import (
	"context"
	"errors"

	dauth "cixing/internal/modules/auth/domain"
	"cixing/internal/shared/common"
)

type LoginService struct {
	CodeSvc           *CodeService
	AuthRepo          dauth.Repository
	TokenSvc          *TokenService
	PasswordMinLength int
}

func (s *LoginService) SendCode(ctx context.Context, email string) error {
	return s.CodeSvc.SendCode(ctx, CodePurposeLogin, email)
}

func (s *LoginService) LoginByCode(ctx context.Context, email, code string) (*AuthToken, error) {
	valid, err := s.CodeSvc.VerifyCode(ctx, CodePurposeLogin, email, code)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, ErrCodeVerificationFailed
	}

	identity, err := s.AuthRepo.GetEmailIdentityByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return nil, ErrCodeVerificationFailed
		}
		return nil, err
	}

	tokens, err := s.TokenSvc.IssueToken(identity.UserID)
	if err != nil {
		return nil, err
	}
	return tokens, nil
}

func (s *LoginService) LoginByPassword(ctx context.Context, email, password string) (*AuthToken, error) {
	email = normalizeEmail(email)
	if email == "" {
		return nil, ErrInvalidEmail
	}
	if len(password) < s.PasswordMinLength {
		return nil, ErrInvalidCredentials
	}

	identity, err := s.AuthRepo.GetEmailIdentityByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if identity.PasswordHash == nil || !verifyPassword(*identity.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}

	return s.TokenSvc.IssueToken(identity.UserID)
}
