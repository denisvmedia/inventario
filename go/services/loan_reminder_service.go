package services

import (
	"context"
	"log/slog"
	"strings"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services/notifications"
)

// defaultLoanReminderDueSoonDays is the inclusive forward-looking window
// the worker scans for the LoanReminderKindDueSoon kind. Configurable
// via the --loan-reminder-due-soon-days CLI flag; 7 matches the issue's
// stated default and the warranty worker's 7-day threshold.
const defaultLoanReminderDueSoonDays = 7

// LoanReminderService runs one loan-reminder sweep at a time across
// every group (service-mode). For each kind (overdue / due-soon) it
// asks the loan registry for the matching set of open loans where the
// corresponding reminder_sent_* flag is still false, then for every
// row:
//
//  1. resolves the borrower's contact and the loan's lender user;
//  2. checks the lender's notifications.loan_reminder × channel.email
//     preference — if disabled, SKIP both the email AND the flag flip
//     (issue #1509 explicit decision: "don't flip the flag when skipping
//     for preferences, so re-enabling resumes naturally");
//  3. enqueues SendLoanReminderEmail through the AsyncEmailService;
//  4. only on a successful enqueue, flips the reminder_sent_* flag
//     atomically (the false-only UPDATE filter wins the row at most
//     once); a failed enqueue leaves the flag false so the next sweep
//     retries.
//  5. on a successful flip, emits a CommodityEvent of kind
//     loan_reminder_sent against the loan's commodity so the per-item
//     timeline carries the audit trail.
//
// The service is stateless — RemindOnce takes `now` as a parameter so
// unit tests can pin the clock without injecting a global ticker.
type LoanReminderService struct {
	factorySet *registry.FactorySet
	emailSvc   EmailService
	// commodityURLBuilder builds the deep-link printed in the reminder
	// email (loan tab on a commodity). Optional — when nil, the email
	// suppresses the link block.
	commodityURLBuilder func(groupSlug, commodityID string) string
	// prefs is the per-user notification preferences service. Optional
	// — when nil, every recipient is treated as opted-in (test path).
	prefs *notifications.Service
	// dueSoonDays is the forward-looking window for LoanReminderKindDueSoon.
	// Zero falls back to defaultLoanReminderDueSoonDays.
	dueSoonDays int
}

// NewLoanReminderService constructs the service. emailSvc may be nil in
// tests that only assert the flag-flip / event-emission side; production
// callers always pass a non-nil EmailService.
func NewLoanReminderService(factorySet *registry.FactorySet, emailSvc EmailService, urlBuilder func(groupSlug, commodityID string) string) *LoanReminderService {
	return &LoanReminderService{
		factorySet:          factorySet,
		emailSvc:            emailSvc,
		commodityURLBuilder: urlBuilder,
		dueSoonDays:         defaultLoanReminderDueSoonDays,
	}
}

// WithPreferences attaches a notifications.Service so the worker gates
// each recipient on their `notifications.loan_reminder` × `channel.email`
// toggle. Returns the same service for fluent chaining at the bootstrap
// site.
func (s *LoanReminderService) WithPreferences(prefs *notifications.Service) *LoanReminderService {
	s.prefs = prefs
	return s
}

// WithDueSoonDays overrides the default 7-day window for the due-soon
// kind. Non-positive values are ignored.
func (s *LoanReminderService) WithDueSoonDays(d int) *LoanReminderService {
	if d > 0 {
		s.dueSoonDays = d
	}
	return s
}

// LoanReminderStats summarises one RemindOnce sweep. Sent partitions
// the count by kind so the worker can emit one Prometheus series per
// kind label. Failed is the cross-kind count of per-loan failures
// (enqueue errors or flag-flip races that retry on the next sweep).
type LoanReminderStats struct {
	Sent   map[registry.LoanReminderKind]int
	Failed int
}

// Total returns the cross-kind count of newly-sent reminders.
func (s LoanReminderStats) Total() int {
	total := 0
	for _, v := range s.Sent {
		total += v
	}
	return total
}

