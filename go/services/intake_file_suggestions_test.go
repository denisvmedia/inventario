package services_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/internal/aivision"
	"go.5x5.cz/inventario/internal/aivision/mock"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

func (f *intakeFixture) file(c *qt.C, id string) *models.FileEntity {
	c.Helper()
	files := must.Must(f.factorySet.FileRegistryFactory.CreateUserRegistry(f.ctx))
	return must.Must(files.Get(f.ctx, id))
}

func (f *intakeFixture) pdf(c *qt.C, name string) *models.FileEntity {
	return f.upload(c, name, ".pdf", "application/pdf", 64)
}

// editFile changes a file as the member would in the file dialog.
func (f *intakeFixture) editFile(c *qt.C, id string, edit func(*models.FileEntity)) {
	c.Helper()
	files := must.Must(f.factorySet.FileRegistryFactory.CreateUserRegistry(f.ctx))
	file := must.Must(files.Get(f.ctx, id))
	edit(file)
	must.Must(files.Update(f.ctx, *file))
}

// workerWith is a worker of the fixture's group that scans with provider,
// one source per tick.
func (f *intakeFixture) workerWith(provider aivision.Provider) *services.IntakeWorker {
	scan := services.NewCommodityScanService(provider, f.factorySet.CommodityScanAuditRegistry, services.CommodityScanConfig{
		MaxPhotos: 5, MaxPhotoBytes: 1 << 20, RateLimitPerHour: 100,
	})
	return services.NewIntakeWorker(f.factorySet, scan, f.uploadLocation, services.WithIntakeClock(f.clock), services.WithIntakeSourcesPerTick(1))
}

// assertUntouched checks that a file still has the title and the tags its
// upload gave it.
func assertUntouched(c *qt.C, got *models.FileEntity) {
	c.Helper()
	c.Check(got.Title, qt.Equals, got.Path)
	c.Check([]string(got.Tags), qt.DeepEquals, []string{})
}

func TestIntakeDocumentTypeTag(t *testing.T) {
	want := map[string]string{
		aivision.DocumentTypeInvoice:  models.FileTagInvoice,
		aivision.DocumentTypeReceipt:  models.FileTagInvoice,
		aivision.DocumentTypeManual:   models.FileTagManual,
		aivision.DocumentTypeWarranty: models.FileTagWarranty,
		aivision.DocumentTypePhoto:    models.FileTagPhoto,
	}
	for _, docType := range aivision.FileDocumentTypes {
		t.Run(docType, func(t *testing.T) {
			c := qt.New(t)
			tag, ok := services.IntakeDocumentTypeTag(docType)
			wantTag, wantOK := want[docType]
			c.Check(ok, qt.Equals, wantOK)
			c.Check(tag, qt.Equals, wantTag)
		})
	}
}

// TestIntakeWorker_AppliesFileSuggestions: once a source is scanned, each of
// its files gets the tag of its document type and the suggested title, and
// the tags exist in the group afterwards.
func TestIntakeWorker_AppliesFileSuggestions(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	receipt := f.pdf(c, "receipt-coffee")
	manual := f.pdf(c, "kettle-manual")
	warranty := f.photo(c, "warranty-card")
	kettle := f.photo(c, "kettle")
	f.create(c, "suggest", source(receipt, manual, warranty, kettle))

	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)

	want := map[string]struct {
		title string
		tags  []string
	}{
		receipt.ID:  {"Sample Store receipt, March 2024", []string{models.FileTagInvoice}},
		manual.ID:   {"Sample user manual", []string{models.FileTagManual}},
		warranty.ID: {"Sample warranty card", []string{models.FileTagWarranty}},
		kettle.ID:   {"Sample product photo", []string{models.FileTagPhoto}},
	}
	for id, w := range want {
		got := f.file(c, id)
		c.Check(got.Title, qt.Equals, w.title, qt.Commentf("file %s", got.Path))
		c.Check([]string(got.Tags), qt.DeepEquals, w.tags, qt.Commentf("file %s", got.Path))
		c.Check(got.Path, qt.Not(qt.Equals), got.Title, qt.Commentf("the file name stays"))
	}

	tags := must.Must(f.factorySet.TagRegistryFactory.CreateUserRegistry(f.ctx))
	for _, slug := range []string{models.FileTagInvoice, models.FileTagManual, models.FileTagWarranty, models.FileTagPhoto} {
		_, err := tags.GetBySlug(f.ctx, models.TagKindFile, slug)
		c.Check(err, qt.IsNil, qt.Commentf("tag %s", slug))
	}
}

// storeWatchingIntake is the intake registry factory whose service registry
// runs stored right after each scan it stores, before the worker goes on.
type storeWatchingIntake struct {
	registry.IntakeRegistryFactory
	stored func()
}

func (f storeWatchingIntake) CreateServiceRegistry() registry.IntakeRegistry {
	return storeWatchingRegistry{IntakeRegistry: f.IntakeRegistryFactory.CreateServiceRegistry(), stored: f.stored}
}

