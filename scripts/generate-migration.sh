#!/bin/bash
# generate-migration.sh — Generate a timestamped migration pair using a
# short-lived Docker PostgreSQL container.
#
# Usage:
#   ./scripts/generate-migration.sh [migration-name]
#
# Examples:
#   ./scripts/generate-migration.sh                          # name defaults to "migration"
#   ./scripts/generate-migration.sh add_email_verifications
#
# Prerequisites: docker, go
#
# The script:
#   1. Spins up a fresh postgres container on a Docker-assigned random host port
#   2. Builds inventool from source
#   3. Runs bootstrap to create required roles and extensions
#   4. Applies all existing migrations (so the generator can diff)
#   5. Runs "inventool db migrations generate" to create the UP/DOWN files
#   6. Removes the container on exit (even on error)

set -euo pipefail

MIGRATION_NAME="${1:-migration}"

# ---------------------------------------------------------------------------
# Paths (all relative to the repository root)
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
GO_DIR="${REPO_ROOT}/go"
BIN_DIR="${REPO_ROOT}/bin"
INVENTOOL="${BIN_DIR}/inventool"
MODELS_DIR="${GO_DIR}/models"
MIGRATIONS_DIR="${GO_DIR}/schema/migrations/_sqldata"

# ---------------------------------------------------------------------------
# Ephemeral Postgres settings
# ---------------------------------------------------------------------------
CONTAINER_NAME="inventario-migrate-gen-$$"
POSTGRES_USER=inventario_gen
POSTGRES_PASSWORD=inventario_gen_pw
POSTGRES_DB=inventario_gen
# POSTGRES_PORT and DSN are populated after the container starts (Docker
# assigns a free host port automatically — see step 1).

# ---------------------------------------------------------------------------
# Cleanup — runs on EXIT so the container is always removed
# ---------------------------------------------------------------------------
cleanup() {
    echo ""
    echo "🧹  Removing temporary container ${CONTAINER_NAME}..."
    docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# 1. Start postgres
# ---------------------------------------------------------------------------
echo "🐘  Starting temporary PostgreSQL container..."
docker run -d \
    --name  "${CONTAINER_NAME}" \
    -e      POSTGRES_USER="${POSTGRES_USER}" \
    -e      POSTGRES_PASSWORD="${POSTGRES_PASSWORD}" \
    -e      POSTGRES_DB="${POSTGRES_DB}" \
    -p      "5432" \
    postgres:17-alpine \
    >/dev/null

# Read the host port Docker assigned to container port 5432/tcp.
POSTGRES_PORT="$(docker inspect \
    --format='{{(index (index .NetworkSettings.Ports "5432/tcp") 0).HostPort}}' \
    "${CONTAINER_NAME}")"
if [ -z "${POSTGRES_PORT}" ]; then
    echo "❌  Failed to determine host port for ${CONTAINER_NAME}"
    exit 1
fi
DSN="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"
echo "🔌  PostgreSQL listening on host port ${POSTGRES_PORT}"

# ---------------------------------------------------------------------------
# 2. Wait for postgres to accept connections
# ---------------------------------------------------------------------------
echo "⏳  Waiting for PostgreSQL to be ready..."
for i in $(seq 1 30); do
    if docker exec "${CONTAINER_NAME}" \
            pg_isready -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" \
            >/dev/null 2>&1; then
        echo "✅  PostgreSQL is ready"
        break
    fi
    if [ "${i}" -eq 30 ]; then
        echo "❌  Timed out waiting for PostgreSQL to start"
        exit 1
    fi
    sleep 1
done

# ---------------------------------------------------------------------------
# 3. Build inventool
# ---------------------------------------------------------------------------
echo ""
echo "🔨  Building inventool..."
mkdir -p "${BIN_DIR}"
(cd "${GO_DIR}/cmd/inventool" && go build -o "${INVENTOOL}" .)
echo "✅  inventool built: ${INVENTOOL}"

# ---------------------------------------------------------------------------
# 4. Bootstrap — creates the roles the migrations reference (inventario_app,
#    inventario_background_worker, inventario_migrator, etc.)
# ---------------------------------------------------------------------------
echo ""
echo "🔧  Running bootstrap (creating required roles/extensions)..."
"${INVENTOOL}" db bootstrap apply \
    --db-dsn="${DSN}" \
    --username="${POSTGRES_USER}"
echo "✅  Bootstrap complete"

# ---------------------------------------------------------------------------
# 5. Apply existing migrations so the generator has something to diff against
# ---------------------------------------------------------------------------
echo ""
echo "📦  Applying existing migrations..."
"${INVENTOOL}" db migrations up --db-dsn="${DSN}"
echo "✅  Existing migrations applied"

# ---------------------------------------------------------------------------
# 6. Generate the new migration
# ---------------------------------------------------------------------------
echo ""
echo "✨  Generating migration: ${MIGRATION_NAME}"
"${INVENTOOL}" db migrations generate "${MIGRATION_NAME}" \
    --db-dsn="${DSN}" \
    --go-entities-dir="${MODELS_DIR}" \
    --migrations-dir="${MIGRATIONS_DIR}"

echo ""
echo "🎉  Done! Check ${MIGRATIONS_DIR} for the new files."

