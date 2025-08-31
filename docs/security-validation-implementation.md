# Security Validation Implementation for API Server

## Overview

This document outlines the implementation plan for adding comprehensive security validation to the entire API server to prevent unauthorized access to entities across users and tenants.

**🚨 CRITICAL: System-wide tenant ID security vulnerability discovered in `go/apiserver/tenant_context.go`**

## Security Requirements

Based on the test cases in `go/backup/restore/merge_add_strategy_test.go`, we need to implement the following security controls:

### 0. Tenant ID Security (CRITICAL - SYSTEM-WIDE VULNERABILITY)

**🚨 IMMEDIATE ACTION REQUIRED: Critical security vulnerability in entire API server**

#### Current Vulnerabilities in `go/apiserver/tenant_context.go`:

1. **HeaderTenantResolver (Lines 22-32)**:
   ```go
   func (h *HeaderTenantResolver) ResolveTenant(r *http.Request) (string, error) {
       tenantID := r.Header.Get("X-Tenant-ID")  // 🚨 USER-PROVIDED TENANT ID!
       return tenantID, nil
   }
   ```
   **Attack**: User can set `X-Tenant-ID: other-tenant` header to access any tenant's data

2. **SubdomainTenantResolver (Lines 33-68)**:
   ```go
   // Resolves tenant from subdomain - user can manipulate subdomain
   ```
   **Attack**: User can access `other-tenant.domain.com` to access other tenant's data

3. **TenantMiddleware (Lines 100-129)**:
   - Accepts resolver output without validating user authorization for that tenant
   - No verification that authenticated user belongs to the resolved tenant

#### Affected Components:
- **ALL API endpoints** that use `TenantMiddleware`
- **ALL entity operations** (commodities, locations, areas, files, etc.)
- **ALL restore/import operations**
- **ALL export operations**
- **ALL user data access**

#### Attack Scenarios:
1. **Cross-Tenant Data Breach**: User authenticates as `user@tenant1.com`, sets `X-Tenant-ID: tenant2`, gains access to tenant2's data
2. **Privilege Escalation**: Regular user accesses admin tenant data
3. **Data Exfiltration**: Automated scripts iterate through tenant IDs to harvest data

#### Requirements:
- **NEVER accept user-provided tenant_id** in any form (headers, query params, body)
- **Derive tenant from authenticated JWT token only**
- **Validate user belongs to the tenant** before granting access
- **Log all unauthorized tenant access attempts**
- **Reject requests with tenant_id fields** in request body

#### Comprehensive Vulnerability Assessment:

**🚨 CRITICAL FINDINGS:**

1. **HeaderTenantResolver (go/apiserver/tenant_context.go:26-31)**:
   ```go
   tenantID := r.Header.Get("X-Tenant-ID")  // 🚨 ACCEPTS USER INPUT!
   ```

2. **JSON API Models (go/models/models.go:332-334)**:
   ```go
   TenantID string `json:"tenant_id" db:"tenant_id" userinput:"false"`  // 🚨 JSON TAG PRESENT!
   ```
   Despite `userinput:"false"`, the `json:"tenant_id"` tag allows JSON injection!

3. **No Request Body Validation**: All JSON API handlers accept tenant_id in request bodies

4. **Authentication Gap**: JWT tokens don't include tenant validation

5. **Middleware Chain**: TenantMiddleware trusts resolver output without user authorization check

**AFFECTED ENDPOINTS**: ALL API endpoints using TenantMiddleware (entire system)

### 1. Cross-User Access Prevention
- **Requirement**: Users cannot link files to other users' entities
- **Test**: `TestRestoreService_SecurityValidation_CrossUserAccess`
- **Behavior**: Should fail with errors when User 2 tries to access User 1's commodity

### 2. Cross-Tenant Access Prevention
- **Requirement**: Users cannot access entities from other tenants
- **Critical**: Never accept user-provided tenant_id in any requests
- **Test**: `TestRestoreService_SecurityValidation_CrossTenantAccess`
- **Behavior**: Should fail with errors when Tenant 2 user tries to access Tenant 1's data

