package services_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/jellydator/validation"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/internal/aivision"
	"go.5x5.cz/inventario/internal/aivision/mock"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

// scriptedProvider is the mock AI provider with a call counter and a queue
// of errors returned before it answers. No test calls a real provider.
type scriptedProvider struct {
	mu    sync.Mutex
	inner *mock.Provider
	calls int
	hints []string
	errs  []error
}

func newScriptedProvider() *scriptedProvider {
	return &scriptedProvider{inner: mock.New()}
}

func (*scriptedProvider) Name() string  { return mock.Name }
func (*scriptedProvider) Model() string { return "mock" }

func (p *scriptedProvider) Scan(ctx context.Context, req aivision.ScanRequest) (*aivision.ScanResult, error) {
	p.mu.Lock()
	p.calls++
	p.hints = append(p.hints, req.HintFromUser)
	var err error
	if len(p.errs) > 0 {
		err, p.errs = p.errs[0], p.errs[1:]
	}
	p.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return p.inner.Scan(ctx, req)
}

func (p *scriptedProvider) failNext(errs ...error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.errs = append(p.errs, errs...)
}

func (p *scriptedProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

type intakeFixture struct {
	ctx            context.Context
	factorySet     *registry.FactorySet
	group          *models.LocationGroup
	user           *models.User
	uploadLocation string
	provider       *scriptedProvider
	scan           *services.CommodityScanService
	intake         *services.IntakeService
	worker         *services.IntakeWorker
	// ahead moves the worker's clock forward from the wall clock.
	ahead time.Duration
}

func newIntakeFixture(c *qt.C) *intakeFixture {
	return newIntakeFixtureWith(c, services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20, RateLimitPerHour: 100}, services.IntakeConfig{})
}

func newIntakeFixtureWith(c *qt.C, scanCfg services.CommodityScanConfig, cfg services.IntakeConfig) *intakeFixture {
	c.Helper()
	fs := memory.NewFactorySet()
	f := &intakeFixture{
		factorySet:     fs,
		uploadLocation: newFileUploadLocation(c),
		provider:       newScriptedProvider(),
	}
	f.user = f.member(c, "owner@example.com", nil)
	f.group = must.Must(fs.LocationGroupRegistry.Create(context.Background(), models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		Slug:                "intake-group",
		Name:                "Intake",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           f.user.ID,
		GroupCurrency:       "EUR",
	}))
	f.join(c, f.user, models.GroupRoleOwner)
	f.ctx = f.as(f.user)
	f.scan = services.NewCommodityScanService(f.provider, fs.CommodityScanAuditRegistry, scanCfg)
	f.intake = services.NewIntakeService(fs, f.scan, cfg)
	f.worker = services.NewIntakeWorker(fs, f.scan, f.uploadLocation, services.WithIntakeClock(f.clock))
	return f
}

func (f *intakeFixture) clock() time.Time {
	return time.Now().Add(f.ahead)
}

// member creates a user; with a role it also joins the fixture's group.
func (f *intakeFixture) member(c *qt.C, email string, role *models.GroupRole) *models.User {
	c.Helper()
	u := must.Must(f.factorySet.UserRegistry.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		Email:               email,
		Name:                email,
		IsActive:            true,
	}))
	if role != nil {
		f.join(c, u, *role)
	}
	return u
}

func (f *intakeFixture) join(c *qt.C, u *models.User, role models.GroupRole) {
	c.Helper()
	_, err := f.factorySet.GroupMembershipRegistry.Create(context.Background(), models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		GroupID:             f.group.ID,
		MemberUserID:        u.ID,
		Role:                role,
	})
	c.Assert(err, qt.IsNil)
}

// as is the request context of a member, as on another device.
func (f *intakeFixture) as(u *models.User) context.Context {
	return appctx.WithGroup(appctx.WithUser(context.Background(), u), f.group)
}

// upload stores a file of the group with its blob.
func (f *intakeFixture) upload(c *qt.C, name, ext, mime string, size int) *models.FileEntity {
	c.Helper()
	files := must.Must(f.factorySet.FileRegistryFactory.CreateUserRegistry(f.ctx))
	key := "t/tenant-a/files/" + name + ext
	file := must.Must(files.Create(f.ctx, models.FileEntity{
		Title:    name,
		Type:     models.FileTypeFromMIME(mime),
		Category: models.FileCategoryFromMIME(mime),
		Tags:     models.StringSlice{},
		File:     &models.File{Path: name, OriginalPath: key, Ext: ext, MIMEType: mime, SizeBytes: int64(size)},
	}))
	bucket := must.Must(blob.OpenBucket(f.ctx, f.uploadLocation))
	defer bucket.Close()
	c.Assert(bucket.WriteAll(f.ctx, key, make([]byte, size), nil), qt.IsNil)
	return file
}

