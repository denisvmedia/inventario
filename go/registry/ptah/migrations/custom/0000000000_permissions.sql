-- Grant the inventario_app role to the operational user (idempotent)
DO $$
DECLARE
op_user text := '{{OP_USER}}';
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_auth_members am
        JOIN pg_roles r1 ON am.roleid = r1.oid
        JOIN pg_roles r2 ON am.member = r2.oid
        WHERE r1.rolname = 'inventario_app'
          AND r2.rolname = op_user
    ) THEN
        EXECUTE format('GRANT inventario_app TO %I', op_user);
END IF;
END $$;

-- Ensure inventario_app role has all current permissions (idempotent)
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO inventario_app;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO inventario_app;
GRANT ALL PRIVILEGES ON ALL FUNCTIONS IN SCHEMA public TO inventario_app;
GRANT USAGE ON SCHEMA public TO inventario_app;

-- Ensure default privileges for future objects (idempotent)
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO inventario_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO inventario_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON FUNCTIONS TO inventario_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TYPES TO inventario_app;

-- Grant specific permissions that might be needed for RLS or special functions
GRANT EXECUTE ON FUNCTION get_current_tenant_id() TO inventario_app;
GRANT EXECUTE ON FUNCTION set_tenant_context(TEXT) TO inventario_app;

-- Ensure the operational user can connect to the database
GRANT CONNECT ON DATABASE {{DB_NAME}} TO {{OP_USER}};
GRANT inventario_app TO {{OP_USER}};

-- If using RLS, ensure the operational user can bypass RLS for admin operations (optional)
-- ALTER USER '<operational_user>' SET row_security = off;