// RemindOnce runs one sweep pinned to `now` for both kinds in sequence.
// A non-nil error is only returned when the initial listing fails for
// either kind — per-loan failures bump stats.Failed and are logged.
func (s *LoanReminderService) RemindOnce(ctx context.Context, now time.Time) (LoanReminderStats, error) {
	stats := LoanReminderStats{Sent: map[registry.LoanReminderKind]int{}}
	if s.factorySet == nil {
		return stats, errxtrace.Wrap("loan reminder service: factorySet is required", registry.ErrFieldRequired)
	}
	sweep := &loanSweep{prefs: s.prefsForSweep(), lenders: map[string]loanLender{}, gate: newReminderGate(s.factorySet)}
	for _, kind := range []registry.LoanReminderKind{
		registry.LoanReminderKindOverdue,
		registry.LoanReminderKindDueSoon,
	} {
		sent, failed, err := s.sweepKind(ctx, kind, now, sweep)
		if err != nil {
			return stats, err
		}
		stats.Sent[kind] = sent
		stats.Failed += failed
	}
	return stats, nil
}

// sweepKind runs the registry list + per-loan processing for one kind.
// Returns (sent, failed, listErr) — listErr fails the whole sweep, but
// a per-loan failure just bumps `failed` and the loop moves on.
func (s *LoanReminderService) sweepKind(ctx context.Context, kind registry.LoanReminderKind, now time.Time, sweep *loanSweep) (sent, failed int, listErr error) {
	if s.factorySet.CommodityLoanRegistryFactory == nil {
		// Defensive: if the factory wiring is incomplete (a test or a
		// stripped-down bootstrap), fail the sweep with a controlled
		// error instead of panicking the worker goroutine.
		return 0, 0, errxtrace.Wrap("loan reminder: missing CommodityLoanRegistryFactory", registry.ErrFieldRequired)
	}
	loanReg := s.factorySet.CommodityLoanRegistryFactory.CreateServiceRegistry()
	// A lender may pick a lead time up to models.LoanReminderMaxLeadDays
	// (#2805), so the due-soon query looks that far ahead and processOne
	// drops the loans that are not yet inside their lender's lead time.
	loans, err := loanReg.ListPendingReminders(ctx, kind, now, max(s.dueSoonDays, models.LoanReminderMaxLeadDays))
	if err != nil {
		return 0, 0, errxtrace.Wrap("loan reminder: list pending", err)
	}
	for _, l := range loans {
		skipped, skipErr := sweep.gate.skips(ctx, l.TenantID, l.GroupID, models.GroupSectionLoans)
		if skipErr != nil {
			failed++
			slog.ErrorContext(ctx, "loan reminder failed",
				"loan_id", l.ID,
				"commodity_id", l.CommodityID,
				"kind", string(kind),
				"error", skipErr,
			)
			continue
		}
		if skipped {
			// The tenant is not active or the group hides loans. The flag
			// stays unset, so the reminder goes out if that changes while
			// the loan is still due.
			continue
		}
		ok, processErr := s.processOne(ctx, loanReg, l, kind, now, sweep)
		if processErr != nil {
			failed++
			slog.ErrorContext(ctx, "loan reminder failed",
				"loan_id", l.ID,
				"commodity_id", l.CommodityID,
				"kind", string(kind),
				"error", processErr,
			)
			continue
		}
		if ok {
			sent++
		}
	}
	return sent, failed, nil
}

