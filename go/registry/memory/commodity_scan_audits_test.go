package memory_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func newTestAudit(userID string) models.CommodityScanAudit {
	return models.CommodityScanAudit{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: "tenant-1",
			UserID:   userID,
		},
		Provider:        "mock",
		Model:           "mock-model",
		PhotoCount:      1,
		TotalPhotoBytes: 100,
		Status:          models.CommodityScanStatusOK,
		LatencyMS:       10,
	}
}

func TestCommodityScanAuditRegistry_Record_HappyPath(t *testing.T) {
	c := qt.New(t)
	r := memory.NewCommodityScanAuditRegistry()

	created, err := r.Record(context.Background(), newTestAudit("user-1"))
	c.Assert(err, qt.IsNil)
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.UUID, qt.Not(qt.Equals), "")
	c.Assert(created.CreatedAt.IsZero(), qt.IsFalse)
}

func TestCommodityScanAuditRegistry_Record_MissingFields(t *testing.T) {
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
			r := memory.NewCommodityScanAuditRegistry()
			a := newTestAudit("user-1")
			tc.mut(&a)
			_, err := r.Record(context.Background(), a)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
		})
	}
}

func TestCommodityScanAuditRegistry_CountRecentForUser(t *testing.T) {
	c := qt.New(t)
	r := memory.NewCommodityScanAuditRegistry()

	for range 3 {
		_, err := r.Record(context.Background(), newTestAudit("user-1"))
		c.Assert(err, qt.IsNil)
	}
	_, err := r.Record(context.Background(), newTestAudit("user-2"))
	c.Assert(err, qt.IsNil)

	count, err := r.CountRecentForUser(context.Background(), "tenant-1", "user-1", time.Now().Add(-1*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 3)

	count, err = r.CountRecentForUser(context.Background(), "tenant-1", "user-2", time.Now().Add(-1*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestCommodityScanAuditRegistry_CountRecentForUser_FilterByTime(t *testing.T) {
	c := qt.New(t)
	r := memory.NewCommodityScanAuditRegistry()

	old := newTestAudit("user-1")
	old.CreatedAt = time.Now().Add(-2 * time.Hour)
	_, err := r.Record(context.Background(), old)
	c.Assert(err, qt.IsNil)

	recent := newTestAudit("user-1")
	_, err = r.Record(context.Background(), recent)
	c.Assert(err, qt.IsNil)

	count, err := r.CountRecentForUser(context.Background(), "tenant-1", "user-1", time.Now().Add(-1*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestCommodityScanAuditRegistry_DeleteOlderThan(t *testing.T) {
	c := qt.New(t)
	r := memory.NewCommodityScanAuditRegistry()

	old := newTestAudit("user-1")
	old.CreatedAt = time.Now().Add(-48 * time.Hour)
	_, err := r.Record(context.Background(), old)
	c.Assert(err, qt.IsNil)

	recent := newTestAudit("user-1")
	_, err = r.Record(context.Background(), recent)
	c.Assert(err, qt.IsNil)

	err = r.DeleteOlderThan(context.Background(), time.Now().Add(-24*time.Hour))
	c.Assert(err, qt.IsNil)

	count, err := r.CountRecentForUser(context.Background(), "tenant-1", "user-1", time.Now().Add(-72*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}
