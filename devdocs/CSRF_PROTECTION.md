# CSRF Protection System

This document describes the Cross-Site Request Forgery (CSRF) protection implementation in Inventario.

## Overview

Inventario implements comprehensive CSRF protection for all state-changing operations (POST, PUT, PATCH, DELETE requests). The system uses per-user CSRF tokens that are generated on login and validated on every mutating request.

## Architecture

### Components

1. **CSRF Service factory** (`go/services/csrf_service.go`) and backends (`go/csrf/{inmemory,redis,noop}`)
   - `NewCSRFService(redisURL)` selects the backend: Redis when a URL is set, otherwise in-memory with a warning.
   - The token lifecycle (generation, validation, revocation, expiry) lives in the backend packages under `go/csrf/` — `inmemory` (default), `redis` (production / multi-instance), and `noop` (testing).
   - Tokens expire after 1 hour and are refreshed on token refresh

2. **CSRF Middleware** (`go/apiserver/csrf_middleware.go`)
   - Validates CSRF tokens for state-changing HTTP requests
   - Bypasses safe methods (GET, HEAD, OPTIONS)
   - Implements fail-open design for backend errors

3. **Frontend Integration** (`frontend/src/lib/http.ts`)
   - The single fetch-based HTTP wrapper (no axios) stores the CSRF token in memory
   - Automatically includes the token in mutating requests
   - Recovers the token from login/refresh responses

## Token Flow

### Login Flow
```
1. User submits credentials to POST /api/v1/auth/login
2. Backend validates credentials
3. Backend generates JWT access token
4. Backend generates CSRF token and stores in Redis/memory
5. Backend returns both tokens in response
6. Frontend stores JWT in localStorage
7. Frontend stores CSRF token in memory
```

### Request Flow
```
1. Frontend makes mutating request (POST/PUT/PATCH/DELETE)
2. The fetch wrapper (src/lib/http.ts) adds the X-CSRF-Token header
3. CSRF middleware validates token against stored value
4. Request proceeds if token is valid
5. Request is rejected with 403 if token is missing/invalid
```

### Token Refresh Flow
```
1. Frontend calls POST /api/v1/auth/refresh
2. Backend validates refresh token cookie
3. Backend generates new JWT access token
4. Backend generates new CSRF token (replaces old one)
5. Backend returns both tokens in response
6. Frontend updates both tokens
```

## Configuration

### Backend Configuration

#### Development (In-Memory)
```bash
# No configuration needed - uses in-memory storage by default
./inventario run
```

#### Production (Redis)
```bash
# Using CLI flag
./inventario run --csrf-redis-url="redis://localhost:6379/0"

# Using environment variable
export INVENTARIO_RUN_CSRF_REDIS_URL="redis://localhost:6379/0"
./inventario run
```

### CORS Configuration

CSRF protection requires proper CORS configuration:

```bash
# Development with the in-memory backend (memory:// DSN): a fixed local
# dev allowlist (DefaultDevAllowedOrigins) is applied automatically.
./inventario run

# Production (whitelist specific origins)
./inventario run --allowed-origins="https://example.com,https://app.example.com"
```

CORS is **fail-closed**: when `AllowedOrigins` is empty the middleware rejects
every cross-origin request (`AllowOriginFunc` returns `false`). The convenience
dev allowlist is only seeded when the database DSN is `memory://`; `*` and `null`
origins are rejected outright. See `go/apiserver/cors_config.go`.

The CORS configuration automatically includes:
- `AllowCredentials: true` (required for httpOnly cookies)
- `X-CSRF-Token` in allowed and exposed headers
- Restricted methods: GET, POST, PUT, PATCH, DELETE, OPTIONS

## Security Features

### 1. Token Generation
- Cryptographically secure random tokens (32 bytes, base64-encoded)
- Unique per user
- Stored server-side only

### 2. Token Validation
- Required for all POST/PUT/PATCH/DELETE requests
- Bypassed for safe methods (GET/HEAD/OPTIONS)
- Validated against server-side storage

### 3. Token Lifecycle
- Generated on login
- Refreshed on token refresh
- Deleted on logout
- Expires after 1 hour (TTL)

### 4. Fail-Open Design
- Redis/storage outages don't block all writes
- Errors are logged for monitoring
- Operators should monitor CSRF service health

### 5. Multi-Instance Support
- Redis backend shares state across instances
- In-memory backend warns about multi-instance limitations

## Testing

### Unit Tests

**CSRF backend tests** (`go/csrf/inmemory/service_test.go`, `go/csrf/redis/service_test.go`, `go/csrf/noop/service_test.go`):
- Token generation and retrieval
- Token revocation / delete-all
- Multi-user isolation
- LRU eviction
- Concurrent access
- No-op service behavior

**CSRF factory tests** (`go/services/csrf_service_test.go`):
- Backend selection (`TestNewCSRFService_FallbackToInMemory`, `TestNewCSRFService_InvalidRedisURL`)

**CSRF Middleware Tests** (`go/apiserver/csrf_middleware_test.go`):
- Safe method bypass
- Valid token acceptance
- Missing token rejection
- Invalid token rejection
- Expired token handling
- Nil service disables CSRF
- Service error fail-open behavior
- All mutating methods require tokens

### Running Tests

```bash
# Run all CSRF tests
cd go
go test -v ./csrf/...
go test -v ./services -run CSRF
go test -v ./apiserver -run CSRF

# Run a specific backend test
go test -v ./csrf/inmemory -run TestService_GenerateToken
```

## Troubleshooting

### Common Issues

1. **403 Forbidden on mutating requests**
   - Ensure CSRF token is included in X-CSRF-Token header
   - Check that token hasn't expired (1 hour TTL)
   - Verify user is authenticated (JWT token valid)

2. **CSRF token not received on login**
   - Check backend logs for CSRF service errors
   - Verify Redis is accessible (if using Redis backend)
   - Ensure frontend is reading csrf_token from response

3. **Token mismatch errors**
   - User may have multiple sessions
   - Token may have been regenerated
   - Check for clock skew if using distributed systems

4. **Redis connection errors**
   - Verify Redis URL is correct
   - Check Redis is running and accessible
   - Review network/firewall rules
   - System will fall back to in-memory with warning

## Production Checklist

- [ ] Configure Redis for CSRF token storage (`--csrf-redis-url`)
- [ ] Set allowed CORS origins (`--allowed-origins`)
- [ ] Monitor CSRF service errors in logs
- [ ] Ensure Redis has proper backup/replication
- [ ] Test CSRF protection with security scanning tools
- [ ] Verify fail-open behavior is acceptable for your use case
- [ ] Document CSRF token handling for API consumers

## References

- Issue: [#837 - MVP Phase 1.3: Implement CSRF Protection](https://github.com/denisvmedia/inventario/issues/837)
- Related: [#497 - CRITICAL SECURITY: Missing Security Headers](https://github.com/denisvmedia/inventario/issues/497)

