package jsonapi

import (
	"context"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
)

// --- LocationGroup responses ---

type LocationGroupResponse struct {
	HTTPStatusCode int                        `json:"-"`
	Data           *LocationGroupResponseData `json:"data"`
}

type LocationGroupResponseData struct {
	ID         string                `json:"id"`
	Type       string                `json:"type" example:"groups" enums:"groups"`
	Attributes *models.LocationGroup `json:"attributes"`
}

func NewLocationGroupResponse(group *models.LocationGroup) *LocationGroupResponse {
	return &LocationGroupResponse{
		Data: &LocationGroupResponseData{
			ID:         group.ID,
			Type:       "groups",
			Attributes: group,
		},
	}
}

func (rd *LocationGroupResponse) WithStatusCode(statusCode int) *LocationGroupResponse {
	tmp := *rd
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

func (rd *LocationGroupResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(rd.HTTPStatusCode, http.StatusOK))
	return nil
}

type LocationGroupsMeta struct {
	Groups     int `json:"groups" example:"1" format:"int64"`
	Page       int `json:"page" example:"1" format:"int64"`
	PerPage    int `json:"per_page" example:"50" format:"int64"`
	TotalPages int `json:"total_pages" example:"1" format:"int64"`
}

// LocationGroupResponseItem represents a single group in list responses with
// the full model exposed (all server-controlled fields included).
type LocationGroupResponseItem struct {
	ID         string                `json:"id"`
	Type       string                `json:"type" example:"groups" enums:"groups"`
	Attributes *models.LocationGroup `json:"attributes"`
}

type LocationGroupsResponse struct {
	Data []LocationGroupResponseItem `json:"data"`
	Meta LocationGroupsMeta          `json:"meta"`
}

func NewLocationGroupsResponse(groups []*models.LocationGroup, total, page, perPage int) *LocationGroupsResponse {
	groupData := make([]LocationGroupResponseItem, 0)
	for _, g := range groups {
		g := *g
		groupData = append(groupData, LocationGroupResponseItem{
			ID:         g.ID,
			Type:       "groups",
			Attributes: &g,
		})
	}

	return &LocationGroupsResponse{
		Data: groupData,
		Meta: LocationGroupsMeta{
			Groups:     total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: ComputeTotalPages(total, perPage),
		},
	}
}

func (*LocationGroupsResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// --- LocationGroup request ---

var _ render.Binder = (*LocationGroupRequest)(nil)

type LocationGroupRequest struct {
	Data *LocationGroupData `json:"data"`
}

// LocationGroupAttributes is the user-settable subset of LocationGroup fields
// accepted in create/update requests (server-generated fields like slug,
// status, created_by are excluded).
type LocationGroupAttributes struct {
	Name string `json:"name"`
	Icon string `json:"icon,omitempty"`
}

type LocationGroupData struct {
	ID         string                   `json:"id,omitempty"`
	Type       string                   `json:"type" example:"groups" enums:"groups"`
	Attributes *LocationGroupAttributes `json:"attributes"`
}

func (ld *LocationGroupData) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&ld.Type, validation.Required, validation.In("groups")),
		validation.Field(&ld.Attributes, validation.Required),
	)

	if httpMethod, ok := ctx.Value(httpMethodKey).(string); ok && httpMethod == "POST" {
		fields = append(fields,
			validation.Field(&ld.ID, validation.Empty.Error("ID field not allowed in create requests")),
		)
	}

	return validation.ValidateStructWithContext(ctx, ld, fields...)
}

func (la *LocationGroupAttributes) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, la,
		validation.Field(&la.Name, validation.Required, validation.Length(1, 100)),
		validation.Field(&la.Icon, validation.Length(0, 10)),
	)
}

func (lr *LocationGroupRequest) Bind(r *http.Request) error {
	ctx := context.WithValue(r.Context(), httpMethodKey, r.Method)
	if err := lr.ValidateWithContext(ctx); err != nil {
		return err
	}
	if lr.Data != nil && lr.Data.Attributes != nil {
		return lr.Data.Attributes.ValidateWithContext(ctx)
	}
	return nil
}

func (lr *LocationGroupRequest) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields, validation.Field(&lr.Data, validation.Required))
	return validation.ValidateStructWithContext(ctx, lr, fields...)
}

// --- GroupMembership responses ---

type GroupMembershipResponse struct {
	HTTPStatusCode int                          `json:"-"`
	Data           *GroupMembershipResponseData `json:"data"`
}

type GroupMembershipResponseData struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type" example:"memberships" enums:"memberships"`
	Attributes *models.GroupMembership `json:"attributes"`
}

func NewGroupMembershipResponse(membership *models.GroupMembership) *GroupMembershipResponse {
	return &GroupMembershipResponse{
		Data: &GroupMembershipResponseData{
			ID:         membership.ID,
			Type:       "memberships",
			Attributes: membership,
		},
	}
}

