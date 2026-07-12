// OrphanFileGCWorker mirrors the OperationSlotCleanupWorker /
// EmailVerificationCleanupWorker lifecycle by design — same
// Start/Stop/runCleanup/sweepOnce shape and the same soft-pause skip
// (#1308). It is deliberately NOT folded into a shared generic base: this
// is the only DESTRUCTIVE periodic worker in the tree, and keeping its
// lifecycle spelled out in full is what lets a reviewer read the delete
// path top-to-bottom without chasing a base class.
package services

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"gocloud.dev/blob"
	"gocloud.dev/gcerrors"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/blobkeys"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

const (
	// defaultOrphanFileGCInterval is how often the sweep runs. The row scan
	// is a single indexed anti-join and the blob scan is one bucket LIST per
	// tenant, so a daily cadence is plenty: orphans do not decay, and waiting
	// costs nothing while being wrong is irreversible.
	defaultOrphanFileGCInterval = 24 * time.Hour

	// defaultOrphanFileGCMinAge is how old a row/blob must be before it can
	// even be considered. It is measured against every bounded in-flight
	// window in the system with four-to-five orders of magnitude to spare:
	// an upload's blob-before-row window is the HTTP handler (seconds), the
	// thumbnail worker's Get→write window is bounded at 2 minutes by the
	// detached-job timeout, and blobbackfill's copy→repoint window is one
	// copy plus one UPDATE. The one UNBOUNDED window — a restore — is not
	// covered by the age gate at all; the concurrency gate covers it.
	defaultOrphanFileGCMinAge = 72 * time.Hour

	// MinOrphanFileGCMinAge is the hard floor an operator may not configure
	// below. Startup fails rather than accepting a shorter window.
	MinOrphanFileGCMinAge = 24 * time.Hour

	// defaultOrphanRowPageSize bounds ONE candidate query so a pathological
	// install cannot blow memory in a single fetch. It is a page size, not a
	// budget: sweepRows keeps paging with a (created_at, id) keyset cursor
	// until the per-tick row budget is spent or the scan is exhausted.
	defaultOrphanRowPageSize = 500

	// defaultOrphanRowBudgetPerTick bounds how many candidate rows one tick
	// re-verifies in total. Whatever is left is picked up on the NEXT tick,
	// resuming from the cursor — never from the top of the scan, because most
	// candidates are KEPT rather than deleted and several keep-reasons never
	// clear (see OrphanFileGCWorker.rowCursor).
	defaultOrphanRowBudgetPerTick = 5000

	// defaultOrphanThumbnailBudgetPerTenant bounds the number of thumbnail
	// keys listed for ONE tenant in one tick. Per-TENANT on purpose: a single
	// shared budget is spent by whichever tenant is enumerated first (its LIVE
	// thumbnails count against it, and they outnumber orphans by orders of
	// magnitude), which silently disables the blob sweep for every tenant
	// after it — permanently, since the listing is lexicographic and no key is
	// ever removed.
	defaultOrphanThumbnailBudgetPerTenant = 5000
)

// orphanGCThumbnailSizes is the exhaustive set of thumbnail size suffixes the
// file service ever writes (see FileService.thumbnailSizes). A key whose size
// segment is not in this set does not round-trip and is KEPT.
var orphanGCThumbnailSizes = map[string]bool{
	"small":  true,
	"medium": true,
}

// orphanGCLinkAllowlist is the positive allowlist of linked_entity_type values
// the GC understands, re-asserted in Go on every candidate even though the
// registry query already applied it. It excludes, structurally:
//
//   - ""       — a STANDALONE file (#2235). First-class, exported in backups.
//     "No link" is NOT "orphan"; an orphan is a file whose link points at a
//     NONEXISTENT entity.
//   - "export" — owned by the backup subsystem (#2121 and friends). Never
//     probed, so the `deleted_at IS NULL` filter on the exports registry can
//     never be misread as "the export is gone".
//   - anything else — including a link type added in a future release. The
//     registries do not enforce models.FileEntity.ValidateWithContext, so the
//     DB is a superset of the validator's enumeration; an unknown type is kept
//     unconditionally until someone adds it here AND gives it a probe.
var orphanGCLinkAllowlist = map[string]bool{
	"commodity": true,
	"area":      true,
	"location":  true,
}

// Skip reasons, used both as the Prometheus label value and as the `reason`
// field on the forensic log record so a metric spike is greppable in the logs.
const (
	orphanGCSkipTargetExists      = "target_exists"
	orphanGCSkipProbeError        = "probe_error"
	orphanGCSkipMalformedLink     = "malformed_link"
	orphanGCSkipDisallowedLink    = "disallowed_link_type"
	orphanGCSkipMissingProbe      = "missing_probe"
	orphanGCSkipAge               = "age"
	orphanGCSkipInflight          = "inflight"
	orphanGCSkipGroupInactive     = "group_inactive"
	orphanGCSkipTenantInactive    = "tenant_inactive"
	orphanGCSkipOwnerUnresolvable = "owner_unresolvable"
	orphanGCSkipUnparseableKey    = "unparseable_key"
	orphanGCSkipReportMode        = "report_mode"
	orphanGCSkipSharedBlobKey     = "shared_blob_key"
	orphanGCSkipBudgetExhausted   = "budget_exhausted"
)

// Prometheus instrumentation for the orphan-file GC (#2237). Worker-local
// promauto counters, following group_purge_worker.go — internal/metrics is
// reserved for the cross-cutting HTTP/DB/auth/email/business series.
//
// inventario_orphan_gc_candidates_total is THE signal for the report→delete
// rollout: it increments in BOTH modes, so an operator can watch the predicate
// against real production data for a full release cycle before ever enabling
// deletion.
var (
	orphanGCRunsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "inventario_orphan_gc_runs_total",
		Help: "Orphan-file GC ticks by outcome (success, error, skipped_paused, skipped_disabled).",
	}, []string{"result"})
	orphanGCRowsScannedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_orphan_gc_rows_scanned_total",
		Help: "File rows returned by the orphan-candidate query (before per-candidate re-verification).",
	})
	orphanGCBlobsScannedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_orphan_gc_blobs_scanned_total",
		Help: "Thumbnail blob keys enumerated by the orphan-file GC.",
	})
	orphanGCCandidatesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "inventario_orphan_gc_candidates_total",
		Help: "Orphans that passed every safety gate, by kind (row, thumbnail). Incremented in report AND delete mode.",
	}, []string{"kind"})
	orphanGCDeletedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "inventario_orphan_gc_deleted_total",
		Help: "Orphans actually deleted by the orphan-file GC, by kind (row, thumbnail). Delete mode only.",
	}, []string{"kind"})
	orphanGCSkippedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "inventario_orphan_gc_skipped_total",
		Help: "Orphan-file GC candidates deliberately KEPT, by reason.",
	}, []string{"reason"})
	orphanGCFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "inventario_orphan_gc_failures_total",
		Help: "Orphan-file GC deletions that raised an error (retried next tick).",
	})
	orphanGCBlockedTenants = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "inventario_orphan_gc_blocked_tenants",
		Help: "Tenants skipped this tick because an export/restore is in flight (or recently finished). A value that stays >0 means a stuck operation is pinning a tenant — investigate the operation, do not bypass the gate.",
	})
	orphanGCLastSuccessTimestamp = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "inventario_orphan_gc_last_success_timestamp_seconds",
		Help: "Unix timestamp of the last orphan-file GC tick that completed without error.",
	})
)

// OrphanFileGCMode selects what a tick is allowed to do.
type OrphanFileGCMode string

const (
	// OrphanFileGCModeOff skips the tick entirely — no candidate query, no
	// bucket LIST. For operators who do not want to pay the listing cost.
	OrphanFileGCModeOff OrphanFileGCMode = "off"

	// OrphanFileGCModeReport is the SHIPPING DEFAULT and the mandatory dry
	// run. It evaluates the identical predicate, applies the identical gates,
	// emits the identical forensic log record and increments
	// inventario_orphan_gc_candidates_total — and stops one line short of the
	// delete. A false positive here is irreversible user data loss with no
	// undo (files has no soft-delete column and DeleteFileWithPhysical takes
	// the blob and thumbnails with it), so the predicate has to earn the right
	// to delete by being observed against real data first.
	OrphanFileGCModeReport OrphanFileGCMode = "report"

	// OrphanFileGCModeDelete enforces. Explicit operator opt-in only.
	OrphanFileGCModeDelete OrphanFileGCMode = "delete"
)