// processOne handles one (loan, kind) pair. Order is fixed:
//
//  0. A due-soon loan that is not yet inside the lender's lead time
//     (leadTimeFor) is left alone, flag unset, for a later sweep.
//  1. Resolve the lender + recipient email. If the lender has no email
//     on file or no user record at all, treat the loan as "no recipient"
//     and flip the flag anyway (no point re-evaluating it every tick).
//     A lookup that fails for any other reason returns the error and
//     leaves the flag false.
//     For symmetry with the issue's preferences decision, this case
//     still flips — it's a structural skip, not a user opt-out.
//  2. Honour notifications.loan_reminder × channel.email. If off, skip
//     both the send AND the flag flip (#1509 explicit rule).
//  3. Enqueue the email. On error, leave the flag false so the next
//     sweep retries.
//  4. Flip the flag atomically. (false, nil) — a parallel worker won
//     the row; we count it as not-sent but not failed (the parallel
//     worker emitted the audit event).
//  5. Emit the loan_reminder_sent commodity event on a successful flip.
//
// Returns:
//   - (true,  nil) — email enqueued AND flag flipped by us.
//   - (false, nil) — not inside the lead time yet (flag NOT flipped) OR
//     no recipient (flag still flipped) OR opted out (flag NOT flipped)
//     OR concurrent worker won the flip.
//   - (false, err) — lender lookup, enqueue or flip-side failure that
//     should retry.
func (s *LoanReminderService) processOne(ctx context.Context, loanReg registry.CommodityLoanRegistry, l *models.CommodityLoan, kind registry.LoanReminderKind, now time.Time, sweep *loanSweep) (bool, error) {
	prefsCache := sweep.prefs
	found, err := s.cachedLender(ctx, l, sweep)
	if err != nil {
		// A failed lookup is not "no recipient": flipping the flag now
		// would drop the reminder for good. Leave it for the next sweep.
		return false, errxtrace.Wrap("loan reminder: look up lender", err)
	}
	lender, recipientEmail, recipientName := found.user, found.email, found.name
	if kind == registry.LoanReminderKindDueSoon && computeDaysDelta(l.DueBackAt, now) > s.leadTimeFor(ctx, lender, prefsCache) {
		// Not inside the lender's lead time yet. The flag stays false,
		// so a later sweep sends the reminder once the date is close
		// enough.
		return false, nil
	}
	if recipientEmail == "" {
		// No lender / no email on file. Flip the flag anyway so this row
		// stops matching on every tick — there's no one to email, and a
		// future Update on the loan won't reset the flag (per the issue:
		// "we don't reset the flags on return; they're terminal"). When
		// emailSvc is nil (stub/test mode), this same path is hit and
		// the flag still flips so stats are meaningful.
		flipped, err := loanReg.MarkReminderSent(ctx, l.ID, kind)
		if err != nil {
			return false, errxtrace.Wrap("loan reminder: flip flag without recipient", err)
		}
		if flipped {
			slog.WarnContext(ctx, "loan reminder: no recipient on file; flag flipped",
				"loan_id", l.ID,
				"kind", string(kind),
			)
			if emitErr := s.emitAuditEvent(ctx, l, kind, ""); emitErr != nil {
				// Non-fatal — the flag flip is the source of truth for
				// idempotency. Log and move on.
				slog.WarnContext(ctx, "loan reminder: emit audit event failed (no recipient)",
					"loan_id", l.ID,
					"error", emitErr,
				)
			}
		}
		return false, nil
	}

	// Per-recipient opt-out: skip BOTH the send AND the flag flip so
	// re-enabling preferences resumes naturally (issue #1509 explicit
	// decision). The warranty worker writes its idempotency row even
	// when opted-out; loans deliberately don't, because the loan flag
	// IS the row (no separate idempotency table) — flipping it would
	// silence the recipient permanently for this loan even after they
	// re-enable the category.
	if prefsCache != nil && lender != nil && !prefsCache.IsEnabled(ctx, lender, notifications.CategoryLoanReminder, notifications.ChannelEmail) {
		slog.DebugContext(ctx, "loan reminder: lender opted out; skipping send and flag flip",
			"loan_id", l.ID,
			"kind", string(kind),
			"user_id", lender.ID,
		)
		return false, nil
	}

	// Stub-mode short-circuit: no email service configured (tests / dev
	// without an SMTP target). Flip the flag so the stats counter is
	// meaningful — same idempotency contract as the recipient-resolved
	// path below.
	if s.emailSvc == nil {
		flipped, err := loanReg.MarkReminderSent(ctx, l.ID, kind)
		if err != nil {
			return false, errxtrace.Wrap("loan reminder: flip flag (stub mode)", err)
		}
		if flipped {
			if emitErr := s.emitAuditEvent(ctx, l, kind, recipientEmail); emitErr != nil {
				slog.WarnContext(ctx, "loan reminder: emit audit event failed (stub mode)",
					"loan_id", l.ID,
					"error", emitErr,
				)
			}
		}
		return flipped, nil
	}

	url := s.buildCommodityURL(ctx, l)
	commodityName := s.lookupCommodityName(ctx, l.CommodityID)
	daysDelta := computeDaysDelta(l.DueBackAt, now)
	sendErr := s.emailSvc.SendLoanReminderEmail(
		// Localize to the lender's chosen UI language (#2090).
		withReminderLanguage(ctx, prefsCache, lender),
		recipientEmail,
		recipientName,
		commodityName,
		l.BorrowerName,
		string(l.LentAt),
		string(*l.DueBackAt),
		url,
		string(kind),
		daysDelta,
	)
	if sendErr != nil {
		return false, errxtrace.Wrap("loan reminder: enqueue failed", sendErr)
	}

	flipped, flipErr := loanReg.MarkReminderSent(ctx, l.ID, kind)
	if flipErr != nil {
		return false, errxtrace.Wrap("loan reminder: flip flag after send", flipErr)
	}
	if !flipped {
		// Concurrent worker won the row. We already enqueued an email
		// — accept the harmless duplicate (a duplicate beats silent
		// loss) and don't double-emit the audit event since the
		// winning worker will own that side.
		return false, nil
	}
	if emitErr := s.emitAuditEvent(ctx, l, kind, recipientEmail); emitErr != nil {
		slog.WarnContext(ctx, "loan reminder: emit audit event failed",
			"loan_id", l.ID,
			"kind", string(kind),
			"error", emitErr,
		)
	}
	return true, nil
}

