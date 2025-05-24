#!/bin/bash

# PostgreSQL Test Setup Script for Linux/macOS
# This script helps set up PostgreSQL for testing the Inventario application

set -e

# Default values
POSTGRESQL_VERSION="15"
DATABASE_NAME="inventario_test"
USERNAME="inventario_test"
PASSWORD="test_password"
PORT="5432"
USE_DOCKER=false
SHOW_HELP=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --postgresql-version)
            POSTGRESQL_VERSION="$2"
            shift 2
            ;;
        --database-name)
            DATABASE_NAME="$2"
            shift 2
            ;;
        --username)
            USERNAME="$2"
            shift 2
            ;;
        --password)
            PASSWORD="$2"
            shift 2
            ;;
        --port)
            PORT="$2"
            shift 2
            ;;
        --use-docker)
            USE_DOCKER=true
            shift
            ;;
        --help|-h)
            SHOW_HELP=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

if [ "$SHOW_HELP" = true ]; then
    cat << EOF
PostgreSQL Test Setup Script

This script helps set up PostgreSQL for testing the Inventario application.

USAGE:
    ./scripts/setup-postgresql-test.sh [OPTIONS]

OPTIONS:
    --postgresql-version <version>  PostgreSQL version to use (default: 15)
    --database-name <name>          Test database name (default: inventario_test)
    --username <user>               Test user name (default: inventario_test)
    --password <pass>               Test user password (default: test_password)
    --port <port>                   PostgreSQL port (default: 5432)
    --use-docker                    Use Docker instead of local PostgreSQL
    --help, -h                      Show this help message

EXAMPLES:
    # Set up using Docker (recommended)
    ./scripts/setup-postgresql-test.sh --use-docker

    # Set up using local PostgreSQL
    ./scripts/setup-postgresql-test.sh

    # Custom configuration
    ./scripts/setup-postgresql-test.sh --use-docker --database-name "my_test_db" --username "testuser"

AFTER SETUP:
    Set the environment variable:
    export POSTGRES_TEST_DSN="postgres://$USERNAME:$PASSWORD@localhost:$PORT/$DATABASE_NAME?sslmode=disable"

    Then run PostgreSQL tests:
    make test-go-postgresql
EOF
    exit 0
fi

echo "🐘 PostgreSQL Test Setup for Inventario"
echo "======================================="

if [ "$USE_DOCKER" = true ]; then
    echo "🐳 Setting up PostgreSQL using Docker..."
    
    # Check if Docker is available
    if ! command -v docker &> /dev/null; then
        echo "❌ Docker is not installed or not in PATH"
        echo "   Please install Docker from: https://docs.docker.com/get-docker/"
        exit 1
    fi
    echo "✅ Docker is available"

    # Stop and remove existing container if it exists
    CONTAINER_NAME="inventario-postgres-test"
    if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        echo "🧹 Cleaning up existing container..."
        docker stop "$CONTAINER_NAME" >/dev/null 2>&1 || true
        docker rm "$CONTAINER_NAME" >/dev/null 2>&1 || true
    fi

    # Start PostgreSQL container
    echo "🚀 Starting PostgreSQL container..."
    if docker run --name "$CONTAINER_NAME" \
        -e POSTGRES_DB="$DATABASE_NAME" \
        -e POSTGRES_USER="$USERNAME" \
        -e POSTGRES_PASSWORD="$PASSWORD" \
        -p "${PORT}:5432" \
        -d "postgres:$POSTGRESQL_VERSION" >/dev/null; then
        echo "✅ PostgreSQL container started successfully"
    else
        echo "❌ Failed to start PostgreSQL container"
        exit 1
    fi

    # Wait for PostgreSQL to be ready
    echo "⏳ Waiting for PostgreSQL to be ready..."
    MAX_ATTEMPTS=30
    ATTEMPT=0
    
    while [ $ATTEMPT -lt $MAX_ATTEMPTS ]; do
        ATTEMPT=$((ATTEMPT + 1))
        sleep 2
        if docker exec "$CONTAINER_NAME" pg_isready -U "$USERNAME" -d "$DATABASE_NAME" >/dev/null 2>&1; then
            echo "✅ PostgreSQL is ready!"
            break
        fi
        
        if [ $ATTEMPT -eq $MAX_ATTEMPTS ]; then
            echo "❌ PostgreSQL failed to start within expected time"
            echo "   Check container logs: docker logs $CONTAINER_NAME"
            exit 1
        fi
    done

else
    echo "🏠 Setting up PostgreSQL using local installation..."
    
    # Check if PostgreSQL is available
    if ! command -v psql &> /dev/null; then
        echo "❌ PostgreSQL client (psql) is not installed or not in PATH"
        echo "   Please install PostgreSQL:"
        echo "   - Ubuntu/Debian: sudo apt-get install postgresql postgresql-client"
        echo "   - CentOS/RHEL: sudo yum install postgresql postgresql-server"
        echo "   - macOS: brew install postgresql"
        exit 1
    fi
    echo "✅ PostgreSQL client is available"

    # Create database and user
    echo "🔧 Creating test database and user..."
    
    CREATE_SCRIPT="
-- Create test database
DROP DATABASE IF EXISTS $DATABASE_NAME;
CREATE DATABASE $DATABASE_NAME;

-- Create test user
DROP USER IF EXISTS $USERNAME;
CREATE USER $USERNAME WITH PASSWORD '$PASSWORD';

-- Grant privileges
GRANT ALL PRIVILEGES ON DATABASE $DATABASE_NAME TO $USERNAME;
"

    if echo "$CREATE_SCRIPT" | psql -U postgres -h localhost -p "$PORT" >/dev/null 2>&1; then
        echo "✅ Database and user created successfully"
    else
        echo "❌ Failed to create database and user"
        echo "   Make sure PostgreSQL is running and you have admin access"
        echo "   You may need to run: sudo -u postgres psql"
        exit 1
    fi
fi

# Set environment variable
DSN="postgres://${USERNAME}:${PASSWORD}@localhost:${PORT}/${DATABASE_NAME}?sslmode=disable"
export POSTGRES_TEST_DSN="$DSN"

echo ""
echo "🎉 PostgreSQL test setup completed successfully!"
echo ""
echo "📋 Configuration:"
echo "   Database: $DATABASE_NAME"
echo "   Username: $USERNAME"
echo "   Password: $PASSWORD"
echo "   Port: $PORT"
echo "   DSN: $DSN"
echo ""
echo "🔧 Environment variable set for current session:"
echo "   POSTGRES_TEST_DSN=$DSN"
echo ""
echo "💡 To make this permanent, add to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
echo "   export POSTGRES_TEST_DSN=\"$DSN\""
echo ""
echo "🧪 Now you can run PostgreSQL tests:"
echo "   make test-go-postgresql"
echo "   # or"
echo "   cd go && go test -v ./registry/postgresql/..."
echo ""

if [ "$USE_DOCKER" = true ]; then
    echo "🐳 Docker container management:"
    echo "   Stop:    docker stop $CONTAINER_NAME"
    echo "   Start:   docker start $CONTAINER_NAME"
    echo "   Remove:  docker stop $CONTAINER_NAME && docker rm $CONTAINER_NAME"
    echo "   Logs:    docker logs $CONTAINER_NAME"
    echo ""
fi
