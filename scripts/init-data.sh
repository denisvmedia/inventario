#!/bin/sh
set -e

if [ ! -f /app/state/data-initialized ]; then
  echo "=== RUNNING INITIAL DATA SETUP ==="
  echo "Database DSN: $INVENTARIO_DB_DSN"

  # Wait for database to be ready with retry mechanism
  echo "Waiting for database to be ready for initial data setup..."
  for i in $(seq 1 15); do
    if ./inventario db migrate data \
      --default-tenant-name="$INVENTARIO_MIGRATE_DATA_DEFAULT_TENANT_NAME" \
      --default-tenant-slug="$INVENTARIO_MIGRATE_DATA_DEFAULT_TENANT_SLUG" \
      --admin-email="$INVENTARIO_MIGRATE_DATA_ADMIN_EMAIL" \
      --admin-password="$INVENTARIO_MIGRATE_DATA_ADMIN_PASSWORD" \
      --admin-name="$INVENTARIO_MIGRATE_DATA_ADMIN_NAME"; then

      echo "Initial tenant and user created successfully"

      # Optionally seed database with example data and settings
      if [ "${SEED_DATABASE:-false}" = "true" ]; then
        echo "=== SEEDING DATABASE WITH EXAMPLE DATA ==="
        echo "Starting temporary server to seed database..."

        # Override DB_DSN to use app user (not migrator) for seeding
        export INVENTARIO_DB_DSN="${INVENTARIO_SEED_DB_DSN:-$INVENTARIO_DB_DSN}"
        echo "Using DSN: $INVENTARIO_DB_DSN (for seeding)"

        # Start server in background
        ./inventario run &
        SERVER_PID=$!

        # Wait for server to be ready
        echo "Waiting for server to be ready..."
        for j in $(seq 1 30); do
          if curl -sf http://localhost:3333/api/health > /dev/null 2>&1; then
            echo "Server is ready"
            break
          fi
          echo "Waiting for server... attempt $j/30"
          sleep 1
        done

        # Seed the database with parameters for the created admin user and tenant
        echo "Calling seed endpoint with user_email=${INVENTARIO_MIGRATE_DATA_ADMIN_EMAIL}, tenant_slug=${INVENTARIO_MIGRATE_DATA_DEFAULT_TENANT_SLUG}..."
        SEED_JSON="{\"user_email\":\"${INVENTARIO_MIGRATE_DATA_ADMIN_EMAIL}\",\"tenant_slug\":\"${INVENTARIO_MIGRATE_DATA_DEFAULT_TENANT_SLUG}\"}"
        echo "JSON payload: $SEED_JSON"
        if printf '%s' "$SEED_JSON" | curl -f -X POST -H "Content-Type: application/json" --data-binary @- http://localhost:3333/api/v1/seed; then
          echo ""
          echo "Database seeded successfully with example data and settings for ${INVENTARIO_MIGRATE_DATA_ADMIN_EMAIL}"
        else
          echo "Warning: Failed to seed database, but continuing..."
        fi

        # Stop the server
        echo "Stopping temporary server..."
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
        sleep 2
      else
        echo "Skipping database seeding (set SEED_DATABASE=true to enable)"
        echo ""
        echo "⚠️  IMPORTANT: You need to configure system settings before using the application"
        echo "   After first login, go to System Settings and set at minimum:"
        echo "   - Main Currency (e.g., USD, EUR, CZK)"
      fi

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
