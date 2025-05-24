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
# DB_DSN=postgresql://postgres:password@localhost:5432/inventario

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
	$(BINARY_PATH) run --addr $(SERVER_ADDR) --upload-location $(UPLOAD_LOCATION) --db-dsn postgresql://postgres:password@localhost:5432/inventario

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

# Run Go tests
.PHONY: test-go
test-go:
	$(CD) $(FRONTEND_DIR) && $(GO_CMD) test -v ./...

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

# Generate help
.PHONY: help
help:
ifeq ($(OS),Windows_NT)
	@echo Available commands:
	@echo   all              - Build everything (default)
	@echo   build            - Build backend and frontend
	@echo   build-backend    - Build only the backend
	@echo   build-frontend   - Build only the frontend
	@echo   run-backend      - Run the backend server
	@echo   run-backend-postgres - Run the backend server with PostgreSQL
	@echo   run-frontend     - Run the frontend dev server
	@echo   run-dev          - Run both servers concurrently (for development)
	@echo   test             - Run all tests
	@echo   test-go          - Run Go tests
	@echo   test-frontend    - Run frontend tests
	@echo   test-e2e         - Run end-to-end tests
	@echo   seed-db          - Seed the database with test data
	@echo   lint             - Run all linters
	@echo   lint-go          - Lint Go code
	@echo   lint-frontend    - Lint frontend code
	@echo   clean            - Clean build artifacts
	@echo   deps             - Install dependencies
else
	@echo "Available commands:"
	@echo "  all              - Build everything (default)"
	@echo "  build            - Build backend and frontend"
	@echo "  build-backend    - Build only the backend"
	@echo "  build-frontend   - Build only the frontend"
	@echo "  run-backend      - Run the backend server"
	@echo "  run-backend-postgres - Run the backend server with PostgreSQL"
	@echo "  run-frontend     - Run the frontend dev server"
	@echo "  run-dev          - Run both servers concurrently (for development)"
	@echo "  test             - Run all tests"
	@echo "  test-go          - Run Go tests"
	@echo "  test-frontend    - Run frontend tests"
	@echo "  test-e2e         - Run end-to-end tests"
	@echo "  seed-db          - Seed the database with test data"
	@echo "  lint             - Run all linters"
	@echo "  lint-go          - Lint Go code"
	@echo "  lint-frontend    - Lint frontend code"
	@echo "  clean            - Clean build artifacts"
	@echo "  deps             - Install dependencies"
endif
