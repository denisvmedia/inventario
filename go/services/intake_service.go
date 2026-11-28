package services

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/internal/mimekit"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// Defaults of IntakeConfig.
const (
	// DefaultIntakeMaxFilesPerBatch caps the files of one drop.
	DefaultIntakeMaxFilesPerBatch = 50
	// DefaultIntakeMaxQueuedPerGroup caps the sources of a group that wait
	// for a scan at once.
	DefaultIntakeMaxQueuedPerGroup = 200
)

// ErrIntakeQueueFull is returned when a new batch would put more sources in
// the group's scan queue than IntakeConfig.MaxQueuedPerGroup allows. It is
// the registry's sentinel, which checks the limit atomically.
var ErrIntakeQueueFull = registry.ErrIntakeQueueFull

// ErrIntakeSourceDelivered is returned when a rescan reaches a source another
// system delivered (#2809): it carries the values that system sent, and no
// scan runs for it.
var ErrIntakeSourceDelivered = errx.NewSentinel("the intake source was delivered by another system")

// IntakeConfig carries the limits of batch intake. The per-purchase file
// count, the file size and the accepted types come from the scan service,
// since every source is one scan.
type IntakeConfig struct {
	MaxFilesPerBatch  int
	MaxQueuedPerGroup int
}

// IntakeLimits are the limits a client shows and checks before uploading.
type IntakeLimits struct {
	Enabled           bool `json:"enabled"`
	MaxFilesPerBatch  int  `json:"max_files_per_batch"`
	MaxFilesPerSource int  `json:"max_files_per_source"`
	MaxFileBytes      int  `json:"max_file_bytes"`
	MaxQueuedPerGroup int  `json:"max_queued_per_group"`
	Queued            int  `json:"queued"`
	RateLimitPerHour  int  `json:"rate_limit_per_hour"`
	// AcceptedMIMETypes are the types a file can be uploaded as and scanned
	// in: the scan reads HEIC photos, but the upload does not take them.
	AcceptedMIMETypes []string `json:"accepted_mime_types"`
}

// IntakeSourceRequest is one purchase of a new batch.
type IntakeSourceRequest struct {
	FileIDs []string
	OneItem bool
}

// IntakeSourceDetail is a source with its live files.
type IntakeSourceDetail struct {
	Source *models.IntakeSource
	Files  []*models.FileEntity
}

// IntakeSessionDetail is a session with everything a review page shows.
type IntakeSessionDetail struct {
	Session    *models.IntakeSession
	Summary    models.IntakeSessionSummary
	Sources    []IntakeSourceDetail
	Candidates []*models.IntakeCandidate
}

// IntakeSessionListItem is a session with its counts, for the sessions list.
type IntakeSessionListItem struct {
	Session *models.IntakeSession
	Summary models.IntakeSessionSummary
}

// IntakeDraftsResult is the outcome of IntakeService.CreateDrafts.
type IntakeDraftsResult struct {
	// Created counts the drafts this call made or found made, Skipped the
	// products left out, Failed the products whose draft could not be made.
	Created int
	Skipped int
	Failed  int
	Detail  *IntakeSessionDetail
}

// IntakeService runs batch intake (#2810): a member drops files, the
// background worker scans each purchase once, the member keeps some of the
// detected products, and every kept product gets exactly one draft (#2812)
// that carries the source files as links (#2806). Everything is stored, so a
// review can continue on another device, a lost response can be retried, and
// reopening a session never calls the AI provider again.
type IntakeService struct {
	factorySet *registry.FactorySet
	scan       *CommodityScanService
	drafts     *CommodityDraftService
	cfg        IntakeConfig
	now        func() time.Time
}

// NewIntakeService builds the service. scan may be nil or have no provider;
// creating a session then fails with ErrScanProviderDisabled.
func NewIntakeService(factorySet *registry.FactorySet, scan *CommodityScanService, cfg IntakeConfig) *IntakeService {
	if cfg.MaxFilesPerBatch <= 0 {
		cfg.MaxFilesPerBatch = DefaultIntakeMaxFilesPerBatch
	}
	if cfg.MaxQueuedPerGroup <= 0 {
		cfg.MaxQueuedPerGroup = DefaultIntakeMaxQueuedPerGroup
	}
	return &IntakeService{
		factorySet: factorySet,
		scan:       scan,
		drafts:     NewCommodityDraftService(factorySet),
		cfg:        cfg,
		now:        time.Now,
	}
}

