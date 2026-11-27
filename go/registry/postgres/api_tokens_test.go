package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
)

type apiTokenPGEnv struct {
	dbx    *sqlx.DB
	tokens registry.APITokenRegistry
	set    *registry.Set
	user   *models.User
}

func newAPITokenPGEnv(t *testing.T) *apiTokenPGEnv {
	t.Helper()
	c := qt.New(t)
	set, _ := setupTestRegistrySet(t)
	user := getTestUser(c, set)

	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	fs := postgres.NewFactorySet(dbx)
	c.Assert(fs.APITokenRegistry, qt.IsNotNil)
	return &apiTokenPGEnv{dbx: dbx, tokens: fs.APITokenRegistry, set: fs.CreateServiceRegistrySet(), user: user}
}

func (e *apiTokenPGEnv) create(c *qt.C, mutate func(*models.APIToken)) (*models.APIToken, string) {
	c.Helper()
	gen, err := models.GenerateAPIToken()
	c.Assert(err, qt.IsNil)
	token := models.APIToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: e.user.TenantID, UserID: e.user.ID},
		Name:                    "script",
		TokenHash:               gen.Hash,
		TokenPrefix:             gen.DisplayPrefix,
		Scope:                   models.APITokenScopeReadOnly,
	}
	if mutate != nil {
		mutate(&token)
	}
	created, err := e.tokens.Create(context.Background(), token)
	c.Assert(err, qt.IsNil)
	return created, gen.Token
}

