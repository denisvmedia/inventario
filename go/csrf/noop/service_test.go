package noop_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/csrf"
	"github.com/denisvmedia/inventario/csrf/noop"
)

// Compile-time interface check.
var _ csrf.Service = noop.Service{}

func TestService_GenerateToken_AlwaysReturnsStaticToken(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc := noop.Service{}

	token, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Equals, "noop-csrf-token")
}

func TestService_GenerateToken_SameTokenForAllUsers(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc := noop.Service{}

	tok1, err := svc.GenerateToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)

	tok2, err := svc.GenerateToken(ctx, "user-2")
	c.Assert(err, qt.IsNil)

	c.Assert(tok1, qt.Equals, tok2)
}

func TestService_GetToken_AlwaysReturnsStaticToken(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc := noop.Service{}

	token, err := svc.GetToken(ctx, "any-user")
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Equals, "noop-csrf-token")
}

// TestService_ValidateToken_AlwaysTrue verifies that every token/user combination
// is accepted, making the no-op service suitable only for test environments.
func TestService_ValidateToken_AlwaysTrue(t *testing.T) {
	ctx := context.Background()
	svc := noop.Service{}

	cases := []struct{ userID, token string }{
		{"user-1", "noop-csrf-token"},
		{"user-2", "some-random-string"},
		{"user-3", ""},
		{"", "anything"},
	}

	for _, tc := range cases {
		t.Run(tc.userID+"/"+tc.token, func(t *testing.T) {
			c := qt.New(t)
			valid, err := svc.ValidateToken(ctx, tc.userID, tc.token)
			c.Assert(err, qt.IsNil)
			c.Assert(valid, qt.IsTrue)
		})
	}
}

// TestService_RevokeToken_IsNoOp verifies that RevokeToken returns no error and
// leaves all other methods behaving identically.
func TestService_RevokeToken_IsNoOp(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc := noop.Service{}

	err := svc.RevokeToken(ctx, "user-1", "any-token")
	c.Assert(err, qt.IsNil)

	// All methods must still return their no-op responses afterwards.
	token, err := svc.GetToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Equals, "noop-csrf-token")

	valid, err := svc.ValidateToken(ctx, "user-1", "anything")
	c.Assert(err, qt.IsNil)
	c.Assert(valid, qt.IsTrue)
}

// TestService_DeleteAllTokens_IsNoOp verifies that DeleteAllTokens returns no
// error and leaves all other methods behaving identically.
func TestService_DeleteAllTokens_IsNoOp(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	svc := noop.Service{}

	err := svc.DeleteAllTokens(ctx, "user-1")
	c.Assert(err, qt.IsNil)

	token, err := svc.GetToken(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Equals, "noop-csrf-token")

	valid, err := svc.ValidateToken(ctx, "user-1", "anything")
	c.Assert(err, qt.IsNil)
	c.Assert(valid, qt.IsTrue)
}