func (s *IntakeService) intake(ctx context.Context) (registry.IntakeRegistry, error) {
	reg, err := s.factorySet.IntakeRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create intake registry", err)
	}
	return reg, nil
}

func (s *IntakeService) files(ctx context.Context) (registry.FileRegistry, error) {
	reg, err := s.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create file registry", err)
	}
	return reg, nil
}

// Limits returns the limits of a new batch and how many of the group's
// sources wait for a scan now.
func (s *IntakeService) Limits(ctx context.Context) (IntakeLimits, error) {
	cfg := s.scan.Config()
	limits := IntakeLimits{
		Enabled:           s.scan.Enabled(),
		MaxFilesPerBatch:  s.cfg.MaxFilesPerBatch,
		MaxFilesPerSource: cfg.MaxPhotos,
		MaxFileBytes:      cfg.MaxPhotoBytes,
		MaxQueuedPerGroup: s.cfg.MaxQueuedPerGroup,
		RateLimitPerHour:  cfg.RateLimitPerHour,
	}
	limits.AcceptedMIMETypes = IntakeAcceptedMIMETypes()
	reg, err := s.intake(ctx)
	if err != nil {
		return limits, err
	}
	if limits.Queued, err = reg.CountQueued(ctx); err != nil {
		return limits, err
	}
	return limits, nil
}

// IntakeAcceptedMIMETypes returns, sorted, the types a batch file can have:
// the ones the upload takes (mimekit.AllContentTypes) that the scan reads
// (AllowedMIMETypes).
func IntakeAcceptedMIMETypes() []string {
	var out []string
	for _, mime := range mimekit.AllContentTypes() {
		if AllowedMIMETypes[mime] && !slices.Contains(out, mime) {
			out = append(out, mime)
		}
	}
	slices.Sort(out)
	return out
}

// CreateSession stores a new batch and queues its sources for the worker.
// clientKey is the client's identity for the batch: a key the group already
// has returns that session with created=false and nothing else changes, so
// a retry after a lost response does not queue the files twice. The replay
// does not compare the sources: a client that reuses a key for other files
// gets the first batch back, so a client takes a new key for every batch.
//
// Every file must be a live file of the group, of an accepted type and
// within the size limit; a file may appear once in a batch. Validation
// problems are a validation.Errors keyed by the request field. A batch that
// does not fit in the group's scan queue fails with ErrIntakeQueueFull; the
// registry counts the queue and stores the batch atomically.
func (s *IntakeService) CreateSession(ctx context.Context, clientKey string, sources []IntakeSourceRequest) (*IntakeSessionDetail, bool, error) {
	reg, err := s.intake(ctx)
	if err != nil {
		return nil, false, err
	}
	session := models.IntakeSession{ClientKey: clientKey, Origin: models.IntakeOriginUpload}
	if err := validation.Validate(clientKey, validation.Required, validation.RuneLength(1, models.IntakeClientKeyMaxLength),
		models.MemberSessionClientKeyRule); err != nil {
		return nil, false, validation.Errors{"client_key": err}
	}
	if existing, err := reg.GetSessionByClientKey(ctx, clientKey); err == nil {
		detail, err := s.detail(ctx, reg, existing)
		return detail, false, err
	} else if !errors.Is(err, registry.ErrNotFound) {
		return nil, false, err
	}

	if !s.scan.Enabled() {
		return nil, false, errxtrace.Classify(ErrScanProviderDisabled)
	}
	inputs, err := s.checkSources(ctx, sources)
	if err != nil {
		return nil, false, err
	}

	stored, created, err := reg.CreateSession(ctx, session, inputs, s.cfg.MaxQueuedPerGroup)
	if err != nil {
		return nil, false, err
	}
	detail, err := s.detail(ctx, reg, stored)
	return detail, created, err
}

