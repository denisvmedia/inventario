package services_test

import (
	"context"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// recordingEmailService is a minimal services.EmailService that captures
// SendWarrantyReminderEmail invocations so tests can assert (a) the
// number of sends and (b) the specific (commodity, threshold) pairs
// each invocation matched. The other Send* methods are no-ops — the
// warranty service only ever calls SendWarrantyReminderEmail.
type recordingEmailService struct {
	mu    sync.Mutex
	calls []recordedWarrantyEmail
}

type recordedWarrantyEmail struct {
	to            string
	name          string
	commodityName string
	expiryDate    string
	commodityURL  string
	thresholdDays int
}

func (r *recordingEmailService) SendVerificationEmail(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (r *recordingEmailService) SendPasswordResetEmail(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (r *recordingEmailService) SendPasswordChangedEmail(_ context.Context, _ string, _ string, _ time.Time) error {
	return nil
}

func (r *recordingEmailService) SendWelcomeEmail(_ context.Context, _ string, _ string) error {
	return nil
}

func (r *recordingEmailService) SendWarrantyReminderEmail(_ context.Context, to, name, commodityName, expiryDate, commodityURL string, thresholdDays int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, recordedWarrantyEmail{
		to:            to,
		name:          name,
		commodityName: commodityName,
		expiryDate:    expiryDate,
		commodityURL:  commodityURL,
		thresholdDays: thresholdDays,
	})
	return nil
}

func (r *recordingEmailService) snapshot() []recordedWarrantyEmail {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedWarrantyEmail, len(r.calls))
	copy(out, r.calls)
	return out
}

// TestWarrantyReminderService_RemindOnce_TickClock pins the
// acceptance criteria from issue #1367: a commodity with expiry 65d
// out is "active"; tick the clock to 30d and the reminder fires once;
// re-running the same window must not re-send (idempotency row
// guarantees the second tick is a no-op).
func TestWarrantyReminderService_RemindOnce_TickClock(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, areaID := newWarrantyServiceFixture(c)

	expiresInDays := func(now time.Time, days int) string {
		return now.AddDate(0, 0, days).Format("2006-01-02")
	}

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)

	// Commodity expiring 65d out at "now". Status = active, no
	// reminder fires.
	d := models.Date(expiresInDays(now, 65))
	out, err := regSet.CommodityRegistry.Create(ctx, models.Commodity{
		AreaID:            areaID,
		Name:              "fridge",
		ShortName:         "fridge",
		Type:              models.CommodityTypeWhiteGoods,
		Status:            models.CommodityStatusInUse,
		Count:             1,
		WarrantyExpiresAt: &d,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(out.Status, qt.Equals, models.CommodityStatusInUse)

	emailSvc := &recordingEmailService{}
	factorySet := getFactorySetFromCtx(c)
	svc := services.NewWarrantyReminderService(factorySet, emailSvc, func(slug, id string) string {
		return "https://example.test/g/" + slug + "/commodities/" + id
	})

	// Tick 1: 65 days out → no thresholds match, no email.
	sent, failed, err := svc.RemindOnce(ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(failed, qt.Equals, 0)
	c.Assert(sent, qt.Equals, 0)
	c.Assert(emailSvc.snapshot(), qt.HasLen, 0)

	// Tick 2: jump the clock so the warranty has 30 days remaining.
	// matchedThresholds returns [60, 30] → two reminders.
	tick2 := now.AddDate(0, 0, 35)
	sent, failed, err = svc.RemindOnce(ctx, tick2)
	c.Assert(err, qt.IsNil)
	c.Assert(failed, qt.Equals, 0)
	c.Assert(sent, qt.Equals, 2)
	calls := emailSvc.snapshot()
	c.Assert(calls, qt.HasLen, 2)
	thresholdsSeen := []int{calls[0].thresholdDays, calls[1].thresholdDays}
	c.Assert(thresholdsSeen, qt.ContentEquals, []int{60, 30})
	for _, k := range calls {
		c.Assert(k.commodityName, qt.Equals, "fridge")
	}

	// Tick 3: re-run at the same clock — idempotency row blocks the
	// second emission. Counter stays at 0; email service stays at the
	// same 2 calls.
	sent, failed, err = svc.RemindOnce(ctx, tick2)
	c.Assert(err, qt.IsNil)
	c.Assert(failed, qt.Equals, 0)
	c.Assert(sent, qt.Equals, 0)
	c.Assert(emailSvc.snapshot(), qt.HasLen, 2)
}

// TestWarrantyReminderService_RemindOnce_NoExpiryDate confirms a
// commodity without warranty_expires_at is silently skipped (no
// thresholds match, no email).
func TestWarrantyReminderService_RemindOnce_NoExpiryDate(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, areaID := newWarrantyServiceFixture(c)

	_, err := regSet.CommodityRegistry.Create(ctx, models.Commodity{
		AreaID:    areaID,
		Name:      "no-warranty",
		ShortName: "no-warranty",
		Type:      models.CommodityTypeOther,
		Status:    models.CommodityStatusInUse,
		Count:     1,
	})
	c.Assert(err, qt.IsNil)

	emailSvc := &recordingEmailService{}
	factorySet := getFactorySetFromCtx(c)
	svc := services.NewWarrantyReminderService(factorySet, emailSvc, nil)
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	sent, failed, err := svc.RemindOnce(ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(sent, qt.Equals, 0)
	c.Assert(failed, qt.Equals, 0)
	c.Assert(emailSvc.snapshot(), qt.HasLen, 0)
}

// newWarrantyServiceFixture wires a memory-backed factory + user/area
// pair the warranty tests can populate with commodities. Stores the
// factory set on the qt.C value so getFactorySetFromCtx can reach back
// to it without thread-locals.
func newWarrantyServiceFixture(c *qt.C) (context.Context, *registry.Set, string) {
	c.Helper()
	factorySet := memory.NewFactorySet()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "warr-svc-user"},
			TenantID: "warr-svc-tenant",
		},
		Email: "owner@example.com",
		Name:  "Warranty Owner",
	}
	userReg := factorySet.CreateServiceRegistrySet().UserRegistry
	u, err := userReg.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)
	ctx := appctx.WithUser(context.Background(), u)
	regSet := must.Must(factorySet.CreateUserRegistrySet(ctx))
	loc, err := regSet.LocationRegistry.Create(ctx, models.Location{Name: "L"})
	c.Assert(err, qt.IsNil)
	area, err := regSet.AreaRegistry.Create(ctx, models.Area{Name: "A", LocationID: loc.ID})
	c.Assert(err, qt.IsNil)
	currentFactorySet = factorySet
	return ctx, regSet, area.ID
}

// currentFactorySet is the package-private fixture handle the tests
// share. It works because the table-driven test layout never runs two
// fixtures in the same Goroutine.
var currentFactorySet *registry.FactorySet

// getFactorySetFromCtx returns the factory set most recently
// constructed by newWarrantyServiceFixture. The indirection exists
// because the fixture has no place to thread the factory set through
// the registry.Set return type.
func getFactorySetFromCtx(c *qt.C) *registry.FactorySet {
	c.Helper()
	c.Assert(currentFactorySet, qt.Not(qt.IsNil))
	return currentFactorySet
}
