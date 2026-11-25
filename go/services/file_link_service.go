package services

import (
	"context"
	"errors"
	"slices"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

var (
	// ErrFileLinkTargetNotFound is returned when the entity a file is being
	// attached to does not exist in the caller's group.
	ErrFileLinkTargetNotFound = errx.NewSentinel("the entity to attach the file to was not found")
	// ErrFileLinkInvalid is returned for an attachment request with an entity
	// type a user cannot attach to, or a meta that does not belong to it.
	ErrFileLinkInvalid = errx.NewSentinel("invalid file attachment")
	// ErrExportFileNotAttachable is returned when the file is a backup export
	// bundle. Export bundles belong to the backup subsystem and are never
	// attached to inventory.
	ErrExportFileNotAttachable = errx.NewSentinel("an export file cannot be attached to inventory")
	// ErrExportLinkNotRemovable is returned when the link to remove is the
	// export link of a backup bundle.
	ErrExportLinkNotRemovable = errx.NewSentinel("the export link of a backup file cannot be removed")
)

// FileLinkService attaches files to entities and removes attachments. It is the
// only path that changes the links of an existing file: attaching creates a
// relation and nothing else (no new file row, no upload, no quota charge), and
// removing an attachment removes the relation and nothing else. Deleting the
// file is a separate operation (FileService.DeleteFileWithPhysical).
type FileLinkService struct {
	factorySet *registry.FactorySet
}

// NewFileLinkService builds the service over the request's registries.
func NewFileLinkService(factorySet *registry.FactorySet) *FileLinkService {
	return &FileLinkService{factorySet: factorySet}
}

// Attach links the file to the entity. The file and the entity are both read
// through the caller's group-scoped registries, so a file can only be attached
// to an entity in its own group. An empty meta picks the bucket from the file's
// category. Attaching a file to an entity it is already attached to returns the
// existing link with created=false. A file can be attached to an open draft
// (#2812) like to an item; the link moves to the item when the draft is
// promoted.
func (s *FileLinkService) Attach(ctx context.Context, fileID, entityType, entityID, meta string) (*models.FileEntity, bool, error) {
	if !models.IsAttachableFileLinkType(entityType) {
		return nil, false, errxtrace.Classify(ErrFileLinkInvalid, errx.Attrs("entity_type", entityType))
	}
	if entityID == "" {
		return nil, false, errxtrace.Classify(ErrFileLinkInvalid, errx.Attrs("field_name", "entity_id"))
	}

	fileReg, err := s.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, false, errxtrace.Wrap("failed to create file registry", err)
	}
	file, err := fileReg.Get(ctx, fileID)
	if err != nil {
		return nil, false, errxtrace.Wrap("failed to get file", err)
	}
	if file.IsExportOwned() {
		return nil, false, errxtrace.Classify(ErrExportFileNotAttachable, errx.Attrs("file_id", fileID))
	}

	chosenMeta := meta != ""
	if !chosenMeta {
		meta = models.DefaultFileLinkMeta(entityType, file.Category)
	}
	if !slices.Contains(models.FileLinkMetaValues(entityType), meta) {
		return nil, false, errxtrace.Classify(ErrFileLinkInvalid, errx.Attrs("meta", meta))
	}

	if err := s.requireTarget(ctx, entityType, entityID); err != nil {
		return nil, false, err
	}

	firstPlacement := len(file.OrdinaryLinks()) == 0
	_, created, err := fileReg.AddLink(ctx, models.FileLink{
		FileID:     file.ID,
		EntityType: entityType,
		EntityID:   entityID,
		Meta:       meta,
	})
	if err != nil {
		return nil, false, errxtrace.Wrap("failed to attach file", err)
	}

	// The first attachment into a bucket the caller chose places the file: a
	// JPEG scan put in an item's documents takes the Documents category, as an
	// upload into that bucket always has. A default bucket follows the
	// category, so it never changes it, and later attachments leave the
	// category alone.
	if created && firstPlacement && chosenMeta && file.File != nil {
		if category := models.FileCategoryFromContext(entityType, meta, file.MIMEType); category != file.Category {
			file.Category = category
			if _, err := fileReg.Update(ctx, *file); err != nil {
				return nil, false, errxtrace.Wrap("failed to update file category", err)
			}
		}
	}

	updated, err := fileReg.Get(ctx, file.ID)
	if err != nil {
		return nil, false, errxtrace.Wrap("failed to reload file", err)
	}
	return updated, created, nil
}

