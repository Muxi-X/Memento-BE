# cixing

一个关键词引导的图片分享平台后端，Go + PostgreSQL + Redis。

## 功能

每日关键词推送、作品批量上传发布、自定义关键词、灵感/共鸣互动、个人主页、邮箱认证。

## 技术栈

Go · Gin · PostgreSQL · Redis · 七牛云 OSS · JWT (RS256) · Docker Compose

## 快速开始

```bash
git clone <repo-url> && cd cixing
cp configs/config.local.yaml.example configs/config.local.yaml
export CONFIG_FILE=configs/config.local.yaml
make -f Makefile/makefile dev-up      # Postgres + Redis + Mailpit
make -f Makefile/makefile migrate-up  # 数据库迁移
make -f Makefile/makefile run-api     # 启动服务
```

## 开发

```bash
make -f Makefile/makefile fmt      # 格式化
make -f Makefile/makefile lint     # 静态检查
make -f Makefile/makefile test     # 单元测试
make -f Makefile/makefile it       # 集成测试（需 TEST_DATABASE_URL）
```

修改 SQL 或 OpenAPI 后运行 `make -f Makefile/makefile gen` 重新生成代码。

## 部署

CI/CD 通过 GitHub Actions。push `develop` 自动部署测试环境，正式环境手动 `workflow_dispatch`。详见 `deploy/compose/` 和 `.github/workflows/`。
