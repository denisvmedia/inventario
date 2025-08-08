# Ptah Improvements Required for Full Multi-Tenancy Support

This document outlines the Ptah migration system enhancements needed to implement complete multi-tenancy support in the inventario application.

**Note**: This document has been updated to correct an initial error regarding GIN index support. Ptah actually provides excellent and comprehensive index support, including GIN indexes for JSONB fields, partial indexes, and advanced features like trigram operators. The missing features are primarily related to Row-Level Security and custom PostgreSQL functions.

## Current Ptah Capabilities

Ptah currently supports the following features that are useful for multi-tenancy:

✅ **Foreign Key Constraints**
```go
//migrator:schema:field name="tenant_id" type="TEXT" not_null="true" foreign="tenants(id)" foreign_key_name="fk_entity_tenant"
```

✅ **Standard B-tree Indexes**
```go
//migrator:schema:index name="idx_users_tenant_id" fields="tenant_id" table="users"
```

✅ **Composite Indexes**
```go
//migrator:schema:index name="idx_commodities_tenant_area" fields="tenant_id,area_id" table="commodities"
```

✅ **GIN Indexes for JSONB Fields**
```go
//migrator:schema:index name="commodities_tags_gin_idx" fields="tags" type="GIN" table="commodities"
//migrator:schema:index name="commodities_extra_serial_numbers_gin_idx" fields="extra_serial_numbers" type="GIN" table="commodities"
```

✅ **Advanced GIN Indexes with Operators**
```go
//migrator:schema:index name="commodities_name_trgm_idx" fields="name" type="GIN" ops="gin_trgm_ops" table="commodities"
//migrator:schema:index name="files_title_trgm_idx" fields="title" type="GIN" ops="gin_trgm_ops" table="files"
```

✅ **Partial Indexes with WHERE Clauses**
```go
//migrator:schema:index name="commodities_active_idx" fields="status,area_id" condition="draft = false" table="commodities"
```

✅ **Unique Constraints**
```go
//migrator:schema:field name="slug" type="TEXT" not_null="true" unique="true"
```

✅ **Default Values and Functions**
```go
//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
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

Some multi-tenancy patterns could benefit from additional advanced PostgreSQL index types.

**Required Features:**
- GiST indexes for specialized data types
- Expression indexes on computed values
- Additional specialized index types

**Required Annotation Format:**
```go
//migrator:schema:index name="idx_spatial_data" fields="location" type="GiST" table="locations"
//migrator:schema:index name="idx_computed_field" fields="UPPER(name)" type="BTREE" table="tenants"
```

**Current Workaround:** Standard B-tree and GIN indexes (which cover most use cases)
**Impact:** Minor - most multi-tenancy patterns are well-served by existing index support.

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

3. **LOW Priority**: Additional Advanced Index Types
   - Minor performance optimizations for specialized use cases
   - Ptah already supports comprehensive indexing including GIN indexes for JSONB fields
   - Current index support covers the vast majority of multi-tenancy patterns

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

**Status**: ✅ **PARTIALLY COMPLETE** - Application-level multi-tenancy implemented

The multi-tenancy implementation provides robust application-level tenant isolation with comprehensive database constraints and performance optimizations. Ptah's excellent index support (including GIN indexes for JSONB fields) ensures optimal query performance for tenant-aware operations.

**What's Working:**
- Complete tenant-aware schema with foreign key constraints
- Comprehensive performance indexes including GIN indexes for JSONB fields
- Application-level tenant isolation with proper data validation
- Full migration support with rollback capabilities

**What's Missing:**
- Database-level tenant isolation (RLS policies)
- Tenant context management (custom PostgreSQL functions)

**Recommendation**: The current implementation is production-ready for most use cases. Consider implementing RLS support in Ptah for enhanced security in highly sensitive multi-tenant applications.

## Alternative Approaches

Given Ptah's excellent schema and index support, the current implementation options are:

1. **Current Implementation** (Recommended): Application-level isolation with comprehensive database constraints
   - ✅ Excellent performance with GIN indexes for JSONB queries
   - ✅ Strong data integrity with foreign key constraints
   - ✅ Full Ptah migration support with rollback capabilities
   - ⚠️ Tenant isolation enforced at application level

2. **Hybrid Approach**: Current implementation + manual RLS setup
   - ✅ All benefits of current implementation
   - ✅ Database-level tenant isolation
   - ⚠️ Manual SQL management outside Ptah

3. **Schema-per-Tenant**: Separate PostgreSQL schemas for each tenant
   - ✅ Complete tenant isolation
   - ⚠️ Requires significant changes to Ptah and application architecture
   - ⚠️ Complex maintenance and migration management

**Recommendation**: The current implementation provides excellent multi-tenancy support for most use cases, with the option to add manual RLS for enhanced security when needed.
