package models

import (
	"context"
	"slices"
	"time"

	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models/rules"
)

// Limits on saved views. They are checked on write; the frontend counts
// against the same numbers.
const (
	// MaxSavedViewNameRunes caps a view's name, counted in runes.
	MaxSavedViewNameRunes = 60
	// MaxSavedViewQueryBytes caps the stored query string. The item query
	// contract bounds every parameter on its own; this is a guard against a
	// request that repeats one many times.
	MaxSavedViewQueryBytes = 4096
	// MaxSavedViewsPerUser is how many views one member may create in one
	// group, personal and shared together.
	MaxSavedViewsPerUser = 50
	// MaxSharedSavedViews is how many views a group may share.
	MaxSharedSavedViews = 50
)

// SavedViewQueryVersion is the version of the item query contract the
// stored query strings are written in. A view stored under another version
// is not run: it is reported as needing an update, so a parameter whose
// meaning changed can neither broaden nor change what the view selects.
const SavedViewQueryVersion = 1

// SavedViewEntity names the list a saved view filters. Only the items list
// has views; the column leaves room for others.
type SavedViewEntity string

const (
	SavedViewEntityCommodities SavedViewEntity = "commodities"
)

// Validate makes SavedViewEntity a validation.Validatable.
func (e SavedViewEntity) Validate() error {
	if e != SavedViewEntityCommodities {
		return validation.NewError("invalid_saved_view_entity", "entity must be commodities")
	}
	return nil
}

// SavedViewLayout is how the list shows the items of a view. Empty means
// the viewer's own default.
type SavedViewLayout string

const (
	SavedViewLayoutDefault SavedViewLayout = ""
	SavedViewLayoutGrid    SavedViewLayout = "grid"
	SavedViewLayoutList    SavedViewLayout = "list"
)

// IsValid reports whether l is one of the known layouts, the empty default
// included.
func (l SavedViewLayout) IsValid() bool {
	switch l {
	case SavedViewLayoutDefault, SavedViewLayoutGrid, SavedViewLayoutList:
		return true
	}
	return false
}

// Validate makes SavedViewLayout a validation.Validatable.
func (l SavedViewLayout) Validate() error {
	if !l.IsValid() {
		return validation.NewError("invalid_saved_view_layout", "layout must be grid, list or empty")
	}
	return nil
}

// SavedViewIcon is the closed set of icons a view can carry. The frontend
// maps each name to a Lucide icon; both sides change together.
type SavedViewIcon string

const (
	SavedViewIconBookmark SavedViewIcon = "bookmark"
	SavedViewIconStar     SavedViewIcon = "star"
	SavedViewIconHeart    SavedViewIcon = "heart"
	SavedViewIconTag      SavedViewIcon = "tag"
	SavedViewIconShield   SavedViewIcon = "shield"
	SavedViewIconClock    SavedViewIcon = "clock"
	SavedViewIconAlert    SavedViewIcon = "alert"
	SavedViewIconWrench   SavedViewIcon = "wrench"
	SavedViewIconPackage  SavedViewIcon = "package"
	SavedViewIconHome     SavedViewIcon = "home"
	SavedViewIconCar      SavedViewIcon = "car"
	SavedViewIconZap      SavedViewIcon = "zap"
	SavedViewIconGift     SavedViewIcon = "gift"
	SavedViewIconArchive  SavedViewIcon = "archive"
	SavedViewIconBanknote SavedViewIcon = "banknote"
	SavedViewIconMapPin   SavedViewIcon = "map-pin"
)

// ValidSavedViewIcons lists every icon in the order the picker shows them.
var ValidSavedViewIcons = []SavedViewIcon{
	SavedViewIconBookmark,
	SavedViewIconStar,
	SavedViewIconHeart,
	SavedViewIconTag,
	SavedViewIconShield,
	SavedViewIconClock,
	SavedViewIconAlert,
	SavedViewIconWrench,
	SavedViewIconPackage,
	SavedViewIconHome,
	SavedViewIconCar,
	SavedViewIconZap,
	SavedViewIconGift,
	SavedViewIconArchive,
	SavedViewIconBanknote,
	SavedViewIconMapPin,
}

// IsValid reports whether i is one of ValidSavedViewIcons.
func (i SavedViewIcon) IsValid() bool {
	return slices.Contains(ValidSavedViewIcons, i)
}

