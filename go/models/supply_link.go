package models

import (
	"context"
	"net/url"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

var (
	_ validation.Validatable            = (*SupplyLink)(nil)
	_ validation.ValidatableWithContext = (*SupplyLink)(nil)
	_ TenantGroupAwareIDable            = (*SupplyLink)(nil)
)

// SupplyLink is a per-commodity "where do I re-buy the consumable for
// this" URL — vacuum bags, water-filter cartridges, AC filters, etc.
// (issue #1369 Phase 1).
//
// Distinct from the generic Commodity.URLs slice: that field captures
// product page / manual / support links and is undifferentiated. Supply
// links carry a Label so the user can answer "I'm at the store, what
// do I need to re-buy for this?" without opening each URL.
//
// Modelled as its own table (rather than extending URLs to a typed
// JSONB) so we can later answer cross-commodity questions ("what
// supplies do I have on order across the household?") with plain SQL.
// Mirrors the per-commodity relationship of commodity_loans /
// commodity_services to commodities.
//
// Phase 2 (price-drop alerts) is explicitly out of scope and tracked
// as a follow-up issue. No price columns ship here.
//
// Enable RLS for multi-tenant isolation.
//
//migrator:schema:rls:enable table="commodity_supply_links" comment="Enable RLS for multi-tenant supply-link isolation"
//migrator:schema:rls:policy name="supply_link_isolation" table="commodity_supply_links" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures supply links can only be accessed and modified by their tenant and group with required contexts"
//migrator:schema:rls:policy name="supply_link_background_worker_access" table="commodity_supply_links" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all supply links for processing"
//migrator:schema:table name="commodity_supply_links"
type SupplyLink struct {
	//migrator:embedded mode="inline"
	TenantGroupAwareEntityID

	// CommodityID — the consumable's parent item. ON DELETE CASCADE is
	// added manually to the generated migration (mirrors commodity_loans):
	// hard-deleting a commodity drops its supply links — the link only
	// exists in the context of that item.
	//migrator:schema:field name="commodity_id" type="TEXT" not_null="true" foreign="commodities(id)" foreign_key_name="fk_supply_link_commodity" on_delete="CASCADE"
	CommodityID string `json:"commodity_id" db:"commodity_id"`

	// Label names the consumable ("Water filter", "Vacuum bags M-style").
	// Required, capped at 200 chars to match similar text caps.
	//migrator:schema:field name="label" type="TEXT" not_null="true"
	Label string `json:"label" db:"label"`

	// URL is the re-buy link. Required. Validated as an absolute URL
	// (http/https) — the value is rendered as an <a target="_blank">
	// on the detail card, so relative URLs would silently break.
	//migrator:schema:field name="url" type="TEXT" not_null="true"
	URL string `json:"url" db:"url"`

	// Notes is an optional aide-mémoire ("buy 2-pack, lasts ~6mo",
	// "matches socket type GU10"). Capped at 1000 chars to mirror
	// other free-form note fields in the project.
	//migrator:schema:field name="notes" type="TEXT"
	Notes string `json:"notes" db:"notes"`

	// SortOrder lets the user reorder rows in the form. Persistent so
	// the order survives reload. Densely renumbered server-side on
	// every reorder (no gaps to worry about) — see SupplyLinkService.
	//migrator:schema:field name="sort_order" type="INTEGER" not_null="true" default="0"
	SortOrder int `json:"sort_order" db:"sort_order"`

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`

	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// SupplyLinkIndexes carries the migrator:schema:index tags. The empty
// struct fields are a Ptah convention — only the tags are read.
type SupplyLinkIndexes struct {
	// Unique index for the immutable UUID (deduplication key for
	// import/restore).
	//migrator:schema:index name="idx_supply_links_uuid" fields="uuid" unique="true" table="commodity_supply_links"
	_ int

	// Index for tenant-based queries.
	//migrator:schema:index name="idx_supply_links_tenant_id" fields="tenant_id" table="commodity_supply_links"
	_ int

	// Composite index for tenant+group RLS-filtered queries.
	//migrator:schema:index name="idx_supply_links_tenant_group" fields="tenant_id,group_id" table="commodity_supply_links"
	_ int

	// Composite index for the per-commodity render path. The Supplies
	// card reads `WHERE commodity_id = ? ORDER BY sort_order, created_at`
	// — both columns covered.
	//migrator:schema:index name="idx_supply_links_commodity" fields="commodity_id,sort_order" table="commodity_supply_links"
	_ int
}

func (*SupplyLink) Validate() error {
	return ErrMustUseValidateWithContext
}

func (s *SupplyLink) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, s,
		validation.Field(&s.CommodityID, rules.NotEmpty),
		validation.Field(&s.Label, rules.NotEmpty, validation.Length(1, 200)),
		validation.Field(&s.URL, rules.NotEmpty, validation.Length(1, 2048), validation.By(validateAbsoluteHTTPURL)),
		validation.Field(&s.Notes, validation.Length(0, 1000)),
	)
}

// validateAbsoluteHTTPURL accepts only absolute http(s) URLs. Stored
// values are rendered as an external <a target="_blank"> on the
// detail card and clicked via window.open(); relative or scheme-less
// strings would silently break or open the wrong target.
func validateAbsoluteHTTPURL(value any) error {
	s, _ := value.(string)
	if s == "" {
		return nil // length/empty rules handle the "missing" case
	}
	u, err := url.Parse(s)
	if err != nil {
		return errInvalidURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errInvalidURL
	}
	if u.Host == "" {
		return errInvalidURL
	}
	return nil
}

var errInvalidURL = validation.NewError("validation_invalid_url", "must be an absolute http(s) URL")
