package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"gocloud.dev/blob"
	"gocloud.dev/gcerrors"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/internal/metrics"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// Timing of the intake worker.
const (
	defaultIntakeInterval = 5 * time.Second
	// intakeStuckMargin is how much longer than the provider deadline a
	// claimed scan may take (reading the files, the audit row, storing the
	// result) before the sweep assumes its worker died.
	intakeStuckMargin = 2 * time.Minute
	// intakeStuckWithoutTimeout is the sweep's wait when the provider has no
	// deadline.
	intakeStuckWithoutTimeout = 10 * time.Minute
	// intakeRateLimitWait is how long a source waits when its author's
	// hourly scan limit is reached.
	intakeRateLimitWait = 5 * time.Minute
	// intakeMaxAttempts is how many scans a source gets for failures that
	// may pass (timeouts, an unavailable provider, a worker that died
	// during the scan) before it fails. A resume or a rescan gives it as
	// many again.
	intakeMaxAttempts = 3
	// defaultIntakeSourcesPerTick bounds one tick, so a long queue does not
	// keep the worker from noticing Stop or a pause.
	defaultIntakeSourcesPerTick = 20
)

// Error codes of a source that could not be scanned.
const (
	IntakeErrorNoFiles             = "no_files"
	IntakeErrorFileUnreadable      = "file_unreadable"
	IntakeErrorUnsupportedFile     = "unsupported_file"
	IntakeErrorFileTooLarge        = "file_too_large"
	IntakeErrorTooManyFiles        = "too_many_files"
	IntakeErrorNoDetails           = "no_details"
	IntakeErrorAuthorUnavailable   = "author_unavailable"
	IntakeErrorProviderDisabled    = "provider_disabled"
	IntakeErrorProviderMisconfig   = "provider_misconfigured"
	IntakeErrorProviderTimeout     = "provider_timeout"
	IntakeErrorProviderUnavailable = "provider_unavailable"
	IntakeErrorProviderError       = "provider_error"
)

// intakeOneItemHint is the hint sent with a source the member said shows one
// item, so the provider does not take several photos of it for a receipt
// with several products.
const intakeOneItemHint = "All attached files show the SAME single item from different sides or its documents. Return exactly one item."

// IntakeWorker scans the sources of intake sessions in the background
// (#2810). It claims one due source at a time, reads its files, and calls the
// scan service as the session's author, so the author's hourly scan limit
// and the audit trail apply exactly as for a scan in the item form. The
// outcome is stored once; a source is never scanned again unless a member
// asks for a rescan or resumes a failed one. Once the outcome is stored, the
// scan's suggestion for each file, a document tag and a title, is applied to
// the file where it takes nothing the member set (see intakeFileSuggestion).
type IntakeWorker struct {
	factorySet     *registry.FactorySet
	scan           *CommodityScanService
	uploadLocation string
	interval       time.Duration
	stuckAfter     time.Duration
	perTick        int
	pause          PauseChecker
	clock          func() time.Time
	stopCh         chan struct{}
	stopOnce       sync.Once
	wg             sync.WaitGroup
	// limitedUntil remembers, per author, until when the hourly scan limit
	// holds, so the other sources of that author wait without another
	// provider-side rejection each.
	limitedUntil map[string]time.Time
}

// IntakeWorkerOption customizes an IntakeWorker.
type IntakeWorkerOption func(*IntakeWorker)

// WithIntakeInterval sets how often the worker looks for due sources.
// Non-positive values are ignored.
func WithIntakeInterval(d time.Duration) IntakeWorkerOption {
	return func(w *IntakeWorker) {
		if d > 0 {
			w.interval = d
		}
	}
}

// WithIntakeSourcesPerTick sets how many sources one tick processes at most.
// Non-positive values are ignored.
func WithIntakeSourcesPerTick(n int) IntakeWorkerOption {
	return func(w *IntakeWorker) {
		if n > 0 {
			w.perTick = n
		}
	}
}

// WithIntakePauseController skips ticks while the intake worker type is
// paused. A nil checker leaves the worker unpaused.
func WithIntakePauseController(pc PauseChecker) IntakeWorkerOption {
	return func(w *IntakeWorker) {
		if pc != nil {
			w.pause = pc
		}
	}
}

