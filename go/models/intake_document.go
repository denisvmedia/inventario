package models

import (
	"context"
	"net/url"
	"time"

	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models/rules"
)

var (
	_ validation.Validatable            = (*IntakeDocument)(nil)
	_ validation.ValidatableWithContext = (*IntakeDocument)(nil)
	_ TenantGroupAwareIDable            = (*IntakeDocument)(nil)
)

// Limits on the values another system delivers with a document.
const (
	// IntakeSourceNameMaxRunes caps the display name of the other system.
	IntakeSourceNameMaxRunes = 100
	// IntakeSourceURLMaxLength caps the link to the document in the other
	// system.
	IntakeSourceURLMaxLength = 2048
)

// IntakeDocumentState says what became of a delivered document.
type IntakeDocumentState string

const (
	// IntakeDocumentPending has no draft yet: the delivery stopped before it
	// made one, and a repeat of it finishes the work.
	IntakeDocumentPending IntakeDocumentState = "pending"
	// IntakeDocumentDraft has an open draft waiting for review.
	IntakeDocumentDraft IntakeDocumentState = "draft"
	// IntakeDocumentItem became an item.
	IntakeDocumentItem IntakeDocumentState = "item"
	// IntakeDocumentRemoved had its draft discarded or its item deleted. A
	// repeat of the delivery does not bring it back.
	IntakeDocumentRemoved IntakeDocumentState = "removed"
)

