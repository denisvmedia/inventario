# Variables
GO_CMD=go
BINARY_NAME=inventario
BIN_DIR=bin
FRONTEND_DIR=frontend
BACKEND_DIR=go

# Detect OS for platform-specific commands
ifeq ($(OS),Windows_NT)
    BINARY_EXT=.exe
    MKDIR=if not exist $(subst /,\,$(1)) mkdir $(subst /,\,$(1))
    RM=if exist $(subst /,\,$(1)) rmdir /s /q $(subst /,\,$(1))
    SHELL=cmd.exe
    CHECK_DIR=if not exist $(subst /,\,$(1))
    SEP=\\
    CURRENT_DIR=$(subst /,\,$(shell cd))
    CD=cd /d
    CURL=curl
else
    BINARY_EXT=
    MKDIR=mkdir -p $(1)
    RM=rm -rf $(1)
    SHELL=/bin/bash
    CHECK_DIR=[ ! -d $(1) ]
    SEP=/
    CURRENT_DIR=$(shell pwd)
    CD=cd
    CURL=curl
endif

BINARY_PATH=$(BIN_DIR)/$(BINARY_NAME)$(BINARY_EXT)

# Server configuration
SERVER_ADDR=:3333
UPLOAD_LOCATION=file://$(CURRENT_DIR)/uploads?create_dir=1
DB_DSN=memory://

# Database configuration examples
# Memory database (default)
# DB_DSN=memory://
# BoltDB database
# DB_DSN=boltdb://$(CURRENT_DIR)/data/inventario.db
# PostgreSQL database
# DB_DSN=postgres://postgres:password@localhost:5432/inventario

# Default target
.PHONY: all
all: build

# Build everything
.PHONY: build
build: build-frontend build-backend

# Build the Go backend
.PHONY: build-backend
build-backend:
        $(call MKDIR,$(BIN_DIR))
        $(CD) $(BACKEND_DIR) && $(GO_CMD) build -tags with_frontend -o ../$(BINARY_PATH) .

.PHONY: build-backend-nofe
build-backend-nofe:
        $(call MKDIR,$(BIN_DIR))
        $(CD) $(BACKEND_DIR) && $(GO_CMD) build -o ../$(BINARY_PATH) .

# Build the frontend
.PHONY: build-frontend
build-frontend:
	$(CD) $(FRONTEND_DIR) && npm install && npm run build

# Run the backend server
.PHONY: run-backend
run-backend: build-backend
	$(BINARY_PATH) run --addr $(SERVER_ADDR) --upload-location $(UPLOAD_LOCATION) --db-dsn $(DB_DSN)

# Run the backend server with PostgreSQL
.PHONY: run-backend-postgres
run-backend-postgres: build-backend
	$(BINARY_PATH) run --addr $(SERVER_ADDR) --upload-location $(UPLOAD_LOCATION) --db-dsn postgres://postgres:password@localhost:5432/inventario

# Run the frontend dev server
.PHONY: run-frontend
run-frontend:
	$(CD) $(FRONTEND_DIR) && npm run serve

# Run both servers concurrently (for development)
.PHONY: run-dev
run-dev:
	@echo "Starting development servers..."
ifeq ($(OS),Windows_NT)
	start /B make run-backend
	start /B make run-frontend
else
	$(MAKE) -j2 run-backend run-frontend
endif

# Run Go tests (excluding PostgreSQL)
.PHONY: test-go
test-go:
ifeq ($(OS),Windows_NT)
	$(CD) $(BACKEND_DIR) && for /f %%i in ('$(GO_CMD) list ./... ^| findstr /v "registry/postgres"') do $(GO_CMD) test -v %%i
else
	$(CD) $(BACKEND_DIR) && $(GO_CMD) test -v $$($(GO_CMD) list ./... | grep -v '/registry/postgres')
endif

# Run PostgreSQL registry tests
.PHONY: test-go-postgres
test-go-postgres:
	@echo "Running PostgreSQL registry tests..."
ifeq ($(OS),Windows_NT)
	@if not defined POSTGRES_TEST_DSN ( \
		echo ❌ POSTGRES_TEST_DSN environment variable is not set && \
		echo    Example: set POSTGRES_TEST_DSN=postgres://user:password@localhost:5432/inventario_test?sslmode=disable && \
		exit /b 1 \
	)
