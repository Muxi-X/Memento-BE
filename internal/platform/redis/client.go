package redis

import (
	"context"
	"fmt"
	"time"

	rds "github.com/redis/go-redis/v9"

	"cixing/internal/config"
)

const defaultTimeout = 2 * time.Second

func NewClient(ctx context.Context, cfg config.RedisConfig) (*rds.Client, error) {
	if cfg.Addr == "" {
		return nil, fmt.Errorf("redis: addr is required")
	}

	opts := &rds.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  defaultTimeout,
		ReadTimeout:  defaultTimeout,
		WriteTimeout: defaultTimeout,
	}

	// 创建 client，ping，超时取消
	cli := rds.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	if err := cli.Ping(pingCtx).Err(); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("redis: ping: %w", err)
	}
	return cli, nil
}