// Validate makes SavedViewIcon a validation.Validatable.
func (i SavedViewIcon) Validate() error {
	if !i.IsValid() {
		return validation.NewError("invalid_saved_view_icon", "unknown icon")
	}
	return nil
}

var (
	_ validation.Validatable            = (*SavedView)(nil)
	_ validation.ValidatableWithContext = (*SavedView)(nil)
	_ TenantGroupAwareIDable            = (*SavedView)(nil)
)

// SavedView keeps a filter of a list under a name: the query string of the
// item query contract (services/commodityquery), its sort included, plus how
// to show it. Opening a view is a navigation to the list with that query, so
// the list, the sidebar count and the dashboard widget all ask the same
// question.
//
// A view belongs to the member who created it (created_by_user_id). It is
// personal unless Shared is set, in which case every member of the group sees
// it. RLS scopes rows to the tenant and the group; which of those rows a
// member sees is decided in application logic, because the group transaction
// carries no user context. Sharing a view grants no access to data: each
// member runs its query in their own group scope.
//
//ptah:schema:rls:enable table="saved_views" comment="Enable RLS for multi-tenant saved view isolation"
//ptah:schema:rls:policy name="saved_view_isolation" table="saved_views" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures saved views can only be accessed and modified by their tenant and group with required contexts; per-user visibility is applied in application logic"
//ptah:schema:rls:policy name="saved_view_background_worker_access" table="saved_views" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all saved views for purges"
//ptah:schema:table name="saved_views"
type SavedView struct {
	//ptah:embedded mode="inline"
	TenantGroupAwareEntityID

	// Name is what the sidebar and the dashboard show.
	//ptah:schema:field name="name" type="TEXT" not_null="true"
	Name string `json:"name" db:"name"`

	// Icon is one of ValidSavedViewIcons.
	//ptah:schema:field name="icon" type="TEXT" not_null="true" default="bookmark"
	Icon SavedViewIcon `json:"icon" db:"icon"`

	// Entity names the list the view filters.
	//ptah:schema:field name="entity" type="TEXT" not_null="true" default="commodities"
	Entity SavedViewEntity `json:"entity" db:"entity"`

	// Query is the canonical query string of the view (without the leading
	// "?"), in the API names of the item query contract. It is validated
	// and re-encoded on every write.
	//ptah:schema:field name="query" type="TEXT" not_null="true" default=""
	Query string `json:"query" db:"query"`

	// QueryVersion is the SavedViewQueryVersion Query was written in.
	//ptah:schema:field name="query_version" type="INTEGER" not_null="true" default="1"
	QueryVersion int `json:"query_version" db:"query_version"`

	// Layout is grid, list, or empty for the viewer's default.
	//ptah:schema:field name="layout" type="TEXT" not_null="true" default=""
	Layout SavedViewLayout `json:"layout" db:"layout"`

	// Shared makes the view visible to every member of the group. Where a
	// member shows a view (sidebar, dashboard) is their own choice, kept in
	// SavedViewPlacement, so sharing puts the view in nobody else's sidebar.
	//ptah:schema:field name="shared" type="BOOLEAN" not_null="true" default="false"
	Shared bool `json:"shared" db:"shared"`

	// SortOrder orders the views: ascending, then by name. A new view goes
	// after every view of the group. Only the registry changes it (on create
	// and on reorder); an update keeps the stored value.
	//ptah:schema:field name="sort_order" type="INTEGER" not_null="true" default="0"
	SortOrder int `json:"sort_order" db:"sort_order"`

	//ptah:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`

	//ptah:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// SavedViewIndexes defines the postgres indexes for saved_views.
type SavedViewIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore).
	//ptah:schema:index name="idx_saved_views_uuid" fields="uuid" unique="true" table="saved_views"
	_ int

	//ptah:schema:index name="idx_saved_views_tenant_id" fields="tenant_id" table="saved_views"
	_ int

	// Every read is "the views of this group", ordered by sort_order.
	//ptah:schema:index name="idx_saved_views_tenant_group_order" fields="tenant_id,group_id,sort_order" table="saved_views"
	_ int

	// Account deletion removes the views a user created.
	//ptah:schema:index name="idx_saved_views_created_by" fields="created_by_user_id" table="saved_views"
	_ int
}