else
	@if [ -z "$(POSTGRES_TEST_DSN)" ]; then \
		echo "❌ POSTGRES_TEST_DSN environment variable is not set"; \
		echo "   Example: export POSTGRES_TEST_DSN='postgres://user:password@localhost:5432/inventario_test?sslmode=disable'"; \
		exit 1; \
	fi
endif
	$(CD) $(BACKEND_DIR) && $(GO_CMD) test -v ./registry/postgres/...

# Run all Go tests including PostgreSQL
.PHONY: test-go-all
test-go-all:
	$(CD) $(BACKEND_DIR) && $(GO_CMD) test -v ./...

# Run frontend tests
.PHONY: test-frontend
test-frontend:
	$(CD) $(FRONTEND_DIR) && npm run test

# Run all tests
.PHONY: test
test: test-go test-frontend

# Run end-to-end tests
.PHONY: test-e2e
test-e2e:
	$(CD) $(FRONTEND_DIR) && npm run test:e2e

# Seed the database
.PHONY: seed-db
seed-db:
	@echo "Seeding the database..."
	$(CURL) -X POST http://localhost:3333/api/v1/seed

# Lint Go code
.PHONY: lint-go
lint-go:
	$(CD) $(BACKEND_DIR) && golangci-lint run

# Lint frontend code
.PHONY: lint-frontend
lint-frontend:
	$(CD) $(FRONTEND_DIR) && npm run lint

# Run all linters
.PHONY: lint
lint: lint-go lint-frontend

# Clean build artifacts
.PHONY: clean
clean:
	$(call RM,$(BIN_DIR))
	$(CD) $(FRONTEND_DIR) && npm run clean

# Install dependencies
.PHONY: deps
deps:
	$(CD) $(BACKEND_DIR)
	$(GO_CMD) mod download
	$(GO_CMD) mod tidy
	$(CD) $(FRONTEND_DIR) && npm install

# Production Docker operations
.PHONY: docker-build
docker-build:
	docker build --target production -t inventario:latest .

.PHONY: docker-up
docker-up:
	docker-compose --profile production up -d

.PHONY: docker-down
docker-down:
	docker-compose --profile production down

.PHONY: docker-logs
docker-logs:
	docker-compose --profile production logs -f

.PHONY: docker-clean
docker-clean:
	docker-compose --profile production down -v
	docker system prune -f

# Development Docker operations
.PHONY: docker-dev-up
docker-dev-up:
	docker-compose --profile dev up -d

.PHONY: docker-dev-down
docker-dev-down:
	docker-compose --profile dev down

.PHONY: docker-dev-logs
docker-dev-logs:
	docker-compose --profile dev logs -f

# Test Docker operations
.PHONY: docker-test-build
docker-test-build:
	docker build --target test-runner -t inventario:test .

.PHONY: docker-test-up
docker-test-up:
	docker-compose --profile test up -d postgres-test

.PHONY: docker-test-down
docker-test-down:
	docker-compose --profile test down

.PHONY: docker-test-clean
docker-test-clean:
	docker-compose --profile test down -v
	docker rmi inventario:test 2>/dev/null || true

.PHONY: docker-test-migrate
docker-test-migrate:
	@echo "Running database migrations in Docker..."
	docker-compose --profile test run --rm inventario-migrate

.PHONY: docker-test-go
docker-test-go:
	@echo "Running Go tests in Docker..."
	docker-compose --profile test run --rm inventario-test

.PHONY: docker-test-go-postgres
docker-test-go-postgres:
	@echo "Running PostgreSQL tests in Docker..."
	docker-compose --profile test run --rm inventario-test-postgres

.PHONY: docker-test-logs
docker-test-logs:
	docker-compose --profile test logs -f

