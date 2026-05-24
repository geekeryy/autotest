.DEFAULT_GOAL := help

# 如果根目录存在 .env，则自动读取其中的环境变量。
ifneq (,$(wildcard .env))
include .env
export
endif

# Makefile / Docker Compose 工具变量；Go 运行时读 POSTGRES_*（见 internal/config）。
DB_MANAGED ?= external
POSTGRES_USER ?= autotest
POSTGRES_PASSWORD ?= autotest
POSTGRES_HOST ?= localhost
POSTGRES_PORT ?= 5432
POSTGRES_DB ?= autotest
POSTGRES_SSLMODE ?= disable

WITH_DB = ./scripts/with-database-url.sh
WITH_DB_COMPOSE = ./scripts/with-database-url.sh compose --

# Docker
IMAGE ?= geekeryy/autotest:latest
AUTOTEST_IMAGE ?= $(IMAGE)
API_PORT ?= 8080
PROD_DOCKERFILE := deploy/prod/Dockerfile
PLATFORMS ?= linux/amd64,linux/arm64
BUILDX_BUILDER ?= autotest-builder

export POSTGRES_USER POSTGRES_PASSWORD POSTGRES_HOST POSTGRES_PORT POSTGRES_DB POSTGRES_SSLMODE APP_ENV