// ParseOrphanFileGCMode validates s. The second return reports whether s named
// a known mode — callers fail startup on false rather than silently defaulting
// a destructive knob.
func ParseOrphanFileGCMode(s string) (OrphanFileGCMode, bool) {
	m := OrphanFileGCMode(s)
	switch m {
	case OrphanFileGCModeOff, OrphanFileGCModeReport, OrphanFileGCModeDelete:
		return m, true
	}
	return "", false
}

// EntityExistenceProbe answers "does an entity with this id exist, anywhere in
// the database?".
//
// It MUST be backed by an RLS-BYPASSING service registry and match BY ID ONLY
// — never by (tenant, group, id). PUT /files/{id} performs no existence,
// tenant, or group check on the link target, so a file legitimately linked to
// an entity in ANOTHER group is reachable in production; a group-scoped probe
// would return ErrNotFound for that LIVE entity and hand the GC a live file to
// delete. That is the single easiest catastrophic bug in this feature and it
// is closed by construction here.
//
// "Does not exist" means EXACTLY registry.ErrNotFound. Any other error (a DB
// timeout, a connection reset) is a transient failure, never evidence of
// absence.
type EntityExistenceProbe func(ctx context.Context, id string) error

// OrphanFileGCProbes bundles one probe per allowlisted link type. A missing
// probe means the corresponding link type is never swept (fail closed).
type OrphanFileGCProbes struct {
	Commodity EntityExistenceProbe
	Area      EntityExistenceProbe
	Location  EntityExistenceProbe
}

func (p OrphanFileGCProbes) forLinkType(linkType string) EntityExistenceProbe {
	switch linkType {
	case "commodity":
		return p.Commodity
	case "area":
		return p.Area
	case "location":
		return p.Location
	}
	return nil
}

// OrphanFileGCFileRegistry is the (service-mode) slice of registry.FileRegistry
// the GC reads. Read-only: the worker adds no new write path anywhere.
type OrphanFileGCFileRegistry interface {
	ListOrphanCandidates(ctx context.Context, olderThan time.Time, after registry.OrphanCandidateCursor, limit int) ([]*models.FileEntity, error)
	ExistingIDs(ctx context.Context, ids []string) ([]string, error)
	ListIDsByOriginalPath(ctx context.Context, originalPath string) ([]string, error)
	Get(ctx context.Context, id string) (*models.FileEntity, error)
}

// OrphanFileGCExportRegistry is the export-side half of the concurrency gate.
// ListWithDeleted (not List) because a soft-deleted export still owns its
// artifacts, and an in-flight import is an export row too (Imported=true,
// Status=pending — see backup/import/worker.go), so one read covers export
// generation AND import ingestion.
type OrphanFileGCExportRegistry interface {
	ListWithDeleted(ctx context.Context) ([]*models.Export, error)
}

// OrphanFileGCRestoreOperationRegistry is the restore-side half of the
// concurrency gate.
type OrphanFileGCRestoreOperationRegistry interface {
	List(ctx context.Context) ([]*models.RestoreOperation, error)
}

// OrphanFileGCTenantRegistry enumerates tenants for the thumbnail sweep and
// resolves a candidate row's owning tenant for the lifecycle gate.
type OrphanFileGCTenantRegistry interface {
	List(ctx context.Context) ([]*models.Tenant, error)
	Get(ctx context.Context, id string) (*models.Tenant, error)
}

// OrphanFileGCGroupRegistry resolves a candidate row's owning group.
type OrphanFileGCGroupRegistry interface {
	Get(ctx context.Context, id string) (*models.LocationGroup, error)
}

// OrphanFileGCUserRegistry resolves a candidate row's creator so the delete can
// run under that user's own RLS scope.
type OrphanFileGCUserRegistry interface {
	Get(ctx context.Context, id string) (*models.User, error)
}

// OrphanFileDeleter is the destructive half — satisfied by *FileService. The
// GC never deletes a row itself: it calls DeleteFileWithPhysical under a
// USER-scoped (RLS-enforcing) context bound to the file's own tenant+group, so
// discovery is broad but destruction is narrow. That also tears down the
// thumbnail_generation_jobs → user_concurrency_slots NO ACTION FK chain, which
// a raw `DELETE FROM files` would trip.
type OrphanFileDeleter interface {
	DeleteFileWithPhysical(ctx context.Context, fileID string) error
}

// OrphanFileGCDeps is the narrow dependency set the sweeper needs. Every
// registry here is SERVICE-MODE (RLS-bypassing) and READ-ONLY except Deleter.
type OrphanFileGCDeps struct {
	Files          OrphanFileGCFileRegistry
	Probes         OrphanFileGCProbes
	Exports        OrphanFileGCExportRegistry
	Restores       OrphanFileGCRestoreOperationRegistry
	Tenants        OrphanFileGCTenantRegistry
	Groups         OrphanFileGCGroupRegistry
	Users          OrphanFileGCUserRegistry
	Deleter        OrphanFileDeleter
	UploadLocation string
}

// OrphanFileGCWorker periodically reclaims the residues the delete paths cannot
// close by construction (#2237): file ROWS whose linked entity no longer
// exists (crash between an entity-row delete and its DeleteLinkedFiles), and
// THUMBNAIL blobs whose owning file row is gone (the thumbnail worker's
// Get→write race).
//
// It sweeps EXACTLY those two classes. It never enumerates t/<tenant>/files/,
// t/<tenant>/exports/, t/<tenant>/restores/, seed keys, or anything outside the
// t/ prefix — see sweepTenantThumbnails for why each of those is a NEVER-SWEEP.
// Every field below the config block is owned by the sweep goroutine: RunOnce
// is called from the ticker loop only (and, in tests, serially), never
// concurrently with itself.
type OrphanFileGCWorker struct {
	deps        OrphanFileGCDeps
	interval    time.Duration
	minAge      time.Duration
	mode        OrphanFileGCMode
	pause       PauseChecker
	rowPageSize int
	rowBudget   int
	thumbBudget int
	stopCh      chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup

	// rowCursor is where the NEXT tick's candidate scan resumes, in the
	// (created_at, id) keyset order the registry guarantees.
	//
	// It is a liveness requirement, not an optimization. Re-verification KEEPS
	// far more rows than it deletes, and several keep-reasons never clear on a
	// later tick: a tenant pinned by a crashed export/restore (there is no
	// heartbeat — see logBlocked), a suspended tenant, a pending_deletion
	// group, a purged owner. Those rows are also, by construction, among the
	// OLDEST orphans in the installation, so a non-resumable oldest-first
	// window would be squatted on by exactly them and no other orphan would
	// ever be enumerated again. In the shipping REPORT mode nothing is ever
	// deleted at all, so without the cursor every tick would re-report the
	// identical page forever and the rollout signal would be meaningless.
	//
	// Reset to the zero value when a scan runs out of candidates, so the next
	// tick starts a fresh cycle from the oldest row.
	rowCursor registry.OrphanCandidateCursor

	// thumbCursor is the per-tenant resume point for the thumbnail listing:
	// the last key examined by a tick that ran out of per-tenant budget. The
	// next tick skips everything up to and including it, so a tenant whose
	// LIVE thumbnails exceed the budget still has the REST of its prefix
	// examined over successive ticks instead of re-listing the same head
	// forever. Cleared for a tenant as soon as its listing reaches the end.
	thumbCursor map[string]string
}

// OrphanFileGCOption customizes an OrphanFileGCWorker.
type OrphanFileGCOption func(*orphanFileGCOptions)

type orphanFileGCOptions struct {
	interval    time.Duration
	minAge      time.Duration
	mode        OrphanFileGCMode
	pause       PauseChecker
	rowPageSize int
	rowBudget   int
	thumbBudget int
}

// WithOrphanFileGCInterval overrides the sweep interval. Non-positive values
// are ignored.
func WithOrphanFileGCInterval(d time.Duration) OrphanFileGCOption {
	return func(o *orphanFileGCOptions) {
		if d > 0 {
			o.interval = d
		}
	}
}

