.DEFAULT_GOAL := help

# 如果根目录存在 .env，则自动读取其中的环境变量。
ifneq (,$(wildcard .env))
include .env
endif

# 本地开发默认配置，可通过 .env 或 make 命令行参数覆盖。
POSTGRES_DB ?= autotest
POSTGRES_USER ?= autotest
POSTGRES_PASSWORD ?= autotest
POSTGRES_HOST ?= localhost
POSTGRES_PORT ?= 5432
DATABASE_URL ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable
ADDR ?= :8080

export POSTGRES_DB POSTGRES_USER POSTGRES_PASSWORD POSTGRES_HOST POSTGRES_PORT DATABASE_URL ADDR ADMIN_USERNAME ADMIN_PASSWORD JWT_SECRET

# 按文件名顺序执行数据库迁移脚本。
MIGRATIONS := $(sort $(wildcard migrations/*.sql))

.PHONY: help init up wait-db migrate run-api web-install web-dev web-build test down

help: ## 显示可用命令。
	@awk 'BEGIN {FS = ":.*##"; printf "用法: make <目标>\n\n目标:\n"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

init: up migrate ## 初始化本地服务和数据库。

up: ## 使用 docker compose 启动 PostgreSQL。
	docker compose up -d postgres

wait-db: ## 等待 PostgreSQL 就绪。
	@echo "等待 PostgreSQL 就绪..."
	@for i in $$(seq 1 30); do \
		if docker compose exec -T postgres pg_isready -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" >/dev/null 2>&1; then \
			echo "PostgreSQL 已就绪。"; \
			exit 0; \
		fi; \
		sleep 2; \
	done; \
	echo "PostgreSQL 未在预期时间内就绪。"; \
	docker compose logs postgres; \
	exit 1

migrate: wait-db ## 执行 migrations/ 目录下的 SQL 迁移。
	@set -e; \
	for file in $(MIGRATIONS); do \
		echo "执行迁移 $$file"; \
		awk '/^--[[:space:]]+\+goose[[:space:]]+Up[[:space:]]*$$/{in_up=1; next} /^--[[:space:]]+\+goose[[:space:]]+Down[[:space:]]*$$/{in_up=0} in_up {print}' "$$file" | \
			docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)"; \
	done

run-api: ## 使用已加载的环境变量运行 Go API。
	ADDR="$(ADDR)" go run ./cmd/api

web-install: ## 安装管理后台前端依赖。
	npm --prefix web/admin install

web-dev: ## 启动管理后台前端开发服务。
	npm --prefix web/admin run dev

web-build: ## 构建管理后台前端产物。
	npm --prefix web/admin run build

test: ## 运行 Go 测试。
	go test ./...

down: ## 停止 docker compose 服务。
	docker compose down
