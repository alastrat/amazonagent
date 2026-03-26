.PHONY: dev test build migrate lint docker-up docker-down up down logs

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
