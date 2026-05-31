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
	"github.com/denisvmedia/inventario/services/notifications"
)

// recordingLoanEmailService captures SendLoanReminderEmail invocations
// so tests can assert (a) the number of sends, (b) the kind label, and
// (c) the days-delta the renderer received. Implements every other
// EmailService method as a no-op — the loan service only calls
// SendLoanReminderEmail.
type recordingLoanEmailService struct {
	mu    sync.Mutex
	calls []recordedLoanEmail
}

type recordedLoanEmail struct {
	to            string
	name          string
	commodityName string
	borrowerName  string
	lentAt        string
	dueBackAt     string
	commodityURL  string
	kind          string
	daysDelta     int
}

func (r *recordingLoanEmailService) SendVerificationEmail(_ context.Context, _ string, _ string, _ string) error {
	return nil
}
func (r *recordingLoanEmailService) SendPasswordResetEmail(_ context.Context, _ string, _ string, _ string) error {
	return nil
}
func (r *recordingLoanEmailService) SendPasswordChangedEmail(_ context.Context, _ string, _ string, _ time.Time) error {
	return nil
}
func (r *recordingLoanEmailService) SendWelcomeEmail(_ context.Context, _ string, _ string) error {
	return nil
}
func (r *recordingLoanEmailService) SendWarrantyReminderEmail(_ context.Context, _, _, _, _, _ string, _ int) error {
	return nil
}
func (r *recordingLoanEmailService) SendGroupInviteEmail(_ context.Context, _, _, _, _, _ string, _ time.Time) error {
	return nil
}
func (r *recordingLoanEmailService) SendStorageQuotaWarningEmail(_ context.Context, _, _, _ string, _, _ int, _, _ string, _ []string, _, _ string) error {
	return nil
}
func (r *recordingLoanEmailService) SendFeedbackEmail(_ context.Context, _, _, _, _, _, _, _ string, _ []string) error {
	return nil
}

func (r *recordingLoanEmailService) SendMaintenanceReminderEmail(_ context.Context, _, _, _, _, _, _ string, _ int) error {
	return nil
}

func (r *recordingLoanEmailService) SendLoanReminderEmail(_ context.Context, to, name, commodityName, borrowerName, lentAt, dueBackAt, commodityURL, kind string, daysDelta int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, recordedLoanEmail{
		to:            to,
		name:          name,
		commodityName: commodityName,
		borrowerName:  borrowerName,
		lentAt:        lentAt,
		dueBackAt:     dueBackAt,
		commodityURL:  commodityURL,
		kind:          kind,
		daysDelta:     daysDelta,
	})
	return nil
}

func (r *recordingLoanEmailService) snapshot() []recordedLoanEmail {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedLoanEmail, len(r.calls))
	copy(out, r.calls)
	return out
}

// failingLoanEmailService stand-in returns errors from
// SendLoanReminderEmail. Lets the retry test simulate a queue outage
// without touching the recording mock. Implemented as a standalone
// type (not embedding recordingLoanEmailService) so the per-method
// receivers don't drag the sync.Mutex through a copy.
type failingLoanEmailService struct{}

func (failingLoanEmailService) SendVerificationEmail(_ context.Context, _, _, _ string) error {
	return nil
}
func (failingLoanEmailService) SendPasswordResetEmail(_ context.Context, _, _, _ string) error {
	return nil
}
func (failingLoanEmailService) SendPasswordChangedEmail(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}
func (failingLoanEmailService) SendWelcomeEmail(_ context.Context, _, _ string) error { return nil }
func (failingLoanEmailService) SendWarrantyReminderEmail(_ context.Context, _, _, _, _, _ string, _ int) error {
	return nil
}
func (failingLoanEmailService) SendGroupInviteEmail(_ context.Context, _, _, _, _, _ string, _ time.Time) error {
	return nil
}
func (failingLoanEmailService) SendStorageQuotaWarningEmail(_ context.Context, _, _, _ string, _, _ int, _, _ string, _ []string, _, _ string) error {
	return nil
}
func (failingLoanEmailService) SendLoanReminderEmail(_ context.Context, _, _, _, _, _, _, _, _ string, _ int) error {
	return errors.New("queue down")
}
func (failingLoanEmailService) SendMaintenanceReminderEmail(_ context.Context, _, _, _, _, _, _ string, _ int) error {
	return errors.New("queue down")
}
func (failingLoanEmailService) SendFeedbackEmail(_ context.Context, _, _, _, _, _, _, _ string, _ []string) error {
	return nil
}