### 3. Valid User Manipulations
- **Requirement**: Allow any manipulations within user's own context
- **Test**: `TestRestoreService_SecurityValidation_ValidUserManipulations`
- **Behavior**: Should succeed when user reorganizes their own entities

### 4. Security Logging
- **Requirement**: Log all unauthorized access attempts
- **Test**: `TestRestoreService_SecurityValidation_LoggingUnauthorizedAttempts`
- **Behavior**: Should log detailed information about security violations

## Implementation Plan

### Phase 0: Tenant ID Security Validation (CRITICAL)

#### 0.1 Add Tenant ID Validation Interface
```go
// File: go/backup/restore/security/tenant_validator.go
type TenantValidator interface {
    ValidateNoUserProvidedTenantID(request interface{}) error
    ExtractTenantFromContext(ctx context.Context) (string, error)
}

type RestoreTenantValidator struct {
    logger *slog.Logger
}

func (v *RestoreTenantValidator) ValidateNoUserProvidedTenantID(request interface{}) error {
    // Use reflection to check if request contains any tenant_id fields
    requestValue := reflect.ValueOf(request)
    if requestValue.Kind() == reflect.Ptr {
        requestValue = requestValue.Elem()
    }

    if requestValue.Kind() != reflect.Struct {
        return nil // Not a struct, no tenant_id possible
    }

    requestType := requestValue.Type()
    for i := 0; i < requestValue.NumField(); i++ {
        field := requestType.Field(i)
        fieldName := strings.ToLower(field.Name)

        // Check for any tenant-related fields
        if strings.Contains(fieldName, "tenant") {
            v.logger.Error("Security violation: user-provided tenant ID detected",
                "field_name", field.Name,
                "request_type", requestType.Name(),
            )
            return errors.New("security violation: tenant_id cannot be provided by user")
        }

        // Check JSON tags for tenant fields
        jsonTag := field.Tag.Get("json")
        if strings.Contains(strings.ToLower(jsonTag), "tenant") {
            v.logger.Error("Security violation: user-provided tenant ID in JSON tag",
                "field_name", field.Name,
                "json_tag", jsonTag,
                "request_type", requestType.Name(),
            )
            return errors.New("security violation: tenant_id cannot be provided by user")
        }
    }

    return nil
}

func (v *RestoreTenantValidator) ExtractTenantFromContext(ctx context.Context) (string, error) {
    // Extract tenant ID from authenticated context only
    user := appctx.GetUser(ctx)
    if user == nil {
        return "", errors.New("no authenticated user in context")
    }

    // TODO: When multi-tenancy is implemented, extract from user.TenantID
    // For now, derive from user ID or use default tenant
    return "default-tenant", nil
}
```

#### 0.2 Fix JSON API Models (CRITICAL)
```go
// File: go/models/models.go
type TenantAwareEntityID struct {
    EntityID
    // REMOVE json tags to prevent user input!
    TenantID string `db:"tenant_id" userinput:"false"`  // ✅ NO JSON TAG
    UserID   string `db:"user_id" userinput:"false"`    // ✅ NO JSON TAG
}
```

#### 0.3 Replace HeaderTenantResolver (CRITICAL)
```go
// File: go/apiserver/tenant_context.go
// REMOVE HeaderTenantResolver completely - it's a security vulnerability!

// JWTTenantResolver resolves tenant from authenticated JWT token only
type JWTTenantResolver struct {
    jwtSecret []byte
}

func (j *JWTTenantResolver) ResolveTenant(r *http.Request) (string, error) {
    // Extract tenant from JWT token, never from user input
    user := appctx.GetUser(r.Context())
    if user == nil {
        return "", errors.New("no authenticated user")
    }

    // TODO: When multi-tenancy is implemented, get from user.TenantID
    // For now, derive from user context
    return user.TenantID, nil
}
```

