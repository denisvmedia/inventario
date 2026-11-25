package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models/rules"
)

var (
	_ validation.Validatable            = (*FileLink)(nil)
	_ validation.ValidatableWithContext = (*FileLink)(nil)
	_ TenantGroupAwareIDable            = (*FileLink)(nil)
)

// FileLinkEntityType names the kind of entity a file is attached to.
type FileLinkEntityType = string

const (
	// FileLinkCommodity attaches a file to an item. Meta is "images" or
	// "documents".
	FileLinkCommodity FileLinkEntityType = "commodity"
	// FileLinkLocation attaches a file to a location. Meta is "images" or
	// "files".
	FileLinkLocation FileLinkEntityType = "location"
	// FileLinkArea attaches a file to an area. Meta is "images" or "files".
	FileLinkArea FileLinkEntityType = "area"
	// FileLinkExport marks a file as the bundle of a backup export. An
	// export-owned file belongs to the backup subsystem: it is never attached
	// to inventory, its link is never removed through the attachment API, and
	// storage accounting reports it separately. Meta is the bundle format
	// version ("xml-1.0", "inb-2.0").
	FileLinkExport FileLinkEntityType = "export"
	// FileLinkDraft attaches a file to an unfinished item (#2812). Meta is
	// "images" or "documents", as on an item, and promoting the draft turns
	// the link into an item link with the same meta. A draft is not an
	// inventory entity, so its links are not among the ordinary links, and a
	// file whose only links are draft links is still a standalone file to
	// every other part of the app.
	FileLinkDraft FileLinkEntityType = "draft"
	// FileLinkIntake holds a file for an intake source (#2810): a file a
	// member dropped for the AI scan to read. The entity id is the source id
	// and meta is "images" or "documents". The link keeps the file while the
	// session needs it; the drafts made from the source get links of their
	// own. Like a draft link it is not an ordinary link, and nobody attaches
	// to it through the attachment API.
	FileLinkIntake FileLinkEntityType = "intake"
)

// FileLinkMetaImages and the constants below are the buckets a link places a
// file in on its entity's Files tab.
const (
	FileLinkMetaImages    = "images"
	FileLinkMetaDocuments = "documents"
	FileLinkMetaFiles     = "files"
)

// OrdinaryFileLinkTypes are the entity types a user can attach a file to.
// Export links are not among them: they are created by the backup subsystem
// only.
var OrdinaryFileLinkTypes = []FileLinkEntityType{FileLinkCommodity, FileLinkLocation, FileLinkArea}

// IsOrdinaryFileLinkType reports whether entityType is an inventory entity a
// file can be attached to by a user.
func IsOrdinaryFileLinkType(entityType string) bool {
	switch entityType {
	case FileLinkCommodity, FileLinkLocation, FileLinkArea:
		return true
	default:
		return false
	}
}

// IsAttachableFileLinkType reports whether a user can attach a file to an
// entity of this type: an inventory entity or a draft.
func IsAttachableFileLinkType(entityType string) bool {
	return IsOrdinaryFileLinkType(entityType) || entityType == FileLinkDraft
}

// FileLinkMetaValues returns the meta values valid for an entity type, or nil
// for an unknown type.
func FileLinkMetaValues(entityType string) []string {
	switch entityType {
	case FileLinkCommodity, FileLinkDraft, FileLinkIntake:
		return []string{FileLinkMetaImages, FileLinkMetaDocuments}
	case FileLinkLocation, FileLinkArea:
		return []string{FileLinkMetaImages, FileLinkMetaFiles}
	case FileLinkExport:
		return []string{"xml-1.0", "inb-2.0"}
	default:
		return nil
	}
}

// DefaultFileLinkMeta picks the bucket for a new attachment when the caller
// does not name one: images go to "images", everything else to the entity's
// document bucket.
func DefaultFileLinkMeta(entityType string, category FileCategory) string {
	if category == FileCategoryImages {
		return FileLinkMetaImages
	}
	if entityType == FileLinkCommodity || entityType == FileLinkDraft || entityType == FileLinkIntake {
		return FileLinkMetaDocuments
	}
	return FileLinkMetaFiles
}

