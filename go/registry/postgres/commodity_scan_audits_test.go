package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// newTestScanAudit builds a valid CommodityScanAudit for the postgres
// backend. user/tenant must reference real rows because the table
// carries FKs onto users / tenants.
func newTestScanAudit(user *models.User) models.CommodityScanAudit {
	return models.CommodityScanAudit{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: user.TenantID,
			UserID:   user.ID,
		},
		Provider:        "mock",
		Model:           "mock-model",
		PhotoCount:      1,
		TotalPhotoBytes: 100,
		Status:          models.CommodityScanStatusOK,
		LatencyMS:       10,
	}
}

func TestCommodityScanAuditRegistry_Record_HappyPath_Postgres(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	created, err := registrySet.CommodityScanAuditRegistry.Record(ctx, newTestScanAudit(user))
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsNotNil)
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.UUID, qt.Not(qt.Equals), "")
	c.Assert(created.CreatedAt.IsZero(), qt.IsFalse)
}

func TestCommodityScanAuditRegistry_Record_MissingFields_Postgres(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	cases := []struct {
		name string
		mut  func(*models.CommodityScanAudit)
	}{
		{"tenant_id empty", func(a *models.CommodityScanAudit) { a.TenantID = "" }},
		{"user_id empty", func(a *models.CommodityScanAudit) { a.UserID = "" }},
		{"status empty", func(a *models.CommodityScanAudit) { a.Status = "" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			a := newTestScanAudit(user)
			tc.mut(&a)
			_, err := registrySet.CommodityScanAuditRegistry.Record(ctx, a)
			c.Assert(err, qt.IsNotNil)
			c.Assert(errors.Is(err, registry.ErrFieldRequired), qt.IsTrue)
		})
	}
}

func TestCommodityScanAuditRegistry_CountRecentForUser_Postgres(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	for range 3 {
		_, err := registrySet.CommodityScanAuditRegistry.Record(ctx, newTestScanAudit(user))
		c.Assert(err, qt.IsNil)
	}

	count, err := registrySet.CommodityScanAuditRegistry.CountRecentForUser(ctx, user.TenantID, user.ID, time.Now().Add(-1*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 3)
}

func TestCommodityScanAuditRegistry_DeleteOlderThan_Postgres(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	_, err := registrySet.CommodityScanAuditRegistry.Record(ctx, newTestScanAudit(user))
	c.Assert(err, qt.IsNil)

	// Past cutoff in the future deletes everything.
	err = registrySet.CommodityScanAuditRegistry.DeleteOlderThan(ctx, time.Now().Add(1*time.Hour))
	c.Assert(err, qt.IsNil)

	count, err := registrySet.CommodityScanAuditRegistry.CountRecentForUser(ctx, user.TenantID, user.ID, time.Now().Add(-1*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}
