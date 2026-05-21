-- Initial Bootstrap SQL Migrations
--
-- Template variables:
--
--   Username: {{.Username}}
--      - Operational database username
--   UsernameForMigrations: {{.UsernameForMigrations}}
--      - Database username for migrations
--   UsernameForBackgroundWorker: {{.UsernameForBackgroundWorker}}
--      - Database username for background worker
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

-- Create background worker user if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_user WHERE usename = '{{.UsernameForBackgroundWorker}}') THEN
        CREATE USER {{.UsernameForBackgroundWorker}} WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOREPLICATION;
        RAISE NOTICE 'Created user {{.UsernameForBackgroundWorker}}';
    ELSE
        RAISE NOTICE 'User {{.UsernameForBackgroundWorker}} already exists';
    END IF;
END $$;

CREATE EXTENSION IF NOT EXISTS pgcrypto;
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

-- Create migration role for schema changes
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'inventario_background_worker') THEN
        CREATE ROLE inventario_background_worker WITH NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOREPLICATION;
        RAISE NOTICE 'Created role inventario_background_worker';
    END IF;
END $$;

-- Create the cross-tenant admin role.
--
-- inventario_admin is the ONLY role with the BYPASSRLS attribute. The
-- system-admin surfaces (/api/v1/admin/...) read and write rows across
-- every tenant; the application assumes this role via a transaction-scoped
-- `SET LOCAL ROLE inventario_admin` (see store.DoAsAdmin) and resets it on
-- commit/rollback. BYPASSRLS deliberately lives nowhere else: inventario_app
-- and inventario_background_worker stay non-bypass so ordinary request
-- traffic remains fully RLS-enforced and tenant-isolated.
--
-- The pre-existing admin code disabled RLS with `SET LOCAL row_security =
-- off` instead. Postgres rejects that with SQLSTATE 42501 for a role that
-- can neither own the table nor bypass RLS, so every admin group/tenant/user
-- query 500'd on a standard deployment. BYPASSRLS is the supported fix.
--
-- This block is re-run on every (idempotent) bootstrap, so existing
-- databases that were bootstrapped before this change pick up the new role
-- automatically — no separate manual migration step is required.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'inventario_admin') THEN
        CREATE ROLE inventario_admin WITH NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOREPLICATION BYPASSRLS;
        RAISE NOTICE 'Created role inventario_admin';
    ELSE
        -- Ensure the attribute is present even if the role pre-existed
        -- without it (e.g. created manually by an operator).
        ALTER ROLE inventario_admin WITH BYPASSRLS;
        RAISE NOTICE 'Role inventario_admin already exists; ensured BYPASSRLS';
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

-- Grant migration role to the background worker user (only if different from role name)
DO $$
   BEGIN
    IF '{{.UsernameForBackgroundWorker}}' != 'inventario_background_worker' THEN
        GRANT inventario_background_worker TO {{.UsernameForBackgroundWorker}};
        RAISE NOTICE 'Granted inventario_background_worker role to {{.UsernameForBackgroundWorker}}';
    END IF;
END $$;

-- Grant the cross-tenant admin role to the login users that the
-- application connects as. A login user can only `SET ROLE inventario_admin`
-- if it is a member of that role; membership alone does NOT make ordinary
-- queries bypass RLS — BYPASSRLS only takes effect once the role is the
-- session's *active* role, which the app does exclusively inside the
-- transaction-scoped store.DoAsAdmin helper.
DO $$
BEGIN
    IF '{{.Username}}' != 'inventario_admin' THEN
        GRANT inventario_admin TO {{.Username}};
        RAISE NOTICE 'Granted inventario_admin role to {{.Username}}';
    END IF;
    IF '{{.UsernameForBackgroundWorker}}' != 'inventario_admin'
       AND '{{.UsernameForBackgroundWorker}}' != '{{.Username}}' THEN
        GRANT inventario_admin TO {{.UsernameForBackgroundWorker}};
        RAISE NOTICE 'Granted inventario_admin role to {{.UsernameForBackgroundWorker}}';
    END IF;
END $$;

-- App role gets only usage
GRANT USAGE ON SCHEMA public TO inventario_app;

-- Migration role gets schema privileges
GRANT USAGE, CREATE ON SCHEMA public TO inventario_migrator;

-- Background worker role gets schema privileges
GRANT USAGE ON SCHEMA public TO inventario_background_worker;

-- Admin role gets schema usage
GRANT USAGE ON SCHEMA public TO inventario_admin;

-- Default privileges for objects created by migrator role
ALTER DEFAULT PRIVILEGES FOR ROLE inventario_migrator IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO inventario_app;

ALTER DEFAULT PRIVILEGES FOR ROLE inventario_migrator IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO inventario_app;

-- Default privileges for objects created by migrator role for background worker
ALTER DEFAULT PRIVILEGES FOR ROLE inventario_migrator IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO inventario_background_worker;

ALTER DEFAULT PRIVILEGES FOR ROLE inventario_migrator IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO inventario_background_worker;

-- Default privileges for objects created by migrator role for the admin role
ALTER DEFAULT PRIVILEGES FOR ROLE inventario_migrator IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO inventario_admin;

ALTER DEFAULT PRIVILEGES FOR ROLE inventario_migrator IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO inventario_admin;

-- Default privileges for objects created by the current user (whoever runs this bootstrap)
-- This ensures that tables created during migrations get the correct permissions
-- regardless of the actual database username
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO inventario_app;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO inventario_app;

-- Default privileges for objects created by the current user for background worker
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO inventario_background_worker;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO inventario_background_worker;

-- Default privileges for objects created by the current user for the admin role
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO inventario_admin;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO inventario_admin;

-- Grant permissions on all existing tables to inventario_app
-- This is needed for any tables that already exist
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO inventario_app;

-- Grant permissions on all sequences to inventario_app (for auto-increment columns)
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO inventario_app;

ALTER DEFAULT PRIVILEGES FOR ROLE inventario_migrator IN SCHEMA public
    GRANT EXECUTE ON FUNCTIONS TO inventario_app;

ALTER DEFAULT PRIVILEGES FOR ROLE inventario_background_worker IN SCHEMA public
    GRANT EXECUTE ON FUNCTIONS TO inventario_app;

ALTER DEFAULT PRIVILEGES FOR ROLE inventario_migrator IN SCHEMA public
    GRANT EXECUTE ON FUNCTIONS TO inventario_admin;

-- Grant privileges on existing objects to both roles
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO inventario_migrator;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO inventario_migrator;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO inventario_migrator;

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO inventario_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO inventario_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO inventario_app;

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO inventario_background_worker;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO inventario_background_worker;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO inventario_background_worker;

-- Cross-tenant admin role gets the same DML reach as the app role across
-- all existing objects. RLS bypass comes from the BYPASSRLS attribute on
-- the role itself, not from any policy, so no per-table policy is needed.
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO inventario_admin;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO inventario_admin;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO inventario_admin;

