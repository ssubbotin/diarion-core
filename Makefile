.PHONY: help dev infra-up infra-down api mcp web build test lint vuln tidy clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "%-20s %s\n", $$1, $$2}'

dev: infra-up ## Bring up everything for local development
	@echo "Postgres + Redis are up. Start services in separate terminals:"
	@echo "  make api"
	@echo "  make web"

infra-up: ## Start Postgres + Redis
	docker compose up -d postgres redis

infra-down: ## Stop Postgres + Redis
	docker compose down

api: ## Run diarion-api locally
	go run ./cmd/diarion-api

mcp: ## Run diarion-mcp locally
	go run ./cmd/diarion-mcp

web: ## Run SvelteKit dev server
	cd web && npm run dev

build: ## Build all binaries and the web bundle
	go build -o bin/diarion-api ./cmd/diarion-api
	go build -o bin/diarion-mcp ./cmd/diarion-mcp
	cd web && npm run build

test: ## Run Go and Node test suites
	go test ./...
	cd web && npm test --if-present

lint: ## Run linters
	golangci-lint run ./...
	cd web && npm run lint --if-present

vuln: ## Vulnerability scans
	govulncheck ./...
	cd web && npm audit --omit=dev

tidy: ## Tidy Go module deps
	go mod tidy

clean: ## Remove build artefacts
	rm -rf bin/ web/build/ web/.svelte-kit/
