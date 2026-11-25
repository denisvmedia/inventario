package services

import (
	"context"
	"time"

	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// FileTagPatch is a checked change of file tags, ready to apply to files.
type FileTagPatch struct {
	// AddTags and RemoveTags are tag slugs; no slug is in both.
	AddTags    []string
	RemoveTags []string
	// addInput is the add list as written, so a tag EnsureTags creates gets
	// the label the user typed.
	addInput []string
}

// ParseFileTagPatch checks the tags to add (slugs or the text of new tags)
// and to remove (slugs) and returns the patch they describe. It has no side
// effects: tags to add that do not exist yet are created by
// FileBulkService.EnsureTags. The error is a validation.Errors keyed by tags.
func ParseFileTagPatch(add, remove []string) (FileTagPatch, error) {
	if len(add) == 0 && len(remove) == 0 {
		return FileTagPatch{}, validation.Errors{
			"tags": validation.NewError("no_tags", "add or remove at least one tag"),
		}
	}
	addSlugs, removeSlugs, err := parseTagLists(add, remove)
	if err != nil {
		return FileTagPatch{}, validation.Errors{"tags": err}
	}
	return FileTagPatch{AddTags: addSlugs, RemoveTags: removeSlugs, addInput: add}, nil
}

// Apply returns f with the patch applied and whether its tags changed. Tags
// are removed first and added after, keeping the order of the tags the file
// already has.
func (p FileTagPatch) Apply(f models.FileEntity) (models.FileEntity, bool) {
	tags, changed := applyTags(f.Tags, p.AddTags, p.RemoveTags)
	if changed {
		f.Tags = tags
	}
	return f, changed
}

// FileBulkService changes the tags of many files. Only the tags of each file
// change, through the same registry write a single edit uses; a file that
// fails does not stop the others, and the outcome says what happened to
// each.
type FileBulkService struct {
	tags *TagService
}

// NewFileBulkService builds a FileBulkService on the service that owns tags.
func NewFileBulkService(tags *TagService) *FileBulkService {
	return &FileBulkService{tags: tags}
}

// EnsureTags creates the tags patch adds that the group does not have yet,
// with the label as typed, the way saving a single file does.
func (s *FileBulkService) EnsureTags(ctx context.Context, patch *FileTagPatch) error {
	if len(patch.addInput) == 0 {
		return nil
	}
	slugs, err := s.tags.NormalizeAndEnsureSlugs(ctx, models.TagKindFile, patch.addInput)
	if err != nil {
		return err
	}
	patch.AddTags = slugs
	return nil
}

// UpdateTags applies patch to every file in ids. A file whose tags the patch
// does not change is skipped; a file in Trash fails as not found.
func (s *FileBulkService) UpdateTags(ctx context.Context, files registry.FileRegistry, ids []string, patch FileTagPatch) BulkOutcome {
	var out BulkOutcome
	ids = uniqueIDs(ids)
	for i, id := range ids {
		if err := ctx.Err(); err != nil {
			for _, rest := range ids[i:] {
				out.fail(rest, "", err)
			}
			break
		}
		before, err := files.Get(ctx, id)
		if err != nil {
			out.fail(id, "", err)
			continue
		}
		after, changed := patch.Apply(*before)
		if !changed {
			out.Skipped = append(out.Skipped, BulkSkip{ID: id, Name: fileName(before)})
			continue
		}
		after.UpdatedAt = time.Now()
		if _, err := files.Update(ctx, after); err != nil {
			out.fail(id, fileName(before), err)
			continue
		}
		out.Succeeded = append(out.Succeeded, id)
	}
	return out
}

// fileName is the name the files list shows for f: its title, else its
// file name.
func fileName(f *models.FileEntity) string {
	if f.Title != "" {
		return f.Title
	}
	if f.File != nil {
		return f.Path + f.Ext
	}
	return ""
}