# 按文件名顺序执行数据库迁移脚本。
MIGRATIONS := $(sort $(wildcard migrations/*.sql))

.PHONY: help init up wait-db migrate run-api run-e2e-api web-install web-dev web-build web-build-prod firebase-deploy test test-integration docker-buildx docker-buildx-init docker-buildx-push all-in-one-up all-in-one-down

help: ## 显示可用命令。
	@awk 'BEGIN {FS = ":.*##"; printf "用法: make <目标>\n\n目标:\n"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

init: up migrate ## 初始化数据库；默认使用外部 PostgreSQL。

up: ## 准备 PostgreSQL；DB_MANAGED=docker 时启动 all-in-one compose 中的 postgres。
	@if [ "$(DB_MANAGED)" = "docker" ]; then \
		$(WITH_DB_COMPOSE) COMPOSE_PROFILES=bundled-db docker compose --project-directory deploy/all-in-one -f deploy/all-in-one/docker-compose.yml up -d postgres; \
	else \
		echo "使用外部 PostgreSQL，跳过 Docker Compose 启动。"; \
	fi

wait-db: ## 等待 PostgreSQL 就绪（使用 POSTGRES_*）。
	@$(WITH_DB) -- ./scripts/wait-database-url.sh PostgreSQL

migrate: wait-db ## 执行 migrations/ 目录下的 SQL 迁移。
	@set -e; \
	for file in $(MIGRATIONS); do \
		echo "执行迁移 $$file"; \
		if grep -qE '^--[[:space:]]+\+goose[[:space:]]+Up' "$$file"; then \
			sql=$$(awk '/^--[[:space:]]+\+goose[[:space:]]+Up[[:space:]]*$$/{in_up=1; next} /^--[[:space:]]+\+goose[[:space:]]+Down[[:space:]]*$$/{in_up=0} in_up {print}' "$$file"); \
		else \
			echo "警告: $$file 缺少 -- +goose Up 标记，将执行完整文件"; \
			sql=$$(cat "$$file"); \
		fi; \
		if [ -z "$$(printf '%s' "$$sql" | tr -d '[:space:]')" ]; then \
			echo "错误: $$file 未产生可执行 SQL（请检查 -- +goose Up 段）"; \
			exit 1; \
		fi; \
		printf '%s\n' "$$sql" | $(WITH_DB) -- psql -v ON_ERROR_STOP=1; \
	done

run-api: ## 运行 Go API（配置由 internal/config 加载，development 时自动读 .env）。
	go run ./cmd/api

run-e2e-api: ## 运行 e2e 测试目标 API（APP_ENV=test）。
	APP_ENV=test go run ./tests/e2e_api

web-install: ## 安装管理后台前端依赖。
	npm --prefix web/admin install

web-dev: ## 启动管理后台前端开发服务。
	npm --prefix web/admin run dev

web-build: ## 构建前端产物（development 模式，加载 .env.development）。
	npm --prefix web/admin run build:dev
	
firebase-deploy: ## 构建并部署前端到 Firebase Hosting（需 firebase CLI 与 .firebaserc）。
	@if [ ! -f web/admin/.firebaserc ]; then \
		echo "请先复制 web/admin/.firebaserc.example 为 .firebaserc"; \
		exit 1; \
	fi
	@if [ ! -f web/admin/.env.production ] && [ -z "$$VITE_API_BASE_URL" ]; then \
		echo "请先配置 web/admin/.env.production 或导出 VITE_API_BASE_URL"; \
		exit 1; \
	fi
	npm --prefix web/admin run build
	cd web/admin && npm run deploy:hosting

test: ## 运行 Go 单元测试（APP_ENV=test）。
	APP_ENV=test go test ./... ./internal/testdata

test-integration: ## 集成测试（需 POSTGRES_*，建议使用 autotest_test 库）。
	APP_ENV=test go test ./internal/spec/... ./internal/scenario/... -count=1


docker-build-init: ## 初始化 buildx builder（多架构构建需要）。
	@docker buildx inspect $(BUILDX_BUILDER) >/dev/null 2>&1 || \
		docker buildx create --name $(BUILDX_BUILDER) --use --bootstrap
	@docker buildx use $(BUILDX_BUILDER)

docker-build: docker-build-init ## 多架构构建；单平台时 --load 到本地，多平台时输出 OCI 归档到 dist/。
	@mkdir -p dist; \
	if echo "$(PLATFORMS)" | grep -q ','; then \
		out="dist/autotest-$$(echo '$(PLATFORMS)' | tr ',/' '-').oci"; \
		echo "多平台构建 $(PLATFORMS) -> $$out"; \
		docker buildx build --platform $(PLATFORMS) -t $(IMAGE) -f $(PROD_DOCKERFILE) -o "type=oci,dest=$$out" .; \
	else \
		echo "单平台构建 $(PLATFORMS) -> $(IMAGE)"; \
		docker buildx build --platform $(PLATFORMS) -t $(IMAGE) -f $(PROD_DOCKERFILE) --load .; \
	fi

docker-push: docker-build-init ## 构建并推送多架构镜像（需设置可推送的 IMAGE，如 registry.example.com/autotest:tag）。
	docker buildx build --platform $(PLATFORMS) -t $(IMAGE) -f $(PROD_DOCKERFILE) --push .


all-in-one-up: ## 快速试用：内嵌前端 API（deploy/all-in-one）；DB_MANAGED=docker 时含 PostgreSQL。
	@set -a; \
	[ -f deploy/all-in-one/.env ] && . deploy/all-in-one/.env; \
	set +a; \
	db_managed="$${DB_MANAGED:-docker}"; \
	if [ "$$db_managed" = "docker" ]; then \
		$(WITH_DB_COMPOSE) COMPOSE_PROFILES=bundled-db docker compose --project-directory deploy/all-in-one -f deploy/all-in-one/docker-compose.yml up -d --build; \
	else \
		$(WITH_DB) -- docker compose --project-directory deploy/all-in-one -f deploy/all-in-one/docker-compose.yml up -d --build; \
	fi

all-in-one-down: ## 停止 all-in-one 栈。
	@set -a; \
	[ -f deploy/all-in-one/.env ] && . deploy/all-in-one/.env; \
	set +a; \
	db_managed="$${DB_MANAGED:-docker}"; \
	if [ "$$db_managed" = "docker" ]; then \
		docker compose --project-directory deploy/all-in-one --profile bundled-db -f deploy/all-in-one/docker-compose.yml down; \
	else \
		docker compose --project-directory deploy/all-in-one -f deploy/all-in-one/docker-compose.yml down; \
	fi