// checkSources validates the sources of a new batch against the limits and
// the group's files. The shape of the request is checked before any file is
// read, so an oversized batch costs no lookups.
func (s *IntakeService) checkSources(ctx context.Context, sources []IntakeSourceRequest) ([]registry.IntakeSourceInput, error) {
	cfg := s.scan.Config()
	if len(sources) == 0 {
		return nil, validation.Errors{"sources": validation.NewError("validation_intake_no_sources", "add at least one file")}
	}
	seen := map[string]bool{}
	for _, src := range sources {
		if len(src.FileIDs) == 0 {
			return nil, validation.Errors{"sources": validation.NewError("validation_intake_empty_source", "every purchase needs at least one file")}
		}
		if cfg.MaxPhotos > 0 && len(src.FileIDs) > cfg.MaxPhotos {
			return nil, validation.Errors{"sources": validation.NewError("validation_intake_too_many_source_files",
				"too many files for one item").SetParams(map[string]any{"max": cfg.MaxPhotos})}
		}
		for _, id := range src.FileIDs {
			if seen[id] {
				return nil, validation.Errors{"sources": validation.NewError("validation_intake_duplicate_file", "a file is listed twice")}
			}
			seen[id] = true
		}
	}
	if len(seen) > s.cfg.MaxFilesPerBatch {
		return nil, validation.Errors{"sources": validation.NewError("validation_intake_too_many_files",
			"too many files in one batch").SetParams(map[string]any{"max": s.cfg.MaxFilesPerBatch})}
	}

	files, err := s.files(ctx)
	if err != nil {
		return nil, err
	}
	inputs := make([]registry.IntakeSourceInput, 0, len(sources))
	for _, src := range sources {
		for _, id := range src.FileIDs {
			file, err := files.Get(ctx, id)
			if err != nil {
				return nil, err
			}
			if err := checkIntakeFile(file, cfg); err != nil {
				return nil, validation.Errors{"sources": err}
			}
		}
		inputs = append(inputs, registry.IntakeSourceInput{FileIDs: src.FileIDs, OneItem: src.OneItem})
	}
	return inputs, nil
}

// checkIntakeFile reports why the scan could not read a file, or nil.
func checkIntakeFile(file *models.FileEntity, cfg CommodityScanConfig) error {
	if file.File == nil || !AllowedMIMETypes[file.MIMEType] {
		return validation.NewError("validation_intake_unsupported_file", "the file type cannot be scanned")
	}
	if cfg.MaxPhotoBytes > 0 && file.SizeBytes > int64(cfg.MaxPhotoBytes) {
		return validation.NewError("validation_intake_file_too_large", "the file is too large to scan").
			SetParams(map[string]any{"max": cfg.MaxPhotoBytes})
	}
	return nil
}

// ListSessions returns a page of the group's sessions with their counts,
// newest first.
func (s *IntakeService) ListSessions(ctx context.Context, offset, limit int) ([]IntakeSessionListItem, int, error) {
	reg, err := s.intake(ctx)
	if err != nil {
		return nil, 0, err
	}
	sessions, total, err := reg.ListSessions(ctx, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]string, 0, len(sessions))
	for _, sess := range sessions {
		ids = append(ids, sess.ID)
	}
	sources, err := reg.ListSources(ctx, ids...)
	if err != nil {
		return nil, 0, err
	}
	candidates, err := reg.ListCandidates(ctx, ids...)
	if err != nil {
		return nil, 0, err
	}
	sourceIDs := make([]string, 0, len(sources))
	for _, src := range sources {
		sourceIDs = append(sourceIDs, src.ID)
	}
	fileCounts := map[string]int{}
	if len(sourceIDs) > 0 {
		files, err := s.files(ctx)
		if err != nil {
			return nil, 0, err
		}
		if fileCounts, err = files.CountLinkedTo(ctx, models.FileLinkIntake, sourceIDs); err != nil {
			return nil, 0, err
		}
	}
	summaries := map[string]*models.IntakeSessionSummary{}
	for _, id := range ids {
		summaries[id] = &models.IntakeSessionSummary{}
	}
	for _, src := range sources {
		summaries[src.SessionID].AddSource(src, fileCounts[src.ID])
	}
	for _, cand := range candidates {
		summaries[cand.SessionID].AddCandidate(cand)
	}
	out := make([]IntakeSessionListItem, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, IntakeSessionListItem{Session: sess, Summary: *summaries[sess.ID]})
	}
	return out, total, nil
}

