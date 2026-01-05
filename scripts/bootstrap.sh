#!/bin/sh
set -e

echo "=== RUNNING BOOTSTRAP MIGRATIONS (IDEMPOTENT) ==="
echo "Database DSN: $INVENTARIO_DB_DSN"

# Debug: Show what we're trying to connect to
echo "Trying to connect with:"
echo "  DSN: $INVENTARIO_DB_DSN"
echo "  Bootstrap user: $INVENTARIO_BOOTSTRAP_USERNAME"
echo "  Migration user: $INVENTARIO_BOOTSTRAP_USERNAME_FOR_MIGRATIONS"

# Wait for PostgreSQL to be ready and run bootstrap
echo "Waiting for PostgreSQL to be ready..."
for i in $(seq 1 30); do
  echo "=== Attempt $i ==="

  # Try to run bootstrap directly - it will handle connection and user creation
  if ./inventario db bootstrap apply \
    --username="$INVENTARIO_BOOTSTRAP_USERNAME" \
    --username-for-migrations="$INVENTARIO_BOOTSTRAP_USERNAME_FOR_MIGRATIONS" 2>&1; then
    echo "✅ Bootstrap migrations completed successfully!"
    exit 0
  else
    echo "❌ Bootstrap attempt $i failed, retrying in 3 seconds..."
    sleep 3
  fi
done

echo "Failed to apply bootstrap migrations after 30 attempts"
exit 1