// WithOrphanFileGCMinAge overrides the minimum age a row/blob must reach before
// it is eligible. Values below MinOrphanFileGCMinAge are REJECTED (the default
// is retained and the attempt is logged) — bootstrap already fails fast on
// them, and this is the defence-in-depth so a programmatic caller cannot shrink
// the window either.
func WithOrphanFileGCMinAge(d time.Duration) OrphanFileGCOption {
	return func(o *orphanFileGCOptions) {
		if d < MinOrphanFileGCMinAge {
			slog.Warn("Ignoring orphan-file GC min-age below the hard floor",
				"requested", d, "floor", MinOrphanFileGCMinAge, "using", o.minAge)
			return
		}
		o.minAge = d
	}
}

// WithOrphanFileGCMode sets the enforcement mode. An unknown mode is ignored
// (the safe default, report, is retained) — bootstrap fails fast on it.
func WithOrphanFileGCMode(m OrphanFileGCMode) OrphanFileGCOption {
	return func(o *orphanFileGCOptions) {
		if _, ok := ParseOrphanFileGCMode(string(m)); !ok {
			slog.Warn("Ignoring unknown orphan-file GC mode", "requested", m, "using", o.mode)
			return
		}
		o.mode = m
	}
}

// WithOrphanFileGCPauseController wires the soft-pause controller so the worker
// skips its sweep while the orphan-file-gc worker type is paused (#1308). This
// is the operator's emergency stop for the only destructive worker in the tree.
// A nil checker leaves the worker unpaused.
func WithOrphanFileGCPauseController(pc PauseChecker) OrphanFileGCOption {
	return func(o *orphanFileGCOptions) {
		if pc != nil {
			o.pause = pc
		}
	}
}

// WithOrphanFileGCRowBudget bounds the row sweep's work per tick: pageSize rows
// per candidate query, perTick rows re-verified in total before the tick stops
// and hands the rest to the next one (resuming from the keyset cursor, never
// from the top). Non-positive values are ignored.
func WithOrphanFileGCRowBudget(pageSize, perTick int) OrphanFileGCOption {
	return func(o *orphanFileGCOptions) {
		if pageSize > 0 {
			o.rowPageSize = pageSize
		}
		if perTick > 0 {
			o.rowBudget = perTick
		}
	}
}

// WithOrphanFileGCThumbnailBudget bounds how many thumbnail keys are listed for
// ONE TENANT in one tick. The remainder of that tenant's prefix is examined on
// subsequent ticks (see OrphanFileGCWorker.thumbCursor); other tenants are never
// starved, because the budget is not shared between them. Non-positive values
// are ignored.
func WithOrphanFileGCThumbnailBudget(perTenant int) OrphanFileGCOption {
	return func(o *orphanFileGCOptions) {
		if perTenant > 0 {
			o.thumbBudget = perTenant
		}
	}
}

// NewOrphanFileGCWorker creates the sweeper with safe defaults: a 24h interval,
// a 72h min age, and report mode (deletes nothing).
func NewOrphanFileGCWorker(deps OrphanFileGCDeps, opts ...OrphanFileGCOption) *OrphanFileGCWorker {
	options := orphanFileGCOptions{
		interval:    defaultOrphanFileGCInterval,
		minAge:      defaultOrphanFileGCMinAge,
		mode:        OrphanFileGCModeReport,
		rowPageSize: defaultOrphanRowPageSize,
		rowBudget:   defaultOrphanRowBudgetPerTick,
		thumbBudget: defaultOrphanThumbnailBudgetPerTenant,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &OrphanFileGCWorker{
		deps:        deps,
		interval:    options.interval,
		minAge:      options.minAge,
		mode:        options.mode,
		pause:       options.pause,
		rowPageSize: options.rowPageSize,
		rowBudget:   options.rowBudget,
		thumbBudget: options.thumbBudget,
		stopCh:      make(chan struct{}),
		thumbCursor: make(map[string]string),
	}
}

// missingDeps names every dependency RunOnce dereferences unconditionally.
// Checking only Files/Deleter would let a half-wired worker report a successful
// startup and then panic inside the sweep goroutine — on a DESTRUCTIVE worker,
// a nil Exports/Restores registry is not a crash to debug later, it is the
// concurrency gate silently missing.
func (d OrphanFileGCDeps) missingDeps() []string {
	var missing []string
	add := func(name string, ok bool) {
		if !ok {
			missing = append(missing, name)
		}
	}
	add("Files", d.Files != nil)
	add("Deleter", d.Deleter != nil)
	add("Exports", d.Exports != nil)
	add("Restores", d.Restores != nil)
	add("Tenants", d.Tenants != nil)
	add("Groups", d.Groups != nil)
	add("Users", d.Users != nil)
	add("UploadLocation", d.UploadLocation != "")
	// A link type with no probe is not fatal — verifyRowCandidate keeps every
	// candidate of that type (fail closed) — so probes are deliberately NOT
	// required here. A worker with no probes at all simply collects nothing.
	return missing
}

// Start launches the background sweep goroutine. Incomplete wiring is refused
// outright rather than deferred to a panic mid-sweep.
func (w *OrphanFileGCWorker) Start(ctx context.Context) {
	if missing := w.deps.missingDeps(); len(missing) > 0 {
		slog.Error("OrphanFileGCWorker: incomplete dependencies, skipping startup",
			"missing", strings.Join(missing, ","))
		return
	}
	w.wg.Go(func() {
		w.runCleanup(ctx)
	})
	slog.Info("Orphan file GC worker started",
		"interval", w.interval, "min_age", w.minAge, "mode", w.mode)
}

// Stop signals the worker to stop and waits for it to finish.
func (w *OrphanFileGCWorker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
	slog.Info("Orphan file GC worker stopped")
}

func (w *OrphanFileGCWorker) runCleanup(ctx context.Context) {
	// Sweep once at startup rather than waiting a full interval, matching
	// LoginEventRetentionWorker and the reminder workers. With the 24h default
	// the alternative is a day of silence after every deploy — and in the
	// shipping REPORT mode that silence IS the feature not working: the whole
	// point of report mode is to give the operator candidate counts to look at
	// before they ever arm the delete.
	//
	// Safe on this worker specifically because a restart bypasses NOTHING: the
	// soft-pause (#1308) is DB-backed and re-read inside RunOnce, so a paused
	// worker still does nothing on boot; the in-flight gate is DB-backed too, so
	// a tenant mid-restore stays blocked across the restart; and the age gate is
	// wall-clock, so nothing becomes eligible merely because the process is new.
	w.RunOnce(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.RunOnce(ctx)
		}
	}
}

