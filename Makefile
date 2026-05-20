# ============================================================
# league-api — common development tasks.
# Database is Supabase (hosted). DATABASE_URL lives in .env.
# Run `make help` for the full list.
# ============================================================

SHELL := /bin/bash
APP   := league-api
BIN   := bin/$(APP)

.PHONY: help run build test test-race tidy fmt vet \
        docker-up docker-down docker-logs \
        check-env migrate seed seed-dry clean

help: ## Print this help.
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[1m%-14s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

run: ## Run the API locally (loads DATABASE_URL from .env).
	@$(MAKE) --no-print-directory check-env
	@set -a; source .env; set +a; go run ./cmd/server

build: ## Compile the binary into ./bin.
	@mkdir -p bin
	go build -trimpath -ldflags="-s -w" -o $(BIN) ./cmd/server

test: ## Run all unit tests.
	go test ./...

test-race: ## Run unit tests with the race detector.
	go test -race ./...

tidy: ## Tidy the module file.
	go mod tidy

fmt: ## Format every Go file.
	gofmt -s -w .

vet: ## Static analysis via go vet.
	go vet ./...

check-env: ## Verify .env exists and DATABASE_URL looks like a Supabase URL.
	@test -f .env || { \
	  echo "ERROR: .env not found in $(CURDIR)."; \
	  echo "       Run: cp .env.example .env  (then set DATABASE_URL)"; \
	  exit 1; \
	}
	@grep -E '^[[:space:]]*DATABASE_URL=.*supabase\.(co|com)' .env >/dev/null || { \
	  echo "ERROR: DATABASE_URL in .env does not look like a Supabase URL."; \
	  echo "       Expected host substring: 'pooler.supabase.com' or 'db.<ref>.supabase.co'."; \
	  echo "       Grab it from: Supabase Dashboard -> Settings -> Database -> Connection string -> Session pooler."; \
	  exit 1; \
	}

docker-up: ## Build image and run the API container against Supabase.
	@$(MAKE) --no-print-directory check-env
	docker compose up --build -d
	@echo ""
	@echo "league-api is starting. Tail logs with: make docker-logs"
	@echo "Health: curl http://localhost:$$(grep ^PORT= .env | cut -d= -f2 || echo 8080)/health"

docker-down: ## Stop and remove the API container.
	docker compose down

docker-logs: ## Tail the app container logs.
	docker compose logs -f app

seed: ## Fetch EA FC 25 ratings from the drop API and upsert into teams table.
	@$(MAKE) --no-print-directory check-env
	@set -a; source .env; set +a; go run ./cmd/seed

seed-dry: ## Preview EA FC 25 computed ratings without inserting into DB.
	@set -a; source .env; set +a; go run ./cmd/seed --dry-run

migrate: ## Apply migrations against the DATABASE_URL in .env via psql.
	@$(MAKE) --no-print-directory check-env
	@set -a; source .env; set +a; \
	  psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -f db/migrations/001_schema.sql && \
	  psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -f db/migrations/002_seed.sql

clean: ## Remove build artifacts.
	rm -rf bin
