package application

import (
	"time"

	dauth "cixing/internal/modules/auth/domain"
)

type Module struct {
	Login  *LoginService
	Reset  *PasswordResetService
	Signup *SignupService
	Token  *TokenService
}

type Deps struct {
	CodeStore   CodeStore
	EmailSender EmailSender

	AuthRepo         dauth.Repository
	RegistrationRepo dauth.RegistrationRepository
}

type Config struct {
	HashPepper        string
	CodeTTL           time.Duration
	CooldownTTL       time.Duration
	LockTTL           time.Duration
	MaxAttempts       int
	PasswordMinLength int
	ResetTokenTTL     time.Duration
	SignupTokenTTL    time.Duration
	JWT               JWTConfig
}

func NewModule(deps Deps, cfg Config) (*Module, error) {
	token, err := NewTokenService(cfg.JWT)
	if err != nil {
		return nil, err
	}

	code := &CodeService{
		Store:       deps.CodeStore,
		EmailSender: deps.EmailSender,
		HashPepper:  cfg.HashPepper,
		CodeTTL:     cfg.CodeTTL,
		CooldownTTL: cfg.CooldownTTL,
		LockTTL:     cfg.LockTTL,
		MaxAttempts: cfg.MaxAttempts,
	}

	login := &LoginService{
		CodeSvc:           code,
		AuthRepo:          deps.AuthRepo,
		TokenSvc:          token,
		PasswordMinLength: cfg.PasswordMinLength,
	}

	reset := &PasswordResetService{
		CodeSvc:           code,
		AuthRepo:          deps.AuthRepo,
		TokenSvc:          token,
		ResetTokenTTL:     cfg.ResetTokenTTL,
		HashPepper:        cfg.HashPepper,
		PasswordMinLength: cfg.PasswordMinLength,
	}

	signup := &SignupService{
		CodeSvc:           code,
		AuthRepo:          deps.AuthRepo,
		RegistrationRepo:  deps.RegistrationRepo,
		TokenSvc:          token,
		SignupTokenTTL:    cfg.SignupTokenTTL,
		HashPepper:        cfg.HashPepper,
		PasswordMinLength: cfg.PasswordMinLength,
	}

	return &Module{
		Login:  login,
		Reset:  reset,
		Signup: signup,
		Token:  token,
	}, nil
}
