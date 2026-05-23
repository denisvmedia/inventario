package apiserver_test

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/jsonapi"
)

// TestAdminSecurityInvariant_NoHTTPWriteSurfaceForGrants is the
// table-driven regression cover for #1784 AC #3: "A test-invariant
// asserts no route outside the CLI mutates admin privilege."
//
// It walks every chi route registered by the apiserver and asserts
// that NONE of them mount under `/api/v1/admin/grants` (the path a
// reviewer might be tempted to expose), and — belt-and-braces — that
// no admin route pattern contains the literal `system_admin_grants`.
// The CLI is the only sanctioned mutation path; if a future change
// adds an HTTP write surface this test stops CI.
func TestAdminSecurityInvariant_NoHTTPWriteSurfaceForGrants(t *testing.T) {
	t.Parallel()
	c := qt.New(t)

	params, _, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	router, ok := handler.(chi.Router)
	c.Assert(ok, qt.IsTrue, qt.Commentf("APIServer should return a chi.Router-typed handler, got %T", handler))

	var offenders []string
	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		lower := strings.ToLower(route)
		// Any route mounted under /api/v1/admin/grants would be a
		// write-or-read HTTP surface for the privileged grant table.
		// The issue spec explicitly forbids any such surface.
		if strings.HasPrefix(lower, "/api/v1/admin/grants") {
			offenders = append(offenders, method+" "+route)
			return nil
		}
		// Defense-in-depth: any route mentioning the literal table name
		// is a smell — the table is meant to be invisible to HTTP.
		if strings.Contains(lower, "system_admin_grants") {
			offenders = append(offenders, method+" "+route)
		}
		return nil
	})
	c.Assert(err, qt.IsNil)
	c.Assert(offenders, qt.HasLen, 0,
		qt.Commentf(
			"system_admin_grants must NOT have any HTTP write surface — CLI is the only sanctioned mutation path (#1784). Offending route(s): %v",
			offenders,
		),
	)
}

// TestAdminSecurityInvariant_RequestDTOsCannotCarryGrantField asserts
// no user-facing write DTO declares a field that could route into
// system_admin_grants under a future blind-decode mistake. Today the
// privilege has no model field, so this is mostly future-proofing —
// the test catches a drive-by addition of an `IsSystemAdmin` /
// `SystemAdmin` / `Grant` field to a request struct.
func TestAdminSecurityInvariant_RequestDTOsCannotCarryGrantField(t *testing.T) {
	t.Parallel()

	// The set is small and load-bearing — every user-write surface
	// that could plausibly reach UserRegistry.Update or an unsafe
	// decode into models.User. Pin the types by reflect.TypeOf so a
	// rename of the struct in code surfaces here as a compile error
	// rather than a silent drop in coverage.
	dtos := []reflect.Type{
		reflect.TypeFor[apiserver.RegisterRequest](),
		reflect.TypeFor[jsonapi.UpdateProfileRequest](),
		reflect.TypeFor[apiserver.LoginRequest](),
		reflect.TypeFor[apiserver.ChangePasswordRequest](),
		reflect.TypeFor[apiserver.AdminBlockRequest](),
		reflect.TypeFor[apiserver.AdminUnblockRequest](),
	}

	bannedNames := []string{
		"IsSystemAdmin",
		"SystemAdmin",
		"Grant",
		"GrantedBy",
		"GrantedAt",
	}
	bannedJSONTags := []string{
		"is_system_admin",
		"system_admin",
		"system_admin_grant",
		"system_admin_grants",
		"grant",
		"granted_by",
		"granted_at",
	}

	for _, dto := range dtos {
		t.Run(dto.Name(), func(t *testing.T) {
			c := qt.New(t)
			for f := range dto.Fields() {
				for _, banned := range bannedNames {
					c.Assert(f.Name, qt.Not(qt.Equals), banned,
						qt.Commentf(
							"user-write DTO %s must not declare a %q field — #1784 keeps the system-admin privilege physically off every request DTO",
							dto.Name(), banned,
						),
					)
				}
				jsonTag := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
				for _, banned := range bannedJSONTags {
					c.Assert(jsonTag, qt.Not(qt.Equals), banned,
						qt.Commentf(
							"user-write DTO %s.%s must not carry json tag %q — #1784 keeps the system-admin privilege physically off every request DTO",
							dto.Name(), f.Name, banned,
						),
					)
				}
			}
		})
	}
}