func (f *intakeFixture) photo(c *qt.C, name string) *models.FileEntity {
	return f.upload(c, name, ".jpg", "image/jpeg", 64)
}

func (f *intakeFixture) create(c *qt.C, key string, sources ...services.IntakeSourceRequest) *services.IntakeSessionDetail {
	c.Helper()
	detail, created, err := f.intake.CreateSession(f.ctx, key, sources)
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)
	return detail
}

func (f *intakeFixture) get(c *qt.C, ctx context.Context, id string) *services.IntakeSessionDetail {
	c.Helper()
	return must.Must(f.intake.GetSession(ctx, id))
}

func source(files ...*models.FileEntity) services.IntakeSourceRequest {
	ids := make([]string, 0, len(files))
	for _, file := range files {
		ids = append(ids, file.ID)
	}
	return services.IntakeSourceRequest{FileIDs: ids}
}

func (f *intakeFixture) drafts(c *qt.C) []*models.CommodityDraft {
	c.Helper()
	reg := must.Must(f.factorySet.CommodityDraftRegistryFactory.CreateUserRegistry(f.ctx))
	return must.Must(reg.List(f.ctx))
}

func candidateNamed(detail *services.IntakeSessionDetail, name string) *models.IntakeCandidate {
	for _, cand := range detail.Candidates {
		if cand.Fields[aivision.FieldNameName].Value == name {
			return cand
		}
	}
	return nil
}

