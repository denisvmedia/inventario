#!/bin/sh
set -e

echo "=== RUNNING SCHEMA MIGRATIONS ==="
echo "Database DSN: [configured]"

# Wait for database to be ready with retry mechanism. Budget matches the Helm
# chart's dbRetry.* so every consumer of the same database waits the same
# amount before declaring it gone (#2243).
ATTEMPTS="${INVENTARIO_DB_RETRY_ATTEMPTS:-60}"
INTERVAL="${INVENTARIO_DB_RETRY_INTERVAL_SECONDS:-5}"
echo "Waiting for database to be ready for migrations (up to $ATTEMPTS attempts, ${INTERVAL}s apart)..."
i=1
while [ "$i" -le "$ATTEMPTS" ]; do
  set +e
  inventario db migrate up
  status=$?
  set -e
  if [ "$status" -eq 0 ]; then
    echo "Schema migrations completed successfully"
    exit 0
  fi
  # Exit 3: a previous attempt left a migration half-applied. Retrying cannot
  # clear that — stop while the real error is still on screen (#2416).
  if [ "$status" -eq 3 ]; then
    echo "Schema migrations stopped: a migration is recorded as dirty."
    echo "Reconcile the schema and the revision row before retrying."
    exit "$status"
  fi
  echo "Attempt $i/$ATTEMPTS failed (exit $status), retrying in ${INTERVAL}s..."
  i=$((i + 1))
  sleep "$INTERVAL"
done

echo "Failed to apply schema migrations after $ATTEMPTS attempts"
exit 1
