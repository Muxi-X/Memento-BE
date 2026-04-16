# ---------- Project ----------
APP_API   := cixing-api

API_MAIN    := ./cmd/cixing-api

BIN_DIR := bin

# When this makefile is invoked via `make -f Makefile/makefile`, any recursive
# `$(MAKE)` call must also pass `-f` (otherwise GNU Make will look for `Makefile`
# in the cwd, but this repo uses the `Makefile/` directory).
SELF_MAKEFILE := $(lastword $(MAKEFILE_LIST))

ifeq ($(OS),Windows_NT)
PWSH      := powershell -NoProfile -ExecutionPolicy Bypass -Command
MKDIR_BIN := powershell -NoProfile -Command "New-Item -ItemType Directory -Force -Path '$(BIN_DIR)' | Out-Null"
else
SHELL := /bin/bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c
MKDIR_BIN := mkdir -p $(BIN_DIR)
endif

# ---------- Tools ----------
MIGRATE    := go run github.com/golang-migrate/migrate/v4/cmd/migrate

# ---------- Paths ----------
MIGRATIONS_DIR := db/migrations

# ---------- Docker compose ----------
COMPOSE_DEV  := deploy/compose/docker-compose.dev.yml
COMPOSE_TEST := deploy/compose/docker-compose.test.yml

# ---------- Env ----------
PG_DSN ?=

define require-var
$(if $(strip $($1)),,$(error $1 is required))
endef

.PHONY: help
help:
	@echo "Targets:"
	@echo "  make gen              - generate sqlc + openapi"
	@echo "  make gen-sqlc         - sqlc generate"
	@echo "  make gen-openapi      - oapi-codegen generate"
	@echo "  make tidy             - go mod tidy"
	@echo "  make fmt              - gofmt"
	@echo "  make lint             - golangci-lint run (requires golangci-lint installed)"
	@echo "  make test             - go test ./..."
	@echo "  make test-race         - go test -race ./..."
	@echo "  make build            - build api"
	@echo "  make run-api          - run api"
	@echo "  make dev-up           - docker compose up (postgres+redis+mailpit)"
	@echo "  make dev-down         - docker compose down -v"
	@echo "  make migrate-up       - migrate up (needs PG_DSN)"
	@echo "  make migrate-down     - migrate down 1 step (needs PG_DSN)"
	@echo "  make migrate-version  - show current migration version (needs PG_DSN)"
	@echo "  make migrate-new name=xxx - create new migration files"
	@echo "  make it-up            - bring up integration deps (postgres test db)"
	@echo "  make it               - run integration tests (always tears down test db)"

# ---------- Go ----------
.PHONY: tidy fmt test test-race build run-api
tidy:
	go mod tidy

fmt:
	gofmt -w .

test:
	go test ./...

test-race:
	go test -race ./...

build:
	$(MKDIR_BIN)
	go build -o $(BIN_DIR)/$(APP_API) $(API_MAIN)

run-api:
	go run $(API_MAIN)

# ---------- Codegen ----------
.PHONY: gen gen-sqlc gen-openapi
gen: gen-sqlc gen-openapi

ifeq ($(OS),Windows_NT)
gen-sqlc:
	powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\gen_sqlc.ps1

gen-openapi:
	powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\gen_openapi.ps1
else
gen-sqlc:
	bash ./scripts/gen_sqlc.sh

gen-openapi:
	bash ./scripts/gen_openapi.sh
endif

# ---------- Lint ----------
# 建议：本机安装 golangci-lint（或你也可以把它加进 tools.go 再改成 go run）
.PHONY: lint
lint:
	golangci-lint run --timeout=5m

# ---------- Migrations ----------
.PHONY: migrate-up migrate-down migrate-version migrate-force migrate-new
migrate-up:
	$(call require-var,PG_DSN)
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(PG_DSN)" up

migrate-down:
	$(call require-var,PG_DSN)
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(PG_DSN)" down 1

migrate-version:
	$(call require-var,PG_DSN)
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(PG_DSN)" version

# usage: make migrate-force v=NNN
migrate-force:
	$(call require-var,PG_DSN)
	$(call require-var,v)
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(PG_DSN)" force $(v)

# usage: make migrate-new name=create_users
migrate-new:
	$(call require-var,name)
	$(MIGRATE) create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)

# ---------- Docker (dev deps) ----------
.PHONY: dev-up dev-down dev-ps dev-logs
dev-up:
	docker compose -f $(COMPOSE_DEV) up -d --wait

dev-ps:
	docker compose -f $(COMPOSE_DEV) ps

dev-logs:
	docker compose -f $(COMPOSE_DEV) logs -f --tail=200

dev-down:
	docker compose -f $(COMPOSE_DEV) down -v

# ---------- Integration test deps ----------
.PHONY: it-up it-down it
it-up:
	docker compose -f $(COMPOSE_TEST) up -d --wait

it-down:
	docker compose -f $(COMPOSE_TEST) down -v

# 集成测试使用 TEST_DATABASE_URL，例如：
# $env:TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5433/cixing_test?sslmode=disable"
ifeq ($(OS),Windows_NT)
it:
	$(PWSH) "$$ErrorActionPreference='Stop'; $$testExit = 0; docker compose -f '$(COMPOSE_TEST)' up -d --wait; if ($$LASTEXITCODE -ne 0) { exit $$LASTEXITCODE }; try { go test ./tests/integration/...; $$testExit = $$LASTEXITCODE } finally { docker compose -f '$(COMPOSE_TEST)' down -v; $$downExit = $$LASTEXITCODE; if ($$testExit -ne 0) { exit $$testExit }; if ($$downExit -ne 0) { exit $$downExit } }"
else
it:
	trap '$(MAKE) -f $(SELF_MAKEFILE) it-down' EXIT
	$(MAKE) -f $(SELF_MAKEFILE) it-up
	go test ./tests/integration/...
endif