// RunOnce performs exactly one sweep. Exported so an operator-facing one-shot
// (and the test suite) can drive the identical code path the ticker drives —
// there is no separate, less-guarded route into the delete.
func (w *OrphanFileGCWorker) RunOnce(ctx context.Context) {
	// Soft-pause (#1308) FIRST, before any registry or bucket call. The ticker
	// keeps running so resume takes effect on the next tick without a restart.
	// The blocked-tenant gauge asserts something about the LAST EVALUATION. A
	// tick that never evaluates the gate must not leave the previous tick's
	// value standing: the runbook reads a persistently-positive gauge as "a
	// tenant is pinned by a stuck operation", and a paused or disabled worker
	// would otherwise fake exactly that signal forever. Zero is the honest
	// value for "not measured this tick" — it is only meaningful while
	// inventario_orphan_gc_runs_total{result="success"} is advancing.
	if w.pause != nil && w.pause.IsPaused(models.WorkerTypeOrphanFileGC) {
		orphanGCRunsTotal.WithLabelValues("skipped_paused").Inc()
		orphanGCBlockedTenants.Set(0)
		return
	}
	if w.mode == OrphanFileGCModeOff {
		orphanGCRunsTotal.WithLabelValues("skipped_disabled").Inc()
		orphanGCBlockedTenants.Set(0)
		return
	}

	start := time.Now()
	cutoff := start.Add(-w.minAge)

	// Fail closed: any error building the gate aborts the WHOLE tick. A
	// partially-built gate would let a tenant with an in-flight restore
	// through, which is the one thing the gate exists to prevent.
	gate, err := w.buildConcurrencyGate(ctx, start)
	if err != nil {
		slog.Error("Orphan file GC: failed to build the in-flight concurrency gate; aborting tick", "error", err)
		orphanGCRunsTotal.WithLabelValues("error").Inc()
		orphanGCBlockedTenants.Set(0) // the gate was never evaluated — assert nothing
		return
	}
	orphanGCBlockedTenants.Set(float64(len(gate.blocked)))
	gate.logBlocked()

	rows, err := w.sweepRows(ctx, gate, cutoff)
	if err != nil {
		slog.Error("Orphan file GC: row sweep aborted", "error", err)
		orphanGCRunsTotal.WithLabelValues("error").Inc()
		return
	}

	blobs, err := w.sweepThumbnails(ctx, gate, cutoff)
	if err != nil {
		slog.Error("Orphan file GC: thumbnail sweep aborted", "error", err)
		orphanGCRunsTotal.WithLabelValues("error").Inc()
		return
	}

	orphanGCRunsTotal.WithLabelValues("success").Inc()
	orphanGCLastSuccessTimestamp.Set(float64(time.Now().Unix()))

	attrs := []any{
		"event", "orphan_gc.tick",
		"mode", string(w.mode),
		"min_age", w.minAge.String(),
		"rows_scanned", rows.scanned,
		"row_candidates", rows.candidates,
		"rows_deleted", rows.deleted,
		"blobs_scanned", blobs.scanned,
		"blob_candidates", blobs.candidates,
		"blobs_deleted", blobs.deleted,
		"blocked_tenants", len(gate.blocked),
		"duration_ms", time.Since(start).Milliseconds(),
	}
	if rows.candidates > 0 || blobs.candidates > 0 {
		slog.Info("Orphan file GC tick found candidates", attrs...)
		return
	}
	slog.Debug("Orphan file GC tick clean", attrs...)
}

// orphanGCStats is the per-sweep tally reported on the tick log line.
type orphanGCStats struct {
	scanned    int
	candidates int
	deleted    int
}

// ---------------------------------------------------------------------------
// Concurrency gate
// ---------------------------------------------------------------------------

// orphanGCBlocker records why a tenant is off-limits this tick.
type orphanGCBlocker struct {
	kind   string // "export" | "restore"
	id     string
	status string
	age    time.Duration
}

// orphanGCGate is the per-TENANT in-flight gate, built once per tick from
// RLS-bypassing service registries and re-asserted immediately before every
// individual delete.
//
// It has to be DB-backed, not in-process: `run workers --workers-only=housekeeping`
// puts this worker in a DIFFERENT PROCESS from the export/restore/import workers
// (they live in the `archive` group), so any in-memory flag or semaphore would
// be wrong by construction. The operation_slots / user_concurrency_slots tables
// are NOT usable here either — they are per-user upload/thumbnail concurrency
// limiters with a 30–300s expiry and cannot answer "is a restore live for this
// tenant".
type orphanGCGate struct {
	blocked map[string]orphanGCBlocker
}

func (g orphanGCGate) isBlocked(tenantID string) bool {
	_, ok := g.blocked[tenantID]
	return ok
}

// logBlocked surfaces the stuck-operation fail-safe. There is no heartbeat
// anywhere in the system: a crashed restore stays `running` forever and a
// crashed export stays `in_progress`, which under this gate PERMANENTLY
// disables the GC for that tenant. That is the correct direction (under-collect
// forever beats one wrong delete), but it must be diagnosable rather than a
// silent no-op — hence the warn plus inventario_orphan_gc_blocked_tenants.
//
// Do NOT "fix" this with a "stale after N hours → ignore it" rule: that
// re-opens the exact race the gate closes.
func (g orphanGCGate) logBlocked() {
	for tenantID, b := range g.blocked {
		slog.Warn("Orphan file GC: tenant skipped, operation in flight or recently finished",
			"event", "orphan_gc.blocked",
			"tenant_id", tenantID,
			"blocker", b.kind,
			"blocker_id", b.id,
			"blocker_status", b.status,
			"blocker_age_hours", b.age.Hours(),
		)
	}
}

func (w *OrphanFileGCWorker) buildConcurrencyGate(ctx context.Context, now time.Time) (orphanGCGate, error) {
	gate := orphanGCGate{blocked: make(map[string]orphanGCBlocker)}

	exports, err := w.deps.Exports.ListWithDeleted(ctx)
	if err != nil {
		return orphanGCGate{}, err
	}
	for _, e := range exports {
		if e == nil {
			continue
		}
		status := string(e.Status)
		switch e.Status {
		case models.ExportStatusPending, models.ExportStatusInProgress:
			// Covers the export worker AND import ingestion: a pending import
			// is an export row with Imported=true, Status=pending.
			gate.block(e.TenantID, orphanGCBlocker{kind: "export", id: e.ID, status: status})
		case models.ExportStatusCompleted, models.ExportStatusFailed:
			if age, recent := orphanGCTerminalAge(now, e.CompletedDate, e.CreatedDate, w.minAge); recent {
				gate.block(e.TenantID, orphanGCBlocker{kind: "export", id: e.ID, status: status, age: age})
			}
		}
	}

	restores, err := w.deps.Restores.List(ctx)
	if err != nil {
		return orphanGCGate{}, err
	}
	for _, r := range restores {
		if r == nil {
			continue
		}
		status := string(r.Status)
		switch r.Status {
		case models.RestoreStatusPending, models.RestoreStatusRunning:
			gate.block(r.TenantID, orphanGCBlocker{kind: "restore", id: r.ID, status: status})
		case models.RestoreStatusCompleted, models.RestoreStatusFailed:
			if age, recent := orphanGCTerminalAge(now, r.CompletedDate, r.CreatedDate, w.minAge); recent {
				gate.block(r.TenantID, orphanGCBlocker{kind: "restore", id: r.ID, status: status, age: age})
			}
		}
	}

	return gate, nil
}

func (g orphanGCGate) block(tenantID string, b orphanGCBlocker) {
	if tenantID == "" {
		return
	}
	if _, exists := g.blocked[tenantID]; exists {
		return // first blocker wins; one reason is enough to skip the tenant
	}
	g.blocked[tenantID] = b
}

// orphanGCTerminalAge implements the COOLDOWN half of the gate: a tenant stays
// off-limits for minAge after any export/restore reaches a terminal state.
//
// This is what closes the archive-timestamp hole. A restore persists the
// ARCHIVE's created_at/updated_at verbatim onto the rows it writes (see
// backup/restore/types ConvertToFileEntity — the registry Create overrides only
// ID/UUID/tenant/group), so a file row written ten seconds ago can carry
// created_at = 2019 and would clear a naive age gate instantly. The row age
// gate is therefore NOT load-bearing for restore; this wall-clock cooldown is.
//
// A terminal operation with no completed_date falls back to created_date, and a
// terminal operation with neither is treated as RECENT (fail closed).
func orphanGCTerminalAge(now time.Time, completed, created models.PTimestamp, minAge time.Duration) (age time.Duration, recent bool) {
	// Timestamp.ToTime is nil-safe and returns the zero time for a nil or
	// unparseable value.
	t := completed.ToTime()
	if t.IsZero() {
		t = created.ToTime()
	}
	if t.IsZero() {
		return 0, true // no usable timestamp at all ⇒ assume it just happened
	}
	age = now.Sub(t)
	return age, age < minAge
}

// ---------------------------------------------------------------------------
// Row sweep
// ---------------------------------------------------------------------------

// orphanRowVerdict is the outcome of re-verifying one candidate row.
type orphanRowVerdict struct {
	proceed bool
	reason  string
	user    *models.User
	group   *models.LocationGroup
}