// TestIntake_OneReceiptThreeProducts is the end-to-end path of #2810 with the
// mock provider: one receipt with three products and a photo of a drill are
// dropped together, scanned once, two of the receipt's products are kept, a
// second device finishes the review, a lost response and a partial failure
// are retried, and every kept product ends with exactly one draft that links
// the one receipt.
func TestIntake_OneReceiptThreeProducts(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	receipt := f.photo(c, "receipt-coffee")
	drill := f.photo(c, "drill")

	// Dropped together: two purchases by default.
	session := f.create(c, "batch-1", source(receipt), source(drill))
	c.Assert(session.Sources, qt.HasLen, 2)
	c.Check(session.Summary.State(), qt.Equals, models.IntakeSessionProcessing)
	c.Check(session.Summary.Files, qt.Equals, 2)

	// A retry of the create after a lost response returns the same session.
	again, created, err := f.intake.CreateSession(f.ctx, "batch-1", []services.IntakeSourceRequest{source(receipt), source(drill)})
	c.Assert(err, qt.IsNil)
	c.Check(created, qt.IsFalse)
	c.Check(again.Session.ID, qt.Equals, session.Session.ID)
	c.Check(again.Sources, qt.HasLen, 2)

	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 2)
	c.Check(f.provider.callCount(), qt.Equals, 2)
	c.Check(f.worker.Tick(context.Background()), qt.Equals, 0, qt.Commentf("nothing is scanned twice"))

	detail := f.get(c, f.ctx, session.Session.ID)
	c.Check(detail.Summary.State(), qt.Equals, models.IntakeSessionReview)
	c.Assert(detail.Candidates, qt.HasLen, 4)
	machine := candidateNamed(detail, "Espresso Machine")
	frother := candidateNamed(detail, "Milk Frother")
	cups := candidateNamed(detail, "Espresso Cups")
	c.Assert(machine, qt.IsNotNil)
	c.Assert(frother, qt.IsNotNil)
	c.Assert(cups, qt.IsNotNil)
	c.Check(cups.Quantity, qt.Equals, 2)
	c.Check(machine.SourceID, qt.Equals, frother.SourceID)

	// Reopening the session reads what is stored; the provider is not
	// called again.
	for range 3 {
		f.get(c, f.ctx, session.Session.ID)
	}
	must.Must2(f.intake.ListSessions(f.ctx, 0, 10))
	c.Check(f.provider.callCount(), qt.Equals, 2)

	// The member leaves the frother out on the first device.
	_, err = f.intake.ReviewCandidate(f.ctx, session.Session.ID, frother.ID, registry.IntakeCandidateReview{Selected: new(false)})
	c.Assert(err, qt.IsNil)

	// A second device of another member sees the review and goes on.
	role := models.GroupRoleUser
	other := f.member(c, "partner@example.com", &role)
	device2 := f.as(other)
	seen := f.get(c, device2, session.Session.ID)
	c.Check(candidateNamed(seen, "Milk Frother").Selected, qt.IsFalse)
	c.Check(seen.Summary.Selected, qt.Equals, 3)

	// A draft with the cups' key is being discarded, so the cups fail while
	// the others go through.
	draftReg := must.Must(f.factorySet.CommodityDraftRegistryFactory.CreateUserRegistry(device2))
	blocking, _ := must.Must2(draftReg.CreateOrGet(device2, models.CommodityDraft{ClientKey: cups.DraftClientKey(), Values: models.CommodityDraftValues{}}))
	must.Must(draftReg.MarkDiscarding(device2, blocking.ID))

	first := must.Must(f.intake.CreateDrafts(device2, session.Session.ID))
	c.Check(first.Created, qt.Equals, 2)
	c.Check(first.Skipped, qt.Equals, 1)
	c.Check(first.Failed, qt.Equals, 1)
	c.Check(candidateNamed(first.Detail, "Espresso Cups").ErrorCode, qt.Equals, services.IntakeDraftDiscarded)
	c.Check(first.Detail.Summary.State(), qt.Equals, models.IntakeSessionReview)
	c.Check(first.Detail.Summary.NeedsAttention(), qt.IsTrue)

	// The same call again, as after a lost response: nothing new is made.
	idsBefore := map[string]bool{}
	for _, d := range f.drafts(c) {
		idsBefore[d.ID] = true
	}
	lost := must.Must(f.intake.CreateDrafts(f.ctx, session.Session.ID))
	c.Check(lost.Created, qt.Equals, 0)
	c.Check(lost.Failed, qt.Equals, 1)
	c.Check(f.drafts(c), qt.HasLen, len(idsBefore))

	// The blocking discard finishes; a resume finishes only the cups.
	c.Assert(services.NewCommodityDraftService(f.factorySet).Discard(device2, blocking.ID), qt.IsNil)
	resumed := must.Must(f.intake.CreateDrafts(f.ctx, session.Session.ID))
	c.Check(resumed.Created, qt.Equals, 1)
	c.Check(resumed.Failed, qt.Equals, 0)
	c.Check(resumed.Detail.Summary.State(), qt.Equals, models.IntakeSessionDone)
	c.Check(resumed.Detail.Summary.DraftsCreated, qt.Equals, 3)
	c.Check(resumed.Detail.Summary.Skipped, qt.Equals, 1)

	drafts := f.drafts(c)
	c.Assert(drafts, qt.HasLen, 3, qt.Commentf("exactly one draft per kept product"))
	byName := map[string]*models.CommodityDraft{}
	for _, d := range drafts {
		c.Check(idsBefore[d.ID] || d.ClientKey == cups.DraftClientKey(), qt.IsTrue, qt.Commentf("draft %s was made twice", d.ID))
		byName[d.Values.Title()] = d
	}
	c.Assert(byName["Espresso Machine"], qt.IsNotNil)
	c.Assert(byName["Espresso Cups"], qt.IsNotNil)
	c.Check(byName["Milk Frother"], qt.IsNil)

	// Line amounts, never a total; a count for identical units.
	c.Check(byName["Espresso Machine"].Values["original_price"], qt.Equals, "449.00")
	c.Check(byName["Espresso Machine"].Values["original_price_currency"], qt.Equals, "EUR")
	c.Check(byName["Espresso Machine"].Values["type"], qt.Equals, "white_goods")
	c.Check(byName["Espresso Cups"].Values["original_price"], qt.Equals, "24.00")
	c.Check(byName["Espresso Cups"].Values["count"], qt.Equals, float64(2))
	// What the scan did not read stays unknown.
	_, hasSerial := byName["Espresso Machine"].Values["serial_number"]
	c.Check(hasSerial, qt.IsFalse)

	// The one receipt is linked to both of its items, not copied.
	files := must.Must(f.factorySet.FileRegistryFactory.CreateUserRegistry(f.ctx))
	linked := must.Must(files.Get(f.ctx, receipt.ID))
	c.Check(linked.LinkTo(models.FileLinkDraft, byName["Espresso Machine"].ID), qt.IsNotNil)
	c.Check(linked.LinkTo(models.FileLinkDraft, byName["Espresso Cups"].ID), qt.IsNotNil)
	c.Check(must.Must(files.List(f.ctx)), qt.HasLen, 2)
	drillFile := must.Must(files.Get(f.ctx, drill.ID))
	c.Check(drillFile.Links, qt.HasLen, 2, qt.Commentf("the intake source and the drill's draft"))

	// Deleting the session keeps the drafts and their files.
	c.Assert(f.intake.Delete(f.ctx, session.Session.ID), qt.IsNil)
	c.Check(f.drafts(c), qt.HasLen, 3)
	kept := must.Must(files.Get(f.ctx, receipt.ID))
	c.Check(kept.Links, qt.HasLen, 2)
	c.Check(kept.LinkTo(models.FileLinkIntake, session.Sources[0].Source.ID), qt.IsNil)
	c.Check(f.provider.callCount(), qt.Equals, 2)
}

