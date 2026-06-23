#!/bin/sh
set -e

if [ -f /app/state/data-initialized ]; then
  echo "Initial data already setup, skipping..."
  exit 0
fi

echo "=== RUNNING INITIAL DATA SETUP ==="
echo "Database DSN: [configured]"

# Run a command with a short retry loop (the database may still be warming up).
run_with_retry() {
  desc="$1"
  shift
  for i in $(seq 1 15); do
    if "$@"; then
      echo "$desc: completed"
      return 0
    fi
    echo "$desc: attempt $i failed, retrying in 2 seconds..."
    sleep 2
  done
  echo "$desc: failed after 15 attempts"
  return 1
}

# 1) Seed the default tenant + tenant admin user (the regular app login).
run_with_retry "initial data (tenant + admin)" \
  inventario db migrate data \
    --default-tenant-name="$INVENTARIO_MIGRATE_DATA_DEFAULT_TENANT_NAME" \
    --default-tenant-slug="$INVENTARIO_MIGRATE_DATA_DEFAULT_TENANT_SLUG" \
    --admin-email="$INVENTARIO_MIGRATE_DATA_ADMIN_EMAIL" \
    --admin-password="$INVENTARIO_MIGRATE_DATA_ADMIN_PASSWORD" \
    --admin-name="$INVENTARIO_MIGRATE_DATA_ADMIN_NAME"

# 2) Provision the first back-office (platform-operator) user. Back-office users
#    live OUTSIDE the tenant model and authenticate at /backoffice/login.
#    --ensure makes this idempotent: it is a no-op once any operator exists.
run_with_retry "back-office operator" \
  inventario backoffice bootstrap \
    --email="$BACKOFFICE_EMAIL" \
    --name="$BACKOFFICE_NAME" \
    --password="$BACKOFFICE_PASSWORD" \
    --mfa-enforced="$BACKOFFICE_MFA_ENFORCED" \
    --ensure

mkdir -p /app/state
touch /app/state/data-initialized
echo "Initial data setup completed successfully"
