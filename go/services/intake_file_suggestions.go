package services

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"time"

	"go.5x5.cz/inventario/internal/aivision"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// intakeDocumentTypeTags maps the document type a scan suggests for a file
// to the file tag that says the same (#1626). A receipt is a proof of
// purchase like an invoice, so it gets the invoice tag; a file of another
// kind gets no tag.
var intakeDocumentTypeTags = map[string]string{
	aivision.DocumentTypeInvoice:  models.FileTagInvoice,
	aivision.DocumentTypeReceipt:  models.FileTagInvoice,
	aivision.DocumentTypeManual:   models.FileTagManual,
	aivision.DocumentTypeWarranty: models.FileTagWarranty,
	aivision.DocumentTypePhoto:    models.FileTagPhoto,
}

// intakeKindTags are the tags a scan can give a file, the values of
// intakeDocumentTypeTags. A file with one of them is already classified.
var intakeKindTags = slices.Compact(slices.Sorted(maps.Values(intakeDocumentTypeTags)))

// IntakeDocumentTypeTag returns the file tag batch intake gives a file of the
// document type, and false for a type that gets none.
func IntakeDocumentTypeTag(docType string) (string, bool) {
	tag, ok := intakeDocumentTypeTags[docType]
	return tag, ok
}

// intakeFileSuggestion returns file with the scan's suggestion for it
// applied, and whether anything changed. Only a suggestion with a confidence
// of at least IntakeAcceptConfidence counts. The suggestion applies only to a
// file that still has both values its upload gave it: the title is its name
// without the extension, and it has none of intakeKindTags. Once the member
// renamed or classified the file, or an earlier scan did, the file is left
// as it is. Then:
//
//   - the tag of the document type is added;
//   - the suggested title replaces the title.
//
// A file the suggestion changed keeps a title or a tag that is no longer the
// upload's, so applying the suggestion again changes nothing. When the scan
// gave a document type but no title and the member removed the tag, the file
// has its upload values again and a rescan adds the tag back.
func intakeFileSuggestion(file models.FileEntity, guess aivision.FileGuess, now time.Time) (models.FileEntity, bool) {
	if file.File == nil || file.Title != file.Path || slices.ContainsFunc(file.Tags, isIntakeKindTag) {
		return file, false
	}
	changed := false
	if tag, ok := intakeSuggestedTag(guess); ok {
		file.Tags = append(slices.Clone(file.Tags), tag)
		changed = true
	}
	if title, ok := intakeSuggestedTitle(guess); ok && title != file.Title {
		file.Title = title
		changed = true
	}
	if changed {
		file.UpdatedAt = now
	}
	return file, changed
}

func isIntakeKindTag(tag string) bool {
	return slices.Contains(intakeKindTags, tag)
}

func intakeSuggestedTag(guess aivision.FileGuess) (string, bool) {
	if guess.DocumentType == nil || guess.DocumentType.Confidence < IntakeAcceptConfidence {
		return "", false
	}
	docType, _ := guess.DocumentType.Value.(string)
	return IntakeDocumentTypeTag(docType)
}

func intakeSuggestedTitle(guess aivision.FileGuess) (string, bool) {
	if guess.Title == nil || guess.Title.Confidence < IntakeAcceptConfidence {
		return "", false
	}
	raw, _ := guess.Title.Value.(string)
	title := aivision.CleanFileTitle(raw)
	return title, title != ""
}

// intakeFileUpdates holds the scan's suggestion for each file of a source,
// as the file updates StoreScan writes with the scan (#2940), and records
// which files they changed.
type intakeFileUpdates struct {
	updates []registry.IntakeFileUpdate
	changed []intakeFileChange
}

// intakeFileChange is a file a suggestion changed.
type intakeFileChange struct {
	fileID       string
	tags         []string
	titleChanged bool
}

// fileUpdates returns the scan's suggestion for each file of a source.
// fileIDs are the files in the order the scan got them, so a suggestion's
// index names its file. ctx acts as the session's author.
//
// The tags the suggestions may add are created in the group first, with the
// labels a member's new tag gets. When that fails, the scan is stored without
// the suggestions and the files stay as they are. The suggestion itself is
// worked out on the file as StoreScan reads it under its lock, so a change
// the member saved during the scan is seen and kept.
func (w *IntakeWorker) fileUpdates(ctx context.Context, fileIDs []string, guesses []aivision.FileGuess, log *slog.Logger) *intakeFileUpdates {
	out := &intakeFileUpdates{}
	guesses = slices.DeleteFunc(slices.Clone(guesses), func(g aivision.FileGuess) bool {
		return g.Index < 0 || g.Index >= len(fileIDs)
	})
	if len(guesses) == 0 {
		return out
	}
	var tags []string
	for _, guess := range guesses {
		if tag, ok := intakeSuggestedTag(guess); ok && !slices.Contains(tags, tag) {
			tags = append(tags, tag)
		}
	}
	if len(tags) > 0 {
		// A tag a file refers to has to exist in the group.
		if _, err := NewTagService(w.factorySet).NormalizeAndEnsureSlugs(ctx, models.TagKindFile, tags); err != nil {
			log.WarnContext(ctx, "Intake worker: cannot create the suggested file tags", "error", err)
			return out
		}
	}
	now := w.clock()
	for _, guess := range guesses {
		update := registry.IntakeFileUpdate{
			FileID: fileIDs[guess.Index],
			Apply: func(file models.FileEntity) (models.FileEntity, bool) {
				next, changed := intakeFileSuggestion(file, guess, now)
				if changed {
					out.changed = append(out.changed, intakeFileChange{
						fileID: file.ID, tags: []string(next.Tags), titleChanged: next.Title != file.Title,
					})
				}
				return next, changed
			},
		}
		if tag, ok := intakeSuggestedTag(guess); ok {
			update.Tags = []string{tag}
		}
		out.updates = append(out.updates, update)
	}
	return out
}

// logStored logs each file the stored scan changed.
func (u *intakeFileUpdates) logStored(ctx context.Context, log *slog.Logger) {
	for _, ch := range u.changed {
		log.InfoContext(ctx, "Intake worker: applied a file suggestion", "file_id", ch.fileID, "tags", ch.tags, "title_changed", ch.titleChanged)
	}
}
