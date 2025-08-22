#!/bin/sh
set -e

if [ ! -f /app/state/data-initialized ]; then
  echo "=== RUNNING INITIAL DATA SETUP ==="
  echo "Database DSN: $INVENTARIO_DATABASE_DB_DSN"

  # Wait for database to be ready with retry mechanism
  echo "Waiting for database to be ready for initial data setup..."
  for i in $(seq 1 15); do
    if ./inventario db migrate data \
      --default-tenant-name="$INVENTARIO_MIGRATE_DATA_DEFAULT_TENANT_NAME" \
      --default-tenant-slug="$INVENTARIO_MIGRATE_DATA_DEFAULT_TENANT_SLUG" \
      --admin-email="$INVENTARIO_MIGRATE_DATA_ADMIN_EMAIL" \
      --admin-password="$INVENTARIO_MIGRATE_DATA_ADMIN_PASSWORD" \
      --admin-name="$INVENTARIO_MIGRATE_DATA_ADMIN_NAME"; then
      mkdir -p /app/state
      touch /app/state/data-initialized
      echo "Initial data setup completed successfully"
      exit 0
    else
      echo "Attempt $i failed, retrying in 2 seconds..."
      sleep 2
    fi
  done

  echo "Failed to setup initial data after 15 attempts"
  exit 1
else
  echo "Initial data already setup, skipping..."
fi