// WithIntakeClock replaces the wall clock. Test-only.
func WithIntakeClock(fn func() time.Time) IntakeWorkerOption {
	return func(w *IntakeWorker) {
		if fn != nil {
			w.clock = fn
		}
	}
}

// NewIntakeWorker builds the worker.
func NewIntakeWorker(factorySet *registry.FactorySet, scan *CommodityScanService, uploadLocation string, opts ...IntakeWorkerOption) *IntakeWorker {
	w := &IntakeWorker{
		factorySet:     factorySet,
		scan:           scan,
		uploadLocation: uploadLocation,
		interval:       defaultIntakeInterval,
		perTick:        defaultIntakeSourcesPerTick,
		clock:          time.Now,
		stopCh:         make(chan struct{}),
		limitedUntil:   map[string]time.Time{},
	}
	for _, opt := range opts {
		opt(w)
	}
	w.stuckAfter = intakeStuckWithoutTimeout
	if timeout := scan.Config().Timeout; timeout > 0 {
		w.stuckAfter = timeout + intakeStuckMargin
	}
	return w
}

// Start launches the background goroutine. Without a scan provider there is
// nothing to do and the worker does not start; sessions cannot be created
// then either.
func (w *IntakeWorker) Start(ctx context.Context) {
	if !w.scan.Enabled() {
		slog.InfoContext(ctx, "Intake worker not started: no AI scan provider is configured")
		return
	}
	w.wg.Go(func() {
		w.run(ctx)
	})
	slog.InfoContext(ctx, "Intake worker started", "interval", w.interval)
}

// Stop signals the worker to stop and waits for the scan in progress.
func (w *IntakeWorker) Stop() {
	w.stopOnce.Do(func() { close(w.stopCh) })
	w.wg.Wait()
	slog.Info("Intake worker stopped")
}

func (w *IntakeWorker) run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		w.Tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
		}
	}
}

func (w *IntakeWorker) stopped(ctx context.Context) bool {
	select {
	case <-w.stopCh:
		return true
	default:
		return ctx.Err() != nil
	}
}

// Tick requeues stuck scans and then scans due sources until none is due or
// the tick's budget is spent. It returns how many sources it processed.
func (w *IntakeWorker) Tick(ctx context.Context) int {
	if w.pause != nil && w.pause.IsPaused(models.WorkerTypeIntake) {
		return 0
	}
	reg := w.factorySet.IntakeRegistryFactory.CreateServiceRegistry()
	now := w.clock()
	if sweep, err := reg.SweepStuckSources(ctx, now.Add(-w.stuckAfter), now, intakeMaxAttempts, IntakeErrorProviderError); err != nil {
		slog.ErrorContext(ctx, "Intake worker: failed to sweep stuck sources", "error", err)
	} else if sweep.Changed > 0 {
		slog.WarnContext(ctx, "Intake worker: requeued stuck sources", "count", sweep.Changed, "failed", sweep.Failed)
		for range sweep.Failed {
			metrics.RecordJobDeadLettered(string(models.WorkerTypeIntake))
		}
	}
	processed := 0
	for processed < w.perTick && !w.stopped(ctx) {
		src, err := reg.ClaimNextSource(ctx, w.clock())
		if errors.Is(err, registry.ErrNotFound) {
			break
		}
		if err != nil {
			slog.ErrorContext(ctx, "Intake worker: failed to claim a source", "error", err)
			break
		}
		processed++
		w.process(ctx, reg, src)
	}
	return processed
}

