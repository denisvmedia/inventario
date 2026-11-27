package memory

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"math"
	"slices"
	"sync"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/google/uuid"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// IntakeRegistryFactory creates IntakeRegistry instances (#2810). All
// registries it creates share the session map of base and the source and
// candidate maps of state. Postgres links the files of a new session and
// looks up drafts in the same transaction; the memory backend holds the file
// and draft factories instead.
type IntakeRegistryFactory struct {
	base         *Registry[models.IntakeSession, *models.IntakeSession]
	state        *intakeState
	fileFactory  *FileRegistryFactory
	draftFactory *CommodityDraftRegistryFactory
}

// intakeState holds the sources and candidates. mu serializes every
// read-check-write sequence the way postgres row locks do.
type intakeState struct {
	mu         sync.Mutex
	sources    map[string]*models.IntakeSource
	candidates map[string]*models.IntakeCandidate
}

// IntakeRegistry is the context-aware in-memory registry of intake sessions.
type IntakeRegistry struct {
	*Registry[models.IntakeSession, *models.IntakeSession]
	factory  *IntakeRegistryFactory
	tenantID string
	service  bool
}

var (
	_ registry.IntakeRegistry        = (*IntakeRegistry)(nil)
	_ registry.IntakeRegistryFactory = (*IntakeRegistryFactory)(nil)
)

func NewIntakeRegistryFactory(fileFactory *FileRegistryFactory, draftFactory *CommodityDraftRegistryFactory) *IntakeRegistryFactory {
	return &IntakeRegistryFactory{
		base: NewRegistry[models.IntakeSession, *models.IntakeSession](),
		state: &intakeState{
			sources:    map[string]*models.IntakeSource{},
			candidates: map[string]*models.IntakeCandidate{},
		},
		fileFactory:  fileFactory,
		draftFactory: draftFactory,
	}
}

func (f *IntakeRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.IntakeRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *IntakeRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.IntakeRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return &IntakeRegistry{
		Registry: &Registry[models.IntakeSession, *models.IntakeSession]{
			items:   f.base.items,
			lock:    f.base.lock,
			userID:  user.ID,
			groupID: appctx.GroupIDFromContext(ctx),
		},
		factory:  f,
		tenantID: user.TenantID,
	}, nil
}

func (f *IntakeRegistryFactory) CreateServiceRegistry() registry.IntakeRegistry {
	return &IntakeRegistry{
		Registry: &Registry[models.IntakeSession, *models.IntakeSession]{
			items: f.base.items,
			lock:  f.base.lock,
		},
		factory: f,
		service: true,
	}
}

func (r *IntakeRegistry) requireGroup() error {
	if r.service || r.groupID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}
	return nil
}

// visible reports whether a row of the given group is in this registry's
// scope: its group for a user registry, every group for the service one.
func (r *IntakeRegistry) visible(groupID string) bool {
	return r.service || (r.groupID != "" && groupID == r.groupID)
}

func (r *IntakeRegistry) fileRegistry(ctx context.Context) (registry.FileRegistry, error) {
	if r.service {
		return r.factory.fileFactory.CreateServiceRegistry(), nil
	}
	return r.factory.fileFactory.createUserFileRegistry(ctx)
}

func intakeNotFound(entityType, id string) error {
	return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", entityType, "entity_id", id))
}

