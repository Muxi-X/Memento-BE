package migrate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"cixing/internal/config"
)

// 迁移数据库
// 可选命令参数：up | down | version | force <version>
func Run(ctx context.Context, args []string) error {
	// SkipValidate 跳过配置校验
	cfg, err := config.LoadWith(config.Options{
		ConfigFile:   strings.TrimSpace(os.Getenv(config.EnvConfigFile)),
		SkipValidate: true,
	})
	if err != nil {
		return err
	}

	// 获取 Postgres DSN, 并检查是否为空
	dsn := cfg.Postgres.DSN
	if strings.TrimSpace(dsn) == "" {
		return fmt.Errorf("migrate: postgres dsn is required")
	}

	// 检查命令行参数
	if len(args) == 0 {
		return fmt.Errorf("migrate: command required (up|down|version|force)")
	}

	// 创建 migrate 实例
	m, err := migrate.New("file://db/migrations", dsn) // 使用相对路径读取迁移脚本
	if err != nil {
		return err
	}
	// 忽略了关闭错误
	defer func() { _, _ = m.Close() }()

	// 根据子命令执行不同迁移操作
	switch strings.ToLower(args[0]) {
	// up: 执行所有未执行的迁移
	case "up":
		err = m.Up()
		// migrate.ErrNoChange 表示没有需要执行的迁移，已经是最新
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return err

	// down: 回滚最后一次迁移
	case "down":
		err = m.Steps(-1)
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return err

	// version: 获取当前迁移版本和 dirty 状态
	case "version":
		// dirty=true 通常表示上一次迁移中途失败/未完成
		v, dirty, err := m.Version()
		if errors.Is(err, migrate.ErrNilVersion) {
			fmt.Println("no migration applied")
			return nil
		}
		if err != nil {
			return err
		}
		fmt.Printf("version=%d dirty=%v\n", v, dirty)
		return nil

	// force：强制设置数据库的迁移版本号（不会执行迁移脚本）
	case "force":
		if len(args) < 2 {
			return fmt.Errorf("migrate: force requires version number")
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("migrate: invalid version: %w", err)
		}
		// 强制设置到版本 n
		return m.Force(n)
	default:
		return fmt.Errorf("migrate: unknown command %q", args[0])
	}
}