// requireTarget checks that the entity exists in the caller's group.
func (s *FileLinkService) requireTarget(ctx context.Context, entityType, entityID string) error {
	var err error
	switch entityType {
	case models.FileLinkCommodity:
		var reg registry.CommodityRegistry
		if reg, err = s.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx); err == nil {
			_, err = reg.Get(ctx, entityID)
		}
	case models.FileLinkArea:
		var reg registry.AreaRegistry
		if reg, err = s.factorySet.AreaRegistryFactory.CreateUserRegistry(ctx); err == nil {
			_, err = reg.Get(ctx, entityID)
		}
	case models.FileLinkLocation:
		var reg registry.LocationRegistry
		if reg, err = s.factorySet.LocationRegistryFactory.CreateUserRegistry(ctx); err == nil {
			_, err = reg.Get(ctx, entityID)
		}
	case models.FileLinkDraft:
		err = s.requireOpenDraft(ctx, entityID)
	}
	if errors.Is(err, registry.ErrNotFound) {
		return errxtrace.Classify(ErrFileLinkTargetNotFound, errx.Attrs("entity_type", entityType, "entity_id", entityID))
	}
	if err != nil {
		return errxtrace.Wrap("failed to look up the attachment target", err)
	}
	return nil
}

// requireOpenDraft checks that the draft exists and has not been promoted.
// A promoted draft has handed its files to the item, so a late attachment
// would link a file to a draft nobody can open any more.
func (s *FileLinkService) requireOpenDraft(ctx context.Context, id string) error {
	reg, err := s.factorySet.CommodityDraftRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return err
	}
	draft, err := reg.Get(ctx, id)
	if err != nil {
		return err
	}
	if !draft.IsOpen() {
		return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", models.FileLinkDraft, "entity_id", id))
	}
	return nil
}

// Detach removes one attachment of a file. The file stays, also when this was
// its last link: it then is a standalone file. When the link attached the file
// to an item whose cover photo it is, the item's cover override is cleared and
// the item falls back to its first photo; other items keep their covers.
func (s *FileLinkService) Detach(ctx context.Context, fileID, linkID string) (*models.FileEntity, error) {
	fileReg, err := s.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create file registry", err)
	}
	file, err := fileReg.Get(ctx, fileID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get file", err)
	}

	var link *models.FileLink
	for i := range file.Links {
		if file.Links[i].ID == linkID {
			link = &file.Links[i]
			break
		}
	}
	if link == nil {
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file_link", "entity_id", linkID))
	}
	if link.EntityType == models.FileLinkExport {
		return nil, errxtrace.Classify(ErrExportLinkNotRemovable, errx.Attrs("file_id", fileID))
	}

	if _, err := fileReg.RemoveLink(ctx, fileID, linkID); err != nil {
		return nil, errxtrace.Wrap("failed to remove attachment", err)
	}

	if link.EntityType == models.FileLinkCommodity {
		if err := s.clearCover(ctx, link.EntityID, fileID); err != nil {
			return nil, err
		}
	}

	updated, err := fileReg.Get(ctx, fileID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to reload file", err)
	}
	return updated, nil
}

// clearCover drops the commodity's cover override when it points at fileID. A
// commodity that is gone or not visible has nothing to clear.
func (s *FileLinkService) clearCover(ctx context.Context, commodityID, fileID string) error {
	comReg, err := s.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create commodity registry", err)
	}
	commodity, err := comReg.Get(ctx, commodityID)
	if errors.Is(err, registry.ErrNotFound) {
		return nil
	}
	if err != nil {
		return errxtrace.Wrap("failed to get commodity", err)
	}
	if commodity.CoverFileID == nil || *commodity.CoverFileID != fileID {
		return nil
	}
	commodity.CoverFileID = nil
	if _, err := comReg.Update(ctx, *commodity); err != nil {
		return errxtrace.Wrap("failed to clear the cover photo", err)
	}
	return nil
}

// Duplicates returns the other files of the caller's group with the same
// checksum as fileID, newest first. Export bundles are left out: they are
// never offered for attachment.
func (s *FileLinkService) Duplicates(ctx context.Context, fileID string) ([]*models.FileEntity, error) {
	fileReg, err := s.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create file registry", err)
	}
	file, err := fileReg.Get(ctx, fileID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get file", err)
	}
	if file.File == nil || file.SHA256 == "" {
		return []*models.FileEntity{}, nil
	}
	matches, err := fileReg.ListByChecksum(ctx, file.SHA256)
	if err != nil {
		return nil, errxtrace.Wrap("failed to look up duplicates", err)
	}
	out := make([]*models.FileEntity, 0, len(matches))
	for _, m := range matches {
		if m.ID == file.ID || m.IsExportOwned() {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}
