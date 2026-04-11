.PHONY: dev test build migrate lint docker-up docker-down up down logs start stop

# Start all services (Postgres:5433, Inngest:8290, API:8081, Frontend:3001)
start:
	./scripts/start.sh

# Stop all services
stop:
	./scripts/stop.sh

# Run everything via Docker (postgres + inngest + api + web)
up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

# Start only infrastructure (postgres + inngest) for local dev
docker-up:
	docker compose up -d postgres inngest

docker-down:
	docker compose down

# Run Go API server
dev:
	go run ./apps/api/main.go

# Run all Go tests
test:
	go test ./... -v -count=1

# Run a single test by name
test-one:
	go test ./... -v -count=1 -run $(TEST)

# Run E2E tests (requires docker-up + dev running)
test-e2e:
	go test -tags e2e -v -count=1 -timeout=120s ./tests/e2e/

# Build API binary
build:
	go build -o bin/api ./apps/api/main.go

# Run database migrations
migrate:
	go run ./apps/api/main.go migrate

# Lint
lint:
	golangci-lint run ./...

# Frontend dev
web-dev:
	cd apps/web && npm run dev

web-install:
	cd apps/web && npm install

web-build:
	cd apps/web && npm run build

# Playwright E2E tests (auto-starts Next.js dev server)
test-playwright:
	cd apps/web && npx playwright test --reporter=list

# Playwright E2E — onboarding only
test-playwright-onboarding:
	cd apps/web && npx playwright test tests/e2e/onboarding.spec.ts --reporter=list

# Playwright E2E — with browser UI (interactive)
test-playwright-ui:
	cd apps/web && npx playwright test --ui

# Install Playwright browsers (run once)
playwright-install:
	cd apps/web && npx playwright install chromium

# Deploy frontend to Cloudflare
web-deploy:
	cd apps/web && NEXT_PUBLIC_API_URL=https://amazonagent-production.up.railway.app npm run deploy

# Reset and restart assessment (requires API running on 8081)
assess:
	@curl -s -X DELETE "http://localhost:8081/assessment/reset" -H "Authorization: Bearer dev-user-dev-tenant"
	@sleep 1
	@curl -s -X POST "http://localhost:8081/assessment/start" -H "Authorization: Bearer dev-user-dev-tenant" -H "Content-Type: application/json" -d '{}'
	@echo ""
	@echo "Assessment started. Open http://localhost:3001/onboarding"

# Start everything locally for manual testing
local:
	@echo "Starting local dev environment..."
	@echo "1. Starting Docker (postgres + inngest)..."
	docker compose up -d postgres inngest
	@echo "2. Waiting for postgres to be healthy..."
	@sleep 5
	@echo "3. Start the API in another terminal: PORT=8081 make dev"
	@echo "4. Start the frontend in another terminal: make web-dev"
	@echo ""
	@echo "Frontend: http://localhost:3000"
	@echo "API:      http://localhost:8081"
	@echo "Inngest:  http://localhost:8290"
