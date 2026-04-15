package main

import (
	"context"
	"fmt"
	"os"

	"cixing/internal/app/migrate"
)

// 可选命令参数
// up: 执行所有未执行的迁移
// down: 回滚最后一次迁移
// version: 获取当前迁移版本和 dirty 状态
// force <version>: 强制设置到版本 n
func main() {
	if err := migrate.Run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
