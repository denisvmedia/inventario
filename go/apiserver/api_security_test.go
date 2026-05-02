package apiserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// `TestAPISecurity_MaliciousFileUpload` (formerly here) was deleted
// under #1421 alongside the legacy `POST /uploads/commodities/{id}/images`
// route it exercised. The migration path is: POST a multipart file to
// the unified `/uploads/file` (creates an unlinked FileEntity) and then
// PUT `/files/{id}` with `linked_entity_*` to attach the row to a
// commodity / location. Cross-tenant access is blocked by the same
// group-scoped middleware that protects every other write — covered
// by `apiserver/files` integration tests rather than re-tested here.

// TestAPISecurity_CrossTenantExportAttempt tests that users cannot export other tenants' data
func TestAPISecurity_CrossTenantExportAttempt(t *testing.T) {
	c := qt.New(t)

	// Setup test environment
	factorySet := memory.NewFactorySet()
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")

	// Create users in different tenants
	userTenant1 := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-tenant1"},
			TenantID: "tenant-1",
		},
		Email:    "user@tenant1.com",
		Name:     "Tenant 1 User",
		IsActive: true,
	}
	userTenant1.SetPassword("password123")

	userTenant2 := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-tenant2"},
			TenantID: "tenant-2",
		},
		Email:    "user@tenant2.com",
		Name:     "Tenant 2 User",
		IsActive: true,
	}
	userTenant2.SetPassword("password123")

	// registrySet := factorySet.CreateUserRegistrySet()
	userRegistry := factorySet.UserRegistry

	// Add users to registry
	u1, err := userRegistry.Create(context.Background(), *userTenant1)
	c.Assert(err, qt.IsNil)
	u2, err := userRegistry.Create(context.Background(), *userTenant2)
	c.Assert(err, qt.IsNil)

	// Tenant 1 user creates an export
	tenant1Ctx := appctx.WithUser(context.Background(), u1)
	registrySet1, err := factorySet.CreateUserRegistrySet(tenant1Ctx)
	c.Assert(err, qt.IsNil)
	tenant1ExportReg := registrySet1.ExportRegistry
	c.Assert(err, qt.IsNil)

	export := models.Export{
		Type:   models.ExportTypeFullDatabase,
		Status: models.ExportStatusCompleted,
	}
	createdExport, err := tenant1ExportReg.Create(tenant1Ctx, export)
	c.Assert(err, qt.IsNil)

	// Create JWT token for tenant 2 user
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": u2.ID,
		"exp":     time.Now().Add(time.Hour).Unix(),
	})
	tokenString, err := token.SignedString(jwtSecret)
	c.Assert(err, qt.IsNil)

	// Setup API server
	params := Params{
		JWTSecret:      jwtSecret,
		UploadLocation: "file://./test_uploads?create_dir=true",
		EntityService:  services.NewEntityService(factorySet, "file://./test_uploads?create_dir=true"),
	}

	r := chi.NewRouter()
	userMiddlewares := createUserAwareMiddlewares(jwtSecret, factorySet, nil, nil)
	r.With(userMiddlewares...).Route("/exports", Exports(params, nil))

	// Tenant 2 user attempts to access Tenant 1's export (SECURITY VIOLATION)
	req := httptest.NewRequest("GET", "/exports/"+createdExport.ID, nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should fail with 404 (not found) as per project standards for RLS violations
	// Note: The actual response might be 401 if user context validation fails first
	expectedCodes := []int{http.StatusNotFound, http.StatusUnauthorized}
	c.Assert(expectedCodes, qt.Contains, w.Code,
		qt.Commentf("Tenant 2 user should not be able to access Tenant 1's export, got status %d", w.Code))
}

// TestAPISecurity_InvalidUserContexts tests edge cases with invalid user contexts
func TestAPISecurity_InvalidUserContexts(t *testing.T) {
	c := qt.New(t)
	_ = c // Use the variable to avoid compilation error

	tests := []struct {
		name           string
		setupUser      func() *models.User
		expectedStatus int
		description    string
	}{
		{
			name: "empty_user_id",
			setupUser: func() *models.User {
				return &models.User{
					TenantAwareEntityID: models.TenantAwareEntityID{
						EntityID: models.EntityID{ID: ""}, // Empty ID
						TenantID: "test-tenant",
					},
					Email:    "test@example.com",
					Name:     "Test User",
					IsActive: true,
				}
			},
			expectedStatus: http.StatusUnauthorized,
			description:    "Should reject empty user ID",
		},
		{
			name: "empty_tenant_id",
			setupUser: func() *models.User {
				return &models.User{
					TenantAwareEntityID: models.TenantAwareEntityID{
						EntityID: models.EntityID{ID: "valid-user-id"},
						TenantID: "", // Empty tenant ID
					},
					Email:    "test@example.com",
					Name:     "Test User",
					IsActive: true,
				}
			},
			expectedStatus: http.StatusUnauthorized,
			description:    "Should reject empty tenant ID",
		},
		{
			name: "inactive_user",
			setupUser: func() *models.User {
				return &models.User{
					TenantAwareEntityID: models.TenantAwareEntityID{
						EntityID: models.EntityID{ID: "inactive-user-id"},
						TenantID: "test-tenant",
					},
					Email:    "inactive@example.com",
					Name:     "Inactive User",
					IsActive: false, // Inactive user
				}
			},
			expectedStatus: http.StatusUnauthorized, // JWT middleware rejects inactive users with 401
			description:    "Should reject inactive user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Setup test environment
			jwtSecret := []byte("test-secret-32-bytes-minimum-length")

			// Setup user
			user := tt.setupUser()
			user.SetPassword("password123")
			factorySet := memory.NewFactorySet()

			// Add user to registry (even inactive users might be in the registry)
			// Skip creating users with empty tenant ID as they shouldn't exist in the registry
			if user.ID != "" && user.TenantID != "" {
				_, err := factorySet.UserRegistry.Create(context.Background(), *user)
				c.Assert(err, qt.IsNil)
			}

			// Create JWT token
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"user_id": user.ID,
				"exp":     time.Now().Add(time.Hour).Unix(),
			})
			tokenString, err := token.SignedString(jwtSecret)
			c.Assert(err, qt.IsNil)

			// Setup API server with RLS middleware
			r := chi.NewRouter()
			userMiddlewares := createUserAwareMiddlewares(jwtSecret, factorySet, nil, nil)
			r.With(userMiddlewares...).Get("/test", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Make request
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", "Bearer "+tokenString)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			c.Assert(w.Code, qt.Equals, tt.expectedStatus, qt.Commentf(tt.description))
		})
	}
}
