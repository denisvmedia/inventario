#!/bin/bash
# PostgreSQL initialization script for Inventario
# This script creates the necessary database users for the application
# It runs automatically when the PostgreSQL container starts for the first time

set -e

echo "=== PostgreSQL Initialization for Inventario ==="
echo "POSTGRES_USER: $POSTGRES_USER"
echo "POSTGRES_DB: $POSTGRES_DB"
echo "POSTGRES_MIGRATOR_USER: $POSTGRES_MIGRATOR_USER"

echo "Granting superuser privileges to main user: $POSTGRES_USER"
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Ensure main user has superuser privileges for bootstrap operations
    ALTER USER "$POSTGRES_USER" WITH SUPERUSER CREATEDB CREATEROLE;

    -- Show user privileges (usecreaterole column doesn't exist in PostgreSQL 17)
    SELECT usename, usesuper, usecreatedb FROM pg_user WHERE usename = '$POSTGRES_USER';
EOSQL

echo "Main user privileges updated successfully"

# Create migration user if environment variables are provided
if [ -n "$POSTGRES_MIGRATOR_USER" ] && [ -n "$POSTGRES_MIGRATOR_PASSWORD" ]; then
    echo "Creating migration user: $POSTGRES_MIGRATOR_USER"

    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
        DO \$\$
        BEGIN
            IF NOT EXISTS (SELECT 1 FROM pg_user WHERE usename = '$POSTGRES_MIGRATOR_USER') THEN
                CREATE USER "$POSTGRES_MIGRATOR_USER" WITH PASSWORD '$POSTGRES_MIGRATOR_PASSWORD';
                RAISE NOTICE 'Created migration user: $POSTGRES_MIGRATOR_USER';
            ELSE
                RAISE NOTICE 'Migration user already exists: $POSTGRES_MIGRATOR_USER';
            END IF;
        END \$\$;

        -- Grant necessary permissions to migration user
        GRANT CONNECT ON DATABASE "$POSTGRES_DB" TO "$POSTGRES_MIGRATOR_USER";

        -- Grant schema creation privileges for migrations
        GRANT CREATE ON DATABASE "$POSTGRES_DB" TO "$POSTGRES_MIGRATOR_USER";

        -- Verify migration user was created
        SELECT usename FROM pg_user WHERE usename = '$POSTGRES_MIGRATOR_USER';
EOSQL

    echo "Migration user setup completed"
else
    echo "Migration user environment variables not provided, skipping user creation"
fi

echo "PostgreSQL initialization completed successfully"