// loanSweep holds what one sweep reuses across loans and both kinds: the
// preferences cache and the lenders already looked up. The due-soon query
// reaches up to models.LoanReminderMaxLeadDays ahead, so most of its loans
// are dropped by the lead-time check, which needs the lender; one lender
// usually has many loans.
type loanSweep struct {
	prefs   *notifications.Cache
	lenders map[string]loanLender
	// gate skips the loans of a tenant that is not active and of groups
	// that hide the loans section.
	gate *reminderGate
}

type loanLender struct {
	user        *models.User
	email, name string
}

// cachedLender returns lookupLender, reusing a lender already found in
// this sweep. Only a found lender is cached; a missing one is looked up
// again for each loan, and a failed lookup returns its error.
func (s *LoanReminderService) cachedLender(ctx context.Context, l *models.CommodityLoan, sweep *loanSweep) (loanLender, error) {
	if l == nil {
		return loanLender{}, nil
	}
	if cached, ok := sweep.lenders[l.CreatedByUserID]; ok {
		return cached, nil
	}
	lender, err := s.lookupLender(ctx, l)
	if err != nil {
		return loanLender{}, err
	}
	if lender.user != nil {
		sweep.lenders[l.CreatedByUserID] = lender
	}
	return lender, nil
}

// lookupLender returns the resolved lender User + its email + display
// name. Empty email return signals "no recipient on file" so the caller
// can short-circuit; a lender whose user is gone is the same. Any other
// lookup error is returned. The lender is the user who created the loan row;
// fan-out to admins is intentional out-of-scope — admins are not the
// canonical "I lent this" actor for a personal-inventory app.
func (s *LoanReminderService) lookupLender(ctx context.Context, l *models.CommodityLoan) (loanLender, error) {
	if l == nil || l.CreatedByUserID == "" {
		return loanLender{}, nil
	}
	if s.factorySet.UserRegistry == nil {
		return loanLender{}, nil
	}
	u, err := lookupReminderUser(ctx, s.factorySet.UserRegistry, l.CreatedByUserID)
	if err != nil || u == nil {
		return loanLender{}, err
	}
	email := strings.TrimSpace(u.Email)
	if email == "" {
		return loanLender{user: u}, nil
	}
	return loanLender{user: u, email: email, name: u.Name}, nil
}