var (
	_ validation.Validatable            = (*SavedViewPlacement)(nil)
	_ validation.ValidatableWithContext = (*SavedViewPlacement)(nil)
)

// SavedViewPlacement is where one member shows one saved view: in their
// sidebar with its item count, on their dashboard with its first items, both
// or neither. A view without a placement row for a member is shown nowhere
// for them. Each member places the views they see, their own and shared
// ones, so sharing or unpinning a view changes nothing for anyone else.
//
//ptah:schema:rls:enable table="saved_view_placements" comment="Enable RLS for multi-tenant saved view placement isolation"
//ptah:schema:rls:policy name="saved_view_placement_isolation" table="saved_view_placements" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures saved view placements can only be accessed and modified by their tenant and group with required contexts; the member filter is applied in application logic"
//ptah:schema:rls:policy name="saved_view_placement_background_worker_access" table="saved_view_placements" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all saved view placements for purges"
//ptah:schema:table name="saved_view_placements"
type SavedViewPlacement struct {
	//ptah:embedded mode="inline"
	TenantAwareEntityID

	//ptah:schema:field name="group_id" type="TEXT" not_null="true" foreign="location_groups(id)" foreign_key_name="fk_saved_view_placement_group"
	GroupID string `json:"-" db:"group_id" userinput:"false"`

	// SavedViewID is the view placed. Deleting the view deletes its
	// placements.
	//ptah:schema:field name="saved_view_id" type="TEXT" not_null="true" foreign="saved_views(id)" foreign_key_name="fk_saved_view_placement_view" on_delete="CASCADE"
	SavedViewID string `json:"saved_view_id" db:"saved_view_id"`

	// UserID is the member the placement belongs to.
	//ptah:schema:field name="user_id" type="TEXT" not_null="true" foreign="users(id)" foreign_key_name="fk_saved_view_placement_user"
	UserID string `json:"-" db:"user_id" userinput:"false"`

	// ShowInSidebar lists the view in the member's sidebar with its count.
	//ptah:schema:field name="show_in_sidebar" type="BOOLEAN" not_null="true" default="false"
	ShowInSidebar bool `json:"show_in_sidebar" db:"show_in_sidebar"`

	// ShowOnDashboard shows the view on the member's dashboard.
	//ptah:schema:field name="show_on_dashboard" type="BOOLEAN" not_null="true" default="false"
	ShowOnDashboard bool `json:"show_on_dashboard" db:"show_on_dashboard"`

	//ptah:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`

	//ptah:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// SavedViewPlacementIndexes defines the postgres indexes for
// saved_view_placements.
type SavedViewPlacementIndexes struct {
	//ptah:schema:index name="idx_saved_view_placements_uuid" fields="uuid" unique="true" table="saved_view_placements"
	_ int

	// One placement per member and view; the upsert conflicts on it.
	//ptah:schema:index name="idx_saved_view_placements_view_user" fields="saved_view_id,user_id" unique="true" table="saved_view_placements"
	_ int

	// Every read is "this member's placements in this group".
	//ptah:schema:index name="idx_saved_view_placements_group_user" fields="tenant_id,group_id,user_id" table="saved_view_placements"
	_ int

	// Account deletion removes the placements of a user.
	//ptah:schema:index name="idx_saved_view_placements_user" fields="user_id" table="saved_view_placements"
	_ int
}

func (*SavedViewPlacement) Validate() error {
	return ErrMustUseValidateWithContext
}

func (p *SavedViewPlacement) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, p,
		validation.Field(&p.SavedViewID, validation.Required),
	)
}

// VisibleTo reports whether the member userID sees the view: their own views
// and every shared one.
func (v *SavedView) VisibleTo(userID string) bool {
	return v.Shared || v.CreatedByUserID == userID
}

func (*SavedView) Validate() error {
	return ErrMustUseValidateWithContext
}

func (v *SavedView) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, v,
		validation.Field(&v.Name, rules.NotEmpty, validation.RuneLength(1, MaxSavedViewNameRunes)),
		validation.Field(&v.Icon, validation.Required),
		validation.Field(&v.Entity, validation.Required),
		validation.Field(&v.Query, validation.Length(0, MaxSavedViewQueryBytes)),
		validation.Field(&v.QueryVersion, validation.Required),
		validation.Field(&v.Layout),
	)
}