// sweepRows deletes file ROWS whose linked entity no longer exists.
//
// Non-existence of an entity id is MONOTONE and IRREVERSIBLE, which is the
// backbone of the safety argument: entity IDs are server-minted on every
// Create (registry/postgres/store/rlsgroup.go runs SetID(generateID())
// unconditionally, including in service mode; the memory registry does the
// same), there is no raw INSERT with a caller-supplied id anywhere in non-test
// code, and a restore mints fresh ids rather than preserving archive ones. So
// once "entity X does not exist" is true it is true forever — no live
// operation, present or future, can make the file reachable again, and the
// TOCTOU window between the re-verification and the delete cannot invert.
// The scan is KEYSET-PAGINATED and the cursor SURVIVES THE TICK (w.rowCursor).
// Re-verification keeps far more rows than it deletes, and several
// keep-reasons never clear — a tenant pinned by a crashed restore, a suspended
// tenant, a pending_deletion group, a purged owner — so those rows would
// otherwise squat on a non-resumable oldest-first window forever and starve
// every other orphan in the installation (and, in report mode, where nothing is
// ever deleted, the tick would re-report the identical page for eternity).
// Running out of candidates rewinds the cursor and starts a fresh cycle.
func (w *OrphanFileGCWorker) sweepRows(ctx context.Context, gate orphanGCGate, cutoff time.Time) (orphanGCStats, error) {
	var stats orphanGCStats

	cursor := w.rowCursor
	// Persist the scan position even when the tick ABORTS. A probe failure that
	// keeps recurring on the same row (a poison row, a permanently unhealthy
	// replica) would otherwise replay the identical page every tick, forever,
	// and no other orphan in the installation would ever be enumerated. The
	// cursor was already advanced past that row, so the next tick resumes AFTER
	// it: the failing row is simply KEPT until the scan wraps and comes back to
	// it — the safe direction, and the only one that still makes progress.
	defer func() { w.rowCursor = cursor }()

	for stats.scanned < w.rowBudget {
		page, err := w.deps.Files.ListOrphanCandidates(ctx, cutoff, cursor, w.rowPageSize)
		if err != nil {
			return stats, err // fail closed: abort the tick, never sweep a partial result
		}
		if len(page) == 0 {
			cursor = registry.OrphanCandidateCursor{} // exhausted — next tick starts over
			break
		}

		for _, file := range page {
			if file == nil {
				continue
			}
			// Advance the cursor for every row we LOOK at, whether or not it is
			// deleted. A kept row must not be re-examined ahead of everything
			// behind it on the next tick.
			cursor = registry.OrphanCandidateCursor{CreatedAt: file.CreatedAt, ID: file.ID}
			stats.scanned++
			orphanGCRowsScannedTotal.Inc()

			if err := w.sweepRowCandidate(ctx, file, gate, cutoff, &stats); err != nil {
				return stats, err
			}
			if stats.scanned >= w.rowBudget {
				break
			}
		}

		if len(page) < w.rowPageSize {
			cursor = registry.OrphanCandidateCursor{} // the scan is exhausted
			break
		}
	}

	return stats, nil
}

// sweepRowCandidate re-verifies one candidate and, in delete mode, deletes it.
// A non-nil error is a transient failure that aborts the whole tick.
func (w *OrphanFileGCWorker) sweepRowCandidate(
	ctx context.Context,
	file *models.FileEntity,
	gate orphanGCGate,
	cutoff time.Time,
	stats *orphanGCStats,
) error {
	verdict, err := w.verifyRowCandidate(ctx, file, gate, cutoff)
	if err != nil {
		// A non-ErrNotFound probe failure must never read as "gone".
		orphanGCSkippedTotal.WithLabelValues(orphanGCSkipProbeError).Inc()
		// NAME the row that aborted the tick. The cursor deliberately advances
		// past it, so a permanently-failing row (a poison row, a flaky replica)
		// stops a tick every time the scan comes back round to it — and without
		// this record an operator would watch skipped{reason=probe_error} climb
		// with nothing to grep for.
		logOrphanRow(slog.LevelError, w.mode, "failed", orphanGCSkipProbeError, file, "error", err.Error())
		return err
	}
	if !verdict.proceed {
		orphanGCSkippedTotal.WithLabelValues(verdict.reason).Inc()
		logOrphanRow(slog.LevelWarn, w.mode, "skipped", verdict.reason, file)
		return nil
	}

	stats.candidates++
	orphanGCCandidatesTotal.WithLabelValues("row").Inc()

	if w.mode != OrphanFileGCModeDelete {
		orphanGCSkippedTotal.WithLabelValues(orphanGCSkipReportMode).Inc()
		logOrphanRow(slog.LevelWarn, w.mode, "candidate", orphanGCSkipReportMode, file)
		return nil
	}

	// The forensic record is written BEFORE the delete: it is the only artifact
	// from which a destroyed file can be reconstructed. It is logged as
	// "deleting", NOT "deleted" — the delete can still fail (and did, for every
	// orphan whose thumbnail-job chain could not be torn down), and a log that
	// claims a destruction that never happened is worse than no log at all.
	// Success is confirmed by its own record; failure by deleteRow's.
	logOrphanRow(slog.LevelInfo, w.mode, "deleting", "", file)
	if w.deleteRow(ctx, file, verdict) {
		stats.deleted++
		orphanGCDeletedTotal.WithLabelValues("row").Inc()
		logOrphanRow(slog.LevelInfo, w.mode, "deleted", "", file)
	}
	return nil
}

// verifyRowCandidate re-asserts every gate in Go, immediately before the
// delete — the candidate query is a filter, never an authorization.
//
// Returns a non-nil error ONLY for a transient probe failure, which aborts the
// whole tick. Everything else is expressed as proceed=false + a reason, i.e.
// KEEP the file.
func (w *OrphanFileGCWorker) verifyRowCandidate(ctx context.Context, file *models.FileEntity, gate orphanGCGate, cutoff time.Time) (orphanRowVerdict, error) {
	// R1/R2/R4/R5 — the cheap, purely local gates (allowlist, malformed link,
	// age, in-flight tenant). Split out so this function stays inside the
	// gocyclo bound; the ordering is unchanged and each rule keeps its own
	// skip reason.
	if reason := screenRowCandidate(file, gate, cutoff); reason != "" {
		return orphanRowVerdict{reason: reason}, nil
	}

	// R6 — lifecycle of the owning group/tenant.
	group, reason := w.resolveOwnerLifecycle(ctx, file)
	if reason != "" {
		return orphanRowVerdict{reason: reason}, nil
	}

	// R3 — EXISTENCE. Service-mode, by ID only. "Gone" means EXACTLY
	// registry.ErrNotFound; anything else aborts the tick.
	probe := w.deps.Probes.forLinkType(file.LinkedEntityType)
	if probe == nil {
		// NOT disallowed_link_type: screenRowCandidate already rejected every
		// non-allowlisted type above, so reaching here means the type IS
		// allowlisted and the worker was handed a nil probe for it — a MISWIRING,
		// not a policy decision. Both outcomes keep the file (fail closed), which
		// is exactly why they must not share a label: a miswired worker reports
		// zero candidates forever, and "disallowed_link_type" makes that look like
		// the intended handling of an unknown type instead of the bug it is.
		return orphanRowVerdict{reason: orphanGCSkipMissingProbe}, nil
	}
	switch err := probe(ctx, file.LinkedEntityID); {
	case err == nil:
		return orphanRowVerdict{reason: orphanGCSkipTargetExists}, nil
	case errors.Is(err, registry.ErrNotFound):
		// The one and only path to a delete.
	default:
		return orphanRowVerdict{}, err
	}

	// R7 — executability. Deletion runs through FileService.DeleteFileWithPhysical
	// under the file's OWN owner, so a raw service-mode row delete (which would
	// also FK-fail on the thumbnail job chain) is never needed as a fallback.
	user, err := w.deps.Users.Get(ctx, file.CreatedByUserID)
	if err != nil || user == nil {
		return orphanRowVerdict{reason: orphanGCSkipOwnerUnresolvable}, nil
	}

	// R8 — SOLE OWNERSHIP OF THE BLOB. The row delete is RLS-narrow, but the
	// blob delete that follows it inside DeleteFileWithPhysical is KEY-scoped,
	// and a blob key is NOT row-unique: there is no unique index on
	// files.original_path.
	//
	// Uploads minted since #2241 carry a server-side UUID and cannot collide.
	// The rows already sitting in deployed databases can, forever: their key is
	// `t/<tenant>/files/<sanitized-name>-<unix SECONDS><ext>` — no group segment,
	// no row segment, no randomness — so two members of different groups who
	// uploaded the same filename in one second have two DISTINCT rows on ONE
	// key. Deleting this orphan's blob would then destroy the bytes of a LIVE
	// row (a standalone file, #2235, is not even reachable by any other gate
	// here), irreversibly: `files` has no soft-delete and there is no trash.
	//
	// So: unless this row is the ONLY one referencing its key, the file is KEPT
	// whole. Under-collecting a rare legacy collision is free; destroying a live
	// file's bytes is not. DeleteFileWithPhysical re-asserts the same guard
	// (#2241) — this one is here so the GC never even reports such a row as a
	// candidate, and so a delete-mode tick cannot spend its budget on rows the
	// primitive would refuse anyway.
	//
	// No TOCTOU: a candidate must be at least MinOrphanFileGCMinAge old, and no
	// live writer can mint a key colliding with a legacy one (new keys are
	// UUIDs); a restore, the only writer that can re-introduce an archived key,
	// blocks the whole tenant via the concurrency gate.
	if file.File != nil && file.OriginalPath != "" {
		refs, cerr := w.deps.Files.ListIDsByOriginalPath(ctx, file.OriginalPath)
		if cerr != nil {
			return orphanRowVerdict{}, cerr // transient: abort the tick, never guess
		}
		for _, id := range refs {
			if id != file.ID {
				return orphanRowVerdict{reason: orphanGCSkipSharedBlobKey}, nil
			}
		}
	}

	return orphanRowVerdict{proceed: true, user: user, group: group}, nil
}

