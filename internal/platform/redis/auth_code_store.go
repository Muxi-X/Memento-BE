package redis

import (
	"context"
	"fmt"
	"time"

	rds "github.com/redis/go-redis/v9"
)

// AuthCodeStore 是一个 Redis 适配器，将 Redis 操作接入 auth 模块所需的 CodeStore
// 具体的业务逻辑在 auth 模块的 CodeService
type AuthCodeStore struct {
	client *rds.Client
}

func NewAuthCodeStore(client *rds.Client) *AuthCodeStore {
	return &AuthCodeStore{client: client}
}

// key 不存在时设置 key 的值，并设置过期时间
func (s *AuthCodeStore) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	ok, err := s.client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis: setnx: %w", err)
	}
	return ok, nil
}

// 设置 key 的值，并设置过期时间
func (s *AuthCodeStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	if err := s.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("redis: set: %w", err)
	}
	return nil
}

// 删除一个或多个 key
func (s *AuthCodeStore) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	if err := s.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("redis: del: %w", err)
	}
	return nil
}

// 判断 key 是否存在
func (s *AuthCodeStore) Exists(ctx context.Context, key string) (int64, error) {
	n, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("redis: exists: %w", err)
	}
	return n, nil
}

// 获取 key 的值
func (s *AuthCodeStore) Get(ctx context.Context, key string) (string, bool, error) {
	v, err := s.client.Get(ctx, key).Result()
	if err == nil {
		return v, true, nil
	}
	if err == rds.Nil {
		return "", false, nil
	}
	return "", false, fmt.Errorf("redis: get: %w", err)
}

// 自增 key 的值
func (s *AuthCodeStore) Incr(ctx context.Context, key string) (int64, error) {
	n, err := s.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("redis: incr: %w", err)
	}
	return n, nil
}

// 设置 key 的过期时间
func (s *AuthCodeStore) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if err := s.client.Expire(ctx, key, ttl).Err(); err != nil {
		return fmt.Errorf("redis: expire: %w", err)
	}
	return nil
}
