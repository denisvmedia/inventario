# Multi-Tenant Implementation Guide for Inventario

## Table of Contents

1. [Current Architecture Analysis](#current-architecture-analysis)
2. [Multi-Tenancy Strategy Overview](#multi-tenancy-strategy-overview)
3. [Phase-by-Phase Implementation](#phase-by-phase-implementation)
4. [Migration Strategy](#migration-strategy)
5. [Security Considerations](#security-considerations)
6. [Performance Optimization](#performance-optimization)
7. [Testing Strategy](#testing-strategy)
8. [Configuration Management](#configuration-management)

## Current Architecture Analysis

### Existing System Structure

**Technology Stack:**
- **Backend**: Go 1.24+ with Chi router, PostgreSQL primary database
- **Frontend**: Vue.js 3 + TypeScript with PrimeVue components
- **Database**: PostgreSQL with Ptah migrations, supports multiple backends
- **File Storage**: Go Cloud Development Kit (local, S3, Azure, GCS)
- **Authentication**: Currently none - single-user application

**Data Model Hierarchy:**
```
Locations (Top-level containers)
├── Areas (Sub-containers within locations)
    └── Commodities (Individual items)
        ├── Images (Visual documentation)
        ├── Invoices (Purchase documentation)
        └── Manuals (Product documentation)
        └── Files (Generic file system)
```

**Key Architectural Components:**
- **Registry Pattern**: Clean abstraction for different database backends
- **EntityID Structure**: Base entity with string IDs
- **Ptah Migrations**: Schema management with Go struct annotations
- **No Authentication**: Direct API access without user management

### Current Entity Structure

All entities inherit from `EntityID`:
```go
type EntityID struct {
    //migrator:schema:field name="id" type="TEXT" primary="true"
    ID string `json:"id" db:"id" userinput:"false"`
}
```

**Current Tables:**
- `locations` - Top-level containers
- `areas` - Sub-containers within locations  
- `commodities` - Individual inventory items
- `files` - Generic file entities with linking
- `exports` - Data export records
- `images`, `invoices`, `manuals` - Legacy file types
- `settings` - Global application settings

## Multi-Tenancy Strategy Overview

### Recommended Approach: Row-Level Security (RLS)

**Why RLS?**
- **Security**: Database-level isolation prevents data leaks
- **Performance**: Single database with efficient querying
- **Simplicity**: Minimal application-level changes required
- **PostgreSQL Native**: Leverages existing database capabilities

**Alternative Approaches Considered:**
- **Database-per-tenant**: Too complex for current scale
- **Schema-per-tenant**: Maintenance overhead too high
- **Application-level filtering**: Security risks too high

### Implementation Phases

1. **Phase 1**: Foundation - Tenant-aware infrastructure (Weeks 1-2)
2. **Phase 2**: Database schema migration (Week 3)
3. **Phase 3**: Registry layer enhancement (Week 4)
4. **Phase 4**: API layer modifications (Week 5)
5. **Phase 5**: Frontend integration (Week 6)
6. **Phase 6**: Configuration and deployment (Week 7)

## Phase-by-Phase Implementation

### Phase 1: Foundation - Tenant-Aware Infrastructure

#### 1.1 Create Tenant Model

Create `go/models/tenant.go`:
```go
package models

import (
    "context"
    "regexp"
    "time"

    "github.com/jellydator/validation"
    "github.com/denisvmedia/inventario/models/rules"
)

//migrator:schema:table name="tenants"
type Tenant struct {
    //migrator:embedded mode="inline"
    EntityID
    
    //migrator:schema:field name="name" type="TEXT" not_null="true"
    Name string `json:"name" db:"name"`
    
    //migrator:schema:field name="slug" type="TEXT" not_null="true" unique="true"
    Slug string `json:"slug" db:"slug"`
    
    //migrator:schema:field name="domain" type="TEXT"
    Domain string `json:"domain" db:"domain"`
    
    //migrator:schema:field name="status" type="TEXT" not_null="true" default="active"
    Status TenantStatus `json:"status" db:"status"`
    
    //migrator:schema:field name="settings" type="JSONB"
    Settings map[string]any `json:"settings" db:"settings"`
    
    //migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
    CreatedAt time.Time `json:"created_at" db:"created_at"`
    
    //migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
    UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type TenantStatus string

const (
    TenantStatusActive    TenantStatus = "active"
    TenantStatusSuspended TenantStatus = "suspended"
    TenantStatusInactive  TenantStatus = "inactive"
)

func (t *Tenant) ValidateWithContext(ctx context.Context) error {
    return validation.ValidateStructWithContext(ctx, t,
        validation.Field(&t.Name, rules.NotEmpty, validation.Length(1, 100)),
        validation.Field(&t.Slug, rules.NotEmpty, validation.Length(1, 50), 
            validation.Match(regexp.MustCompile(`^[a-z0-9-]+$`))),
        validation.Field(&t.Status, validation.Required),
    )
}
```

#### 1.2 Enhanced EntityID with Tenant Support

Modify `go/models/models.go` to add tenant-aware entity:
```go
// TenantAwareEntityID extends EntityID with tenant information
type TenantAwareEntityID struct {
    //migrator:schema:field name="id" type="TEXT" primary="true"
    ID string `json:"id" db:"id" userinput:"false"`
    
    //migrator:schema:field name="tenant_id" type="TEXT" not_null="true" foreign="tenants(id)" foreign_key_name="fk_entity_tenant"
    TenantID string `json:"tenant_id" db:"tenant_id" userinput:"false"`
}

func (i *TenantAwareEntityID) GetID() string {
    return i.ID
}

func (i *TenantAwareEntityID) SetID(id string) {
    i.ID = id
}

func (i *TenantAwareEntityID) GetTenantID() string {
    return i.TenantID
}

func (i *TenantAwareEntityID) SetTenantID(tenantID string) {
    i.TenantID = tenantID
}
```

#### 1.3 User Model and Authentication

Create `go/models/user.go`:
```go
package models

import (
    "context"
    "time"

    "github.com/jellydator/validation"
    "golang.org/x/crypto/bcrypt"
)

//migrator:schema:table name="users"
type User struct {
    //migrator:embedded mode="inline"
    EntityID
    
    //migrator:schema:field name="email" type="TEXT" not_null="true" unique="true"
    Email string `json:"email" db:"email"`
    
    //migrator:schema:field name="password_hash" type="TEXT" not_null="true"
    PasswordHash string `json:"-" db:"password_hash" userinput:"false"`
    
    //migrator:schema:field name="name" type="TEXT" not_null="true"
    Name string `json:"name" db:"name"`
    
    //migrator:schema:field name="role" type="TEXT" not_null="true" default="user"
    Role UserRole `json:"role" db:"role"`
    
    //migrator:schema:field name="tenant_id" type="TEXT" not_null="true" foreign="tenants(id)" foreign_key_name="fk_user_tenant"
    TenantID string `json:"tenant_id" db:"tenant_id"`
    
    //migrator:schema:field name="is_active" type="BOOLEAN" not_null="true" default="true"
    IsActive bool `json:"is_active" db:"is_active"`
    
    //migrator:schema:field name="last_login_at" type="TIMESTAMP"
    LastLoginAt *time.Time `json:"last_login_at" db:"last_login_at"`
    
    //migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
    CreatedAt time.Time `json:"created_at" db:"created_at"`
    
    //migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
    UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type UserRole string

const (
    UserRoleAdmin UserRole = "admin"
    UserRoleUser  UserRole = "user"
)

func (u *User) SetPassword(password string) error {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return err
    }
    u.PasswordHash = string(hash)
    return nil
}

func (u *User) CheckPassword(password string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
    return err == nil
}
```

#### 1.4 Tenant Context Middleware

Create `go/apiserver/tenant_context.go`:
```go
package apiserver

import (
    "context"
    "errors"
    "net/http"
    "strings"

    "github.com/denisvmedia/inventario/models"
    "github.com/denisvmedia/inventario/registry"
)

const (
    tenantCtxKey ctxValueKey = "tenant"
    userCtxKey   ctxValueKey = "user"
)

// TenantResolver defines how to resolve tenant from request
type TenantResolver interface {
    ResolveTenant(r *http.Request) (string, error)
}

// HeaderTenantResolver resolves tenant from X-Tenant-ID header
type HeaderTenantResolver struct{}

func (h *HeaderTenantResolver) ResolveTenant(r *http.Request) (string, error) {
    tenantID := r.Header.Get("X-Tenant-ID")
    if tenantID == "" {
        return "", errors.New("tenant ID not found in header")
    }
    return tenantID, nil
}

// SubdomainTenantResolver resolves tenant from subdomain
type SubdomainTenantResolver struct {
    tenantRegistry registry.TenantRegistry
}

func (s *SubdomainTenantResolver) ResolveTenant(r *http.Request) (string, error) {
    host := r.Host
    if strings.Contains(host, ":") {
        host = strings.Split(host, ":")[0]
    }

    parts := strings.Split(host, ".")
    if len(parts) < 2 {
        return "", errors.New("invalid subdomain")
    }

    subdomain := parts[0]
    tenant, err := s.tenantRegistry.GetBySlug(r.Context(), subdomain)
    if err != nil {
        return "", err
    }

    return tenant.ID, nil
}

// TenantMiddleware adds tenant context to requests
func TenantMiddleware(resolver TenantResolver, tenantRegistry registry.TenantRegistry) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            tenantID, err := resolver.ResolveTenant(r)
            if err != nil {
                http.Error(w, "Tenant not found", http.StatusBadRequest)
                return
            }

            tenant, err := tenantRegistry.Get(r.Context(), tenantID)
            if err != nil {
                http.Error(w, "Invalid tenant", http.StatusUnauthorized)
                return
            }

            if tenant.Status != models.TenantStatusActive {
                http.Error(w, "Tenant suspended", http.StatusForbidden)
                return
            }

            ctx := context.WithValue(r.Context(), tenantCtxKey, tenant)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func TenantFromContext(ctx context.Context) *models.Tenant {
    tenant, ok := ctx.Value(tenantCtxKey).(*models.Tenant)
    if !ok {
        return nil
    }
    return tenant
}
```

### Phase 2: Database Schema Migration

#### 2.1 Migration Strategy

Create `go/models/migration_tenant.go`:
```go
package models

// TenantMigration represents the tenant migration structure
type TenantMigration struct {
    // Add tenant_id to all existing tables

    //migrator:schema:field name="tenant_id" type="TEXT" not_null="true" foreign="tenants(id)" foreign_key_name="fk_location_tenant" table="locations"
    LocationTenantID string

    //migrator:schema:field name="tenant_id" type="TEXT" not_null="true" foreign="tenants(id)" foreign_key_name="fk_area_tenant" table="areas"
    AreaTenantID string

    //migrator:schema:field name="tenant_id" type="TEXT" not_null="true" foreign="tenants(id)" foreign_key_name="fk_commodity_tenant" table="commodities"
    CommodityTenantID string

    //migrator:schema:field name="tenant_id" type="TEXT" not_null="true" foreign="tenants(id)" foreign_key_name="fk_file_tenant" table="files"
    FileTenantID string

    //migrator:schema:field name="tenant_id" type="TEXT" not_null="true" foreign="tenants(id)" foreign_key_name="fk_export_tenant" table="exports"
    ExportTenantID string

    //migrator:schema:field name="tenant_id" type="TEXT" not_null="true" foreign="tenants(id)" foreign_key_name="fk_image_tenant" table="images"
    ImageTenantID string

    //migrator:schema:field name="tenant_id" type="TEXT" not_null="true" foreign="tenants(id)" foreign_key_name="fk_invoice_tenant" table="invoices"
    InvoiceTenantID string

    //migrator:schema:field name="tenant_id" type="TEXT" not_null="true" foreign="tenants(id)" foreign_key_name="fk_manual_tenant" table="manuals"
    ManualTenantID string
}

// TenantIndexes for performance
type TenantIndexes struct {
    //migrator:schema:index name="locations_tenant_idx" fields="tenant_id" table="locations"
    _ int

    //migrator:schema:index name="areas_tenant_idx" fields="tenant_id" table="areas"
    _ int

    //migrator:schema:index name="commodities_tenant_idx" fields="tenant_id" table="commodities"
    _ int

    //migrator:schema:index name="files_tenant_idx" fields="tenant_id" table="files"
    _ int

    //migrator:schema:index name="exports_tenant_idx" fields="tenant_id" table="exports"
    _ int

    //migrator:schema:index name="users_tenant_idx" fields="tenant_id" table="users"
    _ int
}
```

#### 2.2 Row-Level Security Setup

Create `go/registry/postgres/rls_policies.sql`:
```sql
-- Enable RLS on all tables
ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE locations ENABLE ROW LEVEL SECURITY;
ALTER TABLE areas ENABLE ROW LEVEL SECURITY;
ALTER TABLE commodities ENABLE ROW LEVEL SECURITY;
ALTER TABLE files ENABLE ROW LEVEL SECURITY;
ALTER TABLE exports ENABLE ROW LEVEL SECURITY;
ALTER TABLE images ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE manuals ENABLE ROW LEVEL SECURITY;

-- Create application role for the application
CREATE ROLE inventario_app;

-- Grant necessary permissions to application role
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO inventario_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO inventario_app;

-- Create policies for tenant isolation
CREATE POLICY tenant_isolation_policy ON locations
    FOR ALL TO inventario_app
    USING (tenant_id = current_setting('app.current_tenant_id')::text);

CREATE POLICY tenant_isolation_policy ON areas
    FOR ALL TO inventario_app
    USING (tenant_id = current_setting('app.current_tenant_id')::text);

CREATE POLICY tenant_isolation_policy ON commodities
    FOR ALL TO inventario_app
    USING (tenant_id = current_setting('app.current_tenant_id')::text);

CREATE POLICY tenant_isolation_policy ON files
    FOR ALL TO inventario_app
    USING (tenant_id = current_setting('app.current_tenant_id')::text);

CREATE POLICY tenant_isolation_policy ON exports
    FOR ALL TO inventario_app
    USING (tenant_id = current_setting('app.current_tenant_id')::text);

CREATE POLICY tenant_isolation_policy ON images
    FOR ALL TO inventario_app
    USING (tenant_id = current_setting('app.current_tenant_id')::text);

CREATE POLICY tenant_isolation_policy ON invoices
    FOR ALL TO inventario_app
    USING (tenant_id = current_setting('app.current_tenant_id')::text);

CREATE POLICY tenant_isolation_policy ON manuals
    FOR ALL TO inventario_app
    USING (tenant_id = current_setting('app.current_tenant_id')::text);

-- Users can only see their own tenant
CREATE POLICY user_tenant_policy ON users
    FOR ALL TO inventario_app
    USING (tenant_id = current_setting('app.current_tenant_id')::text);

-- Tenants can only see themselves
CREATE POLICY tenant_self_policy ON tenants
    FOR ALL TO inventario_app
    USING (id = current_setting('app.current_tenant_id')::text);

-- Function to set tenant context
CREATE OR REPLACE FUNCTION set_tenant_context(tenant_id text)
RETURNS void AS $$
BEGIN
    PERFORM set_config('app.current_tenant_id', tenant_id, true);
END;
$$ LANGUAGE plpgsql;
```

#### 2.3 Data Migration Script

Create `scripts/migrate_to_multitenant.sql`:
```sql
-- Step 1: Create default tenant
INSERT INTO tenants (id, name, slug, status, created_at, updated_at)
VALUES (
    'default-tenant-id',
    'Default Organization',
    'default',
    'active',
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
) ON CONFLICT (id) DO NOTHING;

-- Step 2: Add tenant_id columns (if not already added by migration)
-- These should be handled by Ptah migrations, but included for reference

-- Step 3: Update all existing records with default tenant
UPDATE locations SET tenant_id = 'default-tenant-id' WHERE tenant_id IS NULL;
UPDATE areas SET tenant_id = 'default-tenant-id' WHERE tenant_id IS NULL;
UPDATE commodities SET tenant_id = 'default-tenant-id' WHERE tenant_id IS NULL;
UPDATE files SET tenant_id = 'default-tenant-id' WHERE tenant_id IS NULL;
UPDATE exports SET tenant_id = 'default-tenant-id' WHERE tenant_id IS NULL;
UPDATE images SET tenant_id = 'default-tenant-id' WHERE tenant_id IS NULL;
UPDATE invoices SET tenant_id = 'default-tenant-id' WHERE tenant_id IS NULL;
UPDATE manuals SET tenant_id = 'default-tenant-id' WHERE tenant_id IS NULL;

-- Step 4: Create default admin user (optional)
INSERT INTO users (id, email, password_hash, name, role, tenant_id, is_active, created_at, updated_at)
VALUES (
    'default-admin-id',
    'admin@example.com',
    '$2a$10$example_hash_replace_with_real_hash',
    'Administrator',
    'admin',
    'default-tenant-id',
    true,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
) ON CONFLICT (email) DO NOTHING;
```

### Phase 3: Registry Layer Enhancement

#### 3.1 Tenant-Aware Registry Interface

Create `go/registry/tenant_registry.go`:
```go
package registry

import (
    "context"
    "github.com/denisvmedia/inventario/models"
)

// TenantRegistry manages tenant operations
type TenantRegistry interface {
    Registry[models.Tenant]
    GetBySlug(ctx context.Context, slug string) (*models.Tenant, error)
    GetByDomain(ctx context.Context, domain string) (*models.Tenant, error)
}

// UserRegistry manages user operations
type UserRegistry interface {
    Registry[models.User]
    GetByEmail(ctx context.Context, email string) (*models.User, error)
    GetByTenant(ctx context.Context, tenantID string) ([]*models.User, error)
}

// TenantAwareRegistry extends base registry with tenant context
type TenantAwareRegistry[T any] interface {
    Registry[T]

    // Tenant-aware operations
    CreateWithTenant(ctx context.Context, tenantID string, item T) (*T, error)
    ListByTenant(ctx context.Context, tenantID string) ([]*T, error)
    CountByTenant(ctx context.Context, tenantID string) (int, error)
}
```

#### 3.2 PostgreSQL Implementation with Tenant Context

Create `go/registry/postgres/tenant_aware.go`:
```go
package postgres

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/denisvmedia/inventario/registry"
)

// TenantAwarePostgreSQLRegistry wraps the standard registry with tenant context
type TenantAwarePostgreSQLRegistry struct {
    *EnhancedPostgreSQLRegistry
    pool *pgxpool.Pool
}

// SetTenantContext sets the tenant context for the current connection
func (r *TenantAwarePostgreSQLRegistry) SetTenantContext(ctx context.Context, tenantID string) error {
    _, err := r.pool.Exec(ctx, "SELECT set_config('app.current_tenant_id', $1, true)", tenantID)
    return err
}

// WithTenantContext executes a function with tenant context set
func (r *TenantAwarePostgreSQLRegistry) WithTenantContext(ctx context.Context, tenantID string, fn func(context.Context) error) error {
    // Get a connection from the pool
    conn, err := r.pool.Acquire(ctx)
    if err != nil {
        return err
    }
    defer conn.Release()

    // Set tenant context
    _, err = conn.Exec(ctx, "SELECT set_config('app.current_tenant_id', $1, true)", tenantID)
    if err != nil {
        return err
    }

    // Execute the function
    return fn(ctx)
}
```

#### 3.3 Enhanced Registry Set

Modify `go/registry/registry.go` to include tenant registries:
```go
// Enhanced Set with tenant support
type EnhancedSet struct {
    *Set
    TenantRegistry TenantRegistry
    UserRegistry   UserRegistry
}

// Update existing registries to be tenant-aware
func (s *EnhancedSet) LocationRegistryWithTenant() TenantAwareRegistry[models.Location] {
    // Implementation depends on specific registry backend
    return s.LocationRegistry.(TenantAwareRegistry[models.Location])
}

func (s *EnhancedSet) CommodityRegistryWithTenant() TenantAwareRegistry[models.Commodity] {
    return s.CommodityRegistry.(TenantAwareRegistry[models.Commodity])
}

// Add similar methods for other registries...
```

### Phase 4: API Layer Modifications

#### 4.1 Authentication Endpoints

Create `go/apiserver/auth.go`:
```go
package apiserver

import (
    "encoding/json"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/render"
    "github.com/golang-jwt/jwt/v5"
    "github.com/denisvmedia/inventario/models"
    "github.com/denisvmedia/inventario/registry"
)

type AuthAPI struct {
    userRegistry   registry.UserRegistry
    tenantRegistry registry.TenantRegistry
    jwtSecret      []byte
}

type LoginRequest struct {
    Email      string `json:"email"`
    Password   string `json:"password"`
    TenantSlug string `json:"tenant_slug,omitempty"`
}

type LoginResponse struct {
    Token     string         `json:"token"`
    User      *models.User   `json:"user"`
    Tenant    *models.Tenant `json:"tenant"`
    ExpiresAt time.Time      `json:"expires_at"`
}

func (api *AuthAPI) login(w http.ResponseWriter, r *http.Request) {
    var req LoginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    // Get user by email
    user, err := api.userRegistry.GetByEmail(r.Context(), req.Email)
    if err != nil {
        http.Error(w, "Invalid credentials", http.StatusUnauthorized)
        return
    }

    // Check password
    if !user.CheckPassword(req.Password) {
        http.Error(w, "Invalid credentials", http.StatusUnauthorized)
        return
    }

    // Get tenant
    tenant, err := api.tenantRegistry.Get(r.Context(), user.TenantID)
    if err != nil {
        http.Error(w, "Tenant not found", http.StatusUnauthorized)
        return
    }

    // Verify tenant is active
    if tenant.Status != models.TenantStatusActive {
        http.Error(w, "Account suspended", http.StatusForbidden)
        return
    }

    // Generate JWT token
    expiresAt := time.Now().Add(24 * time.Hour)
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "user_id":   user.ID,
        "tenant_id": tenant.ID,
        "role":      user.Role,
        "exp":       expiresAt.Unix(),
    })

    tokenString, err := token.SignedString(api.jwtSecret)
    if err != nil {
        http.Error(w, "Failed to generate token", http.StatusInternalServerError)
        return
    }

    // Update last login
    now := time.Now()
    user.LastLoginAt = &now
    api.userRegistry.Update(r.Context(), *user)

    response := LoginResponse{
        Token:     tokenString,
        User:      user,
        Tenant:    tenant,
        ExpiresAt: expiresAt,
    }

    render.JSON(w, r, response)
}

func (api *AuthAPI) logout(w http.ResponseWriter, r *http.Request) {
    // In a stateless JWT system, logout is handled client-side
    // Could implement token blacklisting here if needed
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"message": "Logged out successfully"})
}

func (api *AuthAPI) getCurrentUser(w http.ResponseWriter, r *http.Request) {
    user := UserFromContext(r.Context())
    if user == nil {
        http.Error(w, "User not found", http.StatusUnauthorized)
        return
    }

    tenant := TenantFromContext(r.Context())
    if tenant == nil {
        http.Error(w, "Tenant not found", http.StatusUnauthorized)
        return
    }

    response := map[string]interface{}{
        "user":   user,
        "tenant": tenant,
    }

    render.JSON(w, r, response)
}

func Auth(userRegistry registry.UserRegistry, tenantRegistry registry.TenantRegistry, jwtSecret []byte) func(r chi.Router) {
    api := &AuthAPI{
        userRegistry:   userRegistry,
        tenantRegistry: tenantRegistry,
        jwtSecret:      jwtSecret,
    }

    return func(r chi.Router) {
        r.Post("/login", api.login)
        r.Post("/logout", api.logout)
        r.Get("/me", api.getCurrentUser)
    }
}
```

#### 4.2 JWT Authentication Middleware

Create `go/apiserver/jwt_middleware.go`:
```go
package apiserver

import (
    "context"
    "net/http"
    "strings"

    "github.com/golang-jwt/jwt/v5"
    "github.com/denisvmedia/inventario/models"
    "github.com/denisvmedia/inventario/registry"
)

func JWTMiddleware(jwtSecret []byte, userRegistry registry.UserRegistry) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                http.Error(w, "Authorization header required", http.StatusUnauthorized)
                return
            }

            tokenString := strings.TrimPrefix(authHeader, "Bearer ")
            if tokenString == authHeader {
                http.Error(w, "Bearer token required", http.StatusUnauthorized)
                return
            }

            token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
                if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                    return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
                }
                return jwtSecret, nil
            })

            if err != nil || !token.Valid {
                http.Error(w, "Invalid token", http.StatusUnauthorized)
                return
            }

            claims, ok := token.Claims.(jwt.MapClaims)
            if !ok {
                http.Error(w, "Invalid token claims", http.StatusUnauthorized)
                return
            }

            userID, ok := claims["user_id"].(string)
            if !ok {
                http.Error(w, "Invalid user ID in token", http.StatusUnauthorized)
                return
            }

            tenantID, ok := claims["tenant_id"].(string)
            if !ok {
                http.Error(w, "Invalid tenant ID in token", http.StatusUnauthorized)
                return
            }

            // Get user from database to ensure they still exist and are active
            user, err := userRegistry.Get(r.Context(), userID)
            if err != nil {
                http.Error(w, "User not found", http.StatusUnauthorized)
                return
            }

            if !user.IsActive {
                http.Error(w, "User account disabled", http.StatusUnauthorized)
                return
            }

            // Verify tenant ID matches user's tenant
            if user.TenantID != tenantID {
                http.Error(w, "Invalid tenant for user", http.StatusUnauthorized)
                return
            }

            // Add user and tenant ID to context
            ctx := context.WithValue(r.Context(), userCtxKey, user)
            ctx = context.WithValue(ctx, "tenant_id", tenantID)

            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func UserFromContext(ctx context.Context) *models.User {
    user, ok := ctx.Value(userCtxKey).(*models.User)
    if !ok {
        return nil
    }
    return user
}

func RequireAuth(jwtSecret []byte, userRegistry registry.UserRegistry) func(http.Handler) http.Handler {
    return JWTMiddleware(jwtSecret, userRegistry)
}

func RequireRole(role models.UserRole) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user := UserFromContext(r.Context())
            if user == nil {
                http.Error(w, "Authentication required", http.StatusUnauthorized)
                return
            }

            if user.Role != role {
                http.Error(w, "Insufficient permissions", http.StatusForbidden)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

#### 4.3 Updated API Server Configuration

Modify `go/apiserver/apiserver.go` to include authentication:
```go
// Add to APIServer function
func APIServer(params Params, restoreWorker RestoreWorkerInterface, jwtSecret []byte) http.Handler {
    render.Decode = JSONAPIAwareDecoder

    r := chi.NewRouter()

    // CORS middleware
    r.Use(cors.AllowAll().Handler)
    r.Use(middleware.Timeout(60 * time.Second))
    r.Use(middleware.RequestID)
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    // Public routes (no authentication required)
    r.Route("/api/v1", func(r chi.Router) {
        // Authentication endpoints
        r.Route("/auth", Auth(params.RegistrySet.UserRegistry, params.RegistrySet.TenantRegistry, jwtSecret))

        // Health check and system info (public)
        r.Route("/system", System(params.RegistrySet.SettingsRegistry, params.DebugInfo, params.StartTime))
    })

    // Protected routes (authentication required)
    r.Route("/api/v1", func(r chi.Router) {
        // Apply authentication middleware
        r.Use(RequireAuth(jwtSecret, params.RegistrySet.UserRegistry))

        // Apply tenant context middleware
        r.Use(TenantMiddleware(&HeaderTenantResolver{}, params.RegistrySet.TenantRegistry))

        // All existing routes with tenant context
        r.With(defaultAPIMiddlewares...).Route("/locations", Locations(params.RegistrySet.LocationRegistry))
        r.With(defaultAPIMiddlewares...).Route("/areas", Areas(params.RegistrySet.AreaRegistry))
        r.With(defaultAPIMiddlewares...).Route("/commodities", Commodities(params))
        r.With(defaultAPIMiddlewares...).Route("/settings", Settings(params.RegistrySet.SettingsRegistry))
        r.With(defaultAPIMiddlewares...).Route("/exports", Exports(params, restoreWorker))
        r.With(defaultAPIMiddlewares...).Route("/files", Files(params))
        r.With(defaultAPIMiddlewares...).Route("/search", Search(params.RegistrySet))
        r.Route("/currencies", Currencies())
        r.Route("/uploads", Uploads(params))
        r.Route("/seed", Seed(params.RegistrySet))
        r.Route("/commodities/values", Values(params.RegistrySet))

        // Admin-only routes
        r.With(RequireRole(models.UserRoleAdmin)).Route("/admin", func(r chi.Router) {
            r.Route("/tenants", Tenants(params.RegistrySet.TenantRegistry))
            r.Route("/users", Users(params.RegistrySet.UserRegistry))
        })
    })

    // Frontend handler
    r.Handle("/*", FrontendHandler())

    return r
}
```

### Phase 5: Frontend Integration

#### 5.1 Authentication Store

Create `frontend/src/stores/auth.ts`:
```typescript
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from 'axios'

interface User {
  id: string
  email: string
  name: string
  role: string
  tenant_id: string
  is_active: boolean
  last_login_at?: string
  created_at: string
  updated_at: string
}

interface Tenant {
  id: string
  name: string
  slug: string
  domain?: string
  status: string
  settings?: Record<string, any>
  created_at: string
  updated_at: string
}

interface LoginResponse {
  token: string
  user: User
  tenant: Tenant
  expires_at: string
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(localStorage.getItem('auth_token'))
  const user = ref<User | null>(null)
  const tenant = ref<Tenant | null>(null)
  const loading = ref(false)

  const isAuthenticated = computed(() => !!token.value && !!user.value)
  const isAdmin = computed(() => user.value?.role === 'admin')
  const tenantName = computed(() => tenant.value?.name || 'Unknown Organization')

  async function login(email: string, password: string, tenantSlug?: string): Promise<void> {
    loading.value = true
    try {
      const response = await axios.post<LoginResponse>('/api/v1/auth/login', {
        email,
        password,
        tenant_slug: tenantSlug
      })

      const data = response.data

      token.value = data.token
      user.value = data.user
      tenant.value = data.tenant

      localStorage.setItem('auth_token', data.token)

      // Set default headers for future requests
      setAuthHeader(data.token)
      setTenantHeader(data.tenant.id)

    } catch (error) {
      console.error('Login failed:', error)
      throw new Error('Invalid credentials')
    } finally {
      loading.value = false
    }
  }

  async function logout(): Promise<void> {
    try {
      await axios.post('/api/v1/auth/logout')
    } catch (error) {
      console.error('Logout request failed:', error)
    } finally {
      // Clear local state regardless of API call success
      token.value = null
      user.value = null
      tenant.value = null
      localStorage.removeItem('auth_token')
      delete axios.defaults.headers.common['Authorization']
      delete axios.defaults.headers.common['X-Tenant-ID']
    }
  }

  async function getCurrentUser(): Promise<void> {
    if (!token.value) return

    try {
      const response = await axios.get('/api/v1/auth/me')
      user.value = response.data.user
      tenant.value = response.data.tenant
    } catch (error) {
      console.error('Failed to get current user:', error)
      // If token is invalid, clear auth state
      await logout()
    }
  }

  function setAuthHeader(authToken: string): void {
    axios.defaults.headers.common['Authorization'] = `Bearer ${authToken}`
  }

  function setTenantHeader(tenantId: string): void {
    axios.defaults.headers.common['X-Tenant-ID'] = tenantId
  }

  // Initialize auth headers if token exists
  if (token.value) {
    setAuthHeader(token.value)
    // Get current user to restore full auth state
    getCurrentUser()
  }

  return {
    token,
    user,
    tenant,
    loading,
    isAuthenticated,
    isAdmin,
    tenantName,
    login,
    logout,
    getCurrentUser
  }
})
```

#### 5.2 Login Component

Create `frontend/src/views/auth/LoginView.vue`:
```vue
<template>
  <div class="login-container">
    <div class="login-wrapper">
      <Card class="login-card">
        <template #title>
          <div class="login-header">
            <h1>Inventario</h1>
            <h2>Sign In</h2>
          </div>
        </template>

        <template #content>
          <form @submit.prevent="handleLogin" class="login-form">
            <div class="field">
              <label for="email">Email Address</label>
              <InputText
                id="email"
                v-model="form.email"
                type="email"
                required
                autocomplete="email"
                :class="{ 'p-invalid': errors.email }"
                placeholder="Enter your email"
              />
              <small v-if="errors.email" class="p-error">{{ errors.email }}</small>
            </div>

            <div class="field">
              <label for="password">Password</label>
              <Password
                id="password"
                v-model="form.password"
                required
                :feedback="false"
                autocomplete="current-password"
                :class="{ 'p-invalid': errors.password }"
                placeholder="Enter your password"
              />
              <small v-if="errors.password" class="p-error">{{ errors.password }}</small>
            </div>

            <div class="field" v-if="showTenantField">
              <label for="tenant">Organization</label>
              <InputText
                id="tenant"
                v-model="form.tenantSlug"
                placeholder="your-organization"
                :class="{ 'p-invalid': errors.tenantSlug }"
              />
              <small class="field-help">Enter your organization's identifier</small>
              <small v-if="errors.tenantSlug" class="p-error">{{ errors.tenantSlug }}</small>
            </div>

            <Button
              type="submit"
              label="Sign In"
              :loading="authStore.loading"
              class="w-full login-button"
              size="large"
            />
          </form>

          <Message v-if="error" severity="error" class="login-error">
            {{ error }}
          </Message>
        </template>
      </Card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const error = ref('')
const showTenantField = ref(false) // Configure based on deployment mode

const form = reactive({
  email: '',
  password: '',
  tenantSlug: ''
})

const errors = reactive({
  email: '',
  password: '',
  tenantSlug: ''
})

async function handleLogin() {
  error.value = ''
  clearErrors()

  if (!validateForm()) {
    return
  }

  try {
    await authStore.login(form.email, form.password, form.tenantSlug || undefined)
    router.push('/')
  } catch (err) {
    error.value = 'Invalid credentials. Please check your email and password and try again.'
  }
}

function validateForm(): boolean {
  let isValid = true

  if (!form.email) {
    errors.email = 'Email is required'
    isValid = false
  } else if (!/\S+@\S+\.\S+/.test(form.email)) {
    errors.email = 'Please enter a valid email address'
    isValid = false
  }

  if (!form.password) {
    errors.password = 'Password is required'
    isValid = false
  }

  if (showTenantField.value && !form.tenantSlug) {
    errors.tenantSlug = 'Organization is required'
    isValid = false
  }

  return isValid
}

function clearErrors() {
  errors.email = ''
  errors.password = ''
  errors.tenantSlug = ''
}

onMounted(() => {
  // If already authenticated, redirect to home
  if (authStore.isAuthenticated) {
    router.push('/')
  }
})
</script>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 1rem;
}

.login-wrapper {
  width: 100%;
  max-width: 400px;
}

.login-card {
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
}

.login-header {
  text-align: center;
  margin-bottom: 1rem;
}

.login-header h1 {
  color: var(--primary-color);
  margin: 0 0 0.5rem 0;
  font-size: 2rem;
  font-weight: 600;
}

.login-header h2 {
  color: var(--text-color-secondary);
  margin: 0;
  font-size: 1.2rem;
  font-weight: 400;
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.field label {
  font-weight: 500;
  color: var(--text-color);
}

.field-help {
  color: var(--text-color-secondary);
  font-size: 0.875rem;
}

.login-button {
  margin-top: 1rem;
}

.login-error {
  margin-top: 1rem;
}
</style>
```

#### 5.3 Router Guards

Update `frontend/src/router/index.ts`:
```typescript
import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

// Add authentication guard
router.beforeEach(async (to, from) => {
  const authStore = useAuthStore()

  // Skip auth check for login page
  if (to.path === '/login') {
    // If already authenticated, redirect to home
    if (authStore.isAuthenticated) {
      return '/'
    }
    return true
  }

  // Check if authentication is required
  if (!authStore.isAuthenticated) {
    return '/login'
  }

  // Verify user is still valid
  try {
    await authStore.getCurrentUser()
  } catch (error) {
    return '/login'
  }

  return true
})
```

#### 5.4 Navigation Component Updates

Update navigation to show user/tenant info:
```vue
<!-- Add to navigation component -->
<template>
  <div class="navbar">
    <div class="navbar-brand">
      <h1>Inventario</h1>
      <span class="tenant-name">{{ authStore.tenantName }}</span>
    </div>

    <div class="navbar-user">
      <Dropdown v-model="selectedUser" :options="userMenuOptions" optionLabel="label" placeholder="User Menu">
        <template #value="slotProps">
          <div class="user-info">
            <i class="pi pi-user"></i>
            <span>{{ authStore.user?.name }}</span>
          </div>
        </template>
        <template #option="slotProps">
          <div @click="handleUserMenuAction(slotProps.option.value)">
            <i :class="slotProps.option.icon"></i>
            <span>{{ slotProps.option.label }}</span>
          </div>
        </template>
      </Dropdown>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const userMenuOptions = computed(() => [
  { label: 'Profile', value: 'profile', icon: 'pi pi-user' },
  { label: 'Settings', value: 'settings', icon: 'pi pi-cog' },
  { label: 'Sign Out', value: 'logout', icon: 'pi pi-sign-out' }
])

async function handleUserMenuAction(action: string) {
  switch (action) {
    case 'profile':
      router.push('/profile')
      break
    case 'settings':
      router.push('/settings')
      break
    case 'logout':
      await authStore.logout()
      router.push('/login')
      break
  }
}
</script>
```

### Phase 6: Configuration and Deployment

#### 6.1 Enhanced Configuration

Update `go/cmd/inventario/run/run.go`:
```go
// Add new flags for multi-tenancy
func NewRunCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "run",
        Short: "Start the Inventario server",
        Long:  `Start the Inventario HTTP server with the specified configuration.`,
        RunE:  runServer,
    }

    // Existing flags...
    cmd.Flags().String("addr", defaults.GetServerAddr(), "Server address")
    cmd.Flags().String("db-dsn", defaults.GetDatabaseDSN(), "Database connection string")
    cmd.Flags().String("upload-location", defaults.GetUploadLocation(), "Upload storage location")

    // New multi-tenant flags
    cmd.Flags().String("jwt-secret", "", "JWT secret for authentication (required in multi-tenant mode)")
    cmd.Flags().String("tenant-mode", "single", "Tenant mode: single, multi-header, multi-subdomain")
    cmd.Flags().String("default-tenant-id", "", "Default tenant ID for single-tenant mode")
    cmd.Flags().Bool("enable-registration", false, "Enable user registration")
    cmd.Flags().Bool("require-auth", false, "Require authentication for all requests")

    return cmd
}
```

#### 6.2 Configuration Structure

Create `go/internal/config/multitenant.go`:
```go
package config

type MultiTenantConfig struct {
    Mode              string `yaml:"tenant-mode" mapstructure:"tenant-mode"`
    JWTSecret         string `yaml:"jwt-secret" mapstructure:"jwt-secret"`
    DefaultTenantID   string `yaml:"default-tenant-id" mapstructure:"default-tenant-id"`
    EnableRegistration bool   `yaml:"enable-registration" mapstructure:"enable-registration"`
    RequireAuth       bool   `yaml:"require-auth" mapstructure:"require-auth"`
    TenantResolver    string `yaml:"tenant-resolver" mapstructure:"tenant-resolver"`
}

const (
    TenantModeSingle      = "single"
    TenantModeMultiHeader = "multi-header"
    TenantModeMultiSubdomain = "multi-subdomain"
)

const (
    TenantResolverHeader    = "header"
    TenantResolverSubdomain = "subdomain"
    TenantResolverDomain    = "domain"
)

func (c *MultiTenantConfig) Validate() error {
    if c.Mode != TenantModeSingle && c.JWTSecret == "" {
        return errors.New("jwt-secret is required for multi-tenant mode")
    }

    if c.Mode == TenantModeSingle && c.DefaultTenantID == "" {
        return errors.New("default-tenant-id is required for single-tenant mode")
    }

    return nil
}
```

#### 6.3 Migration Command Enhancement

Update `go/cmd/inventario/migrate/migrate.go`:
```go
func NewMigrateCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "migrate",
        Short: "Run database migrations",
        Long:  `Run database migrations to set up or update the database schema.`,
        RunE:  runMigrations,
    }

    cmd.Flags().String("db-dsn", defaults.GetDatabaseDSN(), "Database connection string")

    // Tenant migration flags
    cmd.Flags().Bool("create-default-tenant", false, "Create default tenant during migration")
    cmd.Flags().String("default-tenant-name", "Default Organization", "Name for default tenant")
    cmd.Flags().String("default-tenant-slug", "default", "Slug for default tenant")
    cmd.Flags().String("admin-email", "", "Admin user email (creates admin user if provided)")
    cmd.Flags().String("admin-password", "", "Admin user password")
    cmd.Flags().String("admin-name", "Administrator", "Admin user name")

    return cmd
}
```

## Migration Strategy

### Step-by-Step Migration Process

#### Step 1: Backup and Preparation
```bash
# 1. Backup existing database
pg_dump inventario > inventario_backup_$(date +%Y%m%d_%H%M%S).sql

# 2. Stop the current application
systemctl stop inventario  # or however you manage the service

# 3. Update the application code
git pull origin main  # or your deployment process
go build -o inventario ./cmd/inventario
```

#### Step 2: Database Migration
```bash
# 1. Run migrations with default tenant creation
./inventario migrate \
  --db-dsn="postgres://user:pass@localhost/inventario" \
  --create-default-tenant \
  --default-tenant-name="My Organization" \
  --default-tenant-slug="my-org" \
  --admin-email="admin@myorg.com" \
  --admin-password="secure-password" \
  --admin-name="System Administrator"

# 2. Verify migration success
./inventario migrate --db-dsn="postgres://user:pass@localhost/inventario" --check
```

#### Step 3: Gradual Rollout

**Phase A: Single-Tenant with Multi-Tenant Infrastructure**
```bash
# Deploy with single-tenant mode (no authentication required initially)
./inventario run \
  --db-dsn="postgres://user:pass@localhost/inventario" \
  --tenant-mode=single \
  --default-tenant-id="default-tenant-id" \
  --require-auth=false
```

**Phase B: Enable Authentication (Optional)**
```bash
# Enable authentication but keep single-tenant mode
./inventario run \
  --db-dsn="postgres://user:pass@localhost/inventario" \
  --tenant-mode=single \
  --default-tenant-id="default-tenant-id" \
  --require-auth=true \
  --jwt-secret="your-secure-jwt-secret-here"
```

**Phase C: Full Multi-Tenancy**
```bash
# Switch to multi-tenant mode
./inventario run \
  --db-dsn="postgres://user:pass@localhost/inventario" \
  --tenant-mode=multi-header \
  --require-auth=true \
  --jwt-secret="your-secure-jwt-secret-here" \
  --enable-registration=false
```

#### Step 4: Data Verification
```sql
-- Verify all data has tenant_id
SELECT
  'locations' as table_name,
  COUNT(*) as total_records,
  COUNT(tenant_id) as records_with_tenant
FROM locations
UNION ALL
SELECT
  'areas' as table_name,
  COUNT(*) as total_records,
  COUNT(tenant_id) as records_with_tenant
FROM areas
UNION ALL
SELECT
  'commodities' as table_name,
  COUNT(*) as total_records,
  COUNT(tenant_id) as records_with_tenant
FROM commodities;

-- Verify RLS policies are active
SELECT schemaname, tablename, policyname, permissive, roles, cmd, qual
FROM pg_policies
WHERE schemaname = 'public';
```

## Security Considerations

### 1. Row-Level Security (RLS)
- **Database-Level Isolation**: PostgreSQL RLS ensures data isolation at the database level
- **Automatic Enforcement**: Policies automatically filter data based on tenant context
- **Defense in Depth**: Even if application logic fails, database prevents cross-tenant access

### 2. JWT Token Security
- **Tenant Validation**: JWT tokens include tenant_id for additional verification
- **Token Expiration**: Implement reasonable token expiration (24 hours recommended)
- **Secret Management**: Use strong, randomly generated JWT secrets
- **Token Refresh**: Consider implementing refresh tokens for better UX

### 3. API Security
- **Request Validation**: Every API request validates tenant access
- **User Verification**: Active user checks on each authenticated request
- **Role-Based Access**: Admin functions restricted to admin users
- **Input Sanitization**: Prevent injection attacks through proper validation

### 4. File Storage Security
- **Tenant-Specific Paths**: File storage uses tenant-specific directories
- **Access Control**: File access validated through tenant context
- **Upload Restrictions**: File type and size restrictions per tenant

### 5. Database Security
- **Connection Pooling**: Efficient tenant context switching
- **Prepared Statements**: Prevent SQL injection
- **Audit Logging**: Track data access and modifications
- **Backup Security**: Tenant-aware backup and restore procedures

## Performance Optimization

### 1. Database Indexing
```sql
-- Essential tenant-aware indexes
CREATE INDEX CONCURRENTLY idx_locations_tenant_id ON locations(tenant_id);
CREATE INDEX CONCURRENTLY idx_areas_tenant_id ON areas(tenant_id);
CREATE INDEX CONCURRENTLY idx_commodities_tenant_id ON commodities(tenant_id);
CREATE INDEX CONCURRENTLY idx_files_tenant_id ON files(tenant_id);
CREATE INDEX CONCURRENTLY idx_users_tenant_id ON users(tenant_id);

-- Composite indexes for common queries
CREATE INDEX CONCURRENTLY idx_commodities_tenant_area ON commodities(tenant_id, area_id);
CREATE INDEX CONCURRENTLY idx_files_tenant_type ON files(tenant_id, type);
CREATE INDEX CONCURRENTLY idx_areas_tenant_location ON areas(tenant_id, location_id);
```

### 2. Connection Pooling
- **Tenant Context Switching**: Efficient tenant context management
- **Pool Size Optimization**: Right-size connection pools based on tenant count
- **Connection Reuse**: Minimize connection overhead

### 3. Caching Strategy
```go
// Tenant-aware caching
type TenantCache struct {
    cache map[string]map[string]interface{} // tenant_id -> cache_key -> value
    mutex sync.RWMutex
}

func (tc *TenantCache) Get(tenantID, key string) (interface{}, bool) {
    tc.mutex.RLock()
    defer tc.mutex.RUnlock()

    tenantCache, exists := tc.cache[tenantID]
    if !exists {
        return nil, false
    }

    value, exists := tenantCache[key]
    return value, exists
}
```

### 4. Query Optimization
- **Tenant-First Queries**: Always include tenant_id in WHERE clauses
- **Query Planning**: Monitor and optimize query execution plans
- **Batch Operations**: Use bulk operations for multi-record updates

## Testing Strategy

### 1. Unit Tests
```go
// Test tenant isolation in registry layer
func TestLocationRegistry_TenantIsolation(t *testing.T) {
    c := qt.New(t)

    // Create test tenants
    tenant1 := createTestTenant(t, "tenant1")
    tenant2 := createTestTenant(t, "tenant2")

    // Create locations for each tenant
    location1 := createTestLocation(t, tenant1.ID, "Location 1")
    location2 := createTestLocation(t, tenant2.ID, "Location 2")

    // Test tenant1 can only see their location
    ctx1 := setTenantContext(context.Background(), tenant1.ID)
    locations1, err := registry.LocationRegistry.List(ctx1)
    c.Assert(err, qt.IsNil)
    c.Assert(len(locations1), qt.Equals, 1)
    c.Assert(locations1[0].ID, qt.Equals, location1.ID)

    // Test tenant2 can only see their location
    ctx2 := setTenantContext(context.Background(), tenant2.ID)
    locations2, err := registry.LocationRegistry.List(ctx2)
    c.Assert(err, qt.IsNil)
    c.Assert(len(locations2), qt.Equals, 1)
    c.Assert(locations2[0].ID, qt.Equals, location2.ID)
}
```

### 2. Integration Tests
```go
// Test multi-tenant API endpoints
func TestAPI_TenantIsolation(t *testing.T) {
    c := qt.New(t)

    // Setup test server with multi-tenant configuration
    server := setupTestServer(t, MultiTenantConfig{
        Mode: TenantModeMultiHeader,
        JWTSecret: "test-secret",
    })

    // Create test users for different tenants
    user1, token1 := createTestUser(t, "tenant1")
    user2, token2 := createTestUser(t, "tenant2")

    // Test user1 cannot access tenant2 data
    req := httptest.NewRequest("GET", "/api/v1/locations", nil)
    req.Header.Set("Authorization", "Bearer "+token1)
    req.Header.Set("X-Tenant-ID", "tenant2") // Wrong tenant

    resp := httptest.NewRecorder()
    server.ServeHTTP(resp, req)

    c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
}
```

### 3. End-to-End Tests
```typescript
// Test authentication and tenant switching flows
describe('Multi-Tenant Authentication', () => {
  it('should isolate data between tenants', async () => {
    // Login as tenant1 user
    await page.goto('/login')
    await page.fill('[data-testid="email"]', 'user1@tenant1.com')
    await page.fill('[data-testid="password"]', 'password')
    await page.click('[data-testid="login-button"]')

    // Create a location
    await page.goto('/locations')
    await page.click('[data-testid="create-location"]')
    await page.fill('[data-testid="location-name"]', 'Tenant 1 Location')
    await page.click('[data-testid="save-location"]')

    // Logout and login as tenant2 user
    await page.click('[data-testid="user-menu"]')
    await page.click('[data-testid="logout"]')

    await page.fill('[data-testid="email"]', 'user2@tenant2.com')
    await page.fill('[data-testid="password"]', 'password')
    await page.click('[data-testid="login-button"]')

    // Verify tenant2 cannot see tenant1's location
    await page.goto('/locations')
    const locations = await page.locator('[data-testid="location-item"]').count()
    expect(locations).toBe(0)
  })
})
```

### 4. Performance Tests
```go
// Load testing with multiple tenants
func TestMultiTenantPerformance(t *testing.T) {
    c := qt.New(t)

    // Create multiple tenants
    tenantCount := 10
    usersPerTenant := 5

    var wg sync.WaitGroup

    for i := 0; i < tenantCount; i++ {
        for j := 0; j < usersPerTenant; j++ {
            wg.Add(1)
            go func(tenantID, userID int) {
                defer wg.Done()

                // Simulate concurrent requests
                token := getTestToken(tenantID, userID)

                for k := 0; k < 100; k++ {
                    resp := makeAuthenticatedRequest(token, fmt.Sprintf("tenant%d", tenantID))
                    c.Assert(resp.StatusCode, qt.Equals, 200)
                }
            }(i, j)
        }
    }

    wg.Wait()
}
```

This comprehensive implementation guide provides a complete roadmap for transforming Inventario from a single-tenant to a secure, scalable multi-tenant application. The phased approach ensures minimal disruption to existing users while providing a robust foundation for multi-tenancy.
```
```