// screenRowCandidate applies the gates that need no I/O: R1 (positive
// allowlist), R2 (malformed link), R4 (age, BOTH timestamps) and R5 (in-flight
// tenant). Returns the skip reason, or "" when the candidate survives them all.
//
// R1 is re-asserted here even though the candidate query already applied it, so
// a future registry bug cannot widen the blast radius: "" (standalone, #2235),
// 'export', and any unknown/future link type are KEPT.
func screenRowCandidate(file *models.FileEntity, gate orphanGCGate, cutoff time.Time) string {
	if !orphanGCLinkAllowlist[file.LinkedEntityType] {
		return orphanGCSkipDisallowedLink
	}
	// A set link type with an empty id is malformed DATA, not garbage: no probe
	// can prove an empty id absent. The registry predicate already excludes such
	// rows, so this branch is unreachable today and malformed_link never fires —
	// deliberately. The candidate query is a FILTER, never an authorization, and
	// this worker must remain correct against any predicate it is handed,
	// including a future relaxed one.
	if file.LinkedEntityID == "" {
		return orphanGCSkipMalformedLink
	}
	// BOTH timestamps. updated_at is the load-bearing one: PUT /files/{id}
	// stamps it with the app wall clock, so a file attached concurrently with
	// an entity delete is immune for the whole window.
	if !file.CreatedAt.Before(cutoff) || !file.UpdatedAt.Before(cutoff) {
		return orphanGCSkipAge
	}
	// Re-checked per candidate rather than only once per tick.
	if gate.isBlocked(file.TenantID) {
		return orphanGCSkipInflight
	}
	return ""
}

// resolveOwnerLifecycle is R6: the file's group and tenant must both exist and
// be ACTIVE. A pending_deletion group belongs to GroupPurgeWorker and a
// non-active tenant may be mid-administrative-operation (#2115 hard delete), so
// the GC keeps its hands off both. Under-collection is free; a wrong delete is
// not. Returns the resolved group (needed for the impersonated delete) or a
// skip reason.
func (w *OrphanFileGCWorker) resolveOwnerLifecycle(ctx context.Context, file *models.FileEntity) (*models.LocationGroup, string) {
	group, err := w.deps.Groups.Get(ctx, file.GroupID)
	if err != nil || group == nil {
		return nil, orphanGCSkipOwnerUnresolvable
	}
	if group.Status != models.LocationGroupStatusActive {
		return nil, orphanGCSkipGroupInactive
	}
	tenant, err := w.deps.Tenants.Get(ctx, file.TenantID)
	if err != nil || tenant == nil {
		return nil, orphanGCSkipOwnerUnresolvable
	}
	if tenant.Status != models.TenantStatusActive {
		return nil, orphanGCSkipTenantInactive
	}
	return group, ""
}

// deleteRow executes the delete under an impersonated (user, group) context so
// the DELETE statement runs under RLS bound to the file's own tenant+group:
// even a catastrophic bug in the candidate query cannot reach a row outside
// that tuple — a wrong candidate simply matches zero rows.
//
// Reports whether a row was actually removed. registry.ErrNotFound is tolerated
// (idempotent under multiple `run workers` replicas: two replicas racing the
// same orphan produce one delete and one tolerated not-found).
func (w *OrphanFileGCWorker) deleteRow(ctx context.Context, file *models.FileEntity, verdict orphanRowVerdict) bool {
	dctx := appctx.WithUser(ctx, verdict.user)
	dctx = appctx.WithGroup(dctx, verdict.group)

	err := w.deps.Deleter.DeleteFileWithPhysical(dctx, file.ID)
	switch {
	case err == nil:
		return true
	case errors.Is(err, registry.ErrNotFound):
		return false // another replica won the race
	default:
		orphanGCFailuresTotal.Inc()
		slog.Error("Orphan file GC: failed to delete orphan file row",
			"event", "orphan_gc.row", "action", "failed",
			"file_id", file.ID, "tenant_id", file.TenantID, "error", err.Error())
		return false
	}
}

// logOrphanRow writes the forensic record. Because deletion is irreversible and
// `files` has no soft-delete column, THIS LINE IS THE RECOVERY ARTIFACT: an
// operator must be able to reconstruct what was destroyed, and hand-verify a
// report-mode candidate against the UI, from the log alone. The field set is
// identical in both modes so grepping is mode-independent.
//
// The action vocabulary is exact, because an operator reconciling a data-loss
// report reads these as fact:
//
//	skipped   — evaluated and KEPT (reason says why)
//	candidate — would be deleted; report mode, so it was kept
//	deleting  — about to be deleted; the pre-image, written while the row still exists
//	deleted   — the delete RETURNED SUCCESS. Never emitted speculatively.
//	failed    — the delete raised (deleteRow); the row is still there and is retried
func logOrphanRow(level slog.Level, mode OrphanFileGCMode, action, reason string, file *models.FileEntity, extra ...any) {
	attrs := []any{
		"event", "orphan_gc.row",
		"mode", string(mode),
		"action", action,
		"reason", reason,
		"file_id", file.ID,
		"file_uuid", file.GetUUID(),
		"tenant_id", file.TenantID,
		"group_id", file.GroupID,
		"created_by_user_id", file.CreatedByUserID,
		"linked_entity_type", file.LinkedEntityType,
		"linked_entity_id", file.LinkedEntityID,
		"linked_entity_meta", file.LinkedEntityMeta,
		"title", file.Title,
		"tags", strings.Join(file.Tags, ","),
		"created_at", file.CreatedAt,
		"updated_at", file.UpdatedAt,
	}
	if file.File != nil {
		attrs = append(attrs,
			"path", file.Path,
			"original_path", file.OriginalPath,
			"ext", file.Ext,
			"mime_type", file.MIMEType,
			"size_bytes", file.SizeBytes,
		)
	}
	attrs = append(attrs, extra...)
	slog.Log(context.Background(), level, "Orphan file GC: orphan file row", attrs...)
}

// ---------------------------------------------------------------------------
// Thumbnail (blob) sweep
// ---------------------------------------------------------------------------

