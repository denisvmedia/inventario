#!/bin/sh
set -e

echo "=== RUNNING SCHEMA MIGRATIONS ==="
echo "Database DSN: $INVENTARIO_DB_DSN"

# Wait for database to be ready with retry mechanism
echo "Waiting for database to be ready for migrations..."
for i in $(seq 1 15); do
  if ./inventario db migrate up; then
    echo "Schema migrations completed successfully"
    exit 0
  else
    echo "Attempt $i failed, retrying in 2 seconds..."
    sleep 2
  fi
done

echo "Failed to apply schema migrations after 15 attempts"
exit 1
