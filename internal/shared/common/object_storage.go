package common

import (
	"context"
	"time"
)

type PresignResult struct {
	Method    string
	URL       string
	Headers   map[string]string // 客户端必须带上的 header（有的实现需要）
	ExpiresAt time.Time
}

type ObjectStorage interface {
	PresignPut(ctx context.Context, bucket, key, contentType string, size int64, expires time.Duration) (*PresignResult, error)
	PresignGet(ctx context.Context, bucket, key string, expires time.Duration) (*PresignResult, error)

	Get(ctx context.Context, bucket, key string) (body []byte, contentType string, err error)
	Put(ctx context.Context, bucket, key string, body []byte, contentType string) error
	Head(ctx context.Context, bucket, key string) (exists bool, size int64, etag string, contentType string, err error)
	Delete(ctx context.Context, bucket, key string) error
}