// sweepThumbnails reclaims derived thumbnail blobs whose owning file row is
// gone (the thumbnail worker's Get→write race: the worker Gets the row through
// an RLS-bypassing registry and then writes the blobs from a detached
// goroutine, so a file deleted inside that window leaves thumbnails behind).
//
// Thumbnails are the ONLY blob class this worker touches, and the only one it
// safely can:
//
//   - a thumbnail key EMBEDS the owning row's primary key, so orphan-ness is an
//     EXACT single-row existence question — no assumption about the
//     completeness of a globally-scanned keep-set is needed anywhere; and
//   - thumbnails are DERIVED and REGENERABLE, so even a hypothetical false
//     positive costs a re-render, not data.
func (w *OrphanFileGCWorker) sweepThumbnails(ctx context.Context, gate orphanGCGate, cutoff time.Time) (orphanGCStats, error) {
	var stats orphanGCStats

	tenants, err := w.deps.Tenants.List(ctx)
	if err != nil {
		return stats, err
	}

	b, err := blob.OpenBucket(ctx, w.deps.UploadLocation)
	if err != nil {
		return stats, err
	}
	defer b.Close()

	// The listing budget is PER TENANT, never shared. A shared budget is spent
	// by whichever tenant is enumerated first — by its LIVE thumbnails, which
	// outnumber orphans by orders of magnitude — and every tenant after it is
	// then skipped, on this tick and on every future one (the listing is
	// lexicographic from the top and nothing removes those keys). That would
	// make the blob half of the GC a permanent no-op for most of the
	// installation, invisibly.
	for _, tenant := range tenants {
		if tenant == nil {
			continue
		}
		if tenant.Status != models.TenantStatusActive {
			orphanGCSkippedTotal.WithLabelValues(orphanGCSkipTenantInactive).Inc()
			continue
		}
		if gate.isBlocked(tenant.ID) {
			orphanGCSkippedTotal.WithLabelValues(orphanGCSkipInflight).Inc()
			continue
		}
		tstats, terr := w.sweepTenantThumbnails(ctx, b, tenant.ID, gate, cutoff)
		if terr != nil {
			return stats, terr
		}
		stats.scanned += tstats.scanned
		stats.candidates += tstats.candidates
		stats.deleted += tstats.deleted
	}

	return stats, nil
}

// orphanThumbKey is one aged-out thumbnail key awaiting the row probe.
type orphanThumbKey struct {
	key     string
	modTime time.Time
}

// sweepTenantThumbnails sweeps exactly ONE prefix: t/<tenant>/thumbnails/.
//
// Everything else in the bucket is NEVER-SWEEP — not filtered out, but never
// passed to bucket.List in the first place, so no bug in a predicate can reach
// it:
//
//   - t/<tenant>/files/    — two incompatible key shapes (a sanitized basename
//     from the upload path, a file UUID from the restore path), so a key cannot
//     be resolved back to a row; the only correct test is set-membership against
//     a COMPLETE global snapshot of files.original_path, whose completeness
//     cannot be proven under concurrency. Three writers are blob-first with no
//     coordination (upload, restore, the blobbackfill CLI). The failure mode is
//     irreversible loss of the user's file BYTES. NOT SWEPT. (The crash-window
//     blobs #2237 cares about are still reclaimed — by the ROW sweep, because
//     DeleteFileWithPhysical removes the original blob and its thumbnails.)
//   - t/<tenant>/exports/  — a FAILED export leaves its .inb referenced by NO
//     file row AND NO export row (export.FilePath is only assigned on the
//     success path), indistinguishable from an in-flight export's partial
//     artifact. The fix belongs at the source, not in a GC.
//   - t/<tenant>/restores/ — THE most dangerous case (#2121). POST /uploads/restores
//     writes the blob and creates NO ROW OF ANY KIND; it stays rowless until
//     POST /exports/import, rowless forever if the user never submits, and
//     rowless forever if the import FAILS. A "blobs with no owning file row"
//     sweep here would destroy a user's uploaded backup.
//   - t/<tenant>/seed-*    — sits directly under the tenant root; owned by real
//     file rows.
//   - anything outside t/  — legacy flat keys (pre-#1793) are still live
//     wherever `inventario backfill blobs` was never run.
func (w *OrphanFileGCWorker) sweepTenantThumbnails(
	ctx context.Context,
	bucket *blob.Bucket,
	tenantID string,
	gate orphanGCGate,
	cutoff time.Time,
) (orphanGCStats, error) {
	var stats orphanGCStats

	prefix := blobkeys.TenantPrefix(tenantID) + blobkeys.ThumbnailsSegment + "/"

	// Step 1+2 — LIST, and AGE-FILTER BEFORE ANY DB READ. This ordering is what
	// protects the thumbnail worker's detached Get→write window and any
	// regeneration in flight; reading the DB snapshot first and listing second
	// would re-open it.
	aged, err := w.listAgedThumbnails(ctx, bucket, tenantID, prefix, cutoff, &stats)
	if err != nil {
		return stats, err
	}
	if len(aged) == 0 {
		return stats, nil
	}

	// T1/T2 — parse every aged key BEFORE any DB read. Anything that does not
	// parse-and-round-trip exactly is KEPT: this is the structural
	// anti-traversal / anti-confusion guard — we only ever delete a key we
	// REBUILT ourselves.
	parsed := make([]orphanThumbCandidate, 0, len(aged))
	for _, cand := range aged {
		fileID, size, ok := parseThumbnailBlobKey(tenantID, cand.key)
		if !ok {
			orphanGCSkippedTotal.WithLabelValues(orphanGCSkipUnparseableKey).Inc()
			slog.Warn("Orphan file GC: unparseable thumbnail key, keeping",
				"event", "orphan_gc.thumbnail", "action", "skipped",
				"reason", orphanGCSkipUnparseableKey, "blob_key", cand.key, "tenant_id", tenantID)
			continue
		}
		parsed = append(parsed, orphanThumbCandidate{key: cand, fileID: fileID, size: size})
	}
	if len(parsed) == 0 {
		return stats, nil
	}

	// Step 3 — only NOW read the DB, and ask ONLY about the ids the listed keys
	// actually named. A tenant-wide id dump would cost, and hold in memory, one
	// entry per file row in the tenant to answer a question about at most
	// thumbBudget keys — the dominant cost of the sweep on a large install, for
	// no added safety.
	//
	// A row created DURING the listing lands in this set (the query runs after),
	// which is the safe direction: the sweep keeps its thumbnails.
	live, err := w.liveFileIDs(ctx, parsed)
	if err != nil {
		return stats, err
	}

	for _, p := range parsed {
		cand, fileID, size := p.key, p.fileID, p.size

		// T3 — the double check. The membership set alone can never cause a
		// delete: a fresh service-mode Get must independently return EXACTLY
		// registry.ErrNotFound.
		if live[fileID] {
			orphanGCSkippedTotal.WithLabelValues(orphanGCSkipTargetExists).Inc()
			continue
		}
		switch _, err := w.deps.Files.Get(ctx, fileID); {
		case err == nil:
			orphanGCSkippedTotal.WithLabelValues(orphanGCSkipTargetExists).Inc()
			continue
		case errors.Is(err, registry.ErrNotFound):
			// The one and only path to a delete.
		default:
			orphanGCSkippedTotal.WithLabelValues(orphanGCSkipProbeError).Inc()
			// NAME the key that aborted the tick, for the same reason the row
			// path does: the cursor moves past it, so a permanently-failing
			// probe stops a tick each time the cycle returns to it, and the
			// metric alone gives an operator nothing to grep for.
			slog.Error("Orphan file GC: thumbnail row probe failed; aborting tick",
				"event", "orphan_gc.thumbnail", "action", "failed",
				"reason", orphanGCSkipProbeError, "blob_key", cand.key,
				"file_id", fileID, "tenant_id", tenantID, "error", err.Error())
			// Persist the position BEFORE aborting, for the same reason as the
			// row cursor: a recurring failure on one key would otherwise make
			// every tick re-list and re-probe the identical head of this
			// tenant's prefix forever, and no thumbnail past it would ever be
			// reached. Resuming after the failed key KEEPS it (the safe
			// direction) and revisits it when the cycle wraps.
			w.thumbCursor[tenantID] = cand.key
			return stats, err
		}

		stats.candidates++
		orphanGCCandidatesTotal.WithLabelValues("thumbnail").Inc()

		if w.mode != OrphanFileGCModeDelete {
			orphanGCSkippedTotal.WithLabelValues(orphanGCSkipReportMode).Inc()
			logOrphanThumbnail(slog.LevelWarn, w.mode, "candidate", tenantID, fileID, size, cand)
			continue
		}
		// T5 — re-assert the in-flight gate immediately before this delete.
		if gate.isBlocked(tenantID) {
			orphanGCSkippedTotal.WithLabelValues(orphanGCSkipInflight).Inc()
			logOrphanThumbnail(slog.LevelWarn, w.mode, "skipped", tenantID, fileID, size, cand)
			continue
		}
		// Two-phase, exactly like the row path: "deleting" is the pre-image,
		// "deleted" is only ever written after the delete actually succeeded.
		// A record claiming a destruction that never happened is worse than no
		// record — the log IS the recovery artifact.
		logOrphanThumbnail(slog.LevelInfo, w.mode, "deleting", tenantID, fileID, size, cand)
		if w.deleteThumbnail(ctx, bucket, tenantID, fileID, size) {
			stats.deleted++
			orphanGCDeletedTotal.WithLabelValues("thumbnail").Inc()
			logOrphanThumbnail(slog.LevelInfo, w.mode, "deleted", tenantID, fileID, size, cand)
		}
	}

	return stats, nil
}

