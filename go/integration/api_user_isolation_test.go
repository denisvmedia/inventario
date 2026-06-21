package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/golang-jwt/jwt/v5"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/debug"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
	"github.com/denisvmedia/inventario/services"
)

// apiIsolationFixture bundles one user with their own group (incl. an
// owner-membership row so GroupSlugResolverMiddleware admits them) and a ready
// WithUser+WithGroup context for registry-level seeding.
type apiIsolationFixture struct {
	user  *models.User
	group *models.LocationGroup
	ctx   context.Context
}

// setupTestAPIServer creates a test API server with authentication, a real
// tenant, and TWO users — each owning a SEPARATE group. The modern data routes
// are group-scoped (/api/v1/g/{groupSlug}/...) and gated by group membership, so
// isolation is exercised by routing each user through their own group's slug.
func setupTestAPIServer(t *testing.T) (server *httptest.Server, fs *registry.FactorySet, user1, user2 apiIsolationFixture, jwtSecret string, cleanup func()) {
	t.Helper()
	c := qt.New(t)
	dsn := mustTestDSN(t)

	err := setupFreshDatabase(dsn)
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to setup fresh database"))

	registrySetFunc, cleanupFunc := postgres.NewPostgresRegistrySet()
	factorySet, err := registrySetFunc(registry.Config(dsn))
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create factory set"))

	jwtSecretBytes := []byte("test-secret-32-bytes-minimum-length")

	// Real tenant.
	uniq := fmt.Sprintf("%d", time.Now().UnixNano())
	createdTenant := must.Must(factorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "Test Tenant API " + uniq,
		Slug:   "test-tenant-api-" + uniq,
		Status: models.TenantStatusActive,
	}))

	// Two users in the same tenant.
	createdUser1 := mustCreateAPIUser(c, factorySet, createdTenant.ID, "user1-"+uniq+"@api-test.com")
	createdUser2 := mustCreateAPIUser(c, factorySet, createdTenant.ID, "user2-"+uniq+"@api-test.com")

	// GroupService wired exactly like the production apiserver so CreateGroup
	// both creates the group AND inserts the creator's owner-membership row
	// (required by GroupSlugResolverMiddleware).
	groupService := services.NewGroupService(
		factorySet.LocationGroupRegistry,
		factorySet.GroupMembershipRegistry,
		factorySet.GroupInviteRegistry,
	)
	groupService.SetUserRegistry(factorySet.UserRegistry)

	group1 := must.Must(groupService.CreateGroup(context.Background(), createdTenant.ID, createdUser1.ID, "User1 Group", "", "", models.Currency("USD")))
	group2 := must.Must(groupService.CreateGroup(context.Background(), createdTenant.ID, createdUser2.ID, "User2 Group", "", "", models.Currency("USD")))

	user1 = apiIsolationFixture{user: createdUser1, group: group1, ctx: userGroupContext(context.Background(), createdUser1, group1)}
	user2 = apiIsolationFixture{user: createdUser2, group: group2, ctx: userGroupContext(context.Background(), createdUser2, group2)}

	params := apiserver.Params{
		FactorySet:     factorySet,
		EntityService:  services.NewEntityService(factorySet, "file://uploads?memfs=1&create_dir=1"),
		UploadLocation: "file://uploads?memfs=1&create_dir=1",
		DebugInfo:      debug.NewInfo("postgres://test", "file://uploads?memfs=1&create_dir=1"),
		StartTime:      time.Now(),
		JWTSecret:      jwtSecretBytes,
	}

	handler := apiserver.APIServer(params, nil)
	server = httptest.NewServer(handler)

	cleanup = func() {
		server.Close()
		if cleanupFunc != nil {
			cleanupFunc()
		}
	}

	return server, factorySet, user1, user2, string(jwtSecretBytes), cleanup
}