func (rd *GroupMembershipResponse) WithStatusCode(statusCode int) *GroupMembershipResponse {
	tmp := *rd
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

func (rd *GroupMembershipResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(rd.HTTPStatusCode, http.StatusOK))
	return nil
}

type GroupMembershipsResponse struct {
	Data []GroupMembershipData `json:"data"`
}

type GroupMembershipData struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type" example:"memberships" enums:"memberships"`
	Attributes *models.GroupMembership `json:"attributes"`
}

func NewGroupMembershipsResponse(memberships []*models.GroupMembership) *GroupMembershipsResponse {
	data := make([]GroupMembershipData, 0)
	for _, m := range memberships {
		m := *m
		data = append(data, GroupMembershipData{
			ID:         m.ID,
			Type:       "memberships",
			Attributes: &m,
		})
	}

	return &GroupMembershipsResponse{Data: data}
}

func (*GroupMembershipsResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// --- GroupMembership role update request ---

var _ render.Binder = (*GroupMembershipRoleRequest)(nil)

type GroupMembershipRoleRequest struct {
	Data *GroupMembershipRoleData `json:"data"`
}

type GroupMembershipRoleData struct {
	Attributes *GroupMembershipRoleAttributes `json:"attributes"`
}

type GroupMembershipRoleAttributes struct {
	Role models.GroupRole `json:"role"`
}

func (r *GroupMembershipRoleRequest) Bind(_ *http.Request) error {
	if r.Data == nil || r.Data.Attributes == nil {
		return validation.NewError("validation_required", "data.attributes is required")
	}
	return r.Data.Attributes.Role.Validate()
}

// --- GroupInvite responses ---

type GroupInviteResponse struct {
	HTTPStatusCode int                      `json:"-"`
	Data           *GroupInviteResponseData `json:"data"`
}

type GroupInviteResponseData struct {
	ID         string              `json:"id"`
	Type       string              `json:"type" example:"invites" enums:"invites"`
	Attributes *models.GroupInvite `json:"attributes"`
}

func NewGroupInviteResponse(invite *models.GroupInvite) *GroupInviteResponse {
	return &GroupInviteResponse{
		Data: &GroupInviteResponseData{
			ID:         invite.ID,
			Type:       "invites",
			Attributes: invite,
		},
	}
}

func (rd *GroupInviteResponse) WithStatusCode(statusCode int) *GroupInviteResponse {
	tmp := *rd
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

func (rd *GroupInviteResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(rd.HTTPStatusCode, http.StatusOK))
	return nil
}

type GroupInvitesResponse struct {
	Data []GroupInviteData `json:"data"`
}

type GroupInviteData struct {
	ID         string              `json:"id"`
	Type       string              `json:"type" example:"invites" enums:"invites"`
	Attributes *models.GroupInvite `json:"attributes"`
}

func NewGroupInvitesResponse(invites []*models.GroupInvite) *GroupInvitesResponse {
	data := make([]GroupInviteData, 0)
	for _, inv := range invites {
		inv := *inv
		data = append(data, GroupInviteData{
			ID:         inv.ID,
			Type:       "invites",
			Attributes: &inv,
		})
	}

	return &GroupInvitesResponse{Data: data}
}

func (*GroupInvitesResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// --- Invite info (public, limited fields) ---

type InviteInfoResponse struct {
	Data *InviteInfoData `json:"data"`
}

type InviteInfoData struct {
	Type       string          `json:"type" example:"invite_info" enums:"invite_info"`
	Attributes *InviteInfoAttr `json:"attributes"`
}

type InviteInfoAttr struct {
	GroupName string `json:"group_name"`
	GroupIcon string `json:"group_icon"`
	Expired   bool   `json:"expired"`
	Used      bool   `json:"used"`
}

func NewInviteInfoResponse(group *models.LocationGroup, invite *models.GroupInvite) *InviteInfoResponse {
	return &InviteInfoResponse{
		Data: &InviteInfoData{
			Type: "invite_info",
			Attributes: &InviteInfoAttr{
				GroupName: group.Name,
				GroupIcon: group.Icon,
				Expired:   invite.IsExpired(),
				Used:      invite.IsUsed(),
			},
		},
	}
}

func (*InviteInfoResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// --- Group deletion request ---

var _ render.Binder = (*GroupDeleteRequest)(nil)

type GroupDeleteRequest struct {
	ConfirmWord string `json:"confirm_word"`
}

func (r *GroupDeleteRequest) Bind(_ *http.Request) error {
	if r.ConfirmWord == "" {
		return validation.NewError("validation_required", "confirm_word is required")
	}
	return nil
}
