package services_test

import (
	"context"
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// stubFileRegistry implements the bits of registry.FileRegistry that
// StorageUsageService actually exercises. Every other method is left
// as a panicking shim — calling one means the service grew a new
// dependency that the tests need to reflect.
type stubFileRegistry struct {
	breakdown registry.StorageBreakdown
	err       error
}

func (s *stubFileRegistry) SumSizeBreakdown(_ context.Context) (registry.StorageBreakdown, error) {
	return s.breakdown, s.err
}

// --- unused interface methods, panicking on purpose ---

func (s *stubFileRegistry) Create(context.Context, models.FileEntity) (*models.FileEntity, error) {
	panic("not implemented")
}
func (s *stubFileRegistry) Get(context.Context, string) (*models.FileEntity, error) {
	panic("not implemented")
}
func (s *stubFileRegistry) List(context.Context) ([]*models.FileEntity, error) {
	panic("not implemented")
}
func (s *stubFileRegistry) Update(context.Context, models.FileEntity) (*models.FileEntity, error) {
	panic("not implemented")
}
func (s *stubFileRegistry) Delete(context.Context, string) error {
	panic("not implemented")
}
func (s *stubFileRegistry) Count(context.Context) (int, error) {
	panic("not implemented")
}
func (s *stubFileRegistry) ListByType(context.Context, models.FileType) ([]*models.FileEntity, error) {
	panic("not implemented")
}
func (s *stubFileRegistry) ListByLinkedEntity(context.Context, string, string) ([]*models.FileEntity, error) {
	panic("not implemented")
}
func (s *stubFileRegistry) ListByLinkedEntityAndMeta(context.Context, string, string, string) ([]*models.FileEntity, error) {
	panic("not implemented")
}
func (s *stubFileRegistry) ListByGroup(context.Context, string, string) ([]*models.FileEntity, error) {
	panic("not implemented")
}
func (s *stubFileRegistry) Search(context.Context, string, *models.FileType, *models.FileCategory, []string, *string, *string) ([]*models.FileEntity, error) {
	panic("not implemented")
}
func (s *stubFileRegistry) ListPaginated(context.Context, int, int, *models.FileType, *models.FileCategory, *string, *string) ([]*models.FileEntity, int, error) {
	panic("not implemented")
}
func (s *stubFileRegistry) CountByCategory(context.Context, string, *models.FileType, []string) (map[models.FileCategory]int, error) {
	panic("not implemented")
}

var _ registry.FileRegistry = (*stubFileRegistry)(nil)

func TestStorageUsageService_TotalsAndQuota(t *testing.T) {
	c := qt.New(t)

	stub := &stubFileRegistry{
		breakdown: registry.StorageBreakdown{
			Photos:    1024 * 1024,
			Invoices:  2 * 1024 * 1024,
			Documents: 3 * 1024 * 1024,
			Other:     0,
			Exports:   4 * 1024 * 1024,
		},
	}

	svc := services.NewStorageUsageService(stub)
	usage, err := svc.GetUsage(context.Background())
	c.Assert(err, qt.IsNil)
	// 10 MiB across the buckets.
	c.Assert(usage.UsedBytes, qt.Equals, int64(10*1024*1024))
	c.Assert(usage.QuotaBytes, qt.IsNotNil)
	c.Assert(*usage.QuotaBytes, qt.Equals, services.DefaultGroupStorageQuotaBytes)
	c.Assert(usage.Breakdown.Photos, qt.Equals, int64(1024*1024))
	c.Assert(usage.Breakdown.Exports, qt.Equals, int64(4*1024*1024))
}

func TestStorageUsageService_EmptyGroup(t *testing.T) {
	c := qt.New(t)

	svc := services.NewStorageUsageService(&stubFileRegistry{})
	usage, err := svc.GetUsage(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(usage.UsedBytes, qt.Equals, int64(0))
	c.Assert(usage.QuotaBytes, qt.IsNotNil)
	c.Assert(*usage.QuotaBytes, qt.Equals, services.DefaultGroupStorageQuotaBytes)
}

func TestStorageUsageService_RegistryError(t *testing.T) {
	c := qt.New(t)

	want := errors.New("boom")
	svc := services.NewStorageUsageService(&stubFileRegistry{err: want})
	_, err := svc.GetUsage(context.Background())
	c.Assert(err, qt.IsNotNil)
	c.Assert(errors.Is(err, want), qt.IsTrue,
		qt.Commentf("expected wrap of %v, got %v", want, err))
}