// cloneIntakeFields copies fields through JSON, so the stored map shares
// nothing with the caller's and holds the types a JSONB round trip gives.
func cloneIntakeFields(fields models.IntakeFields) (models.IntakeFields, error) {
	if fields == nil {
		return models.IntakeFields{}, nil
	}
	raw, err := json.Marshal(map[string]models.IntakeFieldGuess(fields))
	if err != nil {
		return nil, err
	}
	out := models.IntakeFields{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func copyIntakeSource(s *models.IntakeSource) *models.IntakeSource {
	c := *s
	c.Warnings = slices.Clone(s.Warnings)
	if s.ClaimedAt != nil {
		c.ClaimedAt = new(*s.ClaimedAt)
	}
	if s.ScannedAt != nil {
		c.ScannedAt = new(*s.ScannedAt)
	}
	return &c
}

func copyIntakeCandidate(cand *models.IntakeCandidate) *models.IntakeCandidate {
	c := *cand
	c.AcceptedFields = slices.Clone(cand.AcceptedFields)
	c.Fields = make(models.IntakeFields, len(cand.Fields))
	maps.Copy(c.Fields, cand.Fields)
	if cand.DraftID != nil {
		c.DraftID = new(*cand.DraftID)
	}
	return &c
}

// Create stores a session without sources. CreateSession is the form the
// service uses; Create exists for the Registry contract.
func (r *IntakeRegistry) Create(ctx context.Context, session models.IntakeSession) (*models.IntakeSession, error) {
	stored, _, err := r.CreateSession(ctx, session, nil, 0)
	return stored, err
}

// Update writes the session's client key and origin. The session carries
// no other state.
func (r *IntakeRegistry) Update(ctx context.Context, session models.IntakeSession) (*models.IntakeSession, error) {
	session.UpdatedAt = time.Now()
	return r.Registry.Update(ctx, session)
}

func (r *IntakeRegistry) GetSessionByClientKey(ctx context.Context, clientKey string) (*models.IntakeSession, error) {
	all, err := r.Registry.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, s := range all {
		if s.ClientKey == clientKey {
			return s, nil
		}
	}
	return nil, intakeNotFound("intake_session", clientKey)
}

// CreateSession mirrors the postgres method.
func (r *IntakeRegistry) CreateSession(ctx context.Context, session models.IntakeSession, sources []registry.IntakeSourceInput, maxQueued int) (*models.IntakeSession, bool, error) {
	if err := r.requireGroup(); err != nil {
		return nil, false, err
	}
	if err := registry.CheckIntakeSourceInputs(sources); err != nil {
		return nil, false, err
	}
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()

	if existing, err := r.GetSessionByClientKey(ctx, session.ClientKey); err == nil {
		return existing, false, nil
	}
	if maxQueued > 0 {
		queued := r.countQueued()
		if queued+registry.QueuedIntakeSources(sources) > maxQueued {
			return nil, false, errxtrace.Classify(registry.ErrIntakeQueueFull, errx.Attrs("queued", queued, "limit", maxQueued))
		}
	}

	files, err := r.factory.fileFactory.createUserFileRegistry(ctx)
	if err != nil {
		return nil, false, errxtrace.Wrap("failed to create file registry", err)
	}
	fileByID, err := r.checkNewSources(ctx, files, sources)
	if err != nil {
		return nil, false, err
	}

	now := time.Now()
	session.CreatedAt = now
	session.UpdatedAt = now
	session.TenantID = r.tenantID
	if session.Origin == "" {
		session.Origin = models.IntakeOriginUpload
	}
	stored, err := r.Registry.CreateWithUser(ctx, session)
	if err != nil {
		return nil, false, errxtrace.Wrap("failed to create intake session", err)
	}

	for i, src := range sources {
		source := &models.IntakeSource{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
				EntityID:        models.EntityID{ID: uuid.NewString(), UUID: uuid.NewString()},
				TenantID:        stored.TenantID,
				GroupID:         stored.GroupID,
				CreatedByUserID: stored.CreatedByUserID,
			},
			SessionID:          stored.ID,
			SortOrder:          i,
			OneItem:            src.OneItem,
			Status:             models.IntakeSourcePending,
			NextAttemptAt:      now,
			ExternalSystem:     src.ExternalSystem,
			ExternalInstance:   src.ExternalInstance,
			ExternalDocumentID: src.ExternalDocumentID,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		if src.Delivered != nil {
			source.Status = models.IntakeSourceScanned
			source.ScannedAt = new(now)
			source.Warnings = slices.Clone(src.Delivered.Warnings)
			if err := st.storeCandidates(source, src.Delivered.Candidates, now); err != nil {
				return nil, false, err
			}
		}
		st.sources[source.ID] = source
		for _, fileID := range src.FileIDs {
			file := fileByID[fileID]
			if _, _, err := files.AddLink(ctx, models.FileLink{
				FileID:     fileID,
				EntityType: models.FileLinkIntake,
				EntityID:   source.ID,
				Meta:       models.DefaultFileLinkMeta(models.FileLinkIntake, file.Category),
			}); err != nil {
				return nil, false, errxtrace.Wrap("failed to link intake file", err)
			}
		}
	}
	return stored, true, nil
}

// checkNewSources checks the sources of a new session before anything is
// written, so a refused session leaves no rows behind, and returns their
// files by id. The caller holds state.mu.
func (r *IntakeRegistry) checkNewSources(ctx context.Context, files registry.FileRegistry, sources []registry.IntakeSourceInput) (map[string]*models.FileEntity, error) {
	fileByID := map[string]*models.FileEntity{}
	for _, src := range sources {
		for _, fileID := range src.FileIDs {
			file, err := files.Get(ctx, fileID)
			if err != nil {
				return nil, errxtrace.Wrap("failed to read intake file", err)
			}
			fileByID[fileID] = file
		}
		if src.ExternalDocumentID == "" {
			continue
		}
		for _, existing := range r.factory.state.sources {
			if existing.GroupID == r.groupID && existing.ExternalSystem == src.ExternalSystem &&
				existing.ExternalInstance == src.ExternalInstance && existing.ExternalDocumentID == src.ExternalDocumentID {
				return nil, errxtrace.Classify(registry.ErrAlreadyExists,
					errx.Attrs("entity_type", "intake_source", "external_document_id", src.ExternalDocumentID))
			}
		}
	}
	return fileByID, nil
}

// Delete deletes the session with its sources, candidates and the links of
// its sources.
func (r *IntakeRegistry) Delete(ctx context.Context, id string) error {
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()
	if _, err := r.Registry.Get(ctx, id); err != nil {
		return err
	}
	files, err := r.fileRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create file registry", err)
	}
	for sid, src := range st.sources {
		if src.SessionID != id {
			continue
		}
		if _, err := files.RemoveLinksToEntity(ctx, models.FileLinkIntake, sid); err != nil {
			return errxtrace.Wrap("failed to remove intake file links", err)
		}
		delete(st.sources, sid)
	}
	for cid, cand := range st.candidates {
		if cand.SessionID == id {
			delete(st.candidates, cid)
		}
	}
	return r.Registry.Delete(ctx, id)
}

func (r *IntakeRegistry) ListSessions(ctx context.Context, offset, limit int) ([]*models.IntakeSession, int, error) {
	all, err := r.Registry.List(ctx)
	if err != nil {
		return nil, 0, err
	}
	slices.SortFunc(all, func(a, b *models.IntakeSession) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		return cmp.Compare(b.ID, a.ID)
	})
	total := len(all)
	start := min(max(offset, 0), total)
	end := min(start+max(limit, 0), total)
	return all[start:end], total, nil
}

