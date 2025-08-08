# Ptah Improvements Required for Full Multi-Tenancy Support

This document outlines the Ptah migration system enhancements needed to implement complete multi-tenancy support in the inventario application. Currently, Ptah provides excellent support for basic schema management but lacks some advanced PostgreSQL features required for robust multi-tenancy.

## Current Ptah Capabilities

Ptah currently supports the following features that are useful for multi-tenancy:

✅ **Foreign Key Constraints**
```go
//migrator:schema:field name="tenant_id" type="TEXT" not_null="true" foreign="tenants(id)" foreign_key_name="fk_entity_tenant"
```

✅ **Indexes**
```go
//migrator:schema:index name="idx_users_tenant_id" fields="tenant_id" table="users"
```

✅ **Unique Constraints**
```go
//migrator:schema:field name="slug" type="TEXT" not_null="true" unique="true"
```

✅ **Default Values and Functions**
```go
//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
```

✅ **Composite Indexes**
```go
//migrator:schema:index name="idx_commodities_tenant_area" fields="tenant_id,area_id" table="commodities"
```

## Missing Features for Multi-Tenancy

The following PostgreSQL features are essential for robust multi-tenancy but are not currently supported by Ptah:

### 1. Row-Level Security (RLS) Policies

**Priority: HIGH**

RLS is the cornerstone of database-level tenant isolation. We need the ability to define RLS policies through Ptah annotations.

**Required Annotation Format:**
```go
//migrator:schema:rls:enable table="users"
//migrator:schema:rls:policy name="user_tenant_isolation" table="users" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id()"
```

**Current Workaround:** Manual SQL execution after migration
**Impact:** Without RLS, tenant isolation must be enforced at the application level, which is error-prone and less secure.

### 2. Custom PostgreSQL Functions

**Priority: HIGH**

Multi-tenancy requires custom PostgreSQL functions for tenant context management.

**Required Functions:**
- `set_tenant_context(tenant_id TEXT)` - Sets the current tenant for the session
- `get_current_tenant_id()` - Returns the current tenant ID from session

**Required Annotation Format:**
```go
//migrator:schema:function name="set_tenant_context" params="tenant_id_param TEXT" returns="VOID" language="plpgsql" security="DEFINER" body="BEGIN PERFORM set_config('app.current_tenant_id', tenant_id_param, false); END;"
```

**Current Workaround:** Manual SQL execution after migration
**Impact:** Cannot implement session-based tenant context, limiting RLS effectiveness.

### 3. Database Roles and Permissions

**Priority: MEDIUM**

Multi-tenancy benefits from dedicated database roles with specific permissions.

**Required Features:**
- Create application-specific database roles
- Grant/revoke permissions on tables and schemas
- Set default privileges for future objects

**Required Annotation Format:**
```go
//migrator:schema:role name="inventario_app" login="false"
//migrator:schema:grant role="inventario_app" on="ALL TABLES IN SCHEMA public" privileges="SELECT,INSERT,UPDATE,DELETE"
```

**Current Workaround:** Manual role management
**Impact:** Less secure database access patterns, manual permission management.

### 4. Advanced Index Types

**Priority: LOW**

Some multi-tenancy patterns benefit from advanced PostgreSQL index types.

**Required Features:**
- Partial indexes with WHERE clauses
- Expression indexes
- GIN/GiST indexes for JSONB fields

**Required Annotation Format:**
```go
//migrator:schema:index name="idx_active_tenants" fields="status" table="tenants" where="status = 'active'"
//migrator:schema:index name="idx_tenant_settings_gin" fields="settings" table="tenants" type="GIN"
```

**Current Workaround:** Standard B-tree indexes
**Impact:** Suboptimal query performance for some tenant-aware queries.

### 5. Data Migration and Seeding

**Priority: MEDIUM**

Multi-tenancy often requires data migration and default tenant creation.

**Required Features:**
- Execute custom SQL during migration (INSERT statements)
- Conditional data migration (IF NOT EXISTS patterns)
- Migration rollback with data cleanup

**Required Annotation Format:**
```go
//migrator:schema:data:insert table="tenants" values="('default-tenant', 'Default Tenant', 'default', 'active', '{}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)" on_conflict="DO NOTHING"
```

**Current Workaround:** Manual data insertion after migration
**Impact:** Incomplete migration process, manual setup required.

## Implementation Priority

1. **HIGH Priority**: RLS Policies and Custom Functions
   - These are essential for secure multi-tenancy
   - Without these, tenant isolation is not guaranteed at the database level

2. **MEDIUM Priority**: Database Roles and Data Migration
   - Important for production deployments
   - Can be worked around with manual setup

3. **LOW Priority**: Advanced Index Types
   - Performance optimizations
   - Standard indexes are sufficient for basic functionality

## Recommended Ptah Enhancement Approach

### Phase 1: RLS Support
Add support for Row-Level Security through new annotation types:
- `//migrator:schema:rls:enable`
- `//migrator:schema:rls:policy`

### Phase 2: Function Support
Add support for custom PostgreSQL functions:
- `//migrator:schema:function`
- Support for PL/pgSQL and SQL functions

### Phase 3: Role Management
Add support for database roles and permissions:
- `//migrator:schema:role`
- `//migrator:schema:grant`

### Phase 4: Data Migration
Add support for data insertion and migration:
- `//migrator:schema:data:insert`
- `//migrator:schema:data:update`

## Current Implementation Status

**Status**: ⚠️ **BLOCKED** - Waiting for Ptah enhancements

The multi-tenancy implementation is currently blocked on the lack of RLS and custom function support in Ptah. While we can implement basic multi-tenancy with foreign keys and application-level filtering, true database-level tenant isolation requires the missing features listed above.

**Recommendation**: Implement the HIGH priority features in Ptah before proceeding with the multi-tenancy implementation, or accept the security limitations of application-level tenant isolation.

## Alternative Approaches

If Ptah enhancements are not immediately available, consider:

1. **Hybrid Approach**: Use Ptah for schema, manual SQL for RLS
2. **Application-Level Isolation**: Rely entirely on application code for tenant filtering
3. **Schema-per-Tenant**: Use separate PostgreSQL schemas for each tenant (requires significant Ptah changes)

Each alternative has trade-offs in terms of security, performance, and maintenance complexity.