// listAgedThumbnails enumerates ONE tenant's thumbnail prefix and keeps only the
// keys whose bucket ModTime is comfortably older than cutoff.
//
// ModTime is the authoritative freshness signal on the blob side precisely
// because it is set by the storage backend (S3 LastModified / filesystem
// mtime) and no application or user input can set it — unlike files.created_at,
// which a restore writes verbatim from the archive. A zero/unavailable ModTime
// or one in the FUTURE is KEPT (never compute a negative age and wrap).
//
// The per-tenant budget bounds the listing (blob.List is documented to walk
// keys in lexicographic order), and a tenant that runs out of it RESUMES on the
// next tick from the last key it examined instead of re-listing the same head
// forever: live thumbnails are never deleted, so a truncated window would
// otherwise never advance and every orphan past the cutoff key would be
// unreachable for good. Truncation is recorded (orphanGCSkipBudgetExhausted +
// a warn) so it is never a silent no-op.
func (w *OrphanFileGCWorker) listAgedThumbnails(
	ctx context.Context,
	bucket *blob.Bucket,
	tenantID, prefix string,
	cutoff time.Time,
	stats *orphanGCStats,
) ([]orphanThumbKey, error) {
	var aged []orphanThumbKey

	resume := w.thumbCursor[tenantID]
	budget := w.thumbBudget
	lastKey := ""
	truncated := false

	iter := bucket.List(&blob.ListOptions{Prefix: prefix})
	for {
		obj, err := iter.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if obj.IsDir {
			continue
		}
		if resume != "" && obj.Key <= resume {
			continue // covered by an earlier tick of this cycle
		}
		if budget <= 0 {
			truncated = true
			break
		}
		budget--
		lastKey = obj.Key
		stats.scanned++
		orphanGCBlobsScannedTotal.Inc()

		if obj.ModTime.IsZero() || !obj.ModTime.Before(cutoff) {
			orphanGCSkippedTotal.WithLabelValues(orphanGCSkipAge).Inc()
			continue
		}
		aged = append(aged, orphanThumbKey{key: obj.Key, modTime: obj.ModTime})
	}

	if truncated && lastKey != "" {
		w.thumbCursor[tenantID] = lastKey
		orphanGCSkippedTotal.WithLabelValues(orphanGCSkipBudgetExhausted).Inc()
		slog.Warn("Orphan file GC: thumbnail listing truncated, resuming next tick",
			"event", "orphan_gc.thumbnail", "action", "skipped",
			"reason", orphanGCSkipBudgetExhausted, "tenant_id", tenantID,
			"budget", w.thumbBudget, "resume_after", lastKey)
		return aged, nil
	}
	// Reached the end of the prefix: the next tick starts a fresh cycle.
	delete(w.thumbCursor, tenantID)

	return aged, nil
}

// orphanThumbCandidate is one listed thumbnail key that survived the age filter
// AND the round-trip parse: the key itself plus the components it decomposed
// into. Parsing up front is what lets the DB question be asked about exactly the
// file ids the keys named, and nothing else.
type orphanThumbCandidate struct {
	key    orphanThumbKey
	fileID string
	size   string
}

// liveFileIDs asks, in ONE round-trip, which of the file ids named by the parsed
// keys still have a row. Duplicate ids (a file has a small AND a medium
// thumbnail) collapse before the query.
func (w *OrphanFileGCWorker) liveFileIDs(ctx context.Context, parsed []orphanThumbCandidate) (map[string]bool, error) {
	seen := make(map[string]bool, len(parsed))
	ids := make([]string, 0, len(parsed))
	for _, p := range parsed {
		if seen[p.fileID] {
			continue
		}
		seen[p.fileID] = true
		ids = append(ids, p.fileID)
	}

	existing, err := w.deps.Files.ExistingIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	live := make(map[string]bool, len(existing))
	for _, id := range existing {
		live[id] = true
	}
	return live, nil
}

// deleteThumbnail removes the REBUILT key — never the raw string the bucket
// listing handed us.
//
// There is deliberately NO Exists() pre-check. A stat-then-delete pair does not
// make the delete safer (the key is rebuilt, so it can only ever name a
// thumbnail of a file we just proved gone) — it only opens a window: with two
// `run workers` replicas, the other one can remove the key between our stat and
// our delete, and the resulting NotFound would be counted as a FAILURE and
// logged at error level. That is a lie in exactly the race this worker is
// expected to run in, and it would make delete-mode runs noisy enough to hide a
// real failure.
//
// So: one round trip, and a NotFound from Delete is a no-op, not a failure —
// the same tolerance the row path applies to registry.ErrNotFound. Returns
// whether THIS call is the one that removed the key.
func (w *OrphanFileGCWorker) deleteThumbnail(ctx context.Context, bucket *blob.Bucket, tenantID, fileID, size string) bool {
	key := blobkeys.BuildThumbnailBlobKey(tenantID, fileID, size)

	if err := bucket.Delete(ctx, key); err != nil {
		if gcerrors.Code(err) == gcerrors.NotFound {
			// Already gone: another replica won the race, or it was removed
			// between the listing and here. Nothing to report.
			return false
		}
		orphanGCFailuresTotal.Inc()
		slog.Error("Orphan file GC: failed to delete orphan thumbnail",
			"event", "orphan_gc.thumbnail", "action", "failed",
			"blob_key", key, "tenant_id", tenantID, "error", err.Error())
		return false
	}
	return true
}

// parseThumbnailBlobKey parses a listed key as
// `t/<tenant>/thumbnails/<fileID>_<size>.jpg` and applies the ROUND-TRIP GUARD:
// the parsed components are fed back through blobkeys.BuildThumbnailBlobKey and
// the result must byte-equal the listed key. Any key that does not round-trip
// (a nested path, a bad size, a non-.jpg extension, a crafted traversal) is
// rejected, and the caller KEEPS it.
func parseThumbnailBlobKey(tenantID, key string) (fileID, size string, ok bool) {
	prefix := blobkeys.TenantPrefix(tenantID) + blobkeys.ThumbnailsSegment + "/"
	rest, found := strings.CutPrefix(key, prefix)
	if !found || rest == "" {
		return "", "", false
	}
	if strings.ContainsRune(rest, '/') {
		return "", "", false // no nesting under thumbnails/
	}
	base, found := strings.CutSuffix(rest, ".jpg")
	if !found {
		return "", "", false // all thumbnails are JPEG
	}
	idx := strings.LastIndex(base, "_")
	if idx <= 0 || idx == len(base)-1 {
		return "", "", false
	}
	fileID, size = base[:idx], base[idx+1:]
	if !orphanGCThumbnailSizes[size] {
		return "", "", false
	}
	if blobkeys.BuildThumbnailBlobKey(tenantID, fileID, size) != key {
		return "", "", false // round-trip guard
	}
	return fileID, size, true
}

func logOrphanThumbnail(level slog.Level, mode OrphanFileGCMode, action, tenantID, fileID, size string, cand orphanThumbKey) {
	slog.Log(context.Background(), level, "Orphan file GC: orphan thumbnail blob",
		"event", "orphan_gc.thumbnail",
		"mode", string(mode),
		"action", action,
		"blob_key", cand.key,
		"tenant_id", tenantID,
		"file_id", fileID,
		"size", size,
		"mod_time", cand.modTime,
		"age_hours", time.Since(cand.modTime).Hours(),
	)
}