// TestLoanReminderService_RemindOnce_TickClock pins the acceptance
// criteria from issue #1509: a loan with `due_back_at = today` gets one
// reminder on the overdue tick (T+1), and a second sweep against the
// same clock is a no-op (the reminder_sent_overdue flag flipped).
func TestLoanReminderService_RemindOnce_TickClock(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, commodityID, factorySet := newLoanReminderServiceFixture(c)

	dueTodayDate := models.Date("2026-05-17")
	dueToday := models.PDate(&dueTodayDate)
	loan, err := regSet.CommodityLoanRegistry.Create(ctx, models.CommodityLoan{
		CommodityID:  commodityID,
		BorrowerName: "Pavel",
		LentAt:       "2026-05-10",
		DueBackAt:    dueToday,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(loan.IsOpen(), qt.IsTrue)

	emailSvc := &recordingLoanEmailService{}
	svc := services.NewLoanReminderService(factorySet, emailSvc, func(slug, id string) string {
		return "https://example.test/g/" + slug + "/commodities/" + id
	})

	// Tick 1: clock at "today" — due_soon kind fires once (due_back_at
	// is today, within the 0..+7 window).
	tick1 := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	stats, err := svc.RemindOnce(ctx, tick1)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(stats.Sent[registry.LoanReminderKindDueSoon], qt.Equals, 1)
	c.Assert(stats.Sent[registry.LoanReminderKindOverdue], qt.Equals, 0)
	c.Assert(emailSvc.snapshot(), qt.HasLen, 1)
	c.Assert(emailSvc.snapshot()[0].kind, qt.Equals, "due_soon")

	// Tick 2: same clock — flag flipped, no second email.
	stats, err = svc.RemindOnce(ctx, tick1)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Total(), qt.Equals, 0)
	c.Assert(emailSvc.snapshot(), qt.HasLen, 1)

	// Tick 3: advance clock by one day. Now due_back_at < today →
	// overdue kind fires once (separate flag).
	tick3 := tick1.AddDate(0, 0, 1)
	stats, err = svc.RemindOnce(ctx, tick3)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent[registry.LoanReminderKindOverdue], qt.Equals, 1)
	c.Assert(stats.Sent[registry.LoanReminderKindDueSoon], qt.Equals, 0)
	calls := emailSvc.snapshot()
	c.Assert(calls, qt.HasLen, 2)
	c.Assert(calls[1].kind, qt.Equals, "overdue")
	c.Assert(calls[1].daysDelta, qt.Equals, 1)

	// Tick 4: re-run at the same overdue clock — no third email
	// (reminder_sent_overdue flipped on tick 3).
	stats, err = svc.RemindOnce(ctx, tick3)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Total(), qt.Equals, 0)
	c.Assert(emailSvc.snapshot(), qt.HasLen, 2)
}

// TestLoanReminderService_OpenEndedNeverReminds pins the issue's
// explicit rule: a loan with `due_back_at IS NULL` is open-ended and
// must never trigger a reminder, regardless of clock.
func TestLoanReminderService_OpenEndedNeverReminds(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, commodityID, factorySet := newLoanReminderServiceFixture(c)

	_, err := regSet.CommodityLoanRegistry.Create(ctx, models.CommodityLoan{
		CommodityID:  commodityID,
		BorrowerName: "Ivan",
		LentAt:       "2025-01-01",
		// DueBackAt nil — open-ended.
	})
	c.Assert(err, qt.IsNil)

	emailSvc := &recordingLoanEmailService{}
	svc := services.NewLoanReminderService(factorySet, emailSvc, nil)

	// Tick far in the future. Without a due_back_at the row is never a
	// reminder candidate; both kinds resolve to 0.
	tick := time.Date(2027, 1, 1, 12, 0, 0, 0, time.UTC)
	stats, err := svc.RemindOnce(ctx, tick)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Total(), qt.Equals, 0)
	c.Assert(emailSvc.snapshot(), qt.HasLen, 0)
}

// TestLoanReminderService_ReturnedNeverReminds pins another explicit
// rule: a loan with `returned_at` set never reminds even if its
// due_back_at was in the past.
func TestLoanReminderService_ReturnedNeverReminds(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, commodityID, factorySet := newLoanReminderServiceFixture(c)

	dueDate := models.Date("2026-05-01")
	due := models.PDate(&dueDate)
	returnedDate := models.Date("2026-05-05")
	returned := models.PDate(&returnedDate)
	_, err := regSet.CommodityLoanRegistry.Create(ctx, models.CommodityLoan{
		CommodityID:  commodityID,
		BorrowerName: "Maria",
		LentAt:       "2026-04-20",
		DueBackAt:    due,
		ReturnedAt:   returned,
	})
	c.Assert(err, qt.IsNil)

	emailSvc := &recordingLoanEmailService{}
	svc := services.NewLoanReminderService(factorySet, emailSvc, nil)

	tick := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	stats, err := svc.RemindOnce(ctx, tick)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Total(), qt.Equals, 0)
	c.Assert(emailSvc.snapshot(), qt.HasLen, 0)
}