// emitAuditEvent writes a commodity_events row of kind loan_reminder_sent.
// `recipient` is empty when the flag flipped without a send (no-recipient
// or stub path) so the timeline still records the worker decision —
// useful for an operator scanning "why didn't this loan get a reminder?"
func (s *LoanReminderService) emitAuditEvent(ctx context.Context, l *models.CommodityLoan, kind registry.LoanReminderKind, recipient string) error {
	if s.factorySet.CommodityEventRegistryFactory == nil {
		return nil
	}
	payload := models.CommodityEventPayload{
		"kind":    string(kind),
		"loan_id": l.ID,
	}
	if recipient != "" {
		payload["recipient"] = recipient
	}
	event := models.CommodityEvent{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        l.TenantID,
			GroupID:         l.GroupID,
			CreatedByUserID: l.CreatedByUserID,
		},
		CommodityID: l.CommodityID,
		Kind:        models.CommodityEventKindLoanReminderSent,
		OccurredAt:  time.Now().UTC(),
		After:       payload,
	}
	reg := s.factorySet.CommodityEventRegistryFactory.CreateServiceRegistry()
	if _, err := reg.Create(ctx, event); err != nil {
		return errxtrace.Wrap("emit loan_reminder_sent event", err)
	}
	return nil
}

// lookupCommodityName resolves the commodity name for the subject line.
// Returns the empty string when the registry isn't wired or the
// commodity row was hard-deleted between the loan list and the send —
// the renderer falls back to "your item" in that case, so the email
// is still useful.
func (s *LoanReminderService) lookupCommodityName(ctx context.Context, commodityID string) string {
	if s.factorySet.CommodityRegistryFactory == nil || commodityID == "" {
		return ""
	}
	reg := s.factorySet.CommodityRegistryFactory.CreateServiceRegistry()
	c, err := reg.Get(ctx, commodityID)
	if err != nil || c == nil {
		return ""
	}
	return c.Name
}

// buildCommodityURL composes the deep-link printed in the reminder email.
// Falls back to "" if the resolver is unset (no PublicURL configured)
// OR the loan's group can't be resolved.
func (s *LoanReminderService) buildCommodityURL(ctx context.Context, l *models.CommodityLoan) string {
	if s.commodityURLBuilder == nil {
		return ""
	}
	if s.factorySet.LocationGroupRegistry == nil {
		return ""
	}
	group, err := s.factorySet.LocationGroupRegistry.Get(ctx, l.GroupID)
	if err != nil || group == nil {
		return ""
	}
	return s.commodityURLBuilder(group.Slug, l.CommodityID)
}

// prefsForSweep returns a per-sweep preferences cache when the service
// is wired with a notifications.Service, or nil otherwise. Same lifetime
// rationale as WarrantyReminderService: discarded on RemindOnce return
// so the next sweep observes the user's latest toggle flips.
func (s *LoanReminderService) prefsForSweep() *notifications.Cache {
	if s.prefs == nil {
		return nil
	}
	return s.prefs.NewCache()
}

// leadTimeFor returns how many days before the due date the lender wants
// the due-soon reminder: their notifications.loan_lead_time setting, or the
// server-wide --loan-reminder-due-soon-days when it is unset.
func (s *LoanReminderService) leadTimeFor(ctx context.Context, lender *models.User, prefsCache *notifications.Cache) int {
	if days, ok := prefsCache.LoanLeadTime(ctx, lender); ok {
		return days
	}
	return s.dueSoonDays
}

// computeDaysDelta returns the days-until-due (positive) or days-overdue
// (positive integer, semantics determined by the kind label) for the
// email body. The renderer prints this verbatim in "Due in N days" or
// "N days overdue" copy. Returns 0 if the date is missing or unparsable.
func computeDaysDelta(due models.PDate, now time.Time) int {
	if due == nil || string(*due) == "" {
		return 0
	}
	d := due.ToTime()
	if d.IsZero() {
		return 0
	}
	n := now.UTC()
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	diffDays := int(d.Sub(today).Hours() / 24)
	if diffDays < 0 {
		return -diffDays
	}
	return diffDays
}
