package application

import (
	"context"
	"time"
)

type EmailSender interface {
	SendAuthCode(ctx context.Context, toEmail string, code string) error
}

type CodePurpose string

const (
	CodePurposeSignup CodePurpose = "signup"
	CodePurposeLogin  CodePurpose = "login"
	CodePurposeReset  CodePurpose = "reset"
)

type CodeStore interface {
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, key string) (int64, error)
	Get(ctx context.Context, key string) (value string, ok bool, err error)
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
}