// process scans one claimed source and records the outcome.
func (w *IntakeWorker) process(ctx context.Context, reg registry.IntakeRegistry, src *models.IntakeSource) {
	log := slog.With("source_id", src.ID, "session_id", src.SessionID, "attempt", src.Attempts)
	now := w.clock()

	if until, ok := w.limitedUntil[src.CreatedByUserID]; ok && now.Before(until) {
		w.requeue(ctx, reg, src, until, models.IntakeWaitRateLimited, log)
		return
	}

	author, err := w.authorContext(ctx, src)
	if err != nil {
		log.WarnContext(ctx, "Intake worker: the session's author cannot scan in the group", "error", err)
		w.fail(ctx, reg, src, IntakeErrorAuthorUnavailable, log)
		return
	}

	// The claim is held for stuckAfter; the work stops before the sweep
	// would hand the source to another worker, leaving time to store it.
	workCtx, cancel := context.WithTimeout(author.ctx, w.stuckAfter-intakeStuckMargin/2)
	defer cancel()

	files, code, err := w.readFiles(workCtx, src)
	if err != nil {
		log.ErrorContext(ctx, "Intake worker: failed to read the source files", "error", err)
		w.retryOrFail(ctx, reg, src, IntakeErrorFileUnreadable, log)
		return
	}
	if code != "" {
		w.fail(ctx, reg, src, code, log)
		return
	}

	scanFields, fieldDefs := w.customFields(author.ctx, log)
	input := ScanInput{
		Photos:                files.photos,
		PreferredCurrencyCode: string(author.group.GroupCurrency),
		ItemTypes:             w.itemTypes(author.ctx, log),
		CustomFields:          scanFields,
	}
	if src.OneItem {
		input.HintFromUser = intakeOneItemHint
	}
	result, err := w.scan.Scan(workCtx, author.user.TenantID, author.user.ID, input)
	if err != nil {
		w.handleScanError(ctx, reg, src, author.user.ID, err, log)
		return
	}

	candidates := intakeCandidatesFromScan(result, fieldDefs)
	if src.OneItem && len(candidates) > 1 {
		// The files show one item: keep the most prominent product.
		candidates = candidates[:1]
	}
	if len(candidates) == 0 {
		w.fail(ctx, reg, src, IntakeErrorNoDetails, log)
		return
	}
	warnings := make([]models.IntakeWarning, 0, len(result.Warnings))
	for _, warn := range result.Warnings {
		warnings = append(warnings, models.IntakeWarning{Code: warn.Code, Field: warn.Field, Detail: warn.Detail})
	}
	scan := registry.IntakeScan{Candidates: candidates, Warnings: warnings}
	// The file suggestions are stored with the scan, so a result dropped
	// below leaves the files as they are. A file delivered from another
	// system keeps the title it came with.
	fileUpdates := &intakeFileUpdates{}
	if !src.HasExternalIdentity() {
		fileUpdates = w.fileUpdates(author.ctx, files.ids, result.Files, log)
		scan.Files = fileUpdates.updates
	}
	err = reg.StoreScan(ctx, src.ID, src.Attempts, scan, w.clock())
	switch {
	case errors.Is(err, registry.ErrIntakeSourceChanged):
		log.InfoContext(ctx, "Intake worker: the source changed during the scan; result dropped")
	case err != nil:
		log.ErrorContext(ctx, "Intake worker: failed to store the scan", "error", err)
	default:
		log.InfoContext(ctx, "Intake worker: source scanned", "candidates", len(candidates))
		fileUpdates.logStored(ctx, log)
	}
}

// intakeAuthor is the author of a session acting in the session's group.
type intakeAuthor struct {
	ctx   context.Context
	user  *models.User
	group *models.LocationGroup
}

// authorContext returns the session's author acting in the session's group,
// after checking that the author is still an active member who may edit
// items there.
func (w *IntakeWorker) authorContext(ctx context.Context, src *models.IntakeSource) (*intakeAuthor, error) {
	user, err := w.factorySet.UserRegistry.Get(ctx, src.CreatedByUserID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, fmt.Errorf("user %s is not active", user.ID)
	}
	group, err := w.factorySet.LocationGroupRegistry.Get(ctx, src.GroupID)
	if err != nil {
		return nil, err
	}
	if group.Status != models.LocationGroupStatusActive {
		return nil, fmt.Errorf("group %s is not active", group.ID)
	}
	membership, err := w.factorySet.GroupMembershipRegistry.GetByGroupAndUser(ctx, group.ID, user.ID)
	if err != nil {
		return nil, err
	}
	if !membership.Role.AtLeast(models.GroupRoleUser) {
		return nil, fmt.Errorf("user %s cannot edit items in group %s", user.ID, group.ID)
	}
	return &intakeAuthor{ctx: appctx.WithGroup(appctx.WithUser(ctx, user), group), user: user, group: group}, nil
}

