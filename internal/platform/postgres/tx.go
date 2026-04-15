package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 事务执行 helper
func WithTx(ctx context.Context, pool *pgxpool.Pool, opts pgx.TxOptions, fn func(tx pgx.Tx) error) error {
	// 开启事务
	tx, err := pool.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	// 未成功提交时回滚
	defer func() { _ = tx.Rollback(ctx) }()

	// 执行 fn，fn 内执行对数据库操作，fn 返回 error 则回滚事务
	if err := fn(tx); err != nil {
		return err
	}

	// 提交事务
	return tx.Commit(ctx)
}