// IntakeDocument is a document another system delivered to the group through
// the intake endpoint (#2809), such as an invoice kept in Paperless-ngx.
// ExternalSystem, ExternalInstance and ExternalDocumentID identify it: the
// name of the system, the installation (usually its base URL) and the
// document's id there. Together with the group they are unique, so a delivery
// the other system repeats finds the document it made before.
//
// The document is the delivery's lasting record. Deleting the intake session
// that carried it, discarding its draft or promoting the draft to an item
// leaves the document in place, so a repeated delivery never makes a second
// draft. DraftID is the draft the delivery made; CommodityID is the item that
// draft became, set by the promotion in the transaction that creates the item.
// Neither has a foreign key: a draft can be discarded and an item deleted. A
// DraftID that is set but empty marks a draft the delivery made that a backup
// restore did not bring back; the document still counts as delivered.
//
// The document's identity is not the identity of the product it describes:
// the product is a candidate of the intake session, and its draft carries the
// candidate's client key.
//
// PayloadHash fingerprints what was delivered, the values and the file's
// content. A repeat with the same fingerprint is a replay; a different
// fingerprint means the document changed, which the endpoint refuses rather
// than make a second draft.
//
//ptah:schema:rls:enable table="intake_documents" comment="Enable RLS for multi-tenant intake document isolation"
//ptah:schema:rls:policy name="intake_document_isolation" table="intake_documents" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures intake documents can only be accessed and modified by their tenant and group with required contexts"
//ptah:schema:rls:policy name="intake_document_background_worker_access" table="intake_documents" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to read and purge intake documents across all groups"
//ptah:schema:table name="intake_documents"
type IntakeDocument struct {
	//ptah:embedded mode="inline"
	TenantGroupAwareEntityID

	//ptah:schema:field name="external_system" type="TEXT" not_null="true"
	ExternalSystem string `json:"external_system" db:"external_system"`

	//ptah:schema:field name="external_instance" type="TEXT" not_null="true" default=""
	ExternalInstance string `json:"external_instance" db:"external_instance"`

	//ptah:schema:field name="external_document_id" type="TEXT" not_null="true"
	ExternalDocumentID string `json:"external_document_id" db:"external_document_id"`

	// SourceName is how the other system is shown, e.g. "Paperless-ngx".
	//ptah:schema:field name="source_name" type="TEXT" not_null="true" default=""
	SourceName string `json:"source_name" db:"source_name"`

	// SourceURL links to the document in the other system. It is stored and
	// shown, never fetched.
	//ptah:schema:field name="source_url" type="TEXT" not_null="true" default=""
	SourceURL string `json:"source_url" db:"source_url"`

	//ptah:schema:field name="payload_hash" type="TEXT" not_null="true"
	PayloadHash string `json:"-" db:"payload_hash" userinput:"false"`

	// FileID is the delivered file, stored as a file of the group.
	//ptah:schema:field name="file_id" type="TEXT"
	FileID *string `json:"file_id" db:"file_id" userinput:"false" extensions:"x-nullable=true"`

	// SessionID is the intake session that carried the delivery. The
	// session can be deleted, so there is no foreign key.
	//ptah:schema:field name="session_id" type="TEXT"
	SessionID *string `json:"session_id" db:"session_id" userinput:"false" extensions:"x-nullable=true"`

	//ptah:schema:field name="draft_id" type="TEXT"
	DraftID *string `json:"draft_id" db:"draft_id" userinput:"false" extensions:"x-nullable=true"`

	//ptah:schema:field name="commodity_id" type="TEXT"
	CommodityID *string `json:"commodity_id" db:"commodity_id" userinput:"false" extensions:"x-nullable=true"`

	//ptah:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`

	//ptah:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// IntakeDocumentIndexes defines the postgres indexes for intake_documents.
type IntakeDocumentIndexes struct {
	// Unique index for the immutable UUID.
	//ptah:schema:index name="idx_intake_documents_uuid" fields="uuid" unique="true" table="intake_documents"
	_ int

	// One document per external identity in a group: a repeated delivery
	// finds the document it made.
	//ptah:schema:index name="idx_intake_documents_external" fields="group_id,external_system,external_instance,external_document_id" unique="true" table="intake_documents"
	_ int

	// A draft shows the document it came from.
	//ptah:schema:index name="idx_intake_documents_group_draft" fields="group_id,draft_id" table="intake_documents" condition="draft_id IS NOT NULL"
	_ int

	// An item shows the document it came from; the promotion finds the
	// document by its draft.
	//ptah:schema:index name="idx_intake_documents_group_commodity" fields="group_id,commodity_id" table="intake_documents" condition="commodity_id IS NOT NULL"
	_ int

	// Composite index for tenant+group RLS-filtered queries and the purges.
	//ptah:schema:index name="idx_intake_documents_tenant_group" fields="tenant_id,group_id" table="intake_documents"
	_ int
}

func (*IntakeDocument) Validate() error {
	return ErrMustUseValidateWithContext
}

// ValidateWithContext checks a stored document.
func (d *IntakeDocument) ValidateWithContext(ctx context.Context) error {
	if err := validation.ValidateStructWithContext(ctx, d,
		validation.Field(&d.TenantGroupAwareEntityID),
		validation.Field(&d.PayloadHash, rules.NotEmpty),
	); err != nil {
		return err
	}
	return d.ValidateFields(ctx)
}

// ValidateFields checks the delivered fields only, before the registry fills
// in the tenant, the group and the author.
func (d *IntakeDocument) ValidateFields(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, d,
		validation.Field(&d.ExternalSystem, rules.NotEmpty, validation.Length(1, IntakeExternalValueMaxLength)),
		validation.Field(&d.ExternalInstance, validation.Length(0, IntakeExternalValueMaxLength)),
		validation.Field(&d.ExternalDocumentID, rules.NotEmpty, validation.Length(1, IntakeExternalValueMaxLength)),
		validation.Field(&d.SourceName, validation.RuneLength(0, IntakeSourceNameMaxRunes)),
		validation.Field(&d.SourceURL, validation.Length(0, IntakeSourceURLMaxLength), IntakeSourceURLRule),
	)
}

// IntakeSourceURLRule accepts an empty value or an absolute http(s) URL with
// a host and without credentials. The URL is shown as a link, so a userinfo
// part would put another system's password on the item page.
var IntakeSourceURLRule = validation.By(func(value any) error {
	s, _ := value.(string)
	if s == "" {
		return nil
	}
	if err := validateAbsoluteHTTPURL(s); err != nil {
		return err
	}
	if u, err := url.Parse(s); err != nil || u.User != nil {
		return errURLCredentials
	}
	return nil
})

var errURLCredentials = validation.NewError("validation_url_credentials", "must not carry a user name or password")