type storeWatchingRegistry struct {
	registry.IntakeRegistry
	stored func()
}

func (r storeWatchingRegistry) StoreScan(ctx context.Context, sourceID string, attempt int, scan registry.IntakeScan, now time.Time) error {
	err := r.IntakeRegistry.StoreScan(ctx, sourceID, attempt, scan, now)
	if err == nil {
		r.stored()
	}
	return err
}

// TestIntakeWorker_FilesChangeWithTheStoredScan: the files carry the
// suggestions as soon as the scan is stored (#2940), so a read that finds
// the source scanned finds its files changed too.
func TestIntakeWorker_FilesChangeWithTheStoredScan(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	receipt := f.pdf(c, "receipt-coffee")
	session := f.create(c, "together", source(receipt))
	var (
		status models.IntakeSourceStatus
		seen   *models.FileEntity
	)
	f.factorySet.IntakeRegistryFactory = storeWatchingIntake{
		IntakeRegistryFactory: f.factorySet.IntakeRegistryFactory,
		stored: func() {
			status = f.get(c, f.ctx, session.Session.ID).Sources[0].Source.Status
			seen = f.file(c, receipt.ID)
		},
	}

	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	c.Assert(seen, qt.IsNotNil)
	c.Check(status, qt.Equals, models.IntakeSourceScanned)
	c.Check(seen.Title, qt.Equals, "Sample Store receipt, March 2024")
	c.Check([]string(seen.Tags), qt.DeepEquals, []string{models.FileTagInvoice})
}

// TestIntakeWorker_FileSuggestionsKeepWhatTheMemberSet: a file the member
// renamed or classified before the scan gets no suggestion at all. A tag of
// the member's own that says nothing about the kind of file does not count.
func TestIntakeWorker_FileSuggestionsKeepWhatTheMemberSet(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	renamed := f.pdf(c, "invoice-1")
	classified := f.pdf(c, "invoice-2")
	asPhoto := f.pdf(c, "invoice-3")
	ownTag := f.pdf(c, "invoice-4")
	f.editFile(c, renamed.ID, func(file *models.FileEntity) { file.Title = "My Bosch invoice" })
	f.editFile(c, classified.ID, func(file *models.FileEntity) { file.Tags = models.StringSlice{models.FileTagManual} })
	f.editFile(c, asPhoto.ID, func(file *models.FileEntity) { file.Tags = models.StringSlice{models.FileTagPhoto} })
	f.editFile(c, ownTag.ID, func(file *models.FileEntity) { file.Tags = models.StringSlice{"kitchen"} })
	f.create(c, "kept", source(renamed, classified, asPhoto, ownTag))

	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)

	c.Check(f.file(c, renamed.ID).Title, qt.Equals, "My Bosch invoice")
	c.Check([]string(f.file(c, renamed.ID).Tags), qt.DeepEquals, []string{})
	c.Check(f.file(c, classified.ID).Title, qt.Equals, "invoice-2")
	c.Check([]string(f.file(c, classified.ID).Tags), qt.DeepEquals, []string{models.FileTagManual})
	c.Check(f.file(c, asPhoto.ID).Title, qt.Equals, "invoice-3")
	c.Check([]string(f.file(c, asPhoto.ID).Tags), qt.DeepEquals, []string{models.FileTagPhoto})
	c.Check(f.file(c, ownTag.ID).Title, qt.Equals, "Sample Store invoice, March 2024")
	c.Check([]string(f.file(c, ownTag.ID).Tags), qt.DeepEquals, []string{"kitchen", models.FileTagInvoice})
}

// TestIntakeWorker_FileSuggestionsApplyOnce: a scan that failed and was
// retried applies the suggestions once, with the worker's time, and a rescan
// of the source changes nothing more.
func TestIntakeWorker_FileSuggestionsApplyOnce(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	receipt := f.pdf(c, "receipt-coffee")
	session := f.create(c, "once", source(receipt))
	f.provider.failNext(aivision.ErrProviderUnavailable)

	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	assertUntouched(c, f.file(c, receipt.ID))
	f.ahead += 10 * time.Minute
	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	applied := f.file(c, receipt.ID)
	c.Check(applied.Title, qt.Equals, "Sample Store receipt, March 2024")
	c.Check([]string(applied.Tags), qt.DeepEquals, []string{models.FileTagInvoice})
	c.Check(applied.UpdatedAt.After(time.Now().Add(5*time.Minute)), qt.IsTrue, qt.Commentf("updated at %s, from the worker's clock", applied.UpdatedAt))

	must.Must(f.intake.Rescan(f.ctx, session.Session.ID, session.Sources[0].Source.ID))
	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	c.Check(f.file(c, receipt.ID), qt.DeepEquals, applied)
	c.Check(f.provider.callCount(), qt.Equals, 3)
}

// editingProvider is the mock provider that runs edit while the scan runs,
// once.
type editingProvider struct {
	*mock.Provider
	edit func()
}