// itemTypes returns the item types of the author's group for the scan to
// classify into, as the scan in the item form offers them. When they cannot
// be read, the scan offers the built-in ones.
func (w *IntakeWorker) itemTypes(ctx context.Context, log *slog.Logger) []ScanItemType {
	reg, err := w.factorySet.ItemTypeRegistryFactory.CreateUserRegistry(ctx)
	if err == nil {
		var types []*models.ItemType
		if types, err = reg.List(ctx); err == nil {
			return ScanItemTypes(types)
		}
	}
	log.WarnContext(ctx, "Intake worker: item types unavailable, offering the built-in ones", "error", err)
	return nil
}

// customFields lists the group's custom fields for the scan to fill, as the
// single-item scan offers them, with the definitions by ID that check the
// values it reads. When the list cannot be read the scan goes on without
// them.
func (w *IntakeWorker) customFields(ctx context.Context, log *slog.Logger) ([]ScanCustomField, map[string]*models.CustomFieldDefinition) {
	reg, err := w.factorySet.CustomFieldRegistryFactory.CreateUserRegistry(ctx)
	if err == nil {
		var defs []*models.CustomFieldDefinition
		if defs, err = reg.List(ctx); err == nil {
			byID := make(map[string]*models.CustomFieldDefinition, len(defs))
			for _, d := range defs {
				byID[d.ID] = d
			}
			return ScanCustomFields(defs), byID
		}
	}
	log.WarnContext(ctx, "Intake worker: custom fields unavailable, scanning without them", "error", err)
	return nil, nil
}

// intakeFiles are the files of a source as the scan gets them.
type intakeFiles struct {
	photos []ScanPhotoInput
	// ids are the IDs of the files in the order of photos, which is the
	// order the scan numbers them in.
	ids []string
}

// readFiles reads the live files of the source. A code reports a problem
// with the files themselves, which a retry cannot fix; an error one that
// may pass.
func (w *IntakeWorker) readFiles(ctx context.Context, src *models.IntakeSource) (intakeFiles, string, error) {
	files, err := w.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return intakeFiles{}, "", err
	}
	linked, err := files.ListByLinkedEntity(ctx, models.FileLinkIntake, src.ID)
	if err != nil {
		return intakeFiles{}, "", err
	}
	if len(linked) == 0 {
		return intakeFiles{}, IntakeErrorNoFiles, nil
	}
	cfg := w.scan.Config()
	if cfg.MaxPhotos > 0 && len(linked) > cfg.MaxPhotos {
		return intakeFiles{}, IntakeErrorTooManyFiles, nil
	}
	b, err := blob.OpenBucket(ctx, w.uploadLocation)
	if err != nil {
		return intakeFiles{}, "", err
	}
	defer b.Close()

	out := intakeFiles{photos: make([]ScanPhotoInput, 0, len(linked)), ids: make([]string, 0, len(linked))}
	for _, file := range linked {
		if file.File == nil || !AllowedMIMETypes[file.MIMEType] {
			return intakeFiles{}, IntakeErrorUnsupportedFile, nil
		}
		if cfg.MaxPhotoBytes > 0 && file.SizeBytes > int64(cfg.MaxPhotoBytes) {
			return intakeFiles{}, IntakeErrorFileTooLarge, nil
		}
		data, code, err := readIntakeBlob(ctx, b, file.OriginalPath, cfg.MaxPhotoBytes)
		if err != nil || code != "" {
			return intakeFiles{}, code, err
		}
		out.photos = append(out.photos, ScanPhotoInput{
			Filename:    file.Path + file.Ext,
			ContentType: file.MIMEType,
			Data:        data,
		})
		out.ids = append(out.ids, file.ID)
	}
	return out, "", nil
}

