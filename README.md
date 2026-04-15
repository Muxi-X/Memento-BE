# cixing 后端

这是 `cixing` 的后端服务仓库。项目当前聚焦于：
- HTTP API
- 数据库迁移
- OpenAPI / sqlc 代码生成
- 单元测试与集成测试

产品方向是一个“关键词引导”的图片分享平台，后端目前已经包含：
- 官方关键词与提示词
- 自定义关键词
- 上传发布流程
- 个人主页与通知
- 互动反应与未读计数

## 目录说明
- `cmd/`：程序入口，目前主要是 `cixing-api` 和 `cixing-migrate`
- `internal/`：业务实现、平台能力、HTTP handler、配置加载
- `api/openapi/`：OpenAPI 规格与生成配置
- `db/migrations/`：数据库迁移脚本
- `tests/integration/`：跨模块集成测试
- `deploy/compose/`：本地开发和测试用的 Docker Compose
- `scripts/`：代码生成脚本
- `Makefile/makefile`：常用开发命令入口

## 开发环境
- Go `1.25.1`
- Docker + Docker Compose
- GNU Make

## 配置约定
- 默认配置文件是仓库根目录的 `config.yaml`
- 当前仓库里的 `config.yaml` 是安全样板，不包含真实密钥，启动前需要把 `change-me` 和 JWT PEM 占位内容替换掉
- 如果你不想直接改根目录 `config.yaml`，可以基于 `configs/config.local.yaml.example` 新建本地文件，然后通过 `CONFIG_FILE` 指向它
- 当前只支持少量环境变量覆盖，清单见 `configs/env.example`
- 项目不会自动读取根目录 `.env`

一个常见的本地做法是：

```powershell
Copy-Item configs/config.local.yaml.example configs/config.local.yaml
$env:CONFIG_FILE="configs/config.local.yaml"
```

## 本地启动
先启动本地依赖：

```bash
docker compose -f deploy/compose/docker-compose.dev.yml up -d --wait
```

开发环境会启动：
- Postgres：`127.0.0.1:5432`
- Redis：`127.0.0.1:6379`
- Mailpit SMTP：`127.0.0.1:1025`
- Mailpit Web UI：<http://localhost:8025>

然后准备迁移和启动 API：

```powershell
$env:PG_DSN="postgres://postgres:postgres@127.0.0.1:5432/cixing?sslmode=disable"
make -f Makefile/makefile migrate-up
make -f Makefile/makefile run-api
```

如果你使用的是自定义配置文件，也可以直接走程序入口：

```powershell
$env:CONFIG_FILE="configs/config.local.yaml"
go run ./cmd/cixing-migrate up
go run ./cmd/cixing-api
```

注意：
- 本地 compose 不提供对象存储模拟
- 上传相关功能仍然需要有效的 `oss` 配置

## 常用命令
```bash
make -f Makefile/makefile help
make -f Makefile/makefile test
make -f Makefile/makefile build
make -f Makefile/makefile dev-up
make -f Makefile/makefile dev-down
```

`make lint` 依赖本机已安装 `golangci-lint`。

## 代码生成
统一入口：

```bash
make -f Makefile/makefile gen
```

拆开执行：

```bash
make -f Makefile/makefile gen-sqlc
make -f Makefile/makefile gen-openapi
```

如果你在 Windows PowerShell 下不想依赖 `make`，也可以直接运行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\gen_sqlc.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\gen_openapi.ps1
```

## 测试
仓库里有两层测试：
- `internal/.../*_test.go`：包内单元测试，主要保护配置、领域状态机、纯函数和小范围业务规则
- `tests/integration/`：集成测试，覆盖上传、发布、通知、个人主页等完整链路

全量测试：

```bash
make -f Makefile/makefile test
```

集成测试：

```powershell
$env:TEST_DATABASE_URL="postgres://postgres:postgres@127.0.0.1:5433/cixing_test?sslmode=disable"
make -f Makefile/makefile it
```

`make it` 会：
- 启动 `deploy/compose/docker-compose.test.yml`
- 运行 `tests/integration/...`
- 结束后自动清理测试容器

运行集成测试前需要先确保本机 Docker Engine 已启动。
