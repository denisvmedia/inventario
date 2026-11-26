package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

func pgDelivery(id string) services.IntakeDelivery {
	return services.IntakeDelivery{
		Title: "Invoice " + id, SourceSystem: "paperless-ngx", DocumentID: id,
		SourceURL: "https://paperless.example/documents/" + id + "/", SourceName: "Paperless-ngx",
		Seller: "Hardware Store", PurchaseDate: "2024-03-14", Amount: "EUR129.90",
	}
}

func openDraftsOf(c *qt.C, fx fileLinkPGFixture) []*models.CommodityDraft {
	c.Helper()
	reg, err := fx.factorySet.CommodityDraftRegistryFactory.CreateUserRegistry(fx.ctx)
	c.Assert(err, qt.IsNil)
	all, err := reg.List(fx.ctx)
	c.Assert(err, qt.IsNil)
	var open []*models.CommodityDraft
	for _, d := range all {
		if d.Status == models.CommodityDraftOpen {
			open = append(open, d)
		}
	}
	return open
}

// Deliveries of one document at once make one record, one draft and one
// file on postgres, and each gets the document (#2809).
func TestIntakeDeliver_Postgres_Concurrent(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkPGFixture(t)
	svc := services.NewIntakeService(fx.factorySet, nil, services.IntakeConfig{})
	before, err := fx.set.FileRegistry.List(fx.ctx)
	c.Assert(err, qt.IsNil)

	const n = 12
	var (
		wg            sync.WaitGroup
		created, kept atomic.Int32
		errs          = make([]error, n)
	)
	for i := range n {
		wg.Go(func() {
			blob := &services.IntakeDeliveredFile{BlobKey: fmt.Sprintf("t/x/files/%d.pdf", i), Name: "invoice.pdf", MIMEType: "application/pdf", Size: 10, SHA256: "aaaa"}
			result, err := svc.Deliver(fx.ctx, pgDelivery("117"), blob)
			errs[i] = err
			if err != nil {
				return
			}
			if result.Created {
				created.Add(1)
			}
			if result.File != nil {
				kept.Add(1)
			}
		})
	}
	wg.Wait()
	for _, err := range errs {
		c.Assert(err, qt.IsNil)
	}
	after, err := fx.set.FileRegistry.List(fx.ctx)
	c.Assert(err, qt.IsNil)
	c.Check(created.Load(), qt.Equals, int32(1))
	c.Check(kept.Load(), qt.Equals, int32(1))
	c.Check(openDraftsOf(c, fx), qt.HasLen, 1)
	c.Check(len(after)-len(before), qt.Equals, 1)
}

var errLostDraftWrite = errors.New("the draft write was lost")

// lostDraftWrites is a document registry whose record of a draft fails
// while asked to.
type lostDraftWrites struct {
	registry.IntakeDocumentRegistryFactory
	lose atomic.Bool
}

func (f *lostDraftWrites) CreateUserRegistry(ctx context.Context) (registry.IntakeDocumentRegistry, error) {
	reg, err := f.IntakeDocumentRegistryFactory.CreateUserRegistry(ctx)
	return &lostDraftWriteRegistry{IntakeDocumentRegistry: reg, f: f}, err
}

type lostDraftWriteRegistry struct {
	registry.IntakeDocumentRegistry
	f *lostDraftWrites
}

func (r *lostDraftWriteRegistry) RecordProgress(ctx context.Context, id string, progress registry.IntakeDocumentProgress) (*models.IntakeDocument, error) {
	if progress.DraftID != nil && r.f.lose.Load() {
		return nil, errLostDraftWrite
	}
	return r.IntakeDocumentRegistry.RecordProgress(ctx, id, progress)
}

// The draft was made and its record was lost, then the member deleted the
// session. The repeat finds the draft by its key and makes no second one.
func TestIntakeDeliver_Postgres_DraftNotRecorded(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkPGFixture(t)
	lossy := &lostDraftWrites{IntakeDocumentRegistryFactory: fx.factorySet.IntakeDocumentRegistryFactory}
	fx.factorySet.IntakeDocumentRegistryFactory = lossy
	svc := services.NewIntakeService(fx.factorySet, nil, services.IntakeConfig{})

	lossy.lose.Store(true)
	_, err := svc.Deliver(fx.ctx, pgDelivery("117"), nil)
	lossy.lose.Store(false)
	c.Assert(err, qt.ErrorIs, errLostDraftWrite)
	drafts := openDraftsOf(c, fx)
	c.Assert(drafts, qt.HasLen, 1)
	reg, err := lossy.CreateUserRegistry(fx.ctx)
	c.Assert(err, qt.IsNil)
	doc, err := reg.GetByExternalID(fx.ctx, "paperless-ngx", "", "117")
	c.Assert(err, qt.IsNil)
	c.Assert(svc.Delete(fx.ctx, *doc.SessionID), qt.IsNil)

	for range 2 {
		result, err := svc.Deliver(fx.ctx, pgDelivery("117"), nil)
		c.Assert(err, qt.IsNil)
		c.Assert(result.State, qt.Equals, models.IntakeDocumentDraft)
		c.Assert(*result.Document.DraftID, qt.Equals, drafts[0].ID)
		c.Assert(openDraftsOf(c, fx), qt.HasLen, 1)
	}
}
