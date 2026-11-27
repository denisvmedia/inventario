package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
	"go.5x5.cz/inventario/registry/postgres/store"
)

// moverPGFixture is the seeded test tenant (with its admin, who created the
// test group) plus a second, empty tenant to move users into.
type moverPGFixture struct {
	ctx     context.Context
	set     *registry.Set
	fs      *registry.FactorySet
	dbx     *sqlx.DB
	admin   *models.User
	groupID string
	from    string
	to      string
}

func newMoverPGFixture(t *testing.T) *moverPGFixture {
	t.Helper()
	c := qt.New(t)

	set, _ := setupTestRegistrySet(t)
	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	fs := postgres.NewFactorySet(dbx)

	admin := getTestUser(c, set)
	target, err := fs.TenantRegistry.Create(context.Background(), models.Tenant{
		Name: "Target Organization", Slug: "target-org", Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	return &moverPGFixture{
		ctx:     context.Background(),
		set:     set,
		fs:      fs,
		dbx:     dbx,
		admin:   admin,
		groupID: getTestGroupID(c, set, admin),
		from:    admin.TenantID,
		to:      target.ID,
	}
}

// newUser adds a user to the source tenant who belongs to no group and owns
// nothing.
func (f *moverPGFixture) newUser(c *qt.C, email string) *models.User {
	c.Helper()
	u := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.from},
		Email:               email,
		Name:                email,
		IsActive:            true,
	}
	c.Assert(u.SetPassword("Password123"), qt.IsNil)
	created, err := f.fs.UserRegistry.Create(f.ctx, u)
	c.Assert(err, qt.IsNil)
	return created
}

func (f *moverPGFixture) move(user *models.User) (*models.User, error) {
	return f.fs.UserTenantMover.MoveUserToTenant(f.ctx, registry.UserTenantMove{
		UserID: user.ID, FromTenantID: user.TenantID, ToTenantID: f.to,
	})
}

func (f *moverPGFixture) seedSession(c *qt.C, user *models.User) {
	c.Helper()
	_, err := f.fs.RefreshTokenRegistry.Create(f.ctx, models.RefreshToken{
		TenantUserAwareEntityID: models.WithTenantUserAwareEntityID("", user.TenantID, user.ID),
		TokenHash:               "session-" + user.ID,
		ExpiresAt:               time.Now().Add(time.Hour),
	})
	c.Assert(err, qt.IsNil)
}

func (f *moverPGFixture) sessions(c *qt.C, user *models.User) int {
	c.Helper()
	tokens, err := f.fs.RefreshTokenRegistry.GetByUserID(f.ctx, user.ID)
	c.Assert(err, qt.IsNil)
	return len(tokens)
}

func (f *moverPGFixture) storedTenant(c *qt.C, user *models.User) string {
	c.Helper()
	stored, err := f.fs.UserRegistry.Get(f.ctx, user.ID)
	c.Assert(err, qt.IsNil)
	return stored.TenantID
}

