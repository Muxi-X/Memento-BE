package application

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const authVerificationCodeLength = 6

type CodeService struct {
	Store       CodeStore
	EmailSender EmailSender
	HashPepper  string

	CodeTTL     time.Duration
	CooldownTTL time.Duration
	LockTTL     time.Duration
	MaxAttempts int
}

func (s *CodeService) SendCode(ctx context.Context, purpose CodePurpose, email string) error {
	email = normalizeEmail(email)
	if email == "" {
		return ErrInvalidEmail
	}

	cooldownKey := cooldownKey(purpose, email)
	ok, err := s.Store.SetNX(ctx, cooldownKey, "1", s.CooldownTTL)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	code, err := generateNumericCode(authVerificationCodeLength)
	if err != nil {
		return err
	}

	codeHash := hashWithPepper(s.HashPepper, string(purpose), email, code)
	codeKey := codeKey(purpose, email)
	if err := s.Store.Set(ctx, codeKey, codeHash, s.CodeTTL); err != nil {
		return err
	}

	_ = s.Store.Del(ctx, attemptKey(purpose, email), lockKey(purpose, email))

	if err := s.EmailSender.SendAuthCode(ctx, email, code); err != nil {
		_ = s.Store.Del(ctx, codeKey, cooldownKey)
		return err
	}
	return nil
}

func (s *CodeService) VerifyCode(ctx context.Context, purpose CodePurpose, email, code string) (bool, error) {
	email = normalizeEmail(email)
	if email == "" {
		return false, ErrInvalidEmail
	}
	code = strings.TrimSpace(code)
	if !isValidVerificationCode(code) {
		return false, ErrInvalidVerificationCode
	}

	locked, err := s.Store.Exists(ctx, lockKey(purpose, email))
	if err != nil {
		return false, err
	}
	if locked > 0 {
		return false, nil
	}

	stored, ok, err := s.Store.Get(ctx, codeKey(purpose, email))
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	got := hashWithPepper(s.HashPepper, string(purpose), email, code)
	if stored != got {
		n, err := s.Store.Incr(ctx, attemptKey(purpose, email))
		if err == nil {
			if n == 1 {
				_ = s.Store.Expire(ctx, attemptKey(purpose, email), s.CodeTTL)
			}
			if s.MaxAttempts > 0 && n >= int64(s.MaxAttempts) {
				_ = s.Store.Set(ctx, lockKey(purpose, email), "1", s.LockTTL)
			}
		}
		return false, nil
	}

	_ = s.Store.Del(ctx, codeKey(purpose, email), attemptKey(purpose, email), lockKey(purpose, email))
	return true, nil
}

func codeKey(purpose CodePurpose, email string) string {
	return fmt.Sprintf("auth:code:%s:%s", purpose, email)
}

func cooldownKey(purpose CodePurpose, email string) string {
	return fmt.Sprintf("auth:code:cooldown:%s:%s", purpose, email)
}

func attemptKey(purpose CodePurpose, email string) string {
	return fmt.Sprintf("auth:code:attempt:%s:%s", purpose, email)
}

func lockKey(purpose CodePurpose, email string) string {
	return fmt.Sprintf("auth:code:lock:%s:%s", purpose, email)
}

func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func isValidVerificationCode(code string) bool {
	if len(code) != authVerificationCodeLength {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
