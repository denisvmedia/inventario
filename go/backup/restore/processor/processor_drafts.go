//go:build !legacy_xml_backup

package processor

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// draftsStep is the restore log step of the drafts member.
const draftsStep = "Drafts"

// maxRestoredDraftRevision is the highest archived revision a restore
// accepts. The revision column is a 32-bit INTEGER and every save or
// promotion adds one, so a restored draft keeps at least as many saves of
// room as the revision it starts with.
const maxRestoredDraftRevision = math.MaxInt32 / 2

// handleDraftsMember decodes the drafts document (format 3.5) and recreates
// each draft. It comes after the location documents, so the areas the drafts
// name are mapped, and before the files member, which links files to them.
func (w *inbWalker) handleDraftsMember(hdr *tar.Header, r io.Reader) error {
	var doc types.INBDraftsDoc
	if err := decodeJSONMember(hdr, r, "drafts", &doc); err != nil {
		return err
	}
	if w.proc.factorySet.CommodityDraftRegistryFactory == nil {
		return nil
	}
	return w.restoreRecords(draftsStep, "draft", len(doc.Drafts), func(i int) (bool, error) {
		return w.applyDraft(&doc.Drafts[i])
	})
}

// applyDraft recreates one archived draft with new ids, or keeps the group's
// draft with the same client key. created is false for a kept draft, which
// gets a skipped step of its own in the restore log. A full replace has
// removed the group's drafts before, so a draft is kept only in a merge. The
// files of a draft that is kept or fails to restore are restored as
// standalone files, unless something else the archive restores uses them.
func (w *inbWalker) applyDraft(a *types.INBDraft) (created bool, err error) {
	createdAt, updatedAt, err := a.Timestamps()
	if err != nil {
		return false, errx.Classify(errxtrace.Wrap("failed to convert draft", err), ErrMalformedEntity)
	}
	if a.Revision > maxRestoredDraftRevision {
		return false, fmt.Errorf("draft %s has revision %d; a restore accepts at most %d", a.ID, a.Revision, maxRestoredDraftRevision)
	}
	draft := models.CommodityDraft{ClientKey: a.ClientKey, Values: w.restoreDraftValues(a.Values)}
	if err := draft.ValidateFields(w.ctx); err != nil {
		return false, errxtrace.Wrap("invalid draft", err, errx.Attrs("draft_id", a.ID))
	}

	reg, err := w.proc.factorySet.CommodityDraftRegistryFactory.CreateUserRegistry(w.ctx)
	if err != nil {
		return false, errxtrace.Wrap("failed to create draft registry", err)
	}
	// A dry run of a full replace removes nothing, but the real run would
	// remove the group's drafts first, so it has nothing to keep.
	if !w.options.DryRun || w.options.Strategy != types.RestoreStrategyFullReplace {
		if existing, err := reg.GetByClientKey(w.ctx, a.ClientKey); err == nil {
			w.keepDraft(a, existing)
			return false, nil
		} else if !errors.Is(err, registry.ErrNotFound) {
			return false, errxtrace.Wrap("failed to look up draft", err, errx.Attrs("draft_id", a.ID))
		}
	}
	if w.options.DryRun {
		w.stats.CreatedCount++
		return true, nil
	}

	if err := w.ensureDraftTags(draft.Values); err != nil {
		return false, errxtrace.Wrap("failed to ensure draft tags", err, errx.Attrs("draft_id", a.ID))
	}
	if err := w.restoreDraftType(draft.Values); err != nil {
		return false, errxtrace.Wrap("failed to resolve draft item type", err, errx.Attrs("draft_id", a.ID))
	}
	ctx := registry.WithRestoredDraft(w.ctx, registry.RestoredDraft{
		Revision:  a.Revision,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	})
	stored, isNew, err := reg.CreateOrGet(ctx, draft)
	if err != nil {
		return false, errxtrace.Wrap("failed to create draft", err, errx.Attrs("draft_id", a.ID))
	}
	if !isNew {
		// Another request made a draft with the key since the lookup.
		w.keepDraft(a, stored)
		return false, nil
	}
	w.draftIDs[a.ID] = stored.ID
	w.proc.trackCreatedEntity(stored.ID)
	w.stats.CreatedCount++
	return true, nil
}

