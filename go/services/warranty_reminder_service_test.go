package services_test

import (
	"context"
	"errors"
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
func (*recordingEmailService) SendGroupInviteEmail(_ context.Context, _, _, _, _, _ string, _ time.Time) error {
	return nil
}

func (r *recordingEmailService) snapshot() []recordedWarrantyEmail {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedWarrantyEmail, len(r.calls))
	copy(out, r.calls)
	return out
}

// failingEmailService is a recordingEmailService stand-in that
// returns errors from every Send* call. Used by the
// EnqueueFailureRetries regression test to simulate a queue outage.
type failingEmailService struct{}

func (failingEmailService) SendVerificationEmail(_ context.Context, _ string, _ string, _ string) error {
	return errors.New("queue down")
}
func (failingEmailService) SendPasswordResetEmail(_ context.Context, _ string, _ string, _ string) error {
	return errors.New("queue down")
}
func (failingEmailService) SendPasswordChangedEmail(_ context.Context, _ string, _ string, _ time.Time) error {
	return errors.New("queue down")
}
func (failingEmailService) SendWelcomeEmail(_ context.Context, _ string, _ string) error {
	return errors.New("queue down")
}
func (failingEmailService) SendWarrantyReminderEmail(_ context.Context, _ string, _ string, _ string, _ string, _ string, _ int) error {
	return errors.New("queue down")
}
func (failingEmailService) SendGroupInviteEmail(_ context.Context, _, _, _, _, _ string, _ time.Time) error {
	return errors.New("queue down")
}

// TestWarrantyReminderService_RemindOnce_TickClock pins the
// acceptance criteria from issue #1367: a commodity with expiry 65d
// out is "active"; tick the clock to 30d and the reminder fires once;
// re-running the same window must not re-send (idempotency row
// guarantees the second tick is a no-op).
func TestWarrantyReminderService_RemindOnce_TickClock(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, areaID, factorySet := newWarrantyServiceFixture(c)

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
	svc := services.NewWarrantyReminderService(factorySet, emailSvc, func(slug, id string) string {
		return "https://example.test/g/" + slug + "/commodities/" + id
	})

	// Tick 1: 65 days out → no thresholds match, no email.
	stats, err := svc.RemindOnce(ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(stats.Sent(), qt.Equals, 0)
	c.Assert(emailSvc.snapshot(), qt.HasLen, 0)

	// Tick 2: jump the clock so the warranty has 30 days remaining.
	// matchedThresholds returns [60, 30] → two reminders, partitioned
	// 1+1 across the threshold breakdown.
	tick2 := now.AddDate(0, 0, 35)
	stats, err = svc.RemindOnce(ctx, tick2)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(stats.Sent(), qt.Equals, 2)
	c.Assert(stats.SentByThreshold[models.WarrantyReminder60Days], qt.Equals, 1)
	c.Assert(stats.SentByThreshold[models.WarrantyReminder30Days], qt.Equals, 1)
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
	stats, err = svc.RemindOnce(ctx, tick2)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(stats.Sent(), qt.Equals, 0)
	c.Assert(emailSvc.snapshot(), qt.HasLen, 2)
}

// TestWarrantyReminderService_RemindOnce_EnqueueFailureRetries
// regression-guards the ordering fix from copilot review #2: when
// every recipient's email enqueue fails (e.g. queue down), the
// service must NOT write the idempotency row, so the next sweep
// retries. Before the fix the row was committed before the email
// send and a transient outage permanently dropped the reminder.
func TestWarrantyReminderService_RemindOnce_EnqueueFailureRetries(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, areaID, factorySet := newWarrantyServiceFixture(c)

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	d := models.Date(now.AddDate(0, 0, 30).Format("2006-01-02"))
	_, err := regSet.CommodityRegistry.Create(ctx, models.Commodity{
		AreaID:            areaID,
		Name:              "kettle",
		ShortName:         "kettle",
		Type:              models.CommodityTypeOther,
		Status:            models.CommodityStatusInUse,
		Count:             1,
		WarrantyExpiresAt: &d,
	})
	c.Assert(err, qt.IsNil)

	failing := &failingEmailService{}
	svc := services.NewWarrantyReminderService(factorySet, failing, nil)

	// Tick 1: every enqueue fails → no row, failed counter ticks once
	// per matched threshold (60 + 30 → 2).
	stats, err := svc.RemindOnce(ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent(), qt.Equals, 0)
	c.Assert(stats.Failed, qt.Equals, 2)

	// Tick 2 (queue back online): the row was never persisted, so
	// next sweep can still try — and now succeeds.
	recording := &recordingEmailService{}
	svc2 := services.NewWarrantyReminderService(factorySet, recording, nil)
	stats, err = svc2.RemindOnce(ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent(), qt.Equals, 2,
		qt.Commentf("after a queue outage the next sweep must replay both reminders"))
	c.Assert(stats.Failed, qt.Equals, 0)
}

// TestWarrantyReminderService_RemindOnce_SkipsBundle covers the
// defence-in-depth guard added by issue #1554: a commodity with
// Count > 1 must not trigger a warranty reminder even if its
// warranty_expires_at is set (legacy data left alone by the migration).
// Without the guard the user would get a "your bundle is expiring"
// email they can't act on without splitting the row first.
func TestWarrantyReminderService_RemindOnce_SkipsBundle(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, areaID, factorySet := newWarrantyServiceFixture(c)

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	d := models.Date(now.AddDate(0, 0, 30).Format("2006-01-02"))
	_, err := regSet.CommodityRegistry.Create(ctx, models.Commodity{
		AreaID:            areaID,
		Name:              "12 light bulbs",
		ShortName:         "bulbs",
		Type:              models.CommodityTypeOther,
		Status:            models.CommodityStatusInUse,
		Count:             12,
		WarrantyExpiresAt: &d,
	})
	c.Assert(err, qt.IsNil)

	emailSvc := &recordingEmailService{}
	svc := services.NewWarrantyReminderService(factorySet, emailSvc, nil)
	stats, err := svc.RemindOnce(ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent(), qt.Equals, 0,
		qt.Commentf("Count>1 commodities must be skipped even with a warranty date set"))
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(emailSvc.snapshot(), qt.HasLen, 0)
}

// TestWarrantyReminderService_RemindOnce_NoExpiryDate confirms a
// commodity without warranty_expires_at is silently skipped (no
// thresholds match, no email).
func TestWarrantyReminderService_RemindOnce_NoExpiryDate(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, areaID, factorySet := newWarrantyServiceFixture(c)

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
	svc := services.NewWarrantyReminderService(factorySet, emailSvc, nil)
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	stats, err := svc.RemindOnce(ctx, now)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent(), qt.Equals, 0)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(emailSvc.snapshot(), qt.HasLen, 0)
}

// newWarrantyServiceFixture wires a memory-backed factory + user/area
// pair the warranty tests can populate with commodities. Returns the
// factory set directly so the caller threads it into
// NewWarrantyReminderService — no shared globals, safe to parallelise.
func newWarrantyServiceFixture(c *qt.C) (context.Context, *registry.Set, string, *registry.FactorySet) {
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
	return ctx, regSet, area.ID, factorySet
}