// A user with nothing in the tenant moves: the users row, their settings,
// second factor and login history go to the new tenant, and their sessions and
// API tokens are deleted.
func TestUserTenantMover_Postgres_MovesAnEmptyUser(t *testing.T) {
	c := qt.New(t)
	f := newMoverPGFixture(t)
	user := f.newUser(c, "wrong-tenant@test-org.com")
	userCtx := appctx.WithUser(f.ctx, user)

	f.seedSession(c, user)
	gen, err := models.GenerateAPIToken()
	c.Assert(err, qt.IsNil)
	_, err = f.fs.APITokenRegistry.Create(f.ctx, models.APIToken{
		TenantUserAwareEntityID: models.WithTenantUserAwareEntityID("", f.from, user.ID),
		Name:                    "script",
		TokenHash:               gen.Hash,
		TokenPrefix:             gen.DisplayPrefix,
		Scope:                   models.APITokenScopeReadOnly,
	})
	c.Assert(err, qt.IsNil)
	_, err = f.fs.UserMFASecretRegistry.Create(f.ctx, models.UserMFASecret{
		TenantUserAwareEntityID: models.WithTenantUserAwareEntityID("", f.from, user.ID),
		SecretEncrypted:         "encrypted-secret",
		BackupCodesHashed:       models.ValuerSlice[string]{},
	})
	c.Assert(err, qt.IsNil)
	settings, err := f.fs.SettingsRegistryFactory.CreateUserRegistry(userCtx)
	c.Assert(err, qt.IsNil)
	c.Assert(settings.Save(userCtx, models.SettingsObject{AppearanceLanguage: new("cs")}), qt.IsNil)
	_, err = f.fs.LoginEventRegistry.Create(f.ctx, models.LoginEvent{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.from},
		UserID:              new(user.ID),
		Email:               user.Email,
		Outcome:             models.LoginOutcomeOK,
	})
	c.Assert(err, qt.IsNil)

	owned, err := f.fs.UserTenantMover.CountOwnedContent(f.ctx, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(owned.IsEmpty(), qt.IsTrue, qt.Commentf("owned: %s", owned))

	photo := "t/" + f.to + "/avatars/" + user.ID + "/2-abcdef012345.jpg"
	moved, err := f.fs.UserTenantMover.MoveUserToTenant(f.ctx, registry.UserTenantMove{
		UserID: user.ID, FromTenantID: f.from, ToTenantID: f.to, AvatarPath: &photo,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(moved.TenantID, qt.Equals, f.to)
	c.Assert(moved.DefaultGroupID, qt.IsNil)
	c.Assert(moved.AvatarPath, qt.IsNotNil)
	c.Assert(*moved.AvatarPath, qt.Equals, photo)
	c.Assert(moved.CheckPassword("Password123"), qt.IsTrue)
	c.Assert(f.storedTenant(c, user), qt.Equals, f.to)

	c.Assert(f.sessions(c, user), qt.Equals, 0)
	_, err = f.fs.APITokenRegistry.GetByTokenHash(f.ctx, gen.Hash)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	_, err = f.fs.UserMFASecretRegistry.GetByUser(f.ctx, f.to, user.ID)
	c.Assert(err, qt.IsNil)
	movedCtx := appctx.WithUser(f.ctx, moved)
	movedSettings, err := f.fs.SettingsRegistryFactory.CreateUserRegistry(movedCtx)
	c.Assert(err, qt.IsNil)
	got, err := movedSettings.Get(movedCtx)
	c.Assert(err, qt.IsNil)
	c.Assert(got.AppearanceLanguage, qt.IsNotNil)
	c.Assert(*got.AppearanceLanguage, qt.Equals, "cs")
	events, err := f.fs.LoginEventRegistry.ListByUser(f.ctx, f.to, user.ID, 0)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
}

// The admin created the test group and belongs to it, so they are refused, with
// both reasons listed, and nothing changes.
func TestUserTenantMover_Postgres_RefusesAGroupMember(t *testing.T) {
	c := qt.New(t)
	f := newMoverPGFixture(t)
	f.seedSession(c, f.admin)
	ctx := appctx.WithUser(f.ctx, f.admin)
	seedTagCommodity(c, f.set, ctx, seedTagArea(c, f.set, ctx), "Owned Drill")

	_, err := f.move(f.admin)
	c.Assert(err, qt.ErrorIs, registry.ErrUserTenantMoveBlocked)
	blocked, ok := errors.AsType[*registry.UserTenantMoveBlockedError](err)
	c.Assert(ok, qt.IsTrue)
	c.Assert(blocked.Owned.Count(registry.OwnedLocationGroups), qt.Equals, 1)
	c.Assert(blocked.Owned.Count(registry.OwnedCommodities), qt.Equals, 1)
	c.Assert(blocked.Owned.Count(registry.OwnedLocations), qt.Equals, 1)
	c.Assert(blocked.Owned.Count(registry.OwnedAreas), qt.Equals, 1)

	c.Assert(f.storedTenant(c, f.admin), qt.Equals, f.from)
	c.Assert(f.sessions(c, f.admin), qt.Equals, 1)
}

// Loans and item history pass to another member when an account is deleted,
// but they tie the user to the tenant all the same, so the move counts them.
func TestUserTenantMover_Postgres_RefusesALenderAndEditor(t *testing.T) {
	c := qt.New(t)
	f := newMoverPGFixture(t)
	adminCtx := appctx.WithUser(f.ctx, f.admin)
	commodityID := seedTagCommodity(c, f.set, adminCtx, seedTagArea(c, f.set, adminCtx), "Shared Drill")

	user := f.newUser(c, "lender@test-org.com")
	userCtx := appctx.WithUser(f.ctx, user)
	authored := models.TenantGroupAwareEntityID{TenantID: f.from, GroupID: f.groupID, CreatedByUserID: user.ID}
	_, err := f.fs.CommodityLoanRegistryFactory.CreateServiceRegistry().Create(userCtx, models.CommodityLoan{
		TenantGroupAwareEntityID: authored,
		CommodityID:              commodityID,
		BorrowerName:             "Neighbor",
		LentAt:                   models.Date("2026-05-01"),
	})
	c.Assert(err, qt.IsNil)
	_, err = f.fs.CommodityEventRegistryFactory.CreateServiceRegistry().Create(userCtx, models.CommodityEvent{
		TenantGroupAwareEntityID: authored,
		CommodityID:              commodityID,
		Kind:                     models.CommodityEventKindUpdated,
		OccurredAt:               time.Now(),
	})
	c.Assert(err, qt.IsNil)

	_, err = f.move(user)
	blocked, ok := errors.AsType[*registry.UserTenantMoveBlockedError](err)
	c.Assert(ok, qt.IsTrue, qt.Commentf("err: %v", err))
	c.Assert(blocked.Owned.Count(registry.OwnedCommodityLoans), qt.Equals, 1)
	c.Assert(blocked.Owned.Count(registry.OwnedCommodityEvents), qt.Equals, 1)
	c.Assert(blocked.Owned.Count(registry.OwnedGroupMemberships), qt.Equals, 0)
	c.Assert(f.storedTenant(c, user), qt.Equals, f.from)
}

// A move built from a stale read of the user's tenant is refused.
func TestUserTenantMover_Postgres_RefusesAStaleSourceTenant(t *testing.T) {
	c := qt.New(t)
	f := newMoverPGFixture(t)
	user := f.newUser(c, "stale@test-org.com")

	_, err := f.fs.UserTenantMover.MoveUserToTenant(f.ctx, registry.UserTenantMove{
		UserID: user.ID, FromTenantID: "another-tenant", ToTenantID: f.to,
	})
	c.Assert(err, qt.ErrorIs, registry.ErrUserTenantChanged)
	c.Assert(f.storedTenant(c, user), qt.Equals, f.from)
}

// Emails are unique per tenant, so a move into a tenant that has the email is
// refused.
func TestUserTenantMover_Postgres_RefusesATakenEmail(t *testing.T) {
	c := qt.New(t)
	f := newMoverPGFixture(t)
	user := f.newUser(c, "taken@test-org.com")
	_, err := f.fs.UserRegistry.Create(f.ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.to},
		Email:               user.Email,
		Name:                "Same Email",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)

	_, err = f.move(user)
	c.Assert(err, qt.ErrorIs, registry.ErrEmailAlreadyExists)
	c.Assert(f.storedTenant(c, user), qt.Equals, f.from)
}

// UserRegistry.Update never changes a user's tenant. A write that names
// another tenant, or one built from a copy read before the user moved, is
// refused instead of overwriting the tenant.
func TestUserRegistry_Postgres_UpdateKeepsTheTenant(t *testing.T) {
	c := qt.New(t)
	f := newMoverPGFixture(t)
	user := f.newUser(c, "keeps@test-org.com")

	changed := *user
	changed.TenantID = f.to
	_, err := f.fs.UserRegistry.Update(f.ctx, changed)
	c.Assert(err, qt.ErrorIs, registry.ErrUserTenantChanged)

	_, err = f.move(user)
	c.Assert(err, qt.IsNil)
	stale := *user
	stale.Name = "Renamed From A Stale Copy"
	_, err = f.fs.UserRegistry.Update(f.ctx, stale)
	c.Assert(err, qt.ErrorIs, registry.ErrUserTenantChanged)
	c.Assert(f.storedTenant(c, user), qt.Equals, f.to)
}

// A membership row names the tenant it was written for; once the user is in
// another tenant it is refused.
func TestGroupMembershipRegistry_Postgres_RefusesAMemberOfAnotherTenant(t *testing.T) {
	c := qt.New(t)
	f := newMoverPGFixture(t)
	user := f.newUser(c, "joins-late@test-org.com")
	_, err := f.move(user)
	c.Assert(err, qt.IsNil)

	_, err = f.fs.GroupMembershipRegistry.Create(f.ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.from},
		GroupID:             f.groupID,
		MemberUserID:        user.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.ErrorIs, registry.ErrUserTenantChanged)
}

