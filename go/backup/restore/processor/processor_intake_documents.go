//go:build !legacy_xml_backup

package processor

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// intakeDocumentsStep is the restore log step of the delivered documents
// member.
const intakeDocumentsStep = "Delivered documents"

// handleIntakeDocumentsMember decodes the delivered documents (format 3.7)
// and recreates the record of each. It comes after the items and the drafts,
// so the ones a document names are mapped.
func (w *inbWalker) handleIntakeDocumentsMember(hdr *tar.Header, r io.Reader) error {
	var doc types.INBIntakeDocumentsDoc
	if err := decodeJSONMember(hdr, r, "delivered documents", &doc); err != nil {
		return err
	}
	if w.proc.factorySet.IntakeDocumentRegistryFactory == nil {
		return nil
	}
	return w.restoreRecords(intakeDocumentsStep, "delivered document", len(doc.Documents), func(i int) (bool, error) {
		return w.applyIntakeDocument(&doc.Documents[i])
	})
}

// applyIntakeDocument recreates the record of one delivered document, or
// keeps the group's record with the same identity (created=false). The draft
// and the item are mapped to the ones the restore made or kept. A document
// whose draft was made but which the restore brings back neither as a draft
// nor as an item gets an empty draft id: the delivery made its draft, and a
// repeat of it must not make another.
func (w *inbWalker) applyIntakeDocument(a *types.INBIntakeDocument) (bool, error) {
	createdAt, updatedAt, err := a.Timestamps()
	if err != nil {
		return false, errx.Classify(errxtrace.Wrap("failed to convert delivered document", err), ErrMalformedEntity)
	}
	doc := models.IntakeDocument{
		ExternalSystem:     a.ExternalSystem,
		ExternalInstance:   a.ExternalInstance,
		ExternalDocumentID: a.ExternalDocumentID,
		SourceName:         a.SourceName,
		SourceURL:          a.SourceURL,
		PayloadHash:        a.PayloadHash,
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}
	if err := doc.ValidateFields(w.ctx); err != nil {
		return false, errxtrace.Wrap("invalid delivered document", err, errx.Attrs("intake_document_id", a.ID))
	}
	if doc.PayloadHash == "" {
		return false, fmt.Errorf("delivered document %s has no payload hash", a.ID)
	}
	w.mapIntakeDocumentWork(a, &doc)

	reg, err := w.proc.factorySet.IntakeDocumentRegistryFactory.CreateUserRegistry(w.ctx)
	if err != nil {
		return false, errxtrace.Wrap("failed to create intake document registry", err)
	}
	// A dry run of a full replace removes nothing, but the real run would
	// remove the group's records first, so it has nothing to keep.
	if !w.options.DryRun || w.options.Strategy != types.RestoreStrategyFullReplace {
		kept, err := reg.GetByExternalID(w.ctx, doc.ExternalSystem, doc.ExternalInstance, doc.ExternalDocumentID)
		switch {
		case err == nil:
			return false, w.keepIntakeDocument(reg, kept, a.ID)
		case !errors.Is(err, registry.ErrNotFound):
			return false, errxtrace.Wrap("failed to look up delivered document", err, errx.Attrs("intake_document_id", a.ID))
		}
	}
	if w.options.DryRun {
		w.stats.CreatedCount++
		return true, nil
	}
	stored, created, err := reg.CreateOrGet(w.ctx, doc)
	if err != nil {
		return false, errxtrace.Wrap("failed to create delivered document", err, errx.Attrs("intake_document_id", a.ID))
	}
	if !created {
		return false, w.keepIntakeDocument(reg, stored, a.ID)
	}
	w.proc.trackCreatedEntity(stored.ID)
	if err := w.recordPromotedDraft(reg, stored); err != nil {
		return false, errxtrace.Wrap("failed to record the item of a delivered document", err, errx.Attrs("intake_document_id", a.ID))
	}
	w.stats.CreatedCount++
	return true, nil
}

// keepIntakeDocument keeps the group's record of a document. A record whose
// draft became an item it does not name gets the item, so a restore that
// stored the record and failed to record the item finishes it when it is run
// again. A dry run writes nothing.
func (w *inbWalker) keepIntakeDocument(reg registry.IntakeDocumentRegistry, kept *models.IntakeDocument, archiveID string) error {
	if !w.options.DryRun {
		if err := w.recordPromotedDraft(reg, kept); err != nil {
			return errxtrace.Wrap("failed to record the item of a delivered document", err, errx.Attrs("intake_document_id", archiveID))
		}
	}
	w.stats.SkippedCount++
	return nil
}

// recordPromotedDraft names the item on a document whose draft became one
// that the document does not name: a restored document whose draft was
// promoted while the restore ran, since a promotion records its item only on
// the documents that are there when it commits, or a kept one that an
// earlier restore stored and could not finish.
func (w *inbWalker) recordPromotedDraft(reg registry.IntakeDocumentRegistry, doc *models.IntakeDocument) error {
	if doc.CommodityID != nil || doc.DraftID == nil || *doc.DraftID == "" || w.proc.factorySet.CommodityDraftRegistryFactory == nil {
		return nil
	}
	drafts, err := w.proc.factorySet.CommodityDraftRegistryFactory.CreateUserRegistry(w.ctx)
	if err != nil {
		return err
	}
	draft, err := drafts.Get(w.ctx, *doc.DraftID)
	switch {
	case errors.Is(err, registry.ErrNotFound):
		return nil
	case err != nil:
		return err
	case draft.PromotedCommodityID == nil:
		return nil
	}
	_, err = reg.RecordProgress(w.ctx, doc.ID, registry.IntakeDocumentProgress{CommodityID: draft.PromotedCommodityID})
	return err
}

// mapIntakeDocumentWork points the document at the draft and the item the
// restore made from the ones the archive names. When a merge kept the
// group's own draft with the archived draft's client key, the document names
// that draft whatever its state: an open draft waits in the inbox, a
// promoted one also gives the item it became unless the archive maps one,
// and one being discarded leaves the document removed.
func (w *inbWalker) mapIntakeDocumentWork(a *types.INBIntakeDocument, doc *models.IntakeDocument) {
	var kept *models.CommodityDraft
	if a.DraftID != "" {
		if id := w.draftIDs[a.DraftID]; id != "" {
			doc.DraftID = new(id)
		} else if kept = w.keptDrafts[a.DraftID]; kept != nil {
			doc.DraftID = new(kept.ID)
		}
	}
	if id := w.idMapping.Commodities[a.CommodityID]; a.CommodityID != "" && id != "" {
		doc.CommodityID = new(id)
	} else if kept != nil && kept.PromotedCommodityID != nil {
		doc.CommodityID = new(*kept.PromotedCommodityID)
	}
	if a.DraftMade && doc.DraftID == nil {
		doc.DraftID = new("")
	}
}
