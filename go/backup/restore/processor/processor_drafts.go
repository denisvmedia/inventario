//go:build !legacy_xml_backup

package processor

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// draftsStep is the restore log step of the drafts member.
const draftsStep = "Drafts"

// handleDraftsMember decodes the drafts document (format 3.4) and recreates
// each draft. It comes after the location documents, so the areas the drafts
// name are mapped, and before the files member, which links files to them.
func (w *inbWalker) handleDraftsMember(hdr *tar.Header, r io.Reader) error {
	if hdr.Size < 0 || hdr.Size > maxJSONDocBytes {
		return errx.Classify(ErrJSONDocTooLarge, errx.Attrs("member", hdr.Name, "size", hdr.Size, "max", maxJSONDocBytes))
	}
	data, err := io.ReadAll(io.LimitReader(r, hdr.Size))
	if err != nil {
		return errxtrace.Wrap("failed to read drafts member", err, errx.Attrs("member", hdr.Name))
	}
	var doc types.INBDraftsDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return errxtrace.Wrap("failed to decode drafts document", err, errx.Attrs("member", hdr.Name))
	}
	if w.proc.factorySet.CommodityDraftRegistryFactory == nil {
		return nil
	}

	l := w.proc
	l.createRestoreStep(w.ctx, draftsStep, models.RestoreStepResultInProgress, "")
	restored, kept, failed := 0, 0, 0
	for i := range doc.Drafts {
		created, err := w.applyDraft(&doc.Drafts[i])
		switch {
		case errors.Is(err, ErrMalformedEntity):
			l.updateRestoreStep(w.ctx, draftsStep, models.RestoreStepResultError, err.Error())
			return err
		case err != nil:
			failed++
			w.stats.ErrorCount++
			w.stats.Errors = append(w.stats.Errors, fmt.Sprintf("failed to process draft: %v", err))
		case created:
			restored++
		default:
			kept++
		}
	}
	reason := fmt.Sprintf("%d restored, %d kept as they were, %d failed", restored, kept, failed)
	result := models.RestoreStepResultSuccess
	if failed > 0 {
		result = models.RestoreStepResultError
	}
	l.updateRestoreStep(w.ctx, draftsStep, result, reason)
	return nil
}

// applyDraft recreates one archived draft with new ids, or keeps the group's
// draft with the same client key. created is false for a kept draft, which
// gets a skipped step of its own in the restore log. A full replace has
// removed the group's drafts before, so a draft is kept only in a merge.
func (w *inbWalker) applyDraft(a *types.INBDraft) (created bool, err error) {
	createdAt, updatedAt, err := a.Timestamps()
	if err != nil {
		return false, errx.Classify(errxtrace.Wrap("failed to convert draft", err), ErrMalformedEntity)
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
		if _, err := reg.GetByClientKey(w.ctx, a.ClientKey); err == nil {
			w.keepDraft(a)
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
		w.keepDraft(a)
		return false, nil
	}
	w.draftIDs[a.ID] = stored.ID
	w.proc.trackCreatedEntity(stored.ID)
	w.stats.CreatedCount++
	return true, nil
}

// keepDraft records an archived draft the restore leaves out because the
// group has a draft with its client key, and reports it in the restore log.
func (w *inbWalker) keepDraft(a *types.INBDraft) {
	w.keptDrafts[a.ID] = true
	w.stats.SkippedCount++
	name := a.Values.Title()
	if name == "" {
		name = a.ClientKey
	}
	w.proc.createRestoreStep(w.ctx, "Draft: "+name, models.RestoreStepResultSkipped,
		"The group already has a draft with the same client key, so it was kept and the backup's copy was not restored.")
}

// isKeptDraft reports whether link points at a draft the restore kept.
func (w *inbWalker) isKeptDraft(link inbFileLink) bool {
	return link.linkedType == models.FileLinkDraft && w.keptDrafts[link.entityUUID]
}

// restoreDraftValues maps the references in archived draft values to the
// destination group, the way an item's are mapped: the area through the
// restored areas and the seller through the restored sellers. A reference
// that maps to nothing, and a type that is not an item type, is left out,
// so the form shows its default and the draft never names an entity outside
// the group. An empty reference stays, meaning nothing chosen.
func (w *inbWalker) restoreDraftValues(archived models.CommodityDraftValues) models.CommodityDraftValues {
	values := make(models.CommodityDraftValues, len(archived))
	for key, value := range archived {
		values[key] = value
	}
	restoreDraftReference(values, "area_id", w.idMapping.Areas)
	restoreDraftReference(values, "seller_id", w.sellerIDs)
	if raw, ok := values["type"]; ok {
		if t, isText := raw.(string); !isText || (t != "" && !models.CommodityType(t).IsValid()) {
			delete(values, "type")
		}
	}
	return values
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