#### 0.4 Add Request Validation Middleware
```go
// File: go/apiserver/security_middleware.go
func ValidateNoUserProvidedTenantID() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 1. Check headers for tenant_id
            if tenantHeader := r.Header.Get("X-Tenant-ID"); tenantHeader != "" {
                slog.Error("Security violation: user-provided tenant ID in header",
                    "header", "X-Tenant-ID",
                    "value", tenantHeader,
                    "user_agent", r.UserAgent(),
                    "remote_addr", r.RemoteAddr,
                )
                http.Error(w, "Security violation: tenant_id cannot be provided by user", http.StatusForbidden)
                return
            }

            // 2. Check query parameters for tenant_id
            if tenantQuery := r.URL.Query().Get("tenant_id"); tenantQuery != "" {
                slog.Error("Security violation: user-provided tenant ID in query",
                    "param", "tenant_id",
                    "value", tenantQuery,
                )
                http.Error(w, "Security violation: tenant_id cannot be provided by user", http.StatusForbidden)
                return
            }

            // 3. Check request body for tenant_id fields
            if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
                body, err := io.ReadAll(r.Body)
                if err != nil {
                    http.Error(w, "Failed to read request body", http.StatusBadRequest)
                    return
                }

                // Restore body for downstream handlers
                r.Body = io.NopCloser(bytes.NewBuffer(body))

                // Check for tenant_id in JSON (case-insensitive)
                bodyLower := bytes.ToLower(body)
                if bytes.Contains(bodyLower, []byte("tenant_id")) ||
                   bytes.Contains(bodyLower, []byte("\"tenant\"")) {
                    slog.Error("Security violation: user-provided tenant ID in request body",
                        "content_type", r.Header.Get("Content-Type"),
                        "body_preview", string(body[:min(len(body), 200)]),
                    )
                    http.Error(w, "Security violation: tenant_id cannot be provided by user", http.StatusForbidden)
                    return
                }
            }

            next.ServeHTTP(w, r)
        })
    }
}

// RejectUserProvidedTenantHeaders rejects any request with tenant-related headers
func RejectUserProvidedTenantHeaders() func(http.Handler) http.Handler {
    forbiddenHeaders := []string{
        "X-Tenant-ID",
        "X-Tenant",
        "Tenant-ID",
        "Tenant",
    }

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            for _, header := range forbiddenHeaders {
                if value := r.Header.Get(header); value != "" {
                    slog.Error("Security violation: forbidden tenant header",
                        "header", header,
                        "value", value,
                        "user_agent", r.UserAgent(),
                        "remote_addr", r.RemoteAddr,
                    )
                    http.Error(w, fmt.Sprintf("Security violation: %s header not allowed", header), http.StatusForbidden)
                    return
                }
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

### Phase 1: Entity Ownership Validation

#### 1.1 Add Ownership Validation Interface
```go
// File: go/backup/restore/security/validator.go
type SecurityValidator interface {
    ValidateEntityOwnership(ctx context.Context, entityID string, userID string) error
    ValidateRelationshipIntegrity(ctx context.Context, fileType string, parentEntityType string) error
    ValidateImportScope(ctx context.Context, entityID string, importSession string) error
    LogUnauthorizedAttempt(ctx context.Context, attempt UnauthorizedAttempt)
}

type UnauthorizedAttempt struct {
    UserID           string
    TargetEntityID   string
    EntityType       string
    Operation        string
    AttemptType      string
    Timestamp        time.Time
    RequestDetails   map[string]interface{}
}
```

#### 1.2 Implement Security Validator
```go
// File: go/backup/restore/security/validator.go
type RestoreSecurityValidator struct {
    registrySet *registry.RegistrySet
    logger      *slog.Logger
}