// A membership inserted while the move runs holds a key-share lock on the users
// row until it commits, so the move waits for it and then sees it. The move is
// refused rather than leaving a member behind in the old tenant.
func TestUserTenantMover_Postgres_WaitsForAConcurrentMembership(t *testing.T) {
	c := qt.New(t)
	f := newMoverPGFixture(t)
	user := f.newUser(c, "racing@test-org.com")

	inserted := make(chan struct{})
	release := make(chan struct{})
	insertDone := make(chan error, 1)
	go func() {
		insertDone <- store.DoAsBackgroundWorker(f.ctx, f.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO group_memberships (id, tenant_id, group_id, member_user_id, role, joined_at)
				VALUES ($1, $2, $3, $4, $5, $6)`,
				uuid.New().String(), f.from, f.groupID, user.ID, models.GroupRoleUser, time.Now())
			close(inserted)
			<-release
			return err
		})
	}()
	<-inserted

	moveDone := make(chan error, 1)
	go func() {
		_, err := f.move(user)
		moveDone <- err
	}()
	c.Assert(waitForBlockedLock(f.ctx, f.dbx), qt.IsTrue, qt.Commentf("the move never waited on the users row"))
	close(release)

	c.Assert(<-insertDone, qt.IsNil)
	c.Assert(<-moveDone, qt.ErrorIs, registry.ErrUserTenantMoveBlocked)
	c.Assert(f.storedTenant(c, user), qt.Equals, f.from)
}

// A user who once accepted an invite leaves that audit row in the tenant they
// left. Deleting their account later still works: the purge finds the row by
// user in any tenant, so the final users delete is not blocked by its foreign
// key.
func TestUserPurger_Postgres_DeletesAMovedUserWithInviteAudit(t *testing.T) {
	c := qt.New(t)
	f := newMoverPGFixture(t)
	user := f.newUser(c, "accepted-once@test-org.com")
	now := time.Now()
	_, err := f.fs.GroupInviteAuditRegistry.Create(appctx.WithUser(f.ctx, f.admin), models.GroupInviteAudit{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.from},
		OriginalInviteID:    "orig-invite",
		OriginalInviteUUID:  "orig-uuid",
		OriginalGroupID:     "orig-group",
		OriginalGroupSlug:   "orig-group-slug",
		OriginalGroupName:   "Orig Group",
		Token:               "token-accepted-once",
		CreatedBy:           f.admin.ID,
		UsedBy:              user.ID,
		OriginalCreatedAt:   now.Add(-time.Hour),
		OriginalExpiresAt:   now.Add(time.Hour),
		UsedAt:              now,
	})
	c.Assert(err, qt.IsNil)

	_, err = f.move(user)
	c.Assert(err, qt.IsNil)

	c.Assert(f.fs.UserPurger.PurgeUserDependents(f.ctx, f.to, user.ID), qt.IsNil)
	c.Assert(f.fs.UserRegistry.Delete(f.ctx, user.ID), qt.IsNil)
}

// A personal data export requested in the old tenant goes with the move, so
// the account can still be deleted in the new tenant: the purge runs in the
// new tenant and would leave an old-tenant row holding a key to users (#2177).
func TestUserTenantMover_Postgres_MovedUserWithAPersonalDataExportCanBeDeleted(t *testing.T) {
	c := qt.New(t)
	f := newMoverPGFixture(t)
	user := f.newUser(c, "exported-then-moved@test-org.com")
	now := time.Now().UTC()
	created, blocking, err := f.fs.PersonalDataExportRegistry.CreateUnlessRequestedSince(f.ctx, models.PersonalDataExport{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: f.from, UserID: user.ID},
		CreatedAt:               now,
		ExpiresAt:               now.Add(models.PersonalDataExportRetention),
	}, now.Add(-models.PersonalDataExportRequestInterval))
	c.Assert(err, qt.IsNil)
	c.Assert(blocking, qt.IsNil)
	c.Assert(created, qt.IsNotNil)

	_, err = f.move(user)
	c.Assert(err, qt.IsNil)

	left, err := f.fs.PersonalDataExportRegistry.ListByUserID(f.ctx, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(left, qt.HasLen, 0)

	c.Assert(f.fs.UserPurger.PurgeUserDependents(f.ctx, f.to, user.ID), qt.IsNil)
	c.Assert(f.fs.UserRegistry.Delete(f.ctx, user.ID), qt.IsNil)
}

// A move refuses a user whose photo is not the one the caller read and copied,
// so a photo uploaded meanwhile is neither lost nor left behind.
func TestUserTenantMover_Postgres_RefusesAChangedPhoto(t *testing.T) {
	c := qt.New(t)
	f := newMoverPGFixture(t)
	user := f.newUser(c, "new-photo@test-org.com")

	_, err := f.fs.UserTenantMover.MoveUserToTenant(f.ctx, registry.UserTenantMove{
		UserID: user.ID, FromTenantID: f.from, ToTenantID: f.to, FromAvatarPath: new("t/" + f.from + "/avatars/" + user.ID + "/1-abcdef012345.jpg"),
	})
	c.Assert(err, qt.ErrorIs, registry.ErrUserChangedDuringMove)
	c.Assert(f.storedTenant(c, user), qt.Equals, f.from)
}

// A suspended tenant takes no users.
func TestUserTenantMover_Postgres_RefusesATenantThatIsNotActive(t *testing.T) {
	c := qt.New(t)
	f := newMoverPGFixture(t)
	user := f.newUser(c, "to-suspended@test-org.com")
	target, err := f.fs.TenantRegistry.Get(f.ctx, f.to)
	c.Assert(err, qt.IsNil)
	target.Status = models.TenantStatusSuspended
	_, err = f.fs.TenantRegistry.Update(f.ctx, *target)
	c.Assert(err, qt.IsNil)

	_, err = f.move(user)
	c.Assert(err, qt.ErrorIs, registry.ErrTenantNotActive)
	c.Assert(f.storedTenant(c, user), qt.Equals, f.from)
}

// A user with the same email created in the target tenant while the move runs
// is not seen by the move's email check; the unique index catches it, and the
// move reports the email as taken.
func TestUserTenantMover_Postgres_EmailTakenWhileMoving(t *testing.T) {
	c := qt.New(t)
	f := newMoverPGFixture(t)
	user := f.newUser(c, "racing-email@test-org.com")

	inserted := make(chan struct{})
	release := make(chan struct{})
	insertDone := make(chan error, 1)
	go func() {
		insertDone <- store.DoAsBackgroundWorker(f.ctx, f.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO users (id, email, password_hash, name, is_active, tenant_id, created_at, updated_at)
				VALUES ($1, $2, 'hash', 'Same Email', true, $3, $4, $4)`,
				uuid.New().String(), user.Email, f.to, time.Now())
			close(inserted)
			<-release
			return err
		})
	}()
	<-inserted

	moveDone := make(chan error, 1)
	go func() {
		_, err := f.move(user)
		moveDone <- err
	}()
	c.Assert(waitForBlockedLock(f.ctx, f.dbx), qt.IsTrue, qt.Commentf("the move never waited on the unique index"))
	close(release)

	c.Assert(<-insertDone, qt.IsNil)
	c.Assert(<-moveDone, qt.ErrorIs, registry.ErrEmailAlreadyExists)
	c.Assert(f.storedTenant(c, user), qt.Equals, f.from)
}