// TestIntake_OneItemSource: photos of one thing put in one purchase give one
// product even when the scan reads a receipt with several.
func TestIntake_OneItemSource(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	front := f.photo(c, "front")
	box := f.photo(c, "receipt-box")
	src := source(front, box)
	src.OneItem = true
	session := f.create(c, "one-item", src)

	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	detail := f.get(c, f.ctx, session.Session.ID)
	c.Assert(detail.Candidates, qt.HasLen, 1)
	c.Check(detail.Summary.Files, qt.Equals, 2)
	c.Check(f.provider.hints, qt.HasLen, 1)
	c.Check(f.provider.hints[0], qt.Not(qt.Equals), "")

	result := must.Must(f.intake.CreateDrafts(f.ctx, session.Session.ID))
	c.Check(result.Created, qt.Equals, 1)
	draft := f.drafts(c)[0]
	files := must.Must(services.NewCommodityDraftService(f.factorySet).Files(f.ctx, draft.ID))
	c.Check(files, qt.HasLen, 2, qt.Commentf("both photos go with the one item"))
}

// TestIntake_CancelAndResume: cancel stops queued scans and keeps finished
// work; resume scans only what was not scanned.
func TestIntake_CancelAndResume(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	a, b, d := f.photo(c, "lamp"), f.photo(c, "chair"), f.photo(c, "desk")
	session := f.create(c, "cancel", source(a), source(b), source(d))

	// The first purchase is scanned and drafted before the cancel.
	c.Assert(f.scanOne(c), qt.IsTrue)
	first := must.Must(f.intake.CreateDrafts(f.ctx, session.Session.ID))
	c.Assert(first.Created, qt.Equals, 1)

	canceled := must.Must(f.intake.Cancel(f.ctx, session.Session.ID))
	c.Check(canceled.Summary.Canceled, qt.Equals, 2)
	c.Check(canceled.Summary.DraftsCreated, qt.Equals, 1, qt.Commentf("completed drafts stay"))
	c.Check(f.worker.Tick(context.Background()), qt.Equals, 0)
	c.Check(f.provider.callCount(), qt.Equals, 1)
	c.Check(f.drafts(c), qt.HasLen, 1)

	resumed := must.Must(f.intake.Resume(f.ctx, session.Session.ID))
	c.Check(resumed.Summary.Pending, qt.Equals, 2)
	c.Check(f.worker.Tick(context.Background()), qt.Equals, 2)
	c.Check(f.provider.callCount(), qt.Equals, 3, qt.Commentf("the scanned purchase is not scanned again"))
	done := must.Must(f.intake.CreateDrafts(f.ctx, session.Session.ID))
	c.Check(done.Created, qt.Equals, 2)
	c.Check(f.drafts(c), qt.HasLen, 3)
}

// scanOne lets a worker scan exactly one due source.
func (f *intakeFixture) scanOne(c *qt.C) bool {
	c.Helper()
	one := services.NewIntakeWorker(f.factorySet, f.scan, f.uploadLocation,
		services.WithIntakeClock(f.clock), services.WithIntakeSourcesPerTick(1))
	return one.Tick(context.Background()) == 1
}

