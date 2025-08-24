-- Initial Bootstrap SQL Migrations
--
-- Template variables:
--
--   Username: {{.Username}}
--      - Operational database username
--   UsernameForMigrations: {{.UsernameForMigrations}}
--      - Database username for migrations
--
-- IMPORTANT: Execute these statements with a privileged database user
--

-- Create operational user if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_user WHERE usename = '{{.Username}}') THEN
        CREATE USER {{.Username}} WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOREPLICATION;
        RAISE NOTICE 'Created user {{.Username}}';
    ELSE
        RAISE NOTICE 'User {{.Username}} already exists';
    END IF;
END $$;

-- Create migration user if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_user WHERE usename = '{{.UsernameForMigrations}}') THEN
        CREATE USER {{.UsernameForMigrations}} WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOREPLICATION;
        RAISE NOTICE 'Created user {{.UsernameForMigrations}}';
    ELSE
        RAISE NOTICE 'User {{.UsernameForMigrations}} already exists';
    END IF;
END $$;

CREATE EXTENSION IF NOT EXISTS btree_gin;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Create application role for RLS policies
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'inventario_app') THEN
        CREATE ROLE inventario_app WITH NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOREPLICATION;
        RAISE NOTICE 'Created role inventario_app';
    END IF;
END $$;

-- Create migration role for schema changes
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'inventario_migrator') THEN
        CREATE ROLE inventario_migrator WITH NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOREPLICATION;
        RAISE NOTICE 'Created role inventario_migrator';
    END IF;
END $$;

-- Grant schema usage role to the operational user (only if different from role name)
DO $$
BEGIN
    IF '{{.Username}}' != 'inventario_app' THEN
        GRANT inventario_app TO {{.Username}};
        RAISE NOTICE 'Granted inventario_app role to {{.Username}}';
    END IF;
END $$;

-- Grant migration role to the migration user (only if different from role name)
DO $$
BEGIN
    IF '{{.UsernameForMigrations}}' != 'inventario_migrator' THEN
        GRANT inventario_migrator TO {{.UsernameForMigrations}};
        RAISE NOTICE 'Granted inventario_migrator role to {{.UsernameForMigrations}}';
    END IF;
END $$;

-- Migration role gets schema privileges
GRANT USAGE, CREATE ON SCHEMA public TO inventario_migrator;

-- App role gets only usage
GRANT USAGE ON SCHEMA public TO inventario_app;

-- Default privileges for objects created by migrator role
ALTER DEFAULT PRIVILEGES FOR ROLE inventario_migrator IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO inventario_app;

ALTER DEFAULT PRIVILEGES FOR ROLE inventario_migrator IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO inventario_app;

-- Default privileges for objects created by the current user (whoever runs this bootstrap)
-- This ensures that tables created during migrations get the correct permissions
-- regardless of the actual database username
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO inventario_app;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO inventario_app;

-- Grant permissions on all existing tables to inventario_app
-- This is needed for any tables that already exist
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO inventario_app;

-- Grant permissions on all sequences to inventario_app (for auto-increment columns)
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO inventario_app;

ALTER DEFAULT PRIVILEGES FOR ROLE inventario_migrator IN SCHEMA public
    GRANT EXECUTE ON FUNCTIONS TO inventario_app;

-- Grant privileges on existing objects to both roles
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO inventario_migrator;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO inventario_migrator;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO inventario_migrator;

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO inventario_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO inventario_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO inventario_app;

-- Create default tenant if it doesn't exist (idempotent)
-- This must run after migrations create the tenants table
DO $$
BEGIN
    -- Check if tenants table exists and if default tenant doesn't exist
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'tenants' AND table_schema = 'public') THEN
        IF NOT EXISTS (SELECT 1 FROM tenants WHERE id = 'test-tenant-id') THEN
            INSERT INTO tenants (id, name, slug, status, created_at, updated_at)
            VALUES (
                'test-tenant-id',
                'Test Organization',
                'test-org',
                'active',
                CURRENT_TIMESTAMP,
                CURRENT_TIMESTAMP
            );
            RAISE NOTICE 'Created default tenant: Test Organization';
        ELSE
            RAISE NOTICE 'Default tenant already exists';
        END IF;
    ELSE
        RAISE NOTICE 'Tenants table does not exist yet - skipping default tenant creation';
    END IF;
END $$;
