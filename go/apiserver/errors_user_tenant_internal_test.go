package apiserver

// White-box: no HTTP route moves a user between tenants (the move is a CLI
// operation), so the 409 a refused move maps to cannot be reached through a
// request. The mapping is checked on toJSONAPIError directly, so a route that
// starts returning the refusal gets the documented wire shape.

import (
	"net/http"
	"testing"

	qt "github.com/frankban/quicktest"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/registry"
)

func TestToJSONAPIError_UserTenantMoveBlocked(t *testing.T) {
	c := qt.New(t)

	err := errxtrace.Wrap("failed to update user", &registry.UserTenantMoveBlockedError{
		Owned: registry.NewUserOwnedContent(map[registry.OwnedContentKind]int{
			registry.OwnedGroupMemberships: 2,
			registry.OwnedCommodities:      14,
		}),
	})

	jsErr := toJSONAPIError(err)
	c.Assert(jsErr.HTTPStatusCode, qt.Equals, http.StatusConflict)
	c.Assert(jsErr.Code, qt.Equals, "user.tenant_move_blocked")
	c.Assert(jsErr.Meta, qt.DeepEquals, map[string]any{
		"owned":             "member of 2 groups, 14 items",
		"group_memberships": 2,
		"commodities":       14,
	})
}

func TestToJSONAPIError_UserTenantChanged(t *testing.T) {
	c := qt.New(t)

	jsErr := toJSONAPIError(errxtrace.Classify(registry.ErrUserTenantChanged))
	c.Assert(jsErr.HTTPStatusCode, qt.Equals, http.StatusConflict)
	c.Assert(jsErr.Code, qt.Equals, "user.tenant_changed")
}