// GetSession returns one session with its sources, files and candidates.
func (s *IntakeService) GetSession(ctx context.Context, id string) (*IntakeSessionDetail, error) {
	reg, err := s.intake(ctx)
	if err != nil {
		return nil, err
	}
	session, err := reg.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, reg, session)
}

func (s *IntakeService) detail(ctx context.Context, reg registry.IntakeRegistry, session *models.IntakeSession) (*IntakeSessionDetail, error) {
	sources, err := reg.ListSources(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	candidates, err := reg.ListCandidates(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	files, err := s.files(ctx)
	if err != nil {
		return nil, err
	}
	detail := &IntakeSessionDetail{Session: session, Candidates: candidates}
	for _, src := range sources {
		linked, err := files.ListByLinkedEntity(ctx, models.FileLinkIntake, src.ID)
		if err != nil {
			return nil, err
		}
		detail.Summary.AddSource(src, len(linked))
		detail.Sources = append(detail.Sources, IntakeSourceDetail{Source: src, Files: linked})
	}
	for _, cand := range candidates {
		detail.Summary.AddCandidate(cand)
	}
	return detail, nil
}

// sessionCandidate returns the candidate when it belongs to the session.
func (s *IntakeService) sessionCandidate(ctx context.Context, reg registry.IntakeRegistry, sessionID, candidateID string) (*models.IntakeCandidate, error) {
	cand, err := reg.GetCandidate(ctx, candidateID)
	if err != nil {
		return nil, err
	}
	if cand.SessionID != sessionID {
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "intake_candidate", "entity_id", candidateID))
	}
	return cand, nil
}

// ReviewCandidate stores a member's review of a detected product: whether
// to keep it, which scanned values go into its draft, and how many units it
// covers.
func (s *IntakeService) ReviewCandidate(ctx context.Context, sessionID, candidateID string, review registry.IntakeCandidateReview) (*models.IntakeCandidate, error) {
	errs := validation.Errors{}
	if q := review.Quantity; q != nil && (*q < 1 || *q > models.IntakeCandidateQuantityMax) {
		errs["quantity"] = validation.NewError("validation_intake_quantity", "the quantity must be between 1 and {{.max}}").
			SetParams(map[string]any{"max": models.IntakeCandidateQuantityMax})
	}
	if review.AcceptedFields != nil {
		cleaned, ok := acceptedNames(*review.AcceptedFields, isIntakeReviewField)
		if !ok {
			errs["accepted_fields"] = validation.NewError("validation_intake_unknown_field", "unknown field")
		}
		review.AcceptedFields = &cleaned
	}
	if len(errs) > 0 {
		return nil, errs
	}
	reg, err := s.intake(ctx)
	if err != nil {
		return nil, err
	}
	cand, err := s.sessionCandidate(ctx, reg, sessionID, candidateID)
	if err != nil {
		return nil, err
	}
	if review.AcceptedCustomFields != nil {
		// Only a field the scan read a value for can be accepted.
		cleaned, ok := acceptedNames(*review.AcceptedCustomFields, func(id string) bool {
			_, read := cand.CustomFields[id]
			return read
		})
		if !ok {
			return nil, validation.Errors{
				"accepted_custom_fields": validation.NewError("validation_intake_unknown_field", "unknown field"),
			}
		}
		review.AcceptedCustomFields = &cleaned
	}
	return reg.ReviewCandidate(ctx, candidateID, review)
}

// acceptedNames drops the repeats of names, keeping the first. It reports
// false when known refuses one of them.
func acceptedNames(names []string, known func(string) bool) ([]string, bool) {
	cleaned := []string{}
	seen := map[string]bool{}
	for _, name := range names {
		if !known(name) {
			return cleaned, false
		}
		if !seen[name] {
			seen[name] = true
			cleaned = append(cleaned, name)
		}
	}
	return cleaned, true
}

