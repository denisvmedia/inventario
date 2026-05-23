package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/aivision"
	"github.com/denisvmedia/inventario/internal/aivision/mock"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

func newScanInput(photos ...services.ScanPhotoInput) services.ScanInput {
	return services.ScanInput{Photos: photos}
}

func jpegPhoto(name string, bytes int) services.ScanPhotoInput {
	data := make([]byte, bytes)
	for i := range data {
		data[i] = byte(i % 256)
	}
	return services.ScanPhotoInput{
		Filename:    name,
		ContentType: "image/jpeg",
		Data:        data,
	}
}

func TestCommodityScanService_HappyPath_AuditWritten(t *testing.T) {
	c := qt.New(t)

	audit := memory.NewCommodityScanAuditRegistry()
	provider := mock.New()
	svc := services.NewCommodityScanService(provider, audit, services.CommodityScanConfig{
		MaxPhotos:        5,
		MaxPhotoBytes:    1024,
		RateLimitPerHour: 100,
	})

	result, err := svc.Scan(context.Background(), "tenant-1", "user-1", newScanInput(jpegPhoto("a.jpg", 128)))
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)

	count, err := audit.CountRecentForUser(context.Background(), "user-1", time.Now().Add(-1*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestCommodityScanService_ProviderDisabled(t *testing.T) {
	c := qt.New(t)

	audit := memory.NewCommodityScanAuditRegistry()
	svc := services.NewCommodityScanService(nil, audit, services.CommodityScanConfig{})

	_, err := svc.Scan(context.Background(), "tenant-1", "user-1", newScanInput(jpegPhoto("a.jpg", 64)))
	c.Assert(errors.Is(err, services.ErrScanProviderDisabled), qt.IsTrue)

	// Audit row still recorded so the rate limiter and dashboards see
	// the attempt even though no upstream call went out.
	count, err := audit.CountRecentForUser(context.Background(), "user-1", time.Now().Add(-1*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestCommodityScanService_NoPhotos(t *testing.T) {
	c := qt.New(t)
	svc := services.NewCommodityScanService(mock.New(), memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{})

	_, err := svc.Scan(context.Background(), "tenant-1", "user-1", services.ScanInput{})
	c.Assert(errors.Is(err, services.ErrScanNoPhotos), qt.IsTrue)
}

func TestCommodityScanService_TooManyPhotos(t *testing.T) {
	c := qt.New(t)
	svc := services.NewCommodityScanService(mock.New(), memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{
		MaxPhotos: 2,
	})

	in := newScanInput(jpegPhoto("a.jpg", 64), jpegPhoto("b.jpg", 64), jpegPhoto("c.jpg", 64))
	_, err := svc.Scan(context.Background(), "tenant-1", "user-1", in)
	c.Assert(errors.Is(err, services.ErrScanTooManyPhotos), qt.IsTrue)
}

func TestCommodityScanService_PhotoTooLarge(t *testing.T) {
	c := qt.New(t)
	svc := services.NewCommodityScanService(mock.New(), memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{
		MaxPhotos:     5,
		MaxPhotoBytes: 128,
	})

	_, err := svc.Scan(context.Background(), "tenant-1", "user-1", newScanInput(jpegPhoto("a.jpg", 1024)))
	c.Assert(errors.Is(err, services.ErrScanPhotoTooLarge), qt.IsTrue)
}

func TestCommodityScanService_UnsupportedMIME(t *testing.T) {
	c := qt.New(t)
	svc := services.NewCommodityScanService(mock.New(), memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{
		MaxPhotos: 5,
	})

	in := services.ScanInput{
		Photos: []services.ScanPhotoInput{
			{ContentType: "application/octet-stream", Data: []byte("x")},
		},
	}
	_, err := svc.Scan(context.Background(), "tenant-1", "user-1", in)
	c.Assert(errors.Is(err, services.ErrScanUnsupportedMIME), qt.IsTrue)
}

func TestCommodityScanService_RateLimited(t *testing.T) {
	c := qt.New(t)

	audit := memory.NewCommodityScanAuditRegistry()
	svc := services.NewCommodityScanService(mock.New(), audit, services.CommodityScanConfig{
		MaxPhotos:        5,
		RateLimitPerHour: 1,
	})

	// First call succeeds, increments the counter.
	_, err := svc.Scan(context.Background(), "tenant-1", "user-1", newScanInput(jpegPhoto("a.jpg", 64)))
	c.Assert(err, qt.IsNil)

	// Second call hits the cap.
	_, err = svc.Scan(context.Background(), "tenant-1", "user-1", newScanInput(jpegPhoto("b.jpg", 64)))
	c.Assert(errors.Is(err, services.ErrScanRateLimited), qt.IsTrue)

	// Both attempts are recorded.
	count, err := audit.CountRecentForUser(context.Background(), "user-1", time.Now().Add(-1*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}

func TestCommodityScanService_ProviderTimeout(t *testing.T) {
	c := qt.New(t)
	provider := mock.New(mock.WithDefaultError(aivision.ErrProviderTimeout))
	svc := services.NewCommodityScanService(provider, memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{
		MaxPhotos:     5,
		MaxPhotoBytes: 1024,
	})

	_, err := svc.Scan(context.Background(), "tenant-1", "user-1", newScanInput(jpegPhoto("a.jpg", 64)))
	c.Assert(errors.Is(err, services.ErrScanProviderTimeout), qt.IsTrue)
}

func TestCommodityScanService_ProviderUnavailable(t *testing.T) {
	c := qt.New(t)
	provider := mock.New(mock.WithDefaultError(aivision.ErrProviderUnavailable))
	svc := services.NewCommodityScanService(provider, memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{
		MaxPhotos:     5,
		MaxPhotoBytes: 1024,
	})

	_, err := svc.Scan(context.Background(), "tenant-1", "user-1", newScanInput(jpegPhoto("a.jpg", 64)))
	c.Assert(errors.Is(err, services.ErrScanProviderUnavailable), qt.IsTrue)
}

func TestCommodityScanService_ProviderError(t *testing.T) {
	c := qt.New(t)
	provider := mock.New(mock.WithDefaultError(errors.New("unknown")))
	svc := services.NewCommodityScanService(provider, memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{
		MaxPhotos:     5,
		MaxPhotoBytes: 1024,
	})

	_, err := svc.Scan(context.Background(), "tenant-1", "user-1", newScanInput(jpegPhoto("a.jpg", 64)))
	c.Assert(errors.Is(err, services.ErrScanProviderError), qt.IsTrue)
}

func TestCommodityScanService_MissingIdentity(t *testing.T) {
	c := qt.New(t)
	svc := services.NewCommodityScanService(mock.New(), memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{})

	_, err := svc.Scan(context.Background(), "", "user-1", newScanInput(jpegPhoto("a.jpg", 64)))
	c.Assert(errors.Is(err, services.ErrScanProviderError), qt.IsTrue)

	_, err = svc.Scan(context.Background(), "tenant-1", "", newScanInput(jpegPhoto("a.jpg", 64)))
	c.Assert(errors.Is(err, services.ErrScanProviderError), qt.IsTrue)
}
