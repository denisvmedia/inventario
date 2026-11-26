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

func TestIntakeDocumentTypeTags(t *testing.T) {
	c := qt.New(t)
	c.Check(services.IntakeDocumentTypeTags, qt.DeepEquals, map[string]string{
		aivision.DocumentTypeInvoice:  models.FileTagInvoice,
		aivision.DocumentTypeReceipt:  models.FileTagInvoice,
		aivision.DocumentTypeManual:   models.FileTagManual,
		aivision.DocumentTypeWarranty: models.FileTagWarranty,
	})
}

// TestIntakeWorker_AppliesFileSuggestions: once a source is scanned, each of
// its files gets the tag of its document type and the suggested title. A
// photo gets a title and no tag. The tags exist in the group afterwards.
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
		kettle.ID:   {"Sample product photo", []string{}},
	}
	for id, w := range want {
		got := f.file(c, id)
		c.Check(got.Title, qt.Equals, w.title, qt.Commentf("file %s", got.Path))
		c.Check([]string(got.Tags), qt.DeepEquals, w.tags, qt.Commentf("file %s", got.Path))
		c.Check(got.Path, qt.Not(qt.Equals), got.Title, qt.Commentf("the file name stays"))
	}

	tags := must.Must(f.factorySet.TagRegistryFactory.CreateUserRegistry(f.ctx))
	for _, slug := range []string{models.FileTagInvoice, models.FileTagManual, models.FileTagWarranty} {
		_, err := tags.GetBySlug(f.ctx, models.TagKindFile, slug)
		c.Check(err, qt.IsNil, qt.Commentf("tag %s", slug))
	}
}

// TestIntakeWorker_FileSuggestionsKeepWhatTheMemberSet: a title the member
// changed stays, a file the member classified keeps its document tag, and a
// tag of the member's own is kept next to the suggested one.
func TestIntakeWorker_FileSuggestionsKeepWhatTheMemberSet(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	renamed := f.pdf(c, "invoice-1")
	classified := f.pdf(c, "invoice-2")
	ownTag := f.pdf(c, "invoice-3")
	f.editFile(c, renamed.ID, func(file *models.FileEntity) { file.Title = "My Bosch invoice" })
	f.editFile(c, classified.ID, func(file *models.FileEntity) { file.Tags = models.StringSlice{models.FileTagManual} })
	f.editFile(c, ownTag.ID, func(file *models.FileEntity) { file.Tags = models.StringSlice{"kitchen"} })
	f.create(c, "kept", source(renamed, classified, ownTag))

	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)

	c.Check(f.file(c, renamed.ID).Title, qt.Equals, "My Bosch invoice")
	c.Check([]string(f.file(c, renamed.ID).Tags), qt.DeepEquals, []string{models.FileTagInvoice})
	c.Check([]string(f.file(c, classified.ID).Tags), qt.DeepEquals, []string{models.FileTagManual})
	c.Check(f.file(c, classified.ID).Title, qt.Equals, "Sample Store invoice, March 2024")
	c.Check([]string(f.file(c, ownTag.ID).Tags), qt.DeepEquals, []string{"kitchen", models.FileTagInvoice})
}

// TestIntakeWorker_FileSuggestionsApplyOnce: a scan that failed and was
// retried applies the suggestions once, and a rescan of the source changes
// nothing more.
func TestIntakeWorker_FileSuggestionsApplyOnce(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	receipt := f.pdf(c, "receipt-coffee")
	session := f.create(c, "once", source(receipt))
	f.provider.failNext(aivision.ErrProviderUnavailable)

	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	c.Check(f.file(c, receipt.ID).Title, qt.Equals, "receipt-coffee", qt.Commentf("a failed scan suggests nothing"))
	f.ahead += 10 * time.Minute
	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	applied := f.file(c, receipt.ID)
	c.Check(applied.Title, qt.Equals, "Sample Store receipt, March 2024")
	c.Check([]string(applied.Tags), qt.DeepEquals, []string{models.FileTagInvoice})

	must.Must(f.intake.Rescan(f.ctx, session.Session.ID, session.Sources[0].Source.ID))
	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	c.Check(f.file(c, receipt.ID), qt.DeepEquals, applied)
	c.Check(f.provider.callCount(), qt.Equals, 3)
}

// editingProvider is the mock provider that lets the member change a file
// while the scan runs.
type editingProvider struct {
	*mock.Provider
	edit func()
}

func (p *editingProvider) Scan(ctx context.Context, req aivision.ScanRequest) (*aivision.ScanResult, error) {
	p.edit()
	return p.Provider.Scan(ctx, req)
}

// TestIntakeWorker_FileSuggestionsSeeEditsDuringTheScan: a title the member
// sets while the scan runs stays.
func TestIntakeWorker_FileSuggestionsSeeEditsDuringTheScan(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	receipt := f.pdf(c, "receipt-coffee")
	f.create(c, "during", source(receipt))
	provider := &editingProvider{Provider: mock.New(), edit: func() {
		f.editFile(c, receipt.ID, func(file *models.FileEntity) { file.Title = "Coffee receipt" })
	}}
	scan := services.NewCommodityScanService(provider, f.factorySet.CommodityScanAuditRegistry, services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20, RateLimitPerHour: 100})
	worker := services.NewIntakeWorker(f.factorySet, scan, f.uploadLocation, services.WithIntakeClock(f.clock))

	c.Assert(worker.Tick(context.Background()), qt.Equals, 1)
	got := f.file(c, receipt.ID)
	c.Check(got.Title, qt.Equals, "Coffee receipt")
	c.Check([]string(got.Tags), qt.DeepEquals, []string{models.FileTagInvoice})
}

// TestIntakeWorker_UnsureFileSuggestionsAreLeftOut: a suggestion below the
// confidence the review accepts from changes nothing.
func TestIntakeWorker_UnsureFileSuggestionsAreLeftOut(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	receipt := f.pdf(c, "scan-1")
	f.create(c, "unsure", source(receipt))
	provider := mock.New(mock.WithDefaultResult(aivision.ScanResult{
		Fields: map[string]aivision.FieldGuess{aivision.FieldNameName: {Value: "Kettle", Confidence: 0.9}},
		Files: []aivision.FileGuess{{
			Index:        0,
			DocumentType: &aivision.FieldGuess{Value: aivision.DocumentTypeInvoice, Confidence: services.IntakeAcceptConfidence / 2},
			Title:        &aivision.FieldGuess{Value: "Maybe an invoice", Confidence: services.IntakeAcceptConfidence / 2},
		}},
	}))
	scan := services.NewCommodityScanService(provider, f.factorySet.CommodityScanAuditRegistry, services.CommodityScanConfig{MaxPhotos: 5, MaxPhotoBytes: 1 << 20, RateLimitPerHour: 100})
	worker := services.NewIntakeWorker(f.factorySet, scan, f.uploadLocation, services.WithIntakeClock(f.clock))

	c.Assert(worker.Tick(context.Background()), qt.Equals, 1)
	got := f.file(c, receipt.ID)
	c.Check(got.Title, qt.Equals, "scan-1")
	c.Check([]string(got.Tags), qt.DeepEquals, []string{})
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