func (r *IntakeRegistry) ListOpenSessions(ctx context.Context) ([]*models.IntakeSession, error) {
	if err := r.requireGroup(); err != nil {
		return nil, err
	}
	all, _, err := r.ListSessions(ctx, 0, math.MaxInt)
	if err != nil {
		return nil, err
	}
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()
	summaries := map[string]*models.IntakeSessionSummary{}
	for _, sess := range all {
		summaries[sess.ID] = &models.IntakeSessionSummary{}
	}
	for _, src := range st.sources {
		if sum, ok := summaries[src.SessionID]; ok && r.visible(src.GroupID) {
			sum.AddSource(src, 0)
		}
	}
	for _, cand := range st.candidates {
		if sum, ok := summaries[cand.SessionID]; ok && r.visible(cand.GroupID) {
			sum.AddCandidate(cand)
		}
	}
	return slices.DeleteFunc(all, func(sess *models.IntakeSession) bool {
		return !summaries[sess.ID].Open()
	}), nil
}

func (r *IntakeRegistry) ListSources(_ context.Context, sessionIDs ...string) ([]*models.IntakeSource, error) {
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()
	out := []*models.IntakeSource{}
	for _, src := range st.sources {
		if r.visible(src.GroupID) && slices.Contains(sessionIDs, src.SessionID) {
			out = append(out, copyIntakeSource(src))
		}
	}
	slices.SortFunc(out, func(a, b *models.IntakeSource) int {
		return cmp.Or(cmp.Compare(a.SessionID, b.SessionID), cmp.Compare(a.SortOrder, b.SortOrder), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

func (r *IntakeRegistry) GetSource(_ context.Context, id string) (*models.IntakeSource, error) {
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()
	src, ok := st.sources[id]
	if !ok || !r.visible(src.GroupID) {
		return nil, intakeNotFound("intake_source", id)
	}
	return copyIntakeSource(src), nil
}

func (r *IntakeRegistry) GetSourceByExternalID(_ context.Context, system, instance, documentID string) (*models.IntakeSource, error) {
	if err := r.requireGroup(); err != nil {
		return nil, err
	}
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()
	if documentID != "" {
		for _, src := range st.sources {
			if r.visible(src.GroupID) && src.ExternalSystem == system && src.ExternalInstance == instance && src.ExternalDocumentID == documentID {
				return copyIntakeSource(src), nil
			}
		}
	}
	return nil, intakeNotFound("intake_source", documentID)
}

func (r *IntakeRegistry) ListCandidates(_ context.Context, sessionIDs ...string) ([]*models.IntakeCandidate, error) {
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()
	out := []*models.IntakeCandidate{}
	for _, cand := range st.candidates {
		if r.visible(cand.GroupID) && slices.Contains(sessionIDs, cand.SessionID) {
			out = append(out, copyIntakeCandidate(cand))
		}
	}
	sourceOrder := func(c *models.IntakeCandidate) int {
		if src, ok := st.sources[c.SourceID]; ok {
			return src.SortOrder
		}
		return 0
	}
	slices.SortFunc(out, func(a, b *models.IntakeCandidate) int {
		return cmp.Or(cmp.Compare(a.SessionID, b.SessionID), cmp.Compare(sourceOrder(a), sourceOrder(b)),
			cmp.Compare(a.SortOrder, b.SortOrder), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

func (r *IntakeRegistry) GetCandidate(_ context.Context, id string) (*models.IntakeCandidate, error) {
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()
	cand, ok := st.candidates[id]
	if !ok || !r.visible(cand.GroupID) {
		return nil, intakeNotFound("intake_candidate", id)
	}
	return copyIntakeCandidate(cand), nil
}

// ReviewCandidate mirrors the postgres method.
func (r *IntakeRegistry) ReviewCandidate(_ context.Context, id string, review registry.IntakeCandidateReview) (*models.IntakeCandidate, error) {
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()
	cand, ok := st.candidates[id]
	if !ok || !r.visible(cand.GroupID) {
		return nil, intakeNotFound("intake_candidate", id)
	}
	if cand.Outcome == models.IntakeCandidateCreated {
		return nil, errxtrace.Classify(registry.ErrIntakeCandidateFinished, errx.Attrs("entity_id", id))
	}
	if review.Selected != nil {
		cand.Selected = *review.Selected
		if cand.Selected && cand.Outcome == models.IntakeCandidateSkipped {
			cand.Outcome = models.IntakeCandidatePending
		}
	}
	if review.AcceptedFields != nil {
		cand.AcceptedFields = slices.Clone(*review.AcceptedFields)
	}
	if review.Quantity != nil {
		cand.Quantity = *review.Quantity
	}
	cand.UpdatedAt = time.Now()
	return copyIntakeCandidate(cand), nil
}

// SetCandidateOutcome mirrors the postgres method.
func (r *IntakeRegistry) SetCandidateOutcome(_ context.Context, id string, outcome models.IntakeCandidateOutcome, draftID *string, errorCode string) (*models.IntakeCandidate, error) {
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()
	cand, ok := st.candidates[id]
	if !ok || !r.visible(cand.GroupID) {
		return nil, intakeNotFound("intake_candidate", id)
	}
	applyCandidateOutcome(cand, outcome, draftID, errorCode)
	cand.UpdatedAt = time.Now()
	return copyIntakeCandidate(cand), nil
}

// applyCandidateOutcome sets the outcome fields the way the postgres UPDATE
// does: a draft id only with created, an error code only with failed.
func applyCandidateOutcome(cand *models.IntakeCandidate, outcome models.IntakeCandidateOutcome, draftID *string, errorCode string) {
	cand.Outcome = outcome
	cand.DraftID = nil
	cand.ErrorCode = ""
	switch outcome {
	case models.IntakeCandidateCreated:
		if draftID != nil {
			cand.DraftID = new(*draftID)
		}
	case models.IntakeCandidateFailed:
		cand.ErrorCode = errorCode
	}
}

func (r *IntakeRegistry) CancelSession(_ context.Context, sessionID string) (int, error) {
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()
	now := time.Now()
	n := 0
	for _, src := range st.sources {
		if !r.visible(src.GroupID) || src.SessionID != sessionID || !src.Status.IsQueued() {
			continue
		}
		src.Status = models.IntakeSourceCanceled
		src.WaitReason = ""
		src.UpdatedAt = now
		n++
	}
	return n, nil
}

func (r *IntakeRegistry) ResumeSession(_ context.Context, sessionID string, now time.Time) (int, error) {
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()
	n := 0
	for _, src := range st.sources {
		if !r.visible(src.GroupID) || src.SessionID != sessionID {
			continue
		}
		switch {
		case src.Status == models.IntakeSourceCanceled && src.ClaimedAt != nil:
			// A scan that runs keeps its claim: the cancel is undone.
			src.Status = models.IntakeSourceScanning
			src.UpdatedAt = now
		case src.Status == models.IntakeSourceCanceled, src.Status == models.IntakeSourceFailed:
			requeueIntakeSource(src, now)
		default:
			continue
		}
		n++
	}
	return n, nil
}

// requeueIntakeSource puts a source back in the queue with a fresh allowance
// of attempts. A requeued source is no longer a failure an operator
// discarded: if it fails again, the failed jobs show it again (#1307).
func requeueIntakeSource(src *models.IntakeSource, now time.Time) {
	src.Status = models.IntakeSourcePending
	src.DiscardedAt = nil
	src.RetryBase = src.Attempts
	src.NextAttemptAt = now
	src.ClaimedAt = nil
	src.WaitReason = ""
	src.ErrorCode = ""
	src.UpdatedAt = now
}

func (r *IntakeRegistry) RescanSource(_ context.Context, id string, now time.Time) (*models.IntakeSource, error) {
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()
	src, ok := st.sources[id]
	if !ok || !r.visible(src.GroupID) {
		return nil, intakeNotFound("intake_source", id)
	}
	if src.Status.IsQueued() || src.InScan() {
		return nil, errxtrace.Classify(registry.ErrIntakeSourceBusy, errx.Attrs("entity_id", id))
	}
	for _, cand := range st.candidates {
		if cand.SourceID != id {
			continue
		}
		if cand.Outcome == models.IntakeCandidateCreated || r.factory.draftFactory.hasLiveDraftWithKey(cand.GroupID, cand.DraftClientKey()) {
			return nil, errxtrace.Classify(registry.ErrIntakeSourceHasDrafts, errx.Attrs("entity_id", id))
		}
	}
	for cid, cand := range st.candidates {
		if cand.SourceID == id {
			delete(st.candidates, cid)
		}
	}
	requeueIntakeSource(src, now)
	src.Warnings = nil
	src.ScannedAt = nil
	return copyIntakeSource(src), nil
}

func (r *IntakeRegistry) CountQueued(_ context.Context) (int, error) {
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()
	return r.countQueued(), nil
}

// countQueued counts the visible queued sources; the caller holds state.mu.
func (r *IntakeRegistry) countQueued() int {
	n := 0
	for _, src := range r.factory.state.sources {
		if r.visible(src.GroupID) && src.Status.IsQueued() {
			n++
		}
	}
	return n
}

func (r *IntakeRegistry) SweepStuckSources(_ context.Context, claimedBefore, now time.Time, maxAttempts int, errorCode string) (registry.IntakeSweep, error) {
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()
	var sweep registry.IntakeSweep
	for _, src := range st.sources {
		if !r.visible(src.GroupID) {
			continue
		}
		stuck := src.ClaimedAt == nil || src.ClaimedAt.Before(claimedBefore)
		switch {
		case src.Status == models.IntakeSourceScanning && stuck:
			src.ClaimedAt = nil
			if maxAttempts > 0 && src.CountedAttempts() >= maxAttempts {
				src.Status = models.IntakeSourceFailed
				src.WaitReason = ""
				src.ErrorCode = errorCode
				sweep.Failed++
			} else {
				src.Status = models.IntakeSourcePending
				src.NextAttemptAt = now
				src.WaitReason = models.IntakeWaitRetry
			}
		case src.Status == models.IntakeSourceCanceled && src.ClaimedAt != nil && stuck:
			src.ClaimedAt = nil
		default:
			continue
		}
		src.UpdatedAt = now
		sweep.Changed++
	}
	return sweep, nil
}

func (r *IntakeRegistry) ClaimNextSource(_ context.Context, now time.Time) (*models.IntakeSource, error) {
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()
	var next *models.IntakeSource
	for _, src := range st.sources {
		if !r.visible(src.GroupID) || src.Status != models.IntakeSourcePending || src.NextAttemptAt.After(now) {
			continue
		}
		if next == nil || cmp.Or(src.NextAttemptAt.Compare(next.NextAttemptAt), src.CreatedAt.Compare(next.CreatedAt),
			cmp.Compare(src.SortOrder, next.SortOrder), cmp.Compare(src.ID, next.ID)) < 0 {
			next = src
		}
	}
	if next == nil {
		return nil, intakeNotFound("intake_source", "")
	}
	next.Status = models.IntakeSourceScanning
	next.Attempts++
	next.ClaimedAt = new(now)
	next.UpdatedAt = now
	return copyIntakeSource(next), nil
}

// inScan returns the source when it is still the scan the worker claimed:
// scanning, or canceled after the claim, with the same attempt.
func (r *IntakeRegistry) inScan(sourceID string, attempt int) (*models.IntakeSource, error) {
	src, ok := r.factory.state.sources[sourceID]
	if !ok || !r.visible(src.GroupID) || src.Attempts != attempt || !src.InScan() {
		return nil, errxtrace.Classify(registry.ErrIntakeSourceChanged, errx.Attrs("entity_id", sourceID))
	}
	return src, nil
}

// scanning is inScan for an outcome a canceled scan does not take: a source
// canceled during the scan only loses the claim, so a resume can queue it
// again, and ErrIntakeSourceChanged is returned.
func (r *IntakeRegistry) scanning(sourceID string, attempt int) (*models.IntakeSource, error) {
	src, err := r.inScan(sourceID, attempt)
	if err != nil {
		return nil, err
	}
	if src.Status != models.IntakeSourceScanning {
		src.ClaimedAt = nil
		src.UpdatedAt = time.Now()
		return nil, errxtrace.Classify(registry.ErrIntakeSourceChanged, errx.Attrs("entity_id", sourceID))
	}
	return src, nil
}

func (r *IntakeRegistry) StoreScan(ctx context.Context, sourceID string, attempt int, scan registry.IntakeScan, now time.Time) error {
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()
	src, err := r.inScan(sourceID, attempt)
	if err != nil {
		return err
	}
	// The files change first, under st.mu, so a reader that sees the source
	// scanned sees its files changed. The memory backend has no rollback: a
	// failure leaves the source in its scan, and the files changed so far
	// keep the change, which a scan makes only once.
	if err := r.updateScannedFiles(ctx, src, scan.Files); err != nil {
		return err
	}
	for cid, cand := range st.candidates {
		if cand.SourceID == sourceID {
			delete(st.candidates, cid)
		}
	}
	if err := st.storeCandidates(src, scan.Candidates, now); err != nil {
		return err
	}
	src.Status = models.IntakeSourceScanned
	src.Warnings = slices.Clone(scan.Warnings)
	src.ScannedAt = new(now)
	src.WaitReason = ""
	src.ErrorCode = ""
	src.UpdatedAt = now
	return nil
}

// updateScannedFiles writes a scan's file updates. The memory file registry
// does not check tags, so Tags need no work here. The caller holds st.mu.
func (r *IntakeRegistry) updateScannedFiles(ctx context.Context, src *models.IntakeSource, updates []registry.IntakeFileUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	files := r.factory.fileFactory.CreateServiceRegistry()
	for _, u := range updates {
		file, err := files.Get(ctx, u.FileID)
		if errors.Is(err, registry.ErrNotFound) {
			continue
		}
		if err != nil {
			return errxtrace.Wrap("failed to read a scanned file", err, errx.Attrs("file_id", u.FileID))
		}
		if file.TenantID != src.TenantID || file.GroupID != src.GroupID || file.IsTrashed() {
			continue
		}
		next, changed := u.Apply(*file)
		if !changed {
			continue
		}
		stored := *file
		stored.Title = next.Title
		stored.Tags = next.Tags
		stored.UpdatedAt = next.UpdatedAt
		_, err = files.Update(ctx, stored)
		if errors.Is(err, registry.ErrNotFound) {
			continue
		}
		if err != nil {
			return errxtrace.Wrap("failed to update a scanned file", err, errx.Attrs("file_id", u.FileID))
		}
	}
	return nil
}

// storeCandidates stores the products found in src, pending and in the given
// order. The caller holds st.mu.
func (st *intakeState) storeCandidates(src *models.IntakeSource, candidates []models.IntakeCandidate, now time.Time) error {
	for i, cand := range candidates {
		fields, err := cloneIntakeFields(cand.Fields)
		if err != nil {
			return errxtrace.Wrap("failed to copy candidate fields", err)
		}
		cand.TenantGroupAwareEntityID = models.TenantGroupAwareEntityID{
			EntityID:        models.EntityID{ID: uuid.NewString(), UUID: uuid.NewString()},
			TenantID:        src.TenantID,
			GroupID:         src.GroupID,
			CreatedByUserID: src.CreatedByUserID,
		}
		cand.SessionID = src.SessionID
		cand.SourceID = src.ID
		cand.SortOrder = i
		cand.Fields = fields
		cand.AcceptedFields = slices.Clone(cand.AcceptedFields)
		if cand.Quantity < 1 {
			cand.Quantity = 1
		}
		cand.Outcome = models.IntakeCandidatePending
		cand.DraftID = nil
		cand.ErrorCode = ""
		cand.CreatedAt = now
		cand.UpdatedAt = now
		st.candidates[cand.ID] = new(cand)
	}
	return nil
}

func (r *IntakeRegistry) RequeueSource(_ context.Context, sourceID string, attempt int, next time.Time, waitReason string) error {
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()
	src, err := r.scanning(sourceID, attempt)
	if err != nil {
		return err
	}
	src.Status = models.IntakeSourcePending
	src.NextAttemptAt = next
	src.ClaimedAt = nil
	src.WaitReason = waitReason
	if waitReason == models.IntakeWaitRateLimited && src.RetryBase < src.Attempts {
		src.RetryBase++
	}
	src.UpdatedAt = time.Now()
	return nil
}

func (r *IntakeRegistry) FailSource(_ context.Context, sourceID string, attempt int, errorCode string) error {
	st := r.factory.state
	st.mu.Lock()
	defer st.mu.Unlock()
	src, err := r.scanning(sourceID, attempt)
	if err != nil {
		return err
	}
	src.Status = models.IntakeSourceFailed
	src.ClaimedAt = nil
	src.WaitReason = ""
	src.ErrorCode = errorCode
	src.UpdatedAt = time.Now()
	return nil
}