// waitForBlockedLock reports whether some backend is waiting for a lock within
// a few seconds.
func waitForBlockedLock(ctx context.Context, dbx *sqlx.DB) bool {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		if err := dbx.QueryRowContext(ctx, "SELECT COUNT(*) FROM pg_locks WHERE NOT granted").Scan(&waiting); err == nil && waiting > 0 {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// userTenantMoveUnchanged are user references a move leaves as they are. Each
// entry says why.
var userTenantMoveUnchanged = map[string]string{
	// The audit trail records what happened in the tenant it happened in. The
	// user purger deletes the invite audit rows by user in any tenant.
	"audit_logs.user_id":             "audit trail",
	"group_invites_audit.created_by": "audit trail",
	"group_invites_audit.used_by":    "audit trail",
	// Who moved an item or a file to Trash; the row belongs to the group.
	"commodities.deleted_by": "trash attribution",
	"files.deleted_by":       "trash attribution",
	// An accepted invite is the record of a join, not content. A user who
	// still belongs to the group is refused through their membership.
	"group_invites.used_by": "record of a past join",
	// The system administrator privilege has no tenant.
	"system_admin_grants.user_id":    "not tenant-scoped",
	"system_admin_grants.granted_by": "not tenant-scoped",
}

// userReferenceLookalikes match the column-name filter but do not name a user
// of a tenant.
var userReferenceLookalikes = map[string]string{
	"audit_logs.impersonated_by":                     "a back-office operator",
	"backoffice_refresh_tokens.backoffice_user_id":   "a back-office user",
	"backoffice_user_mfa_secrets.backoffice_user_id": "a back-office user",
	"user_oauth_identities.provider_user_id":         "the account id at the sign-in provider",
	"worker_control.paused_by":                       "a back-office operator",
}

// Every column that names a user, by foreign key or by name, is either content
// that blocks a move, a per-user row the move deletes, one it carries to the
// new tenant, or listed as unchanged with a reason. A table added later without
// a decision fails here.
func TestUserTenantMove_CoversEveryUserReference(t *testing.T) {
	c := qt.New(t)

	dsn := skipIfNoPostgreSQL(t)
	_, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	db, err := sql.Open("pgx", dsn)
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() { _ = db.Close() })

	rows, err := db.QueryContext(context.Background(), `
		SELECT c.conrelid::regclass::text || '.' || a.attname
		FROM pg_constraint c
		JOIN unnest(c.conkey) k ON true
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = k
		WHERE c.contype = 'f' AND c.confrelid = 'users'::regclass
		UNION
		SELECT table_name || '.' || column_name
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name <> 'users'
		  AND (column_name LIKE '%\_by' OR column_name LIKE '%user\_id')
		ORDER BY 1`)
	c.Assert(err, qt.IsNil)
	defer rows.Close()

	var references []string
	for rows.Next() {
		var ref string
		c.Assert(rows.Scan(&ref), qt.IsNil)
		references = append(references, ref)
	}
	c.Assert(rows.Err(), qt.IsNil)
	c.Assert(len(references) > 20, qt.IsTrue,
		qt.Commentf("expected user references, found %d: %v", len(references), references))

	known := postgres.UserTenantMoveColumns()
	var missing []string
	for _, ref := range references {
		if _, ok := known[ref]; ok {
			continue
		}
		if _, ok := userTenantMoveUnchanged[ref]; ok {
			continue
		}
		if _, ok := userReferenceLookalikes[ref]; ok {
			continue
		}
		missing = append(missing, ref)
	}
	sort.Strings(missing)
	c.Check(missing, qt.HasLen, 0,
		qt.Commentf("these columns name a user but the tenant move has no rule for them: %v", missing))

	present := make(map[string]bool, len(references))
	for _, ref := range references {
		present[ref] = true
	}
	for ref, kind := range known {
		c.Check(present[ref], qt.IsTrue, qt.Commentf("%s is %s by the tenant move but is not in the schema", ref, kind))
	}
	for ref, reason := range userTenantMoveUnchanged {
		c.Check(present[ref], qt.IsTrue, qt.Commentf("%s is left unchanged (%s) but is not in the schema", ref, reason))
	}
	for ref, reason := range userReferenceLookalikes {
		c.Check(present[ref], qt.IsTrue, qt.Commentf("%s is excluded (%s) but is not in the schema", ref, reason))
	}
}
