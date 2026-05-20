package jsonapi

import (
	"net/http"
	"time"

	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// AdminTenantListItem is the row shape returned by GET
// /admin/tenants. Mirrors the JSON-API flat-data convention used across
// the codebase (resource fields hoisted to the top level alongside `id`
// + `type`).
//
// The two `*_count` columns are pre-computed by the registry layer via
// correlated subqueries so the FE doesn't need a second roundtrip per
// row to render the at-a-glance table.
type AdminTenantListItem struct {
	ID               string              `json:"id"`
	Type             string              `json:"type" example:"admin_tenants" enums:"admin_tenants"`
	Name             string              `json:"name"`
	Slug             string              `json:"slug"`
	Domain           *string             `json:"domain,omitempty"`
	Status           models.TenantStatus `json:"status"`
	IsDefault        bool                `json:"is_default"`
	PlanID           string              `json:"plan_id"`
	RegistrationMode string              `json:"registration_mode,omitempty"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
	UserCount        int                 `json:"user_count"`
	GroupCount       int                 `json:"group_count"`
}

// AdminListMeta is the meta block on the admin paginated listings. The
// duplicate `events`-style `count` field that
// jsonapi.CommodityEventsMeta uses is replaced by `page`/`per_page`/
// `total`/`total_pages` so the admin FE can render real pagers (the
// commodity-events surface only has "load more" semantics, which is
// not enough for the admin tables).
type AdminListMeta struct {
	Page       int `json:"page" example:"1"`
	PerPage    int `json:"per_page" example:"50"`
	Total      int `json:"total" example:"100"`
	TotalPages int `json:"total_pages" example:"2"`
}

// AdminTenantsResponse is the JSON:API envelope for GET /admin/tenants.
type AdminTenantsResponse struct {
	Data []*AdminTenantListItem `json:"data"`
	Meta AdminListMeta          `json:"meta"`
}

// NewAdminTenantsResponse maps registry-layer rows into the wire-shape
// the FE consumes. Page / PerPage / Total drive the meta block (with
// total_pages computed via ComputeTotalPages so the FE never has to do
// the ceil-divide).
func NewAdminTenantsResponse(items []*registry.AdminTenantListItem, page, perPage, total int) *AdminTenantsResponse {
	data := make([]*AdminTenantListItem, 0, len(items))
	for _, it := range items {
		if it == nil || it.Tenant == nil {
			continue
		}
		t := it.Tenant
		data = append(data, &AdminTenantListItem{
			ID:               t.ID,
			Type:             "admin_tenants",
			Name:             t.Name,
			Slug:             t.Slug,
			Domain:           t.Domain,
			Status:           t.Status,
			IsDefault:        t.IsDefault,
			PlanID:           t.PlanID,
			RegistrationMode: string(t.RegistrationMode),
			CreatedAt:        t.CreatedAt,
			UpdatedAt:        t.UpdatedAt,
			UserCount:        it.UserCount,
			GroupCount:       it.GroupCount,
		})
	}
	return &AdminTenantsResponse{
		Data: data,
		Meta: AdminListMeta{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: ComputeTotalPages(total, perPage),
		},
	}
}

func (*AdminTenantsResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// AdminTenantResponse is the JSON:API envelope for GET
// /admin/tenants/{tenantID}. Same row shape as the list item — the
// detail endpoint is "one list row, no envelope meta" per the issue
// spec.
type AdminTenantResponse struct {
	Data *AdminTenantListItem `json:"data"`
}

// NewAdminTenantResponse wraps a single tenant row for the detail
// endpoint.
func NewAdminTenantResponse(item *registry.AdminTenantListItem) *AdminTenantResponse {
	if item == nil || item.Tenant == nil {
		return &AdminTenantResponse{}
	}
	t := item.Tenant
	return &AdminTenantResponse{
		Data: &AdminTenantListItem{
			ID:               t.ID,
			Type:             "admin_tenants",
			Name:             t.Name,
			Slug:             t.Slug,
			Domain:           t.Domain,
			Status:           t.Status,
			IsDefault:        t.IsDefault,
			PlanID:           t.PlanID,
			RegistrationMode: string(t.RegistrationMode),
			CreatedAt:        t.CreatedAt,
			UpdatedAt:        t.UpdatedAt,
			UserCount:        item.UserCount,
			GroupCount:       item.GroupCount,
		},
	}
}

func (*AdminTenantResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// AdminUserListItem is the row shape returned by GET
// /admin/tenants/{tenantID}/users. `last_login_at` is a *time.Time so
// the FE can distinguish "never logged in" from "logged in long ago" —
// `omitempty` keeps the wire payload tight for never-logged-in users.
type AdminUserListItem struct {
	ID                   string     `json:"id"`
	Type                 string     `json:"type" example:"admin_users" enums:"admin_users"`
	Email                string     `json:"email"`
	Name                 string     `json:"name"`
	IsActive             bool       `json:"is_active"`
	LastLoginAt          *time.Time `json:"last_login_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	GroupMembershipCount int        `json:"group_membership_count"`
}

// AdminUsersResponse is the JSON:API envelope for GET
// /admin/tenants/{tenantID}/users.
type AdminUsersResponse struct {
	Data []*AdminUserListItem `json:"data"`
	Meta AdminListMeta        `json:"meta"`
}

// NewAdminUsersResponse maps registry-layer rows into the wire-shape
// the FE consumes.
func NewAdminUsersResponse(items []*registry.AdminUserListItem, page, perPage, total int) *AdminUsersResponse {
	data := make([]*AdminUserListItem, 0, len(items))
	for _, it := range items {
		if it == nil || it.User == nil {
			continue
		}
		u := it.User
		data = append(data, &AdminUserListItem{
			ID:                   u.ID,
			Type:                 "admin_users",
			Email:                u.Email,
			Name:                 u.Name,
			IsActive:             u.IsActive,
			LastLoginAt:          u.LastLoginAt,
			CreatedAt:            u.CreatedAt,
			GroupMembershipCount: it.GroupMembershipCount,
		})
	}
	return &AdminUsersResponse{
		Data: data,
		Meta: AdminListMeta{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: ComputeTotalPages(total, perPage),
		},
	}
}

func (*AdminUsersResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// AdminUserGroupMembership is the per-membership row on the admin
// user-detail endpoint. The slug + name are denormalised so the FE can
// render a memberships list without a second round-trip per row.
type AdminUserGroupMembership struct {
	GroupID   string           `json:"group_id"`
	GroupSlug string           `json:"group_slug"`
	GroupName string           `json:"group_name"`
	Role      models.GroupRole `json:"role"`
	JoinedAt  time.Time        `json:"joined_at"`
}

// AdminUserDetail is the row shape returned by GET /admin/users/{userID}.
// Compared to AdminUserListItem this adds `tenant_id`, the resolved
// group memberships, and `active_session_count`.
type AdminUserDetail struct {
	ID                 string                     `json:"id"`
	Type               string                     `json:"type" example:"admin_users" enums:"admin_users"`
	Email              string                     `json:"email"`
	Name               string                     `json:"name"`
	TenantID           string                     `json:"tenant_id"`
	IsActive           bool                       `json:"is_active"`
	IsSystemAdmin      bool                       `json:"is_system_admin"`
	LastLoginAt        *time.Time                 `json:"last_login_at,omitempty"`
	CreatedAt          time.Time                  `json:"created_at"`
	UpdatedAt          time.Time                  `json:"updated_at"`
	GroupMemberships   []AdminUserGroupMembership `json:"group_memberships"`
	ActiveSessionCount int                        `json:"active_session_count"`
}

// AdminUserResponse is the JSON:API envelope for GET /admin/users/{userID}.
type AdminUserResponse struct {
	Data *AdminUserDetail `json:"data"`
}

// AdminUserResponseInput carries the inputs NewAdminUserResponse needs
// without forcing handlers to construct a giant struct literal. The
// memberships are pre-joined to (group_id, group_slug, group_name)
// triplets at the handler so the JSON-API package stays decoupled from
// the LocationGroup model.
type AdminUserResponseInput struct {
	User               *models.User
	Memberships        []AdminUserGroupMembership
	ActiveSessionCount int
}

// NewAdminUserResponse wraps a single user row for the detail endpoint.
func NewAdminUserResponse(in AdminUserResponseInput) *AdminUserResponse {
	if in.User == nil {
		return &AdminUserResponse{}
	}
	u := in.User
	memberships := in.Memberships
	if memberships == nil {
		// Always serialise as an empty array (never null) so the FE can
		// safely .map(...) without a null check.
		memberships = []AdminUserGroupMembership{}
	}
	return &AdminUserResponse{
		Data: &AdminUserDetail{
			ID:                 u.ID,
			Type:               "admin_users",
			Email:              u.Email,
			Name:               u.Name,
			TenantID:           u.TenantID,
			IsActive:           u.IsActive,
			IsSystemAdmin:      u.IsSystemAdmin,
			LastLoginAt:        u.LastLoginAt,
			CreatedAt:          u.CreatedAt,
			UpdatedAt:          u.UpdatedAt,
			GroupMemberships:   memberships,
			ActiveSessionCount: in.ActiveSessionCount,
		},
	}
}

func (*AdminUserResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// AdminGroupListItem is the row shape returned by GET /admin/groups.
// Mirrors the JSON-API flat-data convention used across the codebase
// (resource fields hoisted to the top level alongside `id` + `type`).
//
// member_count is pre-computed by the registry layer via a correlated
// subquery on group_memberships (accepted memberships only) so the FE
// doesn't need a second roundtrip per row to render the at-a-glance
// table.
type AdminGroupListItem struct {
	ID          string                     `json:"id"`
	Type        string                     `json:"type" example:"admin_groups" enums:"admin_groups"`
	Name        string                     `json:"name"`
	Slug        string                     `json:"slug"`
	Status      models.LocationGroupStatus `json:"status"`
	Currency    models.Currency            `json:"currency"`
	TenantID    string                     `json:"tenant_id"`
	CreatedBy   string                     `json:"created_by"`
	CreatedAt   time.Time                  `json:"created_at"`
	UpdatedAt   time.Time                  `json:"updated_at"`
	MemberCount int                        `json:"member_count"`
}

// AdminGroupsResponse is the JSON:API envelope for GET /admin/groups.
type AdminGroupsResponse struct {
	Data []*AdminGroupListItem `json:"data"`
	Meta AdminListMeta         `json:"meta"`
}

// NewAdminGroupsResponse maps registry-layer rows into the wire-shape the
// FE consumes. Page / PerPage / Total drive the meta block (with
// total_pages computed via ComputeTotalPages so the FE never has to do
// the ceil-divide).
func NewAdminGroupsResponse(items []*registry.AdminGroupListItem, page, perPage, total int) *AdminGroupsResponse {
	data := make([]*AdminGroupListItem, 0, len(items))
	for _, it := range items {
		if it == nil || it.Group == nil {
			continue
		}
		g := it.Group
		data = append(data, &AdminGroupListItem{
			ID:          g.ID,
			Type:        "admin_groups",
			Name:        g.Name,
			Slug:        g.Slug,
			Status:      g.Status,
			Currency:    g.GroupCurrency,
			TenantID:    g.TenantID,
			CreatedBy:   g.CreatedBy,
			CreatedAt:   g.CreatedAt,
			UpdatedAt:   g.UpdatedAt,
			MemberCount: it.MemberCount,
		})
	}
	return &AdminGroupsResponse{
		Data: data,
		Meta: AdminListMeta{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: ComputeTotalPages(total, perPage),
		},
	}
}

func (*AdminGroupsResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// AdminGroupTenantChip is the compact tenant descriptor embedded on the
// admin group-detail row so the FE can render an owning-tenant chip
// (id + name + slug) without a second round-trip to /admin/tenants.
type AdminGroupTenantChip struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// AdminGroupDetail is the row shape returned by GET /admin/groups/{groupID}.
// Compared to AdminGroupListItem this adds the resolved tenant chip.
type AdminGroupDetail struct {
	ID          string                     `json:"id"`
	Type        string                     `json:"type" example:"admin_groups" enums:"admin_groups"`
	Name        string                     `json:"name"`
	Slug        string                     `json:"slug"`
	Status      models.LocationGroupStatus `json:"status"`
	Currency    models.Currency            `json:"currency"`
	TenantID    string                     `json:"tenant_id"`
	CreatedBy   string                     `json:"created_by"`
	CreatedAt   time.Time                  `json:"created_at"`
	UpdatedAt   time.Time                  `json:"updated_at"`
	MemberCount int                        `json:"member_count"`
	Tenant      *AdminGroupTenantChip      `json:"tenant,omitempty"`
}

// AdminGroupResponse is the JSON:API envelope for GET
// /admin/groups/{groupID} and the DELETE soft-delete response — the
// DELETE returns the post-transition row so the FE can render the
// updated status without a follow-up GET.
type AdminGroupResponse struct {
	Data *AdminGroupDetail `json:"data"`
}

// NewAdminGroupResponse wraps a single group detail row (with tenant chip)
// for the detail + soft-delete endpoints.
func NewAdminGroupResponse(item *registry.AdminGroupDetail) *AdminGroupResponse {
	if item == nil || item.Group == nil {
		return &AdminGroupResponse{}
	}
	g := item.Group
	detail := &AdminGroupDetail{
		ID:          g.ID,
		Type:        "admin_groups",
		Name:        g.Name,
		Slug:        g.Slug,
		Status:      g.Status,
		Currency:    g.GroupCurrency,
		TenantID:    g.TenantID,
		CreatedBy:   g.CreatedBy,
		CreatedAt:   g.CreatedAt,
		UpdatedAt:   g.UpdatedAt,
		MemberCount: item.MemberCount,
	}
	if item.Tenant != nil {
		detail.Tenant = &AdminGroupTenantChip{
			ID:   item.Tenant.ID,
			Name: item.Tenant.Name,
			Slug: item.Tenant.Slug,
		}
	}
	return &AdminGroupResponse{Data: detail}
}

func (*AdminGroupResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}