# Generate help
.PHONY: help
help:
ifeq ($(OS),Windows_NT)
	@echo Available commands:
	@echo   all              - Build everything (default)
	@echo   build            - Build backend and frontend
       @echo   build-backend    - Build backend with embedded frontend
       @echo   build-backend-nofe - Build backend without frontend
	@echo   build-frontend   - Build only the frontend
	@echo   run-backend      - Run the backend server
	@echo   run-backend-postgres - Run the backend server with PostgreSQL
	@echo   run-frontend     - Run the frontend dev server
	@echo   run-dev          - Run both servers concurrently (for development)
	@echo   test             - Run all tests
	@echo   test-go          - Run Go tests (excluding PostgreSQL)
	@echo   test-go-postgres - Run PostgreSQL registry tests
	@echo   test-go-all      - Run all Go tests including PostgreSQL
	@echo   test-frontend    - Run frontend tests
	@echo   test-e2e         - Run end-to-end tests
	@echo   seed-db          - Seed the database with test data
	@echo   lint             - Run all linters
	@echo   lint-go          - Lint Go code
	@echo   lint-frontend    - Lint frontend code
	@echo   clean            - Clean build artifacts
	@echo   deps             - Install dependencies
	@echo   docker-build     - Build Docker image for production
	@echo   docker-up        - Start Docker services (production)
	@echo   docker-down      - Stop Docker services (production)
	@echo   docker-logs      - View Docker logs (production)
	@echo   docker-clean     - Clean Docker containers and volumes (production)
	@echo   docker-dev-up    - Start Docker services (development)
	@echo   docker-dev-down  - Stop Docker services (development)
	@echo   docker-dev-logs  - View Docker logs (development)
	@echo   docker-test-build - Build Docker test image
	@echo   docker-test-up   - Start Docker test database
	@echo   docker-test-down - Stop Docker test services
	@echo   docker-test-clean - Clean Docker test containers and volumes
	@echo   docker-test-migrate - Run database migrations in Docker
	@echo   docker-test-go   - Run Go tests in Docker
	@echo   docker-test-go-postgres - Run PostgreSQL tests in Docker
	@echo   docker-test-logs - View Docker test logs
else
	@echo "Available commands:"
	@echo "  all              - Build everything (default)"
	@echo "  build            - Build backend and frontend"
       @echo "  build-backend    - Build backend with embedded frontend"
       @echo "  build-backend-nofe - Build backend without frontend"
	@echo "  build-frontend   - Build only the frontend"
	@echo "  run-backend      - Run the backend server"
	@echo "  run-backend-postgres - Run the backend server with PostgreSQL"
	@echo "  run-frontend     - Run the frontend dev server"
	@echo "  run-dev          - Run both servers concurrently (for development)"
	@echo "  test             - Run all tests"
	@echo "  test-go          - Run Go tests (excluding PostgreSQL)"
	@echo "  test-go-postgres - Run PostgreSQL registry tests"
	@echo "  test-go-all      - Run all Go tests including PostgreSQL"
	@echo "  test-frontend    - Run frontend tests"
	@echo "  test-e2e         - Run end-to-end tests"
	@echo "  seed-db          - Seed the database with test data"
	@echo "  lint             - Run all linters"
	@echo "  lint-go          - Lint Go code"
	@echo "  lint-frontend    - Lint frontend code"
	@echo "  clean            - Clean build artifacts"
	@echo "  deps             - Install dependencies"
	@echo "  docker-build     - Build Docker image for production"
	@echo "  docker-up        - Start Docker services (production)"
	@echo "  docker-down      - Stop Docker services (production)"
	@echo "  docker-logs      - View Docker logs (production)"
	@echo "  docker-clean     - Clean Docker containers and volumes (production)"
	@echo "  docker-dev-up    - Start Docker services (development)"
	@echo "  docker-dev-down  - Stop Docker services (development)"
	@echo "  docker-dev-logs  - View Docker logs (development)"
	@echo "  docker-test-build - Build Docker test image"
	@echo "  docker-test-up   - Start Docker test database"
	@echo "  docker-test-down - Stop Docker test services"
	@echo "  docker-test-clean - Clean Docker test containers and volumes"
	@echo "  docker-test-migrate - Run database migrations in Docker"
	@echo "  docker-test-go   - Run Go tests in Docker"
	@echo "  docker-test-go-postgres - Run PostgreSQL tests in Docker"
	@echo "  docker-test-logs - View Docker test logs"
endif