func (v *RestoreSecurityValidator) ValidateEntityOwnership(ctx context.Context, entityID string, userID string) error {
    // Check commodity ownership
    if commodity, err := v.registrySet.CommodityRegistry.Get(ctx, entityID); err == nil {
        if commodity.UserID != userID {
            v.LogUnauthorizedAttempt(ctx, UnauthorizedAttempt{
                UserID:         userID,
                TargetEntityID: entityID,
                EntityType:     "commodity",
                Operation:      "restore_link_files",
                AttemptType:    "cross_user_access",
                Timestamp:      time.Now(),
            })
            return errors.New("unauthorized: cannot link to entity owned by different user")
        }
        return nil
    }
    
    // Check area ownership
    if area, err := v.registrySet.AreaRegistry.Get(ctx, entityID); err == nil {
        if area.UserID != userID {
            v.LogUnauthorizedAttempt(ctx, UnauthorizedAttempt{
                UserID:         userID,
                TargetEntityID: entityID,
                EntityType:     "area",
                Operation:      "restore_link_files",
                AttemptType:    "cross_user_access",
                Timestamp:      time.Now(),
            })
            return errors.New("unauthorized: cannot link to entity owned by different user")
        }
        return nil
    }
    
    // Check location ownership
    if location, err := v.registrySet.LocationRegistry.Get(ctx, entityID); err == nil {
        if location.UserID != userID {
            v.LogUnauthorizedAttempt(ctx, UnauthorizedAttempt{
                UserID:         userID,
                TargetEntityID: entityID,
                EntityType:     "location",
                Operation:      "restore_link_files",
                AttemptType:    "cross_user_access",
                Timestamp:      time.Now(),
            })
            return errors.New("unauthorized: cannot link to entity owned by different user")
        }
        return nil
    }
    
    // Entity not found - this is also unauthorized
    v.LogUnauthorizedAttempt(ctx, UnauthorizedAttempt{
        UserID:         userID,
        TargetEntityID: entityID,
        EntityType:     "unknown",
        Operation:      "restore_link_files",
        AttemptType:    "non_existent_entity_access",
        Timestamp:      time.Now(),
    })
    return errors.New("unauthorized: entity not found or access denied")
}
```

### Phase 2: Integration with Restore Processor

#### 2.1 Modify Restore Processor
```go
// File: go/backup/restore/processor/processor.go
type RestoreOperationProcessor struct {
    // ... existing fields ...
    securityValidator security.SecurityValidator
}

func NewRestoreOperationProcessor(operationID string, registrySet *registry.RegistrySet, entityService *services.EntityService, fileStorageURL string) *RestoreOperationProcessor {
    return &RestoreOperationProcessor{
        // ... existing initialization ...
        securityValidator: security.NewRestoreSecurityValidator(registrySet, logger),
    }
}
```

#### 2.2 Add Security Validation to Entity Processing
```go
// File: go/backup/restore/processor/processor.go
func (l *RestoreOperationProcessor) processLocation(ctx context.Context, location *models.Location, originalXMLID string, existing *ExistingEntities, idMapping *IDMapping, options types.RestoreOptions) error {
    // Get current user from context
    currentUser := appctx.GetUser(ctx)
    if currentUser == nil {
        return errors.New("unauthorized: no user context")
    }
    
    // If this is a merge operation and we're trying to link to an existing entity
    if options.Strategy == types.RestoreStrategyMergeAdd {
        existingLocation := existing.Locations[originalXMLID]
        if existingLocation != nil {
            // Validate that the user owns this entity
            err := l.securityValidator.ValidateEntityOwnership(ctx, existingLocation.ID, currentUser.ID)
            if err != nil {
                return err
            }
        }
    }
    
    // ... rest of existing logic ...
}
```

### Phase 3: Relationship Validation

#### 3.1 Add Relationship Integrity Checks
```go
// File: go/backup/restore/security/validator.go
func (v *RestoreSecurityValidator) ValidateRelationshipIntegrity(ctx context.Context, fileType string, parentEntityType string) error {
    allowedRelationships := map[string][]string{
        "invoice": {"commodity"},
        "image":   {"commodity"},
        "manual":  {"commodity"},
    }
    
    allowedParents, exists := allowedRelationships[fileType]
    if !exists {
        return fmt.Errorf("unknown file type: %s", fileType)
    }
    
    for _, allowedParent := range allowedParents {
        if allowedParent == parentEntityType {
            return nil // Valid relationship
        }
    }
    
    return fmt.Errorf("invalid relationship: %s files cannot be linked to %s entities", fileType, parentEntityType)
}
```

### Phase 4: Import Scope Restrictions

#### 4.1 Track Import Session Entities
```go
// File: go/backup/restore/processor/processor.go
type RestoreOperationProcessor struct {
    // ... existing fields ...
    importSessionEntities map[string]bool // Track entities created in this session
}