func (p *editingProvider) Scan(ctx context.Context, req aivision.ScanRequest) (*aivision.ScanResult, error) {
	if p.edit != nil {
		p.edit()
		p.edit = nil
	}
	return p.Provider.Scan(ctx, req)
}

// TestIntakeWorker_FileSuggestionsSeeEditsDuringTheScan: a file the member
// renames while the scan runs gets no suggestion.
func TestIntakeWorker_FileSuggestionsSeeEditsDuringTheScan(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	receipt := f.pdf(c, "receipt-coffee")
	f.create(c, "during", source(receipt))
	worker := f.workerWith(&editingProvider{Provider: mock.New(), edit: func() {
		f.editFile(c, receipt.ID, func(file *models.FileEntity) { file.Title = "Coffee receipt" })
	}})

	c.Assert(worker.Tick(context.Background()), qt.Equals, 1)
	got := f.file(c, receipt.ID)
	c.Check(got.Title, qt.Equals, "Coffee receipt")
	c.Check([]string(got.Tags), qt.DeepEquals, []string{})
}

// TestIntakeWorker_FileSuggestionsNeedTheStoredScan: when the source changes
// during the scan, the result is dropped and the files stay as they are.
func TestIntakeWorker_FileSuggestionsNeedTheStoredScan(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	receipt := f.pdf(c, "receipt-coffee")
	session := f.create(c, "changed", source(receipt))
	worker := f.workerWith(&editingProvider{Provider: mock.New(), edit: func() {
		// The sweep takes the claim away, as when it thinks the worker died.
		reg := f.factorySet.IntakeRegistryFactory.CreateServiceRegistry()
		c.Check(must.Must(reg.SweepStuckSources(context.Background(), time.Now().Add(time.Hour), time.Now(), 0, "")).Changed, qt.Equals, 1)
	}})

	c.Assert(worker.Tick(context.Background()), qt.Equals, 1)
	c.Check(f.get(c, f.ctx, session.Session.ID).Sources[0].Source.Status, qt.Equals, models.IntakeSourcePending)
	assertUntouched(c, f.file(c, receipt.ID))
}

// TestIntakeWorker_ExternalSourceKeepsItsFiles: a source that carries a
// document of another system keeps the files as they came, even when it is
// scanned.
func TestIntakeWorker_ExternalSourceKeepsItsFiles(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	invoice := f.pdf(c, "invoice-9")
	reg := must.Must(f.factorySet.IntakeRegistryFactory.CreateUserRegistry(f.ctx))
	session, _, err := reg.CreateSession(f.ctx,
		models.IntakeSession{ClientKey: "external-test", Origin: models.IntakeOriginExternal},
		[]registry.IntakeSourceInput{{
			FileIDs:            []string{invoice.ID},
			OneItem:            true,
			ExternalSystem:     "paperless-ngx",
			ExternalDocumentID: "9",
		}}, 0)
	c.Assert(err, qt.IsNil)

	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	c.Check(f.get(c, f.ctx, session.ID).Sources[0].Source.Status, qt.Equals, models.IntakeSourceScanned)
	c.Check(f.provider.callCount(), qt.Equals, 1)
	assertUntouched(c, f.file(c, invoice.ID))
}

// TestIntakeWorker_UnsureFileSuggestionsAreLeftOut: a suggestion below the
// confidence the review accepts from changes nothing.
func TestIntakeWorker_UnsureFileSuggestionsAreLeftOut(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	receipt := f.pdf(c, "scan-1")
	f.create(c, "unsure", source(receipt))
	worker := f.workerWith(mock.New(mock.WithDefaultResult(aivision.ScanResult{
		Fields: map[string]aivision.FieldGuess{aivision.FieldNameName: {Value: "Kettle", Confidence: 0.9}},
		Files: []aivision.FileGuess{{
			Index:        0,
			DocumentType: &aivision.FieldGuess{Value: aivision.DocumentTypeInvoice, Confidence: services.IntakeAcceptConfidence / 2},
			Title:        &aivision.FieldGuess{Value: "Maybe an invoice", Confidence: services.IntakeAcceptConfidence / 2},
		}},
	})))

	c.Assert(worker.Tick(context.Background()), qt.Equals, 1)
	assertUntouched(c, f.file(c, receipt.ID))
}

// TestIntakeWorker_DeliveredFileKeepsItsTitle: a file delivered from another
// system is never scanned, so it keeps the title it came with.
func TestIntakeWorker_DeliveredFileKeepsItsTitle(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	result := f.deliver(c, paperlessDelivery("117"), &services.IntakeDeliveredFile{
		BlobKey: "t/tenant-a/files/invoice-117.pdf", Name: "invoice-117.pdf", MIMEType: "application/pdf", Size: 2048, SHA256: "aaaa",
	})

	c.Check(f.worker.Tick(context.Background()), qt.Equals, 0)
	got := f.file(c, result.File.ID)
	c.Check(got.Title, qt.Equals, "invoice-117")
	c.Check(got.Tags, qt.DeepEquals, result.File.Tags)
	c.Check(f.provider.callCount(), qt.Equals, 0)
}