// FileLink attaches a file to one entity. A file can carry any number of
// links, so one receipt serves every item it covers and one manual every unit
// of a model. A file without links is a standalone file: it is listed on the
// Files page and kept until it is deleted explicitly.
//
// The link lives in the file's tenant and group. Its target must be in the same
// group; the attach path checks that through the group-scoped registries.
//
//ptah:schema:rls:enable table="file_links" comment="Enable RLS for multi-tenant file link isolation"
//ptah:schema:rls:policy name="file_link_isolation" table="file_links" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures file links can only be accessed and modified by their tenant and group with required contexts"
//ptah:schema:rls:policy name="file_link_background_worker_access" table="file_links" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all file links for processing"
//ptah:schema:table name="file_links"
type FileLink struct {
	//ptah:embedded mode="inline"
	TenantGroupAwareEntityID

	// FileID is the attached file. Deleting the file deletes its links.
	//ptah:schema:field name="file_id" type="TEXT" not_null="true" foreign="files(id)" foreign_key_name="fk_file_link_file" on_delete="CASCADE"
	FileID string `json:"file_id" db:"file_id" userinput:"false"`

	// EntityType is one of the FileLink* constants.
	//ptah:schema:field name="entity_type" type="TEXT" not_null="true"
	EntityType string `json:"entity_type" db:"entity_type"`

	// EntityID is the database id of the attached entity. The reference is
	// polymorphic, so there is no foreign key: the entity-deletion paths remove
	// the links of the entity they delete.
	//ptah:schema:field name="entity_id" type="TEXT" not_null="true"
	EntityID string `json:"entity_id" db:"entity_id"`

	// Meta is the bucket on the entity's Files tab (see FileLinkMetaValues).
	//ptah:schema:field name="meta" type="TEXT" not_null="true" default=""
	Meta string `json:"meta" db:"meta"`

	//ptah:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`

	// EntityName is the display name of the attached entity, filled by the API
	// handlers for the "Linked to" list. Empty when the entity is not visible
	// to the caller.
	EntityName string `json:"entity_name,omitempty" db:"-" userinput:"false" readonly:"true"`
}

// FileLinkIndexes carries the ptah:schema:index tags for file_links.
type FileLinkIndexes struct {
	// Unique index for the immutable UUID.
	//ptah:schema:index name="idx_file_links_uuid" fields="uuid" unique="true" table="file_links"
	_ int

	// One link per (file, entity): attaching the same file to the same entity
	// twice is a no-op, which is what makes the attach call idempotent.
	//ptah:schema:index name="idx_file_links_file_entity" fields="file_id,entity_type,entity_id" unique="true" table="file_links"
	_ int

	// The Files tab of an entity reads its links by (entity_type, entity_id).
	//ptah:schema:index name="idx_file_links_entity" fields="entity_type,entity_id" table="file_links"
	_ int

	// Composite index for tenant+group RLS-filtered queries and the group purge.
	//ptah:schema:index name="idx_file_links_tenant_group" fields="tenant_id,group_id" table="file_links"
	_ int
}

func (*FileLink) Validate() error {
	return ErrMustUseValidateWithContext
}

// ValidateWithContext checks the entity type and that the meta belongs to it.
func (l *FileLink) ValidateWithContext(ctx context.Context) error {
	metas := make([]any, 0, 4)
	for _, m := range FileLinkMetaValues(l.EntityType) {
		metas = append(metas, m)
	}
	return validation.ValidateStructWithContext(ctx, l,
		validation.Field(&l.EntityType, rules.NotEmpty, validation.In(
			FileLinkCommodity, FileLinkLocation, FileLinkArea, FileLinkExport, FileLinkDraft, FileLinkIntake,
		)),
		validation.Field(&l.EntityID, rules.NotEmpty, validation.Length(1, 255)),
		validation.Field(&l.Meta, rules.NotEmpty, validation.In(metas...)),
	)
}

// LinkTo returns the file's link to the given entity, or nil.
func (fe *FileEntity) LinkTo(entityType, entityID string) *FileLink {
	for i := range fe.Links {
		if fe.Links[i].EntityType == entityType && fe.Links[i].EntityID == entityID {
			return &fe.Links[i]
		}
	}
	return nil
}

// IsExportOwned reports whether the file is the bundle of a backup export.
func (fe *FileEntity) IsExportOwned() bool {
	for i := range fe.Links {
		if fe.Links[i].EntityType == FileLinkExport {
			return true
		}
	}
	return false
}

// OrdinaryLinks returns the file's links to inventory entities, leaving out the
// export link of a backup bundle.
func (fe *FileEntity) OrdinaryLinks() []FileLink {
	out := make([]FileLink, 0, len(fe.Links))
	for _, l := range fe.Links {
		if IsOrdinaryFileLinkType(l.EntityType) {
			out = append(out, l)
		}
	}
	return out
}