// TestIntake_RetryThenFail: a failure that may pass is retried with a delay;
// after the last attempt the purchase fails, and a resume tries again.
func TestIntake_RetryThenFail(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	session := f.create(c, "retry", source(f.photo(c, "kettle")))
	f.provider.failNext(aivision.ErrProviderUnavailable, aivision.ErrProviderUnavailable, aivision.ErrProviderUnavailable)

	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	src := f.get(c, f.ctx, session.Session.ID).Sources[0].Source
	c.Check(src.Status, qt.Equals, models.IntakeSourcePending)
	c.Check(src.WaitReason, qt.Equals, models.IntakeWaitRetry)
	c.Check(src.Attempts, qt.Equals, 1)
	c.Check(f.worker.Tick(context.Background()), qt.Equals, 0, qt.Commentf("the retry waits"))

	for range 2 {
		f.ahead += 10 * time.Minute
		c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	}
	failed := f.get(c, f.ctx, session.Session.ID)
	c.Check(failed.Sources[0].Source.Status, qt.Equals, models.IntakeSourceFailed)
	c.Check(failed.Sources[0].Source.ErrorCode, qt.Equals, services.IntakeErrorProviderUnavailable)
	c.Check(failed.Summary.NeedsAttention(), qt.IsTrue)
	c.Check(failed.Summary.State(), qt.Equals, models.IntakeSessionDone)

	must.Must(f.intake.Resume(f.ctx, session.Session.ID))
	f.ahead = 0
	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	scanned := f.get(c, f.ctx, session.Session.ID)
	c.Check(scanned.Sources[0].Source.Status, qt.Equals, models.IntakeSourceScanned)
	c.Check(scanned.Candidates, qt.HasLen, 1)
	c.Check(f.provider.callCount(), qt.Equals, 4)
}

// TestIntake_RateLimitWaits: the author's hourly scan limit holds sources
// back without counting an attempt and without asking the provider again.
func TestIntake_RateLimitWaits(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixtureWith(c, services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20, RateLimitPerHour: 2}, services.IntakeConfig{})
	session := f.create(c, "limited", source(f.photo(c, "a")), source(f.photo(c, "b")), source(f.photo(c, "c")), source(f.photo(c, "d")))

	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 4, qt.Commentf("two scans, then two waits"))
	c.Check(f.provider.callCount(), qt.Equals, 2)
	detail := f.get(c, f.ctx, session.Session.ID)
	c.Check(detail.Summary.Scanned, qt.Equals, 2)
	c.Check(detail.Summary.RateLimited, qt.Equals, 2)
	for _, src := range detail.Sources {
		if src.Source.Status == models.IntakeSourcePending {
			c.Check(src.Source.Attempts, qt.Equals, 0)
			c.Check(src.Source.WaitReason, qt.Equals, models.IntakeWaitRateLimited)
		}
	}

	// Later, while the limit still holds, the worker does not ask again.
	f.ahead += 6 * time.Minute
	c.Assert(f.worker.Tick(context.Background()) >= 1, qt.IsTrue)
	c.Check(f.provider.callCount(), qt.Equals, 2)
	c.Check(f.get(c, f.ctx, session.Session.ID).Summary.RateLimited, qt.Equals, 2)
}

