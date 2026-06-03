package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/aivision"
	"github.com/denisvmedia/inventario/internal/aivision/mock"
	"github.com/denisvmedia/inventario/models"
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

func pdfDoc(name string, bytes int) services.ScanPhotoInput {
	data := make([]byte, bytes)
	copy(data, []byte("%PDF-1.7\n"))
	return services.ScanPhotoInput{
		Filename:    name,
		ContentType: aivision.PDFMediaType,
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

	count, err := audit.CountRecentForUser(context.Background(), "tenant-1", "user-1", time.Now().Add(-1*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestCommodityScanService_ProviderDisabled(t *testing.T) {
	c := qt.New(t)

	audit := memory.NewCommodityScanAuditRegistry()
	svc := services.NewCommodityScanService(nil, audit, services.CommodityScanConfig{})

	_, err := svc.Scan(context.Background(), "tenant-1", "user-1", newScanInput(jpegPhoto("a.jpg", 64)))
	c.Assert(err, qt.ErrorIs, services.ErrScanProviderDisabled)

	// Audit row is written (provider="none", status="disabled") so
	// dashboards see the attempt, but CountRecentForUser excludes
	// disabled rows from the rate-limit count — a deployment with
	// the feature off shouldn't accrue a per-user budget the user
	// never spent.
	count, err := audit.CountRecentForUser(context.Background(), "tenant-1", "user-1", time.Now().Add(-1*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestCommodityScanService_NoPhotos(t *testing.T) {
	c := qt.New(t)
	svc := services.NewCommodityScanService(mock.New(), memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{})

	_, err := svc.Scan(context.Background(), "tenant-1", "user-1", services.ScanInput{})
	c.Assert(err, qt.ErrorIs, services.ErrScanNoPhotos)
}

func TestCommodityScanService_TooManyPhotos(t *testing.T) {
	c := qt.New(t)
	svc := services.NewCommodityScanService(mock.New(), memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{
		MaxPhotos: 2,
	})

	in := newScanInput(jpegPhoto("a.jpg", 64), jpegPhoto("b.jpg", 64), jpegPhoto("c.jpg", 64))
	_, err := svc.Scan(context.Background(), "tenant-1", "user-1", in)
	c.Assert(err, qt.ErrorIs, services.ErrScanTooManyPhotos)
}

func TestCommodityScanService_PhotoTooLarge(t *testing.T) {
	c := qt.New(t)
	svc := services.NewCommodityScanService(mock.New(), memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{
		MaxPhotos:     5,
		MaxPhotoBytes: 128,
	})

	_, err := svc.Scan(context.Background(), "tenant-1", "user-1", newScanInput(jpegPhoto("a.jpg", 1024)))
	c.Assert(err, qt.ErrorIs, services.ErrScanPhotoTooLarge)
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
	c.Assert(err, qt.ErrorIs, services.ErrScanUnsupportedMIME)
}

func TestCommodityScanService_AcceptsPDF(t *testing.T) {
	c := qt.New(t)

	audit := memory.NewCommodityScanAuditRegistry()
	svc := services.NewCommodityScanService(mock.New(), audit, services.CommodityScanConfig{
		MaxPhotos:     5,
		MaxPhotoBytes: 1024,
	})

	// A PDF receipt/invoice is a valid scan source (#1983 Part B): it must
	// pass validation, reach the provider, and write the usual audit row —
	// not be rejected as an unsupported MIME type.
	result, err := svc.Scan(context.Background(), "tenant-1", "user-1", newScanInput(pdfDoc("receipt.pdf", 256)))
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)

	count, err := audit.CountRecentForUser(context.Background(), "tenant-1", "user-1", time.Now().Add(-1*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
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
	c.Assert(err, qt.ErrorIs, services.ErrScanRateLimited)

	// CountRecentForUser counts provider attempts only — the first
	// call (status=ok) counts, the second (status=rate_limited) does
	// not. This is the load-bearing semantic: a rate-limited row
	// counting itself would self-perpetuate the lockout forever.
	count, err := audit.CountRecentForUser(context.Background(), "tenant-1", "user-1", time.Now().Add(-1*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestCommodityScanService_ProviderTimeout(t *testing.T) {
	c := qt.New(t)
	provider := mock.New(mock.WithDefaultError(aivision.ErrProviderTimeout))
	svc := services.NewCommodityScanService(provider, memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{
		MaxPhotos:     5,
		MaxPhotoBytes: 1024,
	})

	_, err := svc.Scan(context.Background(), "tenant-1", "user-1", newScanInput(jpegPhoto("a.jpg", 64)))
	c.Assert(err, qt.ErrorIs, services.ErrScanProviderTimeout)
}

func TestCommodityScanService_ProviderUnavailable(t *testing.T) {
	c := qt.New(t)
	provider := mock.New(mock.WithDefaultError(aivision.ErrProviderUnavailable))
	svc := services.NewCommodityScanService(provider, memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{
		MaxPhotos:     5,
		MaxPhotoBytes: 1024,
	})

	_, err := svc.Scan(context.Background(), "tenant-1", "user-1", newScanInput(jpegPhoto("a.jpg", 64)))
	c.Assert(err, qt.ErrorIs, services.ErrScanProviderUnavailable)
}

func TestCommodityScanService_ProviderError(t *testing.T) {
	c := qt.New(t)
	provider := mock.New(mock.WithDefaultError(errors.New("unknown")))
	svc := services.NewCommodityScanService(provider, memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{
		MaxPhotos:     5,
		MaxPhotoBytes: 1024,
	})

	_, err := svc.Scan(context.Background(), "tenant-1", "user-1", newScanInput(jpegPhoto("a.jpg", 64)))
	c.Assert(err, qt.ErrorIs, services.ErrScanProviderError)
}

func TestCommodityScanService_MissingIdentity(t *testing.T) {
	c := qt.New(t)
	svc := services.NewCommodityScanService(mock.New(), memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{})

	// Identity-missing is a deployment-wiring bug, not a client error;
	// the service surfaces a dedicated sentinel the handler maps to 500
	// (the previous ErrScanProviderError mapping leaked a misleading
	// 502 "bad gateway" upstream).
	_, err := svc.Scan(context.Background(), "", "user-1", newScanInput(jpegPhoto("a.jpg", 64)))
	c.Assert(err, qt.ErrorIs, services.ErrScanIdentityMissing)

	_, err = svc.Scan(context.Background(), "tenant-1", "", newScanInput(jpegPhoto("a.jpg", 64)))
	c.Assert(err, qt.ErrorIs, services.ErrScanIdentityMissing)
}

// recordingAuditRegistry wraps the memory audit registry and counts
// Record calls so the anonymous-path tests can prove ScanAnonymous never
// persists a row.
type recordingAuditRegistry struct {
	*memory.CommodityScanAuditRegistry
	records int
}

func newRecordingAuditRegistry() *recordingAuditRegistry {
	return &recordingAuditRegistry{CommodityScanAuditRegistry: memory.NewCommodityScanAuditRegistry()}
}

func (r *recordingAuditRegistry) Record(ctx context.Context, audit models.CommodityScanAudit) (*models.CommodityScanAudit, error) {
	r.records++
	return r.CommodityScanAuditRegistry.Record(ctx, audit)
}

func TestCommodityScanService_ScanAnonymous_HappyPath_NoAudit(t *testing.T) {
	c := qt.New(t)

	audit := newRecordingAuditRegistry()
	svc := services.NewCommodityScanService(mock.New(), audit, services.CommodityScanConfig{
		MaxPhotos:        5,
		MaxPhotoBytes:    1024,
		RateLimitPerHour: 1, // present but irrelevant — anonymous skips the per-user limiter
	})

	result, err := svc.ScanAnonymous(context.Background(), newScanInput(jpegPhoto("a.jpg", 128)))
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)

	// The anonymous path must persist NOTHING: no audit row, regardless
	// of how many times it is invoked (the per-user rate limit doesn't
	// apply either, so a second call still succeeds).
	c.Assert(audit.records, qt.Equals, 0)

	_, err = svc.ScanAnonymous(context.Background(), newScanInput(jpegPhoto("b.jpg", 128)))
	c.Assert(err, qt.IsNil)
	c.Assert(audit.records, qt.Equals, 0)
}

func TestCommodityScanService_ScanAnonymous_ProviderDisabled(t *testing.T) {
	c := qt.New(t)

	audit := newRecordingAuditRegistry()
	svc := services.NewCommodityScanService(nil, audit, services.CommodityScanConfig{})

	_, err := svc.ScanAnonymous(context.Background(), newScanInput(jpegPhoto("a.jpg", 64)))
	c.Assert(err, qt.ErrorIs, services.ErrScanProviderDisabled)
	c.Assert(audit.records, qt.Equals, 0)
}

func TestCommodityScanService_ScanAnonymous_NoPhotos(t *testing.T) {
	c := qt.New(t)
	svc := services.NewCommodityScanService(mock.New(), memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{})

	_, err := svc.ScanAnonymous(context.Background(), services.ScanInput{})
	c.Assert(err, qt.ErrorIs, services.ErrScanNoPhotos)
}

func TestCommodityScanService_ScanAnonymous_TooManyPhotos(t *testing.T) {
	c := qt.New(t)
	svc := services.NewCommodityScanService(mock.New(), memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{
		MaxPhotos: 1,
	})

	_, err := svc.ScanAnonymous(context.Background(), newScanInput(jpegPhoto("a.jpg", 64), jpegPhoto("b.jpg", 64)))
	c.Assert(err, qt.ErrorIs, services.ErrScanTooManyPhotos)
}

func TestCommodityScanService_ScanAnonymous_PhotoTooLarge(t *testing.T) {
	c := qt.New(t)
	svc := services.NewCommodityScanService(mock.New(), memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{
		MaxPhotos:     5,
		MaxPhotoBytes: 128,
	})

	_, err := svc.ScanAnonymous(context.Background(), newScanInput(jpegPhoto("a.jpg", 1024)))
	c.Assert(err, qt.ErrorIs, services.ErrScanPhotoTooLarge)
}

func TestCommodityScanService_ScanAnonymous_UnsupportedMIME(t *testing.T) {
	c := qt.New(t)
	svc := services.NewCommodityScanService(mock.New(), memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{
		MaxPhotos: 5,
	})

	in := services.ScanInput{
		Photos: []services.ScanPhotoInput{
			{ContentType: "application/octet-stream", Data: []byte("x")},
		},
	}
	_, err := svc.ScanAnonymous(context.Background(), in)
	c.Assert(err, qt.ErrorIs, services.ErrScanUnsupportedMIME)
}

func TestCommodityScanService_ScanAnonymous_ProviderTimeout(t *testing.T) {
	c := qt.New(t)
	provider := mock.New(mock.WithDefaultError(aivision.ErrProviderTimeout))
	svc := services.NewCommodityScanService(provider, memory.NewCommodityScanAuditRegistry(), services.CommodityScanConfig{
		MaxPhotos:     5,
		MaxPhotoBytes: 1024,
	})

	_, err := svc.ScanAnonymous(context.Background(), newScanInput(jpegPhoto("a.jpg", 64)))
	c.Assert(err, qt.ErrorIs, services.ErrScanProviderTimeout)
}