// TestLoanReminderService_OptOutSkipsFlagFlip pins the issue's
// explicit decision: when a user has opted out of loan_reminder
// notifications, the worker MUST skip BOTH the send AND the flag flip,
// so re-enabling the toggle resumes reminders naturally.
func TestLoanReminderService_OptOutSkipsFlagFlip(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, commodityID, factorySet := newLoanReminderServiceFixture(c)

	dueDate := models.Date("2026-05-01")
	due := models.PDate(&dueDate)
	loan, err := regSet.CommodityLoanRegistry.Create(ctx, models.CommodityLoan{
		CommodityID:  commodityID,
		BorrowerName: "Pavel",
		LentAt:       "2026-04-15",
		DueBackAt:    due,
	})
	c.Assert(err, qt.IsNil)

	off := false
	c.Assert(regSet.SettingsRegistry.Save(ctx, models.SettingsObject{
		NotificationsLoanReminder: &off,
	}), qt.IsNil)

	emailSvc := &recordingLoanEmailService{}
	svc := services.NewLoanReminderService(factorySet, emailSvc, nil).
		WithPreferences(notifications.NewService(factorySet.SettingsRegistryFactory))

	tick := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	stats, err := svc.RemindOnce(ctx, tick)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Total(), qt.Equals, 0,
		qt.Commentf("opted-out user must not get a reminder counted"))
	c.Assert(stats.Failed, qt.Equals, 0,
		qt.Commentf("opt-out is a successful no-op, not a failure"))
	c.Assert(emailSvc.snapshot(), qt.HasLen, 0)

	// Now re-enable the preference. The flag must still be `false` so
	// the sweep emits the reminder — this is the explicit issue
	// requirement: "don't flip the flag when skipping for preferences".
	on := true
	c.Assert(regSet.SettingsRegistry.Save(ctx, models.SettingsObject{
		NotificationsLoanReminder: &on,
	}), qt.IsNil)

	stats, err = svc.RemindOnce(ctx, tick)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent[registry.LoanReminderKindOverdue], qt.Equals, 1,
		qt.Commentf("re-enabling preference must trigger the deferred reminder on the next sweep"))
	c.Assert(emailSvc.snapshot(), qt.HasLen, 1)

	// Sanity: confirm the flag flipped this time so the next sweep is
	// idempotent.
	reloaded, err := regSet.CommodityLoanRegistry.Get(ctx, loan.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reloaded.ReminderSentOverdue, qt.IsTrue)
}

// TestLoanReminderService_EnqueueFailureLeavesFlagFalse pins the
// retry-after-failure contract: a transient email queue outage must
// NOT flip the flag — the next sweep retries.
func TestLoanReminderService_EnqueueFailureLeavesFlagFalse(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, commodityID, factorySet := newLoanReminderServiceFixture(c)

	dueDate := models.Date("2026-05-01")
	due := models.PDate(&dueDate)
	loan, err := regSet.CommodityLoanRegistry.Create(ctx, models.CommodityLoan{
		CommodityID:  commodityID,
		BorrowerName: "Olga",
		LentAt:       "2026-04-15",
		DueBackAt:    due,
	})
	c.Assert(err, qt.IsNil)

	failing := &failingLoanEmailService{}
	svc := services.NewLoanReminderService(factorySet, failing, nil)

	tick := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	// Tick 1: enqueue fails → flag stays false, Failed bumped.
	stats, err := svc.RemindOnce(ctx, tick)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Total(), qt.Equals, 0)
	c.Assert(stats.Failed, qt.Equals, 1)
	reloaded, err := regSet.CommodityLoanRegistry.Get(ctx, loan.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reloaded.ReminderSentOverdue, qt.IsFalse,
		qt.Commentf("flag must stay false after a transient enqueue failure"))

	// Tick 2 with a working queue: retries and succeeds.
	recording := &recordingLoanEmailService{}
	svc2 := services.NewLoanReminderService(factorySet, recording, nil)
	stats, err = svc2.RemindOnce(ctx, tick)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent[registry.LoanReminderKindOverdue], qt.Equals, 1)
	c.Assert(recording.snapshot(), qt.HasLen, 1)
}