// CreateDrafts makes one draft for every kept product of the session that
// has none yet, and marks the products left out as skipped. Each draft gets
// the files of its source as links, so one receipt serves every item made
// from it.
//
// Every step can be repeated: the draft of a product is found by the client
// key "intake:<candidate id>", or for a delivered document by the key
// IntakeSource.DraftClientKey gives it, a link that exists is kept, and a
// product that has its draft is not touched again. A second call, from
// another device or after a lost response, therefore makes nothing twice,
// and a call after a partial failure finishes only what is left. A product
// whose draft cannot be made is marked failed with a code and the others go
// on.
func (s *IntakeService) CreateDrafts(ctx context.Context, sessionID string) (*IntakeDraftsResult, error) {
	reg, err := s.intake(ctx)
	if err != nil {
		return nil, err
	}
	session, err := reg.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	sources, err := reg.ListSources(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	candidates, err := reg.ListCandidates(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	set, err := s.factorySet.CreateUserRegistrySet(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create registry set", err)
	}
	sourceFiles := map[string][]*models.FileEntity{}
	scanned := map[string]*models.IntakeSource{}
	for _, src := range sources {
		if src.Status != models.IntakeSourceScanned {
			continue
		}
		scanned[src.ID] = src
		if sourceFiles[src.ID], err = set.FileRegistry.ListByLinkedEntity(ctx, models.FileLinkIntake, src.ID); err != nil {
			return nil, err
		}
	}
	defs, err := set.CustomFieldRegistry.List(ctx)
	if err != nil {
		return nil, err
	}
	input := intakeDraftInput{sellers: set.SellerRegistry, types: set.ItemTypeRegistry, customFields: make(map[string]*models.CustomFieldDefinition, len(defs))}
	for _, d := range defs {
		input.customFields[d.ID] = d
	}

	result := &IntakeDraftsResult{}
	for _, cand := range candidates {
		src := scanned[cand.SourceID]
		if src == nil {
			continue
		}
		if err := s.finishCandidate(ctx, reg, set, input, src, cand, sourceFiles[src.ID], result); err != nil {
			return nil, err
		}
	}
	if result.Detail, err = s.detail(ctx, reg, session); err != nil {
		return nil, err
	}
	return result, nil
}

// finishCandidate marks a product left out as skipped, or makes the draft
// of a kept product that has none, and counts the outcome in result.
//
// A rescan of the source can replace the products while the call runs: the
// rescan is refused once a product has a draft, but it can land between the
// read of the products and the making of a draft. A product that is gone
// when its outcome is stored is left out of result, and the draft this call
// made for it is discarded: the new scan offers the product again, and
// keeping the draft would give it a second one.
func (s *IntakeService) finishCandidate(ctx context.Context, reg registry.IntakeRegistry, set *registry.Set, input intakeDraftInput,
	src *models.IntakeSource, cand *models.IntakeCandidate, files []*models.FileEntity, result *IntakeDraftsResult,
) error {
	if !cand.Selected && cand.Outcome != models.IntakeCandidateCreated {
		if cand.Outcome != models.IntakeCandidateSkipped {
			if _, err := reg.SetCandidateOutcome(ctx, cand.ID, models.IntakeCandidateSkipped, nil, ""); err != nil {
				return ignoreReplaced(err)
			}
		}
		result.Skipped++
		return nil
	}
	if !cand.IsUnfinished() {
		return nil
	}
	made, err := s.createDraft(ctx, set, input, src.DraftClientKey(cand), cand, files)
	if err != nil {
		return err
	}
	if made.failure != "" {
		if _, err := reg.SetCandidateOutcome(ctx, cand.ID, models.IntakeCandidateFailed, nil, made.failure); err != nil {
			return ignoreReplaced(err)
		}
		result.Failed++
		return nil
	}
	_, err = reg.SetCandidateOutcome(ctx, cand.ID, models.IntakeCandidateCreated, &made.id, "")
	switch {
	case errors.Is(err, registry.ErrNotFound):
		if !made.created {
			// Another call made the draft and discards it in turn.
			return nil
		}
		if err := s.drafts.Discard(ctx, made.id); err != nil && !errors.Is(err, registry.ErrDraftPromoted) {
			return errxtrace.Wrap("failed to discard the draft of a replaced product", err)
		}
		return nil
	case err != nil:
		return err
	}
	result.Created++
	return nil
}

// ignoreReplaced drops the not-found error of a product that a rescan
// replaced since it was read.
func ignoreReplaced(err error) error {
	if errors.Is(err, registry.ErrNotFound) {
		return nil
	}
	return err
}

// Error codes of a product whose draft could not be made.
const (
	// IntakeDraftDiscarded: a draft with the product's key is being
	// discarded.
	IntakeDraftDiscarded = "draft_discarded"
	// IntakeDraftInvalid: the values the scan read are not valid draft
	// values.
	IntakeDraftInvalid = "invalid_values"
)

// madeDraft is what createDraft did for a candidate.
type madeDraft struct {
	id string
	// created reports that this call stored the draft.
	created bool
	// failure is the code of a problem with this candidate only.
	failure string
}

// createDraft makes or finds the draft of a candidate and links the source
// files to it. It returns a failure code for a problem with this candidate
// only, and an error for one that stops the whole call.
func (s *IntakeService) createDraft(ctx context.Context, set *registry.Set, input intakeDraftInput, key string, cand *models.IntakeCandidate, files []*models.FileEntity) (madeDraft, error) {
	values, err := intakeDraftValues(ctx, input, cand)
	if err != nil {
		return madeDraft{}, err
	}
	draft, created, err := s.drafts.Create(ctx, key, values)
	switch {
	case errors.Is(err, registry.ErrDraftDiscarded):
		return madeDraft{failure: IntakeDraftDiscarded}, nil
	case err != nil:
		if _, ok := errors.AsType[validation.Errors](err); ok {
			return madeDraft{failure: IntakeDraftInvalid}, nil
		}
		return madeDraft{}, err
	}
	for _, file := range files {
		meta := models.DefaultFileLinkMeta(models.FileLinkDraft, file.Category)
		_, _, attachErr := set.CommodityDraftRegistry.AttachFile(ctx, draft.ID, models.FileLink{FileID: file.ID, Meta: meta})
		switch {
		case attachErr == nil:
		case errors.Is(attachErr, registry.ErrDraftPromoted):
			// The draft became an item on another device after an earlier
			// attempt made it; the item gets the file instead.
			if linkErr := s.linkToPromoted(ctx, set, draft.ID, file); linkErr != nil {
				return madeDraft{}, linkErr
			}
		case errors.Is(attachErr, registry.ErrDraftDiscarded):
			return madeDraft{failure: IntakeDraftDiscarded}, nil
		case errors.Is(attachErr, registry.ErrNotFound):
			// The file was deleted since the scan; the draft goes on
			// without it.
		default:
			return madeDraft{}, attachErr
		}
	}
	return madeDraft{id: draft.ID, created: created}, nil
}

func (s *IntakeService) linkToPromoted(ctx context.Context, set *registry.Set, draftID string, file *models.FileEntity) error {
	draft, err := set.CommodityDraftRegistry.Get(ctx, draftID)
	if err != nil || draft.PromotedCommodityID == nil {
		return err
	}
	_, _, err = set.FileRegistry.AddLink(ctx, models.FileLink{
		FileID:     file.ID,
		EntityType: models.FileLinkCommodity,
		EntityID:   *draft.PromotedCommodityID,
		Meta:       models.DefaultFileLinkMeta(models.FileLinkCommodity, file.Category),
	})
	if errors.Is(err, registry.ErrNotFound) {
		return nil
	}
	return err
}

// Cancel stops the session's queued scans. Products already detected and
// drafts already made stay; a scan that is running still lands.
func (s *IntakeService) Cancel(ctx context.Context, sessionID string) (*IntakeSessionDetail, error) {
	reg, err := s.intake(ctx)
	if err != nil {
		return nil, err
	}
	session, err := reg.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if _, err := reg.CancelSession(ctx, session.ID); err != nil {
		return nil, err
	}
	return s.detail(ctx, reg, session)
}

// Resume queues the session's canceled and failed sources again. Scanned
// sources are not scanned again.
func (s *IntakeService) Resume(ctx context.Context, sessionID string) (*IntakeSessionDetail, error) {
	if !s.scan.Enabled() {
		return nil, errxtrace.Classify(ErrScanProviderDisabled)
	}
	reg, err := s.intake(ctx)
	if err != nil {
		return nil, err
	}
	session, err := reg.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if _, err := reg.ResumeSession(ctx, session.ID, s.now()); err != nil {
		return nil, err
	}
	return s.detail(ctx, reg, session)
}

// Rescan queues one source for a new scan on the member's request. Its
// detected products are replaced; a source with a product that has its draft
// cannot be rescanned. A draft that a stopped CreateDrafts made without
// recording it is recorded first, so the refusal leaves the session showing
// the draft.
func (s *IntakeService) Rescan(ctx context.Context, sessionID, sourceID string) (*IntakeSessionDetail, error) {
	if !s.scan.Enabled() {
		return nil, errxtrace.Classify(ErrScanProviderDisabled)
	}
	reg, err := s.intake(ctx)
	if err != nil {
		return nil, err
	}
	session, err := reg.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	src, err := reg.GetSource(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	if src.SessionID != session.ID {
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "intake_source", "entity_id", sourceID))
	}
	if session.Origin == models.IntakeOriginExternal {
		// Another system delivered the product with its values; there is
		// nothing to scan.
		return nil, errxtrace.Classify(ErrIntakeSourceDelivered, errx.Attrs("entity_id", sourceID))
	}
	if err := s.recordMadeDrafts(ctx, reg, session.ID, src.ID); err != nil {
		return nil, err
	}
	if _, err := reg.RescanSource(ctx, src.ID, s.now()); err != nil {
		return nil, err
	}
	return s.detail(ctx, reg, session)
}

// recordMadeDrafts marks created the candidates of the source whose draft
// exists although the outcome was never recorded: CreateDrafts made the
// draft and stopped before it wrote the outcome.
func (s *IntakeService) recordMadeDrafts(ctx context.Context, reg registry.IntakeRegistry, sessionID, sourceID string) error {
	candidates, err := reg.ListCandidates(ctx, sessionID)
	if err != nil {
		return err
	}
	drafts, err := s.factorySet.CommodityDraftRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create draft registry", err)
	}
	for _, cand := range candidates {
		if cand.SourceID != sourceID || cand.Outcome == models.IntakeCandidateCreated {
			continue
		}
		draft, err := drafts.GetByClientKey(ctx, cand.DraftClientKey())
		if errors.Is(err, registry.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if draft.Status == models.CommodityDraftDiscarding {
			continue
		}
		if _, err := reg.SetCandidateOutcome(ctx, cand.ID, models.IntakeCandidateCreated, &draft.ID, ""); err != nil {
			return err
		}
	}
	return nil
}

// Delete deletes the session. Drafts made from it stay. Its files that
// nothing else uses go to Trash, where they can be restored until the
// retention runs out; a file a draft or an item uses only loses the link to
// the session.
func (s *IntakeService) Delete(ctx context.Context, sessionID string) error {
	reg, err := s.intake(ctx)
	if err != nil {
		return err
	}
	session, err := reg.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	userID, err := actorID(ctx)
	if err != nil {
		return err
	}
	sources, err := reg.ListSources(ctx, session.ID)
	if err != nil {
		return err
	}
	files, err := s.files(ctx)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	for _, src := range sources {
		linked, err := files.ListByLinkedEntity(ctx, models.FileLinkIntake, src.ID)
		if err != nil {
			return err
		}
		for _, file := range linked {
			if _, err := files.TrashUnlessHeld(ctx, file.ID, models.FileLinkIntake, src.ID, userID, now); err != nil {
				return errxtrace.Wrap("failed to move intake file to Trash", err, errx.Attrs("file_id", file.ID))
			}
		}
	}
	return reg.Delete(ctx, session.ID)
}