// mustCreateAPIUser creates an active user with a known password in the tenant.
func mustCreateAPIUser(c *qt.C, fs *registry.FactorySet, tenantID, email string) *models.User {
	c.Helper()
	u := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               email,
		Name:                "API Test User",
		IsActive:            true,
	}
	c.Assert(u.SetPassword("TestPassword123"), qt.IsNil)
	return must.Must(fs.UserRegistry.Create(context.Background(), u))
}

// generateJWTToken creates a JWT access token for the given user. The
// token_type=access claim is mandatory post-#1778 — the JWT middleware rejects
// any token without it as "invalid token type".
func generateJWTToken(user *models.User, jwtSecret string) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    user.ID,
		"token_type": "access",
		"exp":        now.Add(24 * time.Hour).Unix(),
		"iat":        now.Unix(),
	})

	return token.SignedString([]byte(jwtSecret))
}

// makeAuthenticatedRequest makes an HTTP request with JWT authentication.
func makeAuthenticatedRequest(method, url string, body []byte, token string) (*http.Response, error) {
	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequest(method, url, bytes.NewBuffer(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	return client.Do(req)
}

// seedAPILocationArea creates a location + area in the fixture's group via the
// user-aware registry, returning the area id the HTTP commodity-create needs.
func seedAPILocationArea(c *qt.C, fs *registry.FactorySet, f apiIsolationFixture) string {
	c.Helper()
	locReg := must.Must(fs.LocationRegistryFactory.CreateUserRegistry(f.ctx))
	loc := must.Must(locReg.Create(f.ctx, models.Location{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        f.user.TenantID,
			GroupID:         f.group.ID,
			CreatedByUserID: f.user.ID,
		},
		Name:    "Test Location",
		Address: "123 Test St",
	}))
	areaReg := must.Must(fs.AreaRegistryFactory.CreateUserRegistry(f.ctx))
	area := must.Must(areaReg.Create(f.ctx, models.Area{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        f.user.TenantID,
			GroupID:         f.group.ID,
			CreatedByUserID: f.user.ID,
		},
		Name:       "Test Area",
		LocationID: loc.ID,
	}))
	return area.ID
}

// commoditiesPath builds the group-scoped commodities collection URL.
func commoditiesPath(serverURL, groupSlug string) string {
	return serverURL + "/api/v1/g/" + groupSlug + "/commodities"
}

// TestAPIUserIsolation_Commodities exercises commodity API isolation at the HTTP
// layer: user1 creates a commodity in their group; user2 (a different group) must
// not see it; and user2 is denied membership when targeting user1's group slug.
func TestAPIUserIsolation_Commodities(t *testing.T) {
	server, fs, user1, user2, jwtSecret, cleanup := setupTestAPIServer(t)
	defer cleanup()
	c := qt.New(t)

	areaID := seedAPILocationArea(c, fs, user1)

	token1, err := generateJWTToken(user1.user, jwtSecret)
	c.Assert(err, qt.IsNil)
	token2, err := generateJWTToken(user2.user, jwtSecret)
	c.Assert(err, qt.IsNil)

	// User1 creates a commodity (draft → bypasses group-currency price math).
	obj := &jsonapi.CommodityRequest{
		Data: &jsonapi.CommodityData{
			Type: "commodities",
			Attributes: &models.Commodity{
				Name:                   "New Commodity in Area 2",
				ShortName:              "NewCom2",
				AreaID:                 new(areaID),
				Type:                   models.CommodityTypeElectronics,
				OriginalPrice:          must.Must(decimal.NewFromString("1000.00")),
				OriginalPriceCurrency:  models.Currency("USD"),
				ConvertedOriginalPrice: must.Must(decimal.NewFromString("0")),
				CurrentPrice:           must.Must(decimal.NewFromString("800.00")),
				SerialNumber:           "SN123456",
				ExtraSerialNumbers:     []string{"SN654321"},
				PartNumbers:            []string{"P123", "P456"},
				Tags:                   []string{"tag1", "tag2"},
				Count:                  1,
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
				RegisteredDate:         models.ToPDate("2023-01-02"),
				LastModifiedDate:       models.ToPDate("2023-01-03"),
				Comments:               "New commodity comments",
				Draft:                  true,
			},
		},
	}

	jsonData, err := json.Marshal(obj)
	c.Assert(err, qt.IsNil)

	resp, err := makeAuthenticatedRequest(http.MethodPost, commoditiesPath(server.URL, user1.group.Slug), jsonData, token1)
	c.Assert(err, qt.IsNil)
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		c.Fatalf("Expected status %d, got %d. Response body: %s", http.StatusCreated, resp.StatusCode, string(body))
	}
	resp.Body.Close()

	// User2, on THEIR OWN group, sees an empty list (group-scoped isolation).
	resp, err = makeAuthenticatedRequest(http.MethodGet, commoditiesPath(server.URL, user2.group.Slug), nil, token2)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)
	c.Assert(decodeDataLen(c, resp), qt.Equals, 0)

	// User2 is forbidden from even addressing user1's group slug (not a member).
	resp, err = makeAuthenticatedRequest(http.MethodGet, commoditiesPath(server.URL, user1.group.Slug), nil, token2)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusForbidden)
	resp.Body.Close()

	// User1 sees their one commodity.
	resp, err = makeAuthenticatedRequest(http.MethodGet, commoditiesPath(server.URL, user1.group.Slug), nil, token1)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	var commodities map[string]any
	c.Assert(json.NewDecoder(resp.Body).Decode(&commodities), qt.IsNil)
	resp.Body.Close()
	dataArray, ok := commodities["data"].([]any)
	c.Assert(ok, qt.IsTrue)
	c.Assert(dataArray, qt.HasLen, 1)
	name := dataArray[0].(map[string]any)["attributes"].(map[string]any)["name"]
	c.Assert(name, qt.Equals, obj.Data.Attributes.Name)
}

