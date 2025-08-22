package registry

import (
	"context"
	"fmt"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

// TestUserContext provides a standard test user context for unit tests
var TestUserContext = WithUserContext(context.Background(), "test-user-id")

// TestUser provides a standard test user for unit tests
var TestUser = &models.User{
	TenantAwareEntityID: models.TenantAwareEntityID{
		EntityID: models.EntityID{ID: "test-user-id"},
		TenantID: "test-tenant-id",
	},
	Email:    "test@example.com",
	Name:     "Test User",
	Role:     models.UserRoleUser,
	IsActive: true,
}

// CreateTestUser creates a test user in the given user registry
func CreateTestUser(c *qt.C, userRegistry UserRegistry) *models.User {
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-" + generateID()},
			TenantID: "test-tenant-id",
		},
		Email:    "test-" + generateID() + "@example.com",
		Name:     "Test User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}

	err := user.SetPassword("testpassword123")
	c.Assert(err, qt.IsNil)

	created, err := userRegistry.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsNotNil)

	return created
}

// CreateTestUserContext creates a test user and returns a context with that user
func CreateTestUserContext(c *qt.C, userRegistry UserRegistry) (context.Context, *models.User) {
	user := CreateTestUser(c, userRegistry)
	ctx := WithUserContext(context.Background(), user.ID)
	return ctx, user
}

// WithTestUser creates a context with a test user ID
func WithTestUser(ctx context.Context) context.Context {
	return WithUserContext(ctx, "test-user-id")
}

// generateID generates a simple unique ID for testing
func generateID() string {
	// Simple implementation for testing - in real code this would be more robust
	return "test-" + fmt.Sprintf("%d", time.Now().UnixNano())
}

// TestEntityWithUser is a helper interface for testing entities with user context
type TestEntityWithUser interface {
	GetUserID() string
	SetUserID(userID string)
}

// AssertUserOwnership verifies that an entity belongs to the expected user
func AssertUserOwnership(c *qt.C, entity TestEntityWithUser, expectedUserID string) {
	c.Assert(entity.GetUserID(), qt.Equals, expectedUserID)
}

// AssertUserIsolation verifies that a list of entities all belong to the expected user
func AssertUserIsolation(c *qt.C, entities []TestEntityWithUser, expectedUserID string) {
	for i, entity := range entities {
		c.Assert(entity.GetUserID(), qt.Equals, expectedUserID, 
			qt.Commentf("Entity at index %d belongs to user %s, expected %s", 
				i, entity.GetUserID(), expectedUserID))
	}
}

// TestRegistryWithUserIsolation provides a standard test pattern for registry user isolation
// Note: This is a template function - actual implementations should be created for specific entity types

// MockUserRegistry provides a simple mock for testing
type MockUserRegistry struct {
	users map[string]*models.User
}

// NewMockUserRegistry creates a new mock user registry
func NewMockUserRegistry() *MockUserRegistry {
	return &MockUserRegistry{
		users: make(map[string]*models.User),
	}
}

// Create implements UserRegistry.Create
func (m *MockUserRegistry) Create(ctx context.Context, user models.User) (*models.User, error) {
	if user.ID == "" {
		user.ID = generateID()
	}
	created := user
	m.users[user.ID] = &created
	return &created, nil
}

// Get implements UserRegistry.Get
func (m *MockUserRegistry) Get(ctx context.Context, id string) (*models.User, error) {
	user, exists := m.users[id]
	if !exists {
		return nil, ErrNotFound
	}
	return user, nil
}

// List implements UserRegistry.List
func (m *MockUserRegistry) List(ctx context.Context) ([]*models.User, error) {
	users := make([]*models.User, 0, len(m.users))
	for _, user := range m.users {
		users = append(users, user)
	}
	return users, nil
}

// Update implements UserRegistry.Update
func (m *MockUserRegistry) Update(ctx context.Context, user models.User) (*models.User, error) {
	if _, exists := m.users[user.ID]; !exists {
		return nil, ErrNotFound
	}
	updated := user
	m.users[user.ID] = &updated
	return &updated, nil
}

// Delete implements UserRegistry.Delete
func (m *MockUserRegistry) Delete(ctx context.Context, id string) error {
	if _, exists := m.users[id]; !exists {
		return ErrNotFound
	}
	delete(m.users, id)
	return nil
}

// GetByEmail implements UserRegistry.GetByEmail
func (m *MockUserRegistry) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	for _, user := range m.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, ErrNotFound
}

// Count implements UserRegistry.Count
func (m *MockUserRegistry) Count(ctx context.Context) (int, error) {
	return len(m.users), nil
}

// TestUserIsolationPattern provides a reusable test pattern for user isolation
func TestUserIsolationPattern(t *testing.T, testName string, testFunc func(*qt.C, context.Context, context.Context, *models.User, *models.User)) {
	t.Run(testName, func(t *testing.T) {
		c := qt.New(t)
		
		// Create mock user registry
		userRegistry := NewMockUserRegistry()
		
		// Create two test users
		user1, err := userRegistry.Create(context.Background(), models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "user-1"},
				TenantID: "test-tenant-id",
			},
			Email: "user1@test.com",
			Name:  "User 1",
			Role:  models.UserRoleUser,
		})
		c.Assert(err, qt.IsNil)
		
		user2, err := userRegistry.Create(context.Background(), models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "user-2"},
				TenantID: "test-tenant-id",
			},
			Email: "user2@test.com",
			Name:  "User 2",
			Role:  models.UserRoleUser,
		})
		c.Assert(err, qt.IsNil)
		
		// Create contexts for each user
		ctx1 := WithUserContext(context.Background(), user1.ID)
		ctx2 := WithUserContext(context.Background(), user2.ID)
		
		// Run the test function
		testFunc(c, ctx1, ctx2, user1, user2)
	})
}