// TestIntake_Rescan: a rescan on request replaces the detected products of
// one purchase, and is refused once a product has its draft.
func TestIntake_Rescan(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	session := f.create(c, "rescan", source(f.photo(c, "receipt-a")))
	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	sourceID := session.Sources[0].Source.ID
	before := f.get(c, f.ctx, session.Session.ID).Candidates

	queued := must.Must(f.intake.Rescan(f.ctx, session.Session.ID, sourceID))
	c.Check(queued.Candidates, qt.HasLen, 0)
	c.Check(queued.Summary.Pending, qt.Equals, 1)
	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	after := f.get(c, f.ctx, session.Session.ID).Candidates
	c.Check(after, qt.HasLen, len(before))
	c.Check(after[0].ID, qt.Not(qt.Equals), before[0].ID)
	c.Check(f.provider.callCount(), qt.Equals, 2)

	must.Must(f.intake.CreateDrafts(f.ctx, session.Session.ID))
	_, err := f.intake.Rescan(f.ctx, session.Session.ID, sourceID)
	c.Check(err, qt.ErrorIs, registry.ErrIntakeSourceHasDrafts)

	_, err = f.intake.Rescan(f.ctx, session.Session.ID, "unknown")
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestIntake_ReviewCandidate(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	session := f.create(c, "review", source(f.photo(c, "receipt-r")))
	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	machine := candidateNamed(f.get(c, f.ctx, session.Session.ID), "Espresso Machine")
	c.Check(machine.Accepts(aivision.FieldNameName), qt.IsTrue)

	got := must.Must(f.intake.ReviewCandidate(f.ctx, session.Session.ID, machine.ID, registry.IntakeCandidateReview{
		AcceptedFields: &[]string{aivision.FieldNameName, aivision.FieldNameName, aivision.FieldNameBrand},
		Quantity:       new(3),
	}))
	c.Check([]string(got.AcceptedFields), qt.DeepEquals, []string{aivision.FieldNameName, aivision.FieldNameBrand})
	c.Check(got.Quantity, qt.Equals, 3)

	_, err := f.intake.ReviewCandidate(f.ctx, session.Session.ID, machine.ID, registry.IntakeCandidateReview{Quantity: new(0)})
	c.Check(err, qt.ErrorAs, new(validation.Errors))
	_, err = f.intake.ReviewCandidate(f.ctx, session.Session.ID, machine.ID, registry.IntakeCandidateReview{AcceptedFields: &[]string{"document_type"}})
	c.Check(err, qt.ErrorAs, new(validation.Errors))
	_, err = f.intake.ReviewCandidate(f.ctx, "other-session", machine.ID, registry.IntakeCandidateReview{Selected: new(false)})
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)

	result := must.Must(f.intake.CreateDrafts(f.ctx, session.Session.ID))
	c.Assert(result.Created, qt.Equals, 3)
	var draft *models.CommodityDraft
	for _, d := range f.drafts(c) {
		if d.ClientKey == machine.DraftClientKey() {
			draft = d
		}
	}
	c.Assert(draft, qt.IsNotNil)
	c.Check(draft.Values, qt.DeepEquals, models.CommodityDraftValues{
		"name":  "Espresso Machine",
		"brand": "Sample Brew",
		"count": float64(3),
	}, qt.Commentf("only the accepted fields go into the draft"))

	_, err = f.intake.ReviewCandidate(f.ctx, session.Session.ID, machine.ID, registry.IntakeCandidateReview{Selected: new(false)})
	c.Check(err, qt.ErrorIs, registry.ErrIntakeCandidateFinished)
}

func TestIntake_CreateSession_Validation(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixtureWith(c, services.CommodityScanConfig{MaxPhotos: 2, MaxPhotoBytes: 100, RateLimitPerHour: 100},
		services.IntakeConfig{MaxFilesPerBatch: 3, MaxQueuedPerGroup: 4})
	ok := f.photo(c, "ok")
	text := f.upload(c, "notes", ".txt", "text/plain", 10)
	big := f.upload(c, "big", ".jpg", "image/jpeg", 101)
	p1, p2, p3 := f.photo(c, "p1"), f.photo(c, "p2"), f.photo(c, "p3")

	tests := []struct {
		name    string
		key     string
		sources []services.IntakeSourceRequest
		code    string
	}{
		{name: "no client key", key: "", sources: []services.IntakeSourceRequest{source(ok)}},
		{name: "no sources", key: "k1", code: "validation_intake_no_sources"},
		{name: "empty source", key: "k2", sources: []services.IntakeSourceRequest{{}}, code: "validation_intake_empty_source"},
		{name: "file twice", key: "k3", sources: []services.IntakeSourceRequest{source(ok), source(ok)}, code: "validation_intake_duplicate_file"},
		{name: "unsupported type", key: "k4", sources: []services.IntakeSourceRequest{source(text)}, code: "validation_intake_unsupported_file"},
		{name: "too large", key: "k5", sources: []services.IntakeSourceRequest{source(big)}, code: "validation_intake_file_too_large"},
		{name: "too many files for one item", key: "k6", sources: []services.IntakeSourceRequest{source(ok, p1, p2)}, code: "validation_intake_too_many_source_files"},
		{name: "too many files in a batch", key: "k7", sources: []services.IntakeSourceRequest{source(ok), source(p1), source(p2), source(p3)}, code: "validation_intake_too_many_files"},
	}
	for _, tc := range tests {
		c.Run(tc.name, func(c *qt.C) {
			_, _, err := f.intake.CreateSession(f.ctx, tc.key, tc.sources)
			errs, isValidation := errorsAsValidation(err)
			c.Assert(isValidation, qt.IsTrue, qt.Commentf("got %v", err))
			if tc.code == "" {
				return
			}
			verr, isCoded := errs["sources"].(validation.Error)
			c.Assert(isCoded, qt.IsTrue, qt.Commentf("got %v", errs))
			c.Check(verr.Code(), qt.Equals, tc.code)
		})
	}

	_, _, err := f.intake.CreateSession(f.ctx, "unknown-file", []services.IntakeSourceRequest{{FileIDs: []string{"missing"}}})
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)

	// The queue holds four; three more do not fit after two.
	f.create(c, "q1", source(ok), source(p1))
	_, _, err = f.intake.CreateSession(f.ctx, "q2", []services.IntakeSourceRequest{source(p2), source(p3), source(f.photo(c, "p4"))})
	c.Check(err, qt.ErrorIs, services.ErrIntakeQueueFull)
	limits := must.Must(f.intake.Limits(f.ctx))
	c.Check(limits.Queued, qt.Equals, 2)
	c.Check(limits.MaxQueuedPerGroup, qt.Equals, 4)
	c.Check(limits.MaxFilesPerSource, qt.Equals, 2)
	c.Check(limits.Enabled, qt.IsTrue)
}