// decodeDataLen decodes a JSON:API collection response and returns len(data).
func decodeDataLen(c *qt.C, resp *http.Response) int {
	c.Helper()
	defer resp.Body.Close()
	var payload map[string]any
	c.Assert(json.NewDecoder(resp.Body).Decode(&payload), qt.IsNil)
	data, ok := payload["data"].([]any)
	c.Assert(ok, qt.IsTrue, qt.Commentf("response has no data array: %v", payload))
	return len(data)
}

// TestAPIAuthentication tests authentication requirements for group-scoped API
// endpoints: every data route must reject an unauthenticated request with 401.
func TestAPIAuthentication(t *testing.T) {
	server, _, user1, _, _, cleanup := setupTestAPIServer(t)
	defer cleanup()

	slug := user1.group.Slug
	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/g/" + slug + "/commodities"},
		{http.MethodPost, "/api/v1/g/" + slug + "/commodities"},
		{http.MethodGet, "/api/v1/g/" + slug + "/locations"},
		{http.MethodPost, "/api/v1/g/" + slug + "/locations"},
		{http.MethodGet, "/api/v1/g/" + slug + "/areas"},
		{http.MethodPost, "/api/v1/g/" + slug + "/areas"},
		{http.MethodGet, "/api/v1/g/" + slug + "/files"},
		{http.MethodGet, "/api/v1/g/" + slug + "/exports"},
		{http.MethodPost, "/api/v1/g/" + slug + "/exports"},
	}

	for _, endpoint := range endpoints {
		t.Run(fmt.Sprintf("%s %s", endpoint.method, endpoint.path), func(t *testing.T) {
			c := qt.New(t)
			var req *http.Request
			var err error

			if endpoint.method == http.MethodPost {
				jsonData := must.Must(json.Marshal(map[string]any{"name": "Test"}))
				req, err = http.NewRequest(endpoint.method, server.URL+endpoint.path, bytes.NewBuffer(jsonData))
			} else {
				req, err = http.NewRequest(endpoint.method, server.URL+endpoint.path, nil)
			}
			c.Assert(err, qt.IsNil)
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{}
			resp, err := client.Do(req)
			c.Assert(err, qt.IsNil)
			defer resp.Body.Close()
			c.Assert(resp.StatusCode, qt.Equals, http.StatusUnauthorized)
		})
	}
}
