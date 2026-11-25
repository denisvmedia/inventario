package models

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/jellydator/validation"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"

	"go.5x5.cz/inventario/models/rules"
)

// Limits on a seller's fields. They count runes, so a Cyrillic or CJK name
// gets the same room as a Latin one. The frontend checks the same limits.
const (
	MaxSellerNameRunes    = 100
	MaxSellerWebsiteRunes = 2048
	MaxSellerContactRunes = 500
	MaxSellerNoteRunes    = 2000
)

var (
	_ validation.Validatable            = (*Seller)(nil)
	_ validation.ValidatableWithContext = (*Seller)(nil)
	_ TenantGroupAwareIDable            = (*Seller)(nil)
)

// Seller is a shop or person a group buys items from (#2804). Items point at
// it through Commodity.SellerID. A seller belongs to one group, and its name
// is unique in the group regardless of case, Unicode normalization and runs
// of spaces: NameKey holds the folded name and carries the unique index.
// Deleting a seller leaves its items without one (ON DELETE SET NULL).
//
//ptah:schema:rls:enable table="sellers" comment="Enable RLS for multi-tenant seller isolation"
//ptah:schema:rls:policy name="seller_isolation" table="sellers" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures sellers can only be accessed and modified by their tenant and group with required contexts"
//ptah:schema:rls:policy name="seller_background_worker_access" table="sellers" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all sellers for processing"
//ptah:schema:table name="sellers"
type Seller struct {
	//ptah:embedded mode="inline"
	TenantGroupAwareEntityID

	// Name is the seller as shown on items and in the sellers list.
	//ptah:schema:field name="name" type="TEXT" not_null="true"
	Name string `json:"name" db:"name"`

	// NameKey is SellerNameKey(Name). The registries set it on every write;
	// it is not part of the API.
	//ptah:schema:field name="name_key" type="TEXT" not_null="true"
	NameKey string `json:"-" db:"name_key" userinput:"false"`

	// Website is an absolute http(s) URL, or empty.
	//ptah:schema:field name="website" type="TEXT" not_null="true" default=""
	Website string `json:"website" db:"website"`

	// Contact is free text: an email address, a phone number, a person.
	//ptah:schema:field name="contact" type="TEXT" not_null="true" default=""
	Contact string `json:"contact" db:"contact"`

	//ptah:schema:field name="note" type="TEXT" not_null="true" default=""
	Note string `json:"note" db:"note"`

	//ptah:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`

	//ptah:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// SellerIndexes defines the indexes of the sellers table.
type SellerIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore).
	//ptah:schema:index name="idx_sellers_uuid" fields="uuid" unique="true" table="sellers"
	_ int

	// Composite index for tenant+group RLS-filtered queries.
	//ptah:schema:index name="idx_sellers_tenant_group" fields="tenant_id,group_id" table="sellers"
	_ int

	// One seller per name in a group; see SellerNameKey.
	//ptah:schema:index name="idx_sellers_group_name_key" fields="group_id,name_key" unique="true" table="sellers"
	_ int
}

// SellerNameKey folds a seller name for the uniqueness check: Unicode NFC,
// full case folding, and every run of white space reduced to one space with
// none at either end. "Amazon", "AMAZON" and " amazon " give the same key.
func SellerNameKey(name string) string {
	// A Caser keeps state between calls and must not be shared across
	// goroutines, so each call builds its own.
	folded := norm.NFC.String(cases.Fold().String(norm.NFC.String(name)))
	return strings.Join(strings.FieldsFunc(folded, unicode.IsSpace), " ")
}

// Normalize trims the fields and recomputes NameKey. The registries call it
// before every write.
func (s *Seller) Normalize() {
	s.Name = strings.Join(strings.Fields(s.Name), " ")
	s.Website = strings.TrimSpace(s.Website)
	s.Contact = strings.TrimSpace(s.Contact)
	s.Note = strings.TrimSpace(s.Note)
	s.NameKey = SellerNameKey(s.Name)
}

func (*Seller) Validate() error {
	return ErrMustUseValidateWithContext
}

// ValidateWithContext validates the whole row, the group-scoped identity
// included.
func (s *Seller) ValidateWithContext(ctx context.Context) error {
	if err := validation.ValidateStructWithContext(ctx, s,
		validation.Field(&s.TenantGroupAwareEntityID),
	); err != nil {
		return err
	}
	return s.ValidateFields(ctx)
}

// ValidateFields validates the fields a user sets, before the registry fills
// in tenant, group and author.
func (s *Seller) ValidateFields(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, s,
		validation.Field(&s.Name, rules.NotEmpty, validation.RuneLength(1, MaxSellerNameRunes)),
		validation.Field(&s.Website, validation.RuneLength(0, MaxSellerWebsiteRunes), validation.By(validateAbsoluteHTTPURL)),
		validation.Field(&s.Contact, validation.RuneLength(0, MaxSellerContactRunes)),
		validation.Field(&s.Note, validation.RuneLength(0, MaxSellerNoteRunes)),
	)
}