func TestAPITokenRegistryPostgres_CreateAndLookUp(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenPGEnv(t)
	ctx := context.Background()

	expires := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	created, secret := env.create(c, func(tk *models.APIToken) {
		tk.Scope = models.APITokenScopeReadWrite
		tk.ExpiresAt = new(expires)
	})
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.CreatedAt.IsZero(), qt.IsFalse)

	got, err := env.tokens.GetByTokenHash(ctx, models.HashAPIToken(secret))
	c.Assert(err, qt.IsNil)
	c.Assert(got.ID, qt.Equals, created.ID)
	c.Assert(got.UserID, qt.Equals, env.user.ID)
	c.Assert(got.TenantID, qt.Equals, env.user.TenantID)
	c.Assert(got.Scope, qt.Equals, models.APITokenScopeReadWrite)
	c.Assert(got.ExpiresAt, qt.IsNotNil)
	c.Assert(got.ExpiresAt.Equal(expires), qt.IsTrue)
	c.Assert(got.LastUsedAt, qt.IsNil)
	c.Assert(got.RevokedAt, qt.IsNil)

	_, err = env.tokens.GetByTokenHash(ctx, models.HashAPIToken("inv_pat_unknown"))
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestAPITokenRegistryPostgres_RevokeIsScopedToTheOwner(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenPGEnv(t)
	ctx := context.Background()
	token, secret := env.create(c, nil)

	_, err := env.tokens.RevokeByID(ctx, "someone-else", token.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	revoked, err := env.tokens.RevokeByID(ctx, env.user.ID, token.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(revoked.RevokedAt, qt.IsNotNil)

	again, err := env.tokens.RevokeByID(ctx, env.user.ID, token.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(again.RevokedAt.Equal(*revoked.RevokedAt), qt.IsTrue, qt.Commentf("a second revoke moved the revocation time"))

	got, err := env.tokens.GetByTokenHash(ctx, models.HashAPIToken(secret))
	c.Assert(err, qt.IsNil)
	c.Assert(got.IsRevoked(), qt.IsTrue)

	list, err := env.tokens.ListByUserID(ctx, env.user.ID)
	c.Assert(err, qt.IsNil)
	for _, tk := range list {
		c.Assert(tk.ID, qt.Not(qt.Equals), token.ID, qt.Commentf("a revoked token is listed"))
	}
}

func TestAPITokenRegistryPostgres_ListCountAndRevokeAll(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenPGEnv(t)
	ctx := context.Background()
	now := time.Now()

	// Start from a clean slate for this user; other tests share the database.
	_, err := env.tokens.RevokeAllByUserID(ctx, env.user.ID)
	c.Assert(err, qt.IsNil)

	older, _ := env.create(c, nil)
	newer, _ := env.create(c, nil)
	_, _ = env.create(c, func(tk *models.APIToken) { tk.ExpiresAt = new(now.Add(-time.Hour)) })

	list, err := env.tokens.ListByUserID(ctx, env.user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(list, qt.HasLen, 3)
	ids := []string{list[0].ID, list[1].ID, list[2].ID}
	c.Assert(ids, qt.Contains, older.ID)
	c.Assert(ids, qt.Contains, newer.ID)

	count, err := env.tokens.CountActiveByUserID(ctx, env.user.ID, now)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)

	n, err := env.tokens.RevokeAllByUserID(ctx, env.user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 3)
	count, err = env.tokens.CountActiveByUserID(ctx, env.user.ID, now)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

func TestAPITokenRegistryPostgres_TouchLastUsedIsThrottled(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenPGEnv(t)
	ctx := context.Background()
	token, secret := env.create(c, nil)

	t0 := time.Now().UTC().Truncate(time.Microsecond)
	wrote, err := env.tokens.TouchLastUsed(ctx, token.ID, t0, t0.Add(-models.APITokenLastUsedInterval))
	c.Assert(err, qt.IsNil)
	c.Assert(wrote, qt.IsTrue)

	t1 := t0.Add(time.Minute)
	wrote, err = env.tokens.TouchLastUsed(ctx, token.ID, t1, t1.Add(-models.APITokenLastUsedInterval))
	c.Assert(err, qt.IsNil)
	c.Assert(wrote, qt.IsFalse)

	got, err := env.tokens.GetByTokenHash(ctx, models.HashAPIToken(secret))
	c.Assert(err, qt.IsNil)
	c.Assert(got.LastUsedAt, qt.IsNotNil)
	c.Assert(got.LastUsedAt.Equal(t0), qt.IsTrue)

	_, err = env.tokens.RevokeByID(ctx, env.user.ID, token.ID)
	c.Assert(err, qt.IsNil)
	t2 := t0.Add(time.Hour)
	wrote, err = env.tokens.TouchLastUsed(ctx, token.ID, t2, t2.Add(-models.APITokenLastUsedInterval))
	c.Assert(err, qt.IsNil)
	c.Assert(wrote, qt.IsFalse, qt.Commentf("a revoked token was touched"))
}

// A token restricted to a group goes with the group. SET NULL would turn it
// into a token for every group, so the foreign key cascades instead.
func TestAPITokenRegistryPostgres_DeletingTheGroupDeletesItsTokens(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenPGEnv(t)
	ctx := context.Background()

	slug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	group, err := env.set.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: env.user.TenantID},
		Name:                "Doomed",
		Slug:                slug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           env.user.ID,
		GroupCurrency:       models.Currency("USD"),
	})
	c.Assert(err, qt.IsNil)
	// A token limited to a group needs its user to be a member.
	_, err = env.set.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: env.user.TenantID},
		GroupID:             group.ID,
		MemberUserID:        env.user.ID,
		Role:                models.GroupRoleOwner,
	})
	c.Assert(err, qt.IsNil)

	_, secret := env.create(c, func(tk *models.APIToken) { tk.GroupID = new(group.ID) })

	// The group purge removes the memberships before the group row.
	_, err = env.dbx.ExecContext(ctx, `DELETE FROM group_memberships WHERE group_id = $1`, group.ID)
	c.Assert(err, qt.IsNil)
	_, err = env.dbx.ExecContext(ctx, `DELETE FROM location_groups WHERE id = $1`, group.ID)
	c.Assert(err, qt.IsNil)

	_, err = env.tokens.GetByTokenHash(ctx, models.HashAPIToken(secret))
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}
