package services

import (
	"context"
	"log/slog"
	"slices"

	"go.5x5.cz/inventario/internal/aivision"
	"go.5x5.cz/inventario/models"
)

// IntakeDocumentTypeTags maps the document type a scan suggests for a file
// to the file tag that says the same (#1626). A receipt is a proof of
// purchase like an invoice, so it gets the invoice tag; a photo of the
// product and any other file get no tag.
var IntakeDocumentTypeTags = map[string]string{
	aivision.DocumentTypeInvoice:  models.FileTagInvoice,
	aivision.DocumentTypeReceipt:  models.FileTagInvoice,
	aivision.DocumentTypeManual:   models.FileTagManual,
	aivision.DocumentTypeWarranty: models.FileTagWarranty,
}

// intakeDocumentTags are the tags that say what kind of document a file is.
// A file with one of them is already classified, by its author or by an
// earlier scan, and gets no other.
var intakeDocumentTags = []string{models.FileTagInvoice, models.FileTagManual, models.FileTagWarranty}

// intakeFileSuggestion returns file with the scan's suggestion for it
// applied, and whether anything changed. Only a suggestion with a confidence
// of at least IntakeAcceptConfidence counts, and it never takes a value the
// member set away:
//
//   - the tag of the document type is added while the file has none of the
//     document tags;
//   - the suggested title replaces the title while the title is still the
//     one the upload gave the file: its name without the extension.
//
// A file the suggestion was applied to does not change again, so applying
// it twice is the same as applying it once. addedTag is the tag it added, if
// any.
func intakeFileSuggestion(file models.FileEntity, guess aivision.FileGuess) (updated models.FileEntity, addedTag string, changed bool) {
	if tag, ok := intakeSuggestedTag(guess); ok && !slices.ContainsFunc(file.Tags, isIntakeDocumentTag) {
		file.Tags = append(slices.Clone(file.Tags), tag)
		addedTag = tag
		changed = true
	}
	if title, ok := intakeSuggestedTitle(guess); ok && file.File != nil && file.Title == file.Path && file.Title != title {
		file.Title = title
		changed = true
	}
	return file, addedTag, changed
}

func isIntakeDocumentTag(tag string) bool {
	return slices.Contains(intakeDocumentTags, tag)
}

func intakeSuggestedTag(guess aivision.FileGuess) (string, bool) {
	if guess.DocumentType == nil || guess.DocumentType.Confidence < IntakeAcceptConfidence {
		return "", false
	}
	docType, _ := guess.DocumentType.Value.(string)
	tag, ok := IntakeDocumentTypeTags[docType]
	return tag, ok
}

func intakeSuggestedTitle(guess aivision.FileGuess) (string, bool) {
	if guess.Title == nil || guess.Title.Confidence < IntakeAcceptConfidence {
		return "", false
	}
	raw, _ := guess.Title.Value.(string)
	title := aivision.CleanFileTitle(raw)
	return title, title != ""
}

// applyIntakeFileSuggestions writes the scan's suggestion for each file of a
// source to the file. fileIDs are the files in the order the scan got them,
// so a suggestion's index names its file. ctx acts as the session's author.
//
// The file is read again just before the write, since the member may have
// changed it during the scan. A failure costs that file its suggestion and
// is logged; the scan stands. The read and the write are not one
// transaction: a change the member saves in between is overwritten, as with
// any other two edits of one file that cross.
func (w *IntakeWorker) applyIntakeFileSuggestions(ctx context.Context, fileIDs []string, guesses []aivision.FileGuess, log *slog.Logger) {
	if len(guesses) == 0 {
		return
	}
	files, err := w.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		log.Warn("Intake worker: cannot apply the file suggestions", "error", err)
		return
	}
	tags := NewTagService(w.factorySet)
	for _, guess := range guesses {
		if guess.Index < 0 || guess.Index >= len(fileIDs) {
			continue
		}
		fileLog := log.With("file_id", fileIDs[guess.Index])
		current, err := files.Get(ctx, fileIDs[guess.Index])
		if err != nil {
			fileLog.Warn("Intake worker: cannot read a file to apply its suggestion", "error", err)
			continue
		}
		updated, addedTag, changed := intakeFileSuggestion(*current, guess)
		if !changed {
			continue
		}
		if addedTag != "" {
			// The group may not have the tag yet, and a tag a file refers
			// to has to exist.
			if _, err := tags.NormalizeAndEnsureSlugs(ctx, models.TagKindFile, []string{addedTag}); err != nil {
				fileLog.Warn("Intake worker: cannot create the suggested file tag", "error", err)
				continue
			}
		}
		if _, err := files.Update(ctx, updated); err != nil {
			fileLog.Warn("Intake worker: cannot apply a file suggestion", "error", err)
			continue
		}
		fileLog.Info("Intake worker: applied a file suggestion", "tags", []string(updated.Tags), "title_changed", updated.Title != current.Title)
	}
}