// keepDraft reports in the restore log an archived draft the restore leaves
// out because the group's draft existing has its client key.
func (w *inbWalker) keepDraft(a *types.INBDraft, existing *models.CommodityDraft) {
	w.keptDrafts[a.ID] = existing
	w.stats.SkippedCount++
	name := a.Values.Title()
	if name == "" {
		name = a.ClientKey
	}
	w.proc.createRestoreStep(w.ctx, "Draft: "+name, models.RestoreStepResultSkipped, keptDraftReason(existing))
}

// keptDraftReason says why the backup's copy of a draft was not restored,
// after the state of the group's draft with the same client key.
func keptDraftReason(existing *models.CommodityDraft) string {
	switch {
	case existing.Status == models.CommodityDraftPromoted:
		return "The group's draft with the same client key has already become an item, so the backup's copy was not restored."
	case existing.IsDiscarding():
		return "The group's draft with the same client key is being discarded, so the backup's copy was not restored."
	default:
		return "The group already has a draft with the same client key, so it was kept and the backup's copy was not restored."
	}
}

// isUnrestoredDraft reports whether link points at an archived draft the
// restore did not recreate: one it kept, one that failed, or one the archive
// does not carry.
func (w *inbWalker) isUnrestoredDraft(link inbFileLink) bool {
	return link.linkedType == models.FileLinkDraft && w.draftIDs[link.entityUUID] == ""
}

// restoreDraftValues maps the references in archived draft values to the
// destination group, the way an item's are mapped: the area through the
// restored areas and the seller through the restored sellers. A reference
// that maps to nothing is left out, so the form shows its default and the
// draft never names an entity outside the group. An empty reference stays,
// meaning nothing chosen. The type is mapped by restoreDraftType once the
// draft is known to be restored.
func (w *inbWalker) restoreDraftValues(archived models.CommodityDraftValues) models.CommodityDraftValues {
	values := make(models.CommodityDraftValues, len(archived))
	maps.Copy(values, archived)
	restoreDraftReference(values, "area_id", w.idMapping.Areas)
	restoreDraftReference(values, "seller_id", w.sellerIDs)
	w.restoreDraftCustomFields(values)
	if _, isText := values["type"].(string); !isText {
		delete(values, "type")
	}
	return values
}

// restoreDraftType maps the item type a draft names onto the destination
// group's, as an item's type is mapped (#2802): the same slug, the type with
// the archived name, or a type the restore creates. It runs only for a draft
// the restore creates, so a kept draft creates no type.
func (w *inbWalker) restoreDraftType(values models.CommodityDraftValues) error {
	archived, _ := values["type"].(string)
	if archived == "" {
		return nil
	}
	resolver, err := w.proc.itemTypeResolver(w.ctx, w.options)
	if err != nil {
		return err
	}
	slug, err := resolver.resolve(w.ctx, models.CommodityType(archived), w.stats)
	if err != nil {
		return err
	}
	values["type"] = string(slug)
	return nil
}

// restoreDraftReference replaces the archive UUID in values[key] with the DB
// id dbIDByUUID gives for it, or removes the key when it gives none.
func restoreDraftReference(values models.CommodityDraftValues, key string, dbIDByUUID map[string]string) {
	raw, ok := values[key]
	if !ok {
		return
	}
	uuid, isText := raw.(string)
	switch {
	case isText && uuid == "":
	case isText && dbIDByUUID[uuid] != "":
		values[key] = dbIDByUUID[uuid]
	default:
		delete(values, key)
	}
}

// ensureDraftTags makes sure the group has the tags a draft names, as an
// item restore does, and replaces them with their slugs.
func (w *inbWalker) ensureDraftTags(values models.CommodityDraftValues) error {
	list, ok := values["tags"].([]any)
	if !ok || len(list) == 0 {
		return nil
	}
	refs := make([]string, 0, len(list))
	for _, item := range list {
		if ref, isText := item.(string); isText {
			refs = append(refs, ref)
		}
	}
	slugs, err := w.proc.tagService.NormalizeAndEnsureRestoredSlugs(w.ctx, models.TagKindCommodity, refs)
	if err != nil {
		return err
	}
	tags := make([]any, 0, len(slugs))
	for _, slug := range slugs {
		tags = append(tags, slug)
	}
	values["tags"] = tags
	return nil
}