func errorsAsValidation(err error) (validation.Errors, bool) {
	return errors.AsType[validation.Errors](err)
}

func TestIntake_ProviderDisabled(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	disabled := services.NewIntakeService(f.factorySet, services.NewCommodityScanService(nil, f.factorySet.CommodityScanAuditRegistry, services.CommodityScanConfig{}), services.IntakeConfig{})
	_, _, err := disabled.CreateSession(f.ctx, "off", []services.IntakeSourceRequest{source(f.photo(c, "x"))})
	c.Check(err, qt.ErrorIs, services.ErrScanProviderDisabled)
	limits := must.Must(disabled.Limits(f.ctx))
	c.Check(limits.Enabled, qt.IsFalse)
}

// TestIntakeWorker_AuthorLeftTheGroup: the worker scans as the session's
// author; an author who can no longer edit items in the group gets no scan.
func TestIntakeWorker_AuthorLeftTheGroup(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	role := models.GroupRoleUser
	author := f.member(c, "author@example.com", &role)
	ctx := f.as(author)
	file := f.upload(c, "lamp", ".jpg", "image/jpeg", 64)
	_, _, err := f.intake.CreateSession(ctx, "left", []services.IntakeSourceRequest{source(file)})
	c.Assert(err, qt.IsNil)

	membership := must.Must(f.factorySet.GroupMembershipRegistry.GetByGroupAndUser(context.Background(), f.group.ID, author.ID))
	membership.Role = models.GroupRoleViewer
	must.Must(f.factorySet.GroupMembershipRegistry.Update(context.Background(), *membership))

	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	c.Check(f.provider.callCount(), qt.Equals, 0)
	sessions, _ := must.Must2(f.intake.ListSessions(f.ctx, 0, 10))
	c.Assert(sessions, qt.HasLen, 1)
	c.Check(sessions[0].Summary.Failed, qt.Equals, 1)
}

func TestIntakeWorker_PauseAndMissingBlob(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	file := f.photo(c, "vase")
	session := f.create(c, "paused", source(file))

	pause := &pausedChecker{paused: models.WorkerTypeIntake}
	paused := services.NewIntakeWorker(f.factorySet, f.scan, f.uploadLocation, services.WithIntakePauseController(pause))
	c.Check(paused.Tick(context.Background()), qt.Equals, 0)
	c.Check(pause.asked.Load(), qt.Equals, int64(1))

	bucket := must.Must(blob.OpenBucket(f.ctx, f.uploadLocation))
	c.Assert(bucket.Delete(f.ctx, file.OriginalPath), qt.IsNil)
	c.Assert(bucket.Close(), qt.IsNil)
	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	src := f.get(c, f.ctx, session.Session.ID).Sources[0].Source
	c.Check(src.Status, qt.Equals, models.IntakeSourceFailed)
	c.Check(src.ErrorCode, qt.Equals, services.IntakeErrorFileUnreadable)
	c.Check(f.provider.callCount(), qt.Equals, 0)
}