func readIntakeBlob(ctx context.Context, b *blob.Bucket, key string, maxBytes int) ([]byte, string, error) {
	reader, err := b.NewReader(ctx, key, nil)
	if gcerrors.Code(err) == gcerrors.NotFound {
		return nil, IntakeErrorFileUnreadable, nil
	}
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()
	var r io.Reader = reader
	if maxBytes > 0 {
		r = io.LimitReader(reader, int64(maxBytes)+1)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, "", err
	}
	if maxBytes > 0 && len(data) > maxBytes {
		return nil, IntakeErrorFileTooLarge, nil
	}
	return data, "", nil
}

// handleScanError records the outcome of a scan that failed: a wait for the
// author's rate limit, a retry for a failure that may pass, or a failure.
func (w *IntakeWorker) handleScanError(ctx context.Context, reg registry.IntakeRegistry, src *models.IntakeSource, userID string, err error, log *slog.Logger) {
	switch {
	case errors.Is(err, ErrScanRateLimited):
		until := w.clock().Add(intakeRateLimitWait)
		w.limitedUntil[userID] = until
		w.requeue(ctx, reg, src, until, models.IntakeWaitRateLimited, log)
	case errors.Is(err, ErrScanProviderTimeout):
		w.retryOrFail(ctx, reg, src, IntakeErrorProviderTimeout, log)
	case errors.Is(err, ErrScanProviderUnavailable):
		w.retryOrFail(ctx, reg, src, IntakeErrorProviderUnavailable, log)
	case errors.Is(err, ErrScanProviderError):
		w.retryOrFail(ctx, reg, src, IntakeErrorProviderError, log)
	case errors.Is(err, ErrScanProviderMisconfigured):
		w.fail(ctx, reg, src, IntakeErrorProviderMisconfig, log)
	case errors.Is(err, ErrScanProviderDisabled):
		w.fail(ctx, reg, src, IntakeErrorProviderDisabled, log)
	case errors.Is(err, ErrScanTooManyPhotos):
		w.fail(ctx, reg, src, IntakeErrorTooManyFiles, log)
	case errors.Is(err, ErrScanPhotoTooLarge):
		w.fail(ctx, reg, src, IntakeErrorFileTooLarge, log)
	case errors.Is(err, ErrScanUnsupportedMIME):
		w.fail(ctx, reg, src, IntakeErrorUnsupportedFile, log)
	case errors.Is(err, ErrScanNoPhotos):
		w.fail(ctx, reg, src, IntakeErrorNoFiles, log)
	default:
		log.ErrorContext(ctx, "Intake worker: scan failed", "error", err)
		w.retryOrFail(ctx, reg, src, IntakeErrorProviderError, log)
	}
}

// retryOrFail queues the source again with a growing delay while it has
// attempts left, and fails it with code after the last one.
func (w *IntakeWorker) retryOrFail(ctx context.Context, reg registry.IntakeRegistry, src *models.IntakeSource, code string, log *slog.Logger) {
	if src.CountedAttempts() >= intakeMaxAttempts {
		w.fail(ctx, reg, src, code, log)
		return
	}
	delay := time.Duration(src.CountedAttempts()) * time.Minute
	w.requeue(ctx, reg, src, w.clock().Add(delay), models.IntakeWaitRetry, log)
}

func (w *IntakeWorker) requeue(ctx context.Context, reg registry.IntakeRegistry, src *models.IntakeSource, next time.Time, reason string, log *slog.Logger) {
	if err := reg.RequeueSource(ctx, src.ID, src.Attempts, next, reason); err != nil && !errors.Is(err, registry.ErrIntakeSourceChanged) {
		log.ErrorContext(ctx, "Intake worker: failed to requeue the source", "error", err)
	}
}

func (w *IntakeWorker) fail(ctx context.Context, reg registry.IntakeRegistry, src *models.IntakeSource, code string, log *slog.Logger) {
	log.InfoContext(ctx, "Intake worker: source failed", "code", code)
	err := reg.FailSource(ctx, src.ID, src.Attempts, code)
	switch {
	case err == nil:
		metrics.RecordJobDeadLettered(string(models.WorkerTypeIntake))
	case !errors.Is(err, registry.ErrIntakeSourceChanged):
		log.ErrorContext(ctx, "Intake worker: failed to mark the source failed", "error", err)
	}
}