func (l *RestoreOperationProcessor) trackCreatedEntity(entityID string) {
    if l.importSessionEntities == nil {
        l.importSessionEntities = make(map[string]bool)
    }
    l.importSessionEntities[entityID] = true
}
```

#### 4.2 Validate Import Scope
```go
// File: go/backup/restore/security/validator.go
func (v *RestoreSecurityValidator) ValidateImportScope(ctx context.Context, entityID string, importSession string, sessionEntities map[string]bool) error {
    // Check if entity was created in this import session
    if sessionEntities[entityID] {
        return nil // OK - entity created in this import
    }
    
    // Check if user already owns this entity
    currentUser := appctx.GetUser(ctx)
    if currentUser == nil {
        return errors.New("unauthorized: no user context")
    }
    
    err := v.ValidateEntityOwnership(ctx, entityID, currentUser.ID)
    if err != nil {
        return fmt.Errorf("unauthorized: cannot link to entity outside import scope: %w", err)
    }
    
    return nil // OK - user owns existing entity
}
```

### Phase 5: Security Logging

#### 5.1 Implement Structured Logging
```go
// File: go/backup/restore/security/validator.go
func (v *RestoreSecurityValidator) LogUnauthorizedAttempt(ctx context.Context, attempt UnauthorizedAttempt) {
    v.logger.Warn("Unauthorized entity access attempt",
        "user_id", attempt.UserID,
        "target_entity_id", attempt.TargetEntityID,
        "entity_type", attempt.EntityType,
        "operation", attempt.Operation,
        "attempt_type", attempt.AttemptType,
        "timestamp", attempt.Timestamp,
        "request_details", attempt.RequestDetails,
    )
    
    // TODO: Consider additional security measures:
    // - Rate limiting for repeated attempts
    // - Alerting for suspicious patterns
    // - Temporary account restrictions
}
```

## Implementation Order

1. **Create security package** (`go/backup/restore/security/`)
2. **Implement SecurityValidator interface** with basic ownership validation
3. **Integrate with RestoreOperationProcessor** for entity processing
4. **Add relationship validation** for file-to-entity links
5. **Implement import scope restrictions** 
6. **Add comprehensive security logging**
7. **Run tests** to verify all security requirements are met

## Testing Strategy

### Unit Tests
- Test each security validation function independently
- Mock registry dependencies for isolated testing
- Test both success and failure scenarios

### Integration Tests  
- Use existing test cases in `merge_add_strategy_test.go`
- Verify cross-user and cross-tenant access prevention
- Test valid user manipulations still work
- Verify security logging functionality

### Security Tests
- Attempt various attack scenarios
- Verify no data leakage between users/tenants
- Test edge cases and boundary conditions

## Configuration

### Security Settings
```go
type SecurityConfig struct {
    EnableOwnershipValidation    bool
    EnableRelationshipValidation bool
    EnableImportScopeRestriction bool
    EnableSecurityLogging        bool
    LogLevel                     string
}
```

### Default Configuration
- All security features enabled by default
- Warn-level logging for unauthorized attempts
- Strict validation for production environments

## Monitoring and Alerting

### Metrics to Track
- Number of unauthorized access attempts per user
- Types of security violations (cross-user, cross-tenant, etc.)
- Success/failure rates of restore operations
- Performance impact of security validation

### Alert Conditions
- High frequency of unauthorized attempts from single user
- Attempts to access high-value entities
- Patterns indicating automated attacks
- Security validation failures

## Performance Considerations

### Optimization Strategies
- Cache entity ownership information during import session
- Batch ownership validation for multiple entities
- Use database indexes on user_id fields
- Consider read-through caching for frequently accessed entities

### Expected Impact
- Minimal performance impact for legitimate operations
- Additional database queries for ownership validation
- Logging overhead for security events

## Future Enhancements

### Advanced Security Features
- Role-based access control (RBAC)
- Fine-grained permissions per entity type
- Audit trail for all entity modifications
- Integration with external security systems

### Compliance Features
- GDPR compliance for data access logging
- SOX compliance for financial data protection
- HIPAA compliance for sensitive information

## Conclusion

This implementation provides comprehensive security validation for the restore/import system while maintaining flexibility for legitimate user operations. The test-driven approach ensures all security requirements are met and prevents regressions.
