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

-- Check if operational user exists
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_user WHERE usename = '{{.Username}}') THEN
        RAISE EXCEPTION 'User "{{.Username}}" does not exist';
    END IF;
END $$;

-- Check if migration user exists
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_user WHERE usename = '{{.UsernameForMigrations}}') THEN
        RAISE EXCEPTION 'User "{{.UsernameForMigrations}}" does not exist';
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

-- Grant schema usage role to the operational user
GRANT inventario_app TO {{.Username}};
-- Grant migration role to the migration user
GRANT inventario_migrator TO {{.UsernameForMigrations}};

-- Migration role gets schema privileges
GRANT USAGE, CREATE ON SCHEMA public TO inventario_migrator;

-- App role gets only usage
GRANT USAGE ON SCHEMA public TO inventario_app;

-- Default privileges for objects created by migrator role
ALTER DEFAULT PRIVILEGES FOR ROLE inventario_migrator IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO inventario_app;

ALTER DEFAULT PRIVILEGES FOR ROLE inventario_migrator IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO inventario_app;

ALTER DEFAULT PRIVILEGES FOR ROLE inventario_migrator IN SCHEMA public
    GRANT EXECUTE ON FUNCTIONS TO inventario_app;

-- Grant privileges on existing objects to both roles
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO inventario_migrator;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO inventario_migrator;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO inventario_migrator;

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO inventario_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO inventario_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO inventario_app;
