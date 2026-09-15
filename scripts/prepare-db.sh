#!/bin/bash
# prepare-db.sh — Make a database ready to replay the migration chain.
#
# Usage:
#   ./scripts/prepare-db.sh <privileged-dsn>
#
# Example:
#   ./scripts/prepare-db.sh "postgres://postgres:pw@localhost:5432/scratch?sslmode=disable"
#
# The migration chain is not self-sufficient. It references roles
# (inventario_app, inventario_background_worker, …) and the pg_trgm operator
# class, and no migration creates either. Replaying it into a bare database
# fails with `role "inventario_background_worker" does not exist`, and a little
# later with `operator class "gin_trgm_ops" does not exist`.
#
# Four separate things hit that, and each looked like a different bug: the Ptah
# checkpoint shadow database (#2424), stokaro/ptah-action (#2422),
# TestMigrationFilesSyncWithAnnotations, and anyone creating a scratch database
# by hand. This is the one path they should all use, so the prerequisite has a
# name instead of being folklore. See #2423.
#
# It is a thin wrapper over `inventario db bootstrap apply` that adds a check:
# bootstrap succeeding does not by itself prove the extensions are there, and
# that gap is what made the failure show up three steps later.
#
# Prerequisites: go. (Deliberately no psql — see #2423.)

set -euo pipefail

DSN="${1:-}"

if [ -z "${DSN}" ]; then
    echo "usage: $0 <privileged-dsn>" >&2
    echo "" >&2
    echo "  The DSN must be able to create roles and extensions — a superuser, or a" >&2
    echo "  database owner with CREATE on the database." >&2
    exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
GO_DIR="${REPO_ROOT}/go"

echo "👤  Creating roles and extensions..."
(cd "${GO_DIR}" && go run ./cmd/inventario db bootstrap apply --db-dsn="${DSN}")
echo ""

# Bootstrap reports success for the roles half even where the CREATE EXTENSION
# statements were no-ops against a database that never had them — so confirm
# rather than assume. `db migrate up` performs the same check before it applies
# anything, and this gives the same answer one step earlier.
echo "🔍  Verifying the migration chain's prerequisites..."
(cd "${GO_DIR}" && go run ./cmd/inventario db migrate up --db-dsn="${DSN}" --dry-run) >/dev/null
echo "✅  Prerequisites satisfied"
echo ""

echo "🎉  Database is ready. Apply the chain with:"
echo "    cd ${GO_DIR} && go run ./cmd/inventario db migrate up --db-dsn=\"${DSN}\""
echo ""
echo "Note for Ptah checkpoints (#2424): a shadow database additionally needs its"
echo "extensions outside \`public\`, because checkpoint cleans the managed schema and"
echo "refuses while an extension is owned by it. That relocation is tracked there."
