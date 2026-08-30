.PHONY: help run dev build test test-cover test-race tidy fmt vet lint migrate-up migrate-down migrate-create tools up down down-v logs logs-all ps restart dev-up sh db-shell redis-shell create-admin create-admin-local

# Load DB vars from .env so migrate targets get the connection string.
# Leading "-" so a fresh clone without .env can still run make help/test.
-include .env
export

MIGRATE_DSN = postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)
MIGRATIONS_DIR = migrations

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'

run: ## Run the API
	go run ./cmd/api

dev: ## Run with hot reload (requires: make tools)
	air

build: ## Build the binary into ./bin
	go build -o bin/api ./cmd/api

test: ## Run all tests
	go test ./... -count=1

test-race: ## Run all tests with the race detector
	go test ./... -race -count=1

test-cover: ## Run tests and open an HTML coverage report
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out | tail -1
	go tool cover -html=coverage.out -o coverage.html
	@echo "report written to coverage.html"

fmt: ## Format all Go code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

lint: fmt vet ## Format, vet, and verify the build
	go build ./...

tidy: ## Sync go.mod / go.sum
	go mod tidy

tools: ## Install dev tools (air)
	go install github.com/air-verse/air@latest

migrate-up: ## Apply all up migrations via CLI (app also auto-migrates on startup)
	migrate -path $(MIGRATIONS_DIR) -database "$(MIGRATE_DSN)" up

migrate-down: ## Roll back the last migration
	migrate -path $(MIGRATIONS_DIR) -database "$(MIGRATE_DSN)" down 1

migrate-create: ## Create a new migration: make migrate-create name=add_orders
	migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)

# ---------------------------------------------------------------------------
# Docker
# ---------------------------------------------------------------------------
DC = docker compose
DC_DEV = docker compose -f docker-compose.yml -f docker-compose.dev.yml

up: ## Start the full stack (app + postgres + redis) in the background
	$(DC) up -d --build

down: ## Stop the stack (volumes are kept)
	$(DC) down

down-v: ## Stop the stack AND delete the data volumes (destroys the database)
	$(DC) down -v

logs: ## Tail application logs
	$(DC) logs -f app

logs-all: ## Tail logs from every service
	$(DC) logs -f

ps: ## Show container status and health
	$(DC) ps

restart: ## Rebuild and restart just the app container
	$(DC) up -d --build app

dev-up: ## Start the stack with hot reload (Air) inside the container
	$(DC_DEV) up --build

sh: ## Open a shell in the running app container
	$(DC) exec app sh

db-shell: ## Open psql against the running database
	$(DC) exec postgres psql -U $(DB_USER) -d $(DB_NAME)

redis-shell: ## Open redis-cli against the running Redis
	$(DC) exec redis redis-cli

create-admin: ## Create/promote an admin: make create-admin email=a@b.com password=secret
	$(DC) run --rm app /app/admin -email=$(email) -password=$(password) -role=admin

create-admin-local: ## Same, but against your local (non-Docker) database
	go run ./cmd/admin -email=$(email) -password=$(password) -role=admin