// TestLoanReminderService_NoRecipientFlipsFlag covers the structural
// skip path: a loan whose creator user has no email on file still
// flips the flag (we don't have anyone to send to and we don't want to
// keep re-evaluating the row every tick).
func TestLoanReminderService_NoRecipientFlipsFlag(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, commodityID, factorySet := newLoanReminderServiceFixture(c)

	dueDate := models.Date("2026-05-01")
	due := models.PDate(&dueDate)
	loan, err := regSet.CommodityLoanRegistry.Create(ctx, models.CommodityLoan{
		CommodityID:  commodityID,
		BorrowerName: "Lera",
		LentAt:       "2026-04-15",
		DueBackAt:    due,
	})
	c.Assert(err, qt.IsNil)

	// Strip the creator user's email to simulate a row whose creator
	// has no recipient address. We can't update Email via the
	// user-scoped registry on memory backend cleanly, so we use the
	// service-mode registry directly.
	userReg := factorySet.UserRegistry
	u, err := userReg.Get(ctx, loan.CreatedByUserID)
	c.Assert(err, qt.IsNil)
	u.Email = ""
	_, err = userReg.Update(ctx, *u)
	c.Assert(err, qt.IsNil)

	emailSvc := &recordingLoanEmailService{}
	svc := services.NewLoanReminderService(factorySet, emailSvc, nil)

	tick := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	stats, err := svc.RemindOnce(ctx, tick)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent[registry.LoanReminderKindOverdue], qt.Equals, 0,
		qt.Commentf("no-recipient is a structural skip, not a counted send"))
	c.Assert(emailSvc.snapshot(), qt.HasLen, 0,
		qt.Commentf("no email should be enqueued"))
	// Flag must flip so the worker stops re-evaluating this row.
	reloaded, err := regSet.CommodityLoanRegistry.Get(ctx, loan.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reloaded.ReminderSentOverdue, qt.IsTrue,
		qt.Commentf("no-recipient path still flips the flag (terminal skip)"))
}

// TestLoanReminderService_DueSoonWindow covers the boundary: a loan
// due today gets a reminder, a loan due in `days+1` does not (window
// is inclusive of `today + days` only).
func TestLoanReminderService_DueSoonWindow(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, commodityID, factorySet := newLoanReminderServiceFixture(c)

	// Loan A: due in 7 days — inside the default 7-day window.
	dueIn7Date := models.Date("2026-05-24")
	dueIn7 := models.PDate(&dueIn7Date)
	loanA, err := regSet.CommodityLoanRegistry.Create(ctx, models.CommodityLoan{
		CommodityID:  commodityID,
		BorrowerName: "Anya",
		LentAt:       "2026-05-10",
		DueBackAt:    dueIn7,
	})
	c.Assert(err, qt.IsNil)

	// Loan B: due in 8 days — outside the 7-day window.
	dueIn8Date := models.Date("2026-05-25")
	dueIn8 := models.PDate(&dueIn8Date)
	loanB, err := regSet.CommodityLoanRegistry.Create(ctx, models.CommodityLoan{
		CommodityID:  commodityID,
		BorrowerName: "Boris",
		LentAt:       "2026-05-11",
		DueBackAt:    dueIn8,
	})
	c.Assert(err, qt.IsNil)

	emailSvc := &recordingLoanEmailService{}
	svc := services.NewLoanReminderService(factorySet, emailSvc, nil)

	tick := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	stats, err := svc.RemindOnce(ctx, tick)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent[registry.LoanReminderKindDueSoon], qt.Equals, 1,
		qt.Commentf("loan due in 7 days is inside the default window"))
	calls := emailSvc.snapshot()
	c.Assert(calls, qt.HasLen, 1)

	reloadedA, err := regSet.CommodityLoanRegistry.Get(ctx, loanA.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reloadedA.ReminderSentDueSoon, qt.IsTrue)

	reloadedB, err := regSet.CommodityLoanRegistry.Get(ctx, loanB.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reloadedB.ReminderSentDueSoon, qt.IsFalse,
		qt.Commentf("loan due in 8 days must remain a non-candidate until the clock catches up"))
}

// newLoanReminderServiceFixture wires a memory-backed factory + commodity +
// area + location the loan tests can populate. The returned commodity
// ID is the parent for every loan the test creates.
func newLoanReminderServiceFixture(c *qt.C) (context.Context, *registry.Set, string, *registry.FactorySet) {
	c.Helper()
	factorySet := memory.NewFactorySet()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "loan-svc-user"},
			TenantID: "loan-svc-tenant",
		},
		Email: "lender@example.com",
		Name:  "Loan Owner",
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
	commodity, err := regSet.CommodityRegistry.Create(ctx, models.Commodity{
		AreaID:    new(area.ID),
		Name:      "drill",
		ShortName: "drill",
		Type:      models.CommodityTypeOther,
		Status:    models.CommodityStatusInUse,
		Count:     1,
	})
	c.Assert(err, qt.IsNil)
	return ctx, regSet, commodity.ID, factorySet
}
