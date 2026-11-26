package admin_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"gocloud.dev/blob"
	"golang.org/x/crypto/bcrypt"

	"go.5x5.cz/inventario/appctx"
	_ "go.5x5.cz/inventario/internal/fileblob" // register file:// driver
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
	"go.5x5.cz/inventario/services/admin"
)

const movePassword = "Sup3r-Secret-Pw!"

// moveEnv is two tenants on the memory backend, a user in the first one with
// a session, an API token, MFA, a language setting and a login event, and an
// admin service that can copy photos.
type moveEnv struct {
	ctx      context.Context
	fs       *registry.FactorySet
	svc      *admin.Service
	avatars  *services.AvatarService
	location string
	from     string
	to       string
	user     *models.User
	token    string
}

func newMoveEnv(c *qt.C) *moveEnv {
	c.Helper()
	models.SetBcryptCostForTesting(c.TB.(*testing.T), bcrypt.MinCost)
	ctx := context.Background()
	fs := memory.NewFactorySet()

	from, err := fs.TenantRegistry.Create(ctx, models.Tenant{Name: "From", Slug: "from", Status: models.TenantStatusActive})
	c.Assert(err, qt.IsNil)
	to, err := fs.TenantRegistry.Create(ctx, models.Tenant{Name: "To", Slug: "to", Status: models.TenantStatusActive})
	c.Assert(err, qt.IsNil)

	u := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: from.ID},
		Email:               "wrong-tenant@example.com",
		Name:                "Wrong Tenant",
		IsActive:            true,
	}
	c.Assert(u.SetPassword(movePassword), qt.IsNil)
	user, err := fs.UserRegistry.Create(ctx, u)
	c.Assert(err, qt.IsNil)

	location := "file://" + c.TempDir() + "?create_dir=1"
	avatars := services.NewAvatarService(fs, location)
	svc := admin.NewServiceWithFactorySet(fs)
	svc.SetAvatarService(avatars)

	env := &moveEnv{ctx: ctx, fs: fs, svc: svc, avatars: avatars, location: location, from: from.ID, to: to.ID, user: user}
	env.seedPerUserRows(c)
	return env
}

func (e *moveEnv) seedPerUserRows(c *qt.C) {
	c.Helper()
	_, err := e.fs.RefreshTokenRegistry.Create(e.ctx, models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: e.from, UserID: e.user.ID},
		TokenHash:               "session-hash",
		ExpiresAt:               time.Now().Add(time.Hour),
	})
	c.Assert(err, qt.IsNil)

	gen, err := models.GenerateAPIToken()
	c.Assert(err, qt.IsNil)
	_, err = e.fs.APITokenRegistry.Create(e.ctx, models.APIToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: e.from, UserID: e.user.ID},
		Name:                    "script",
		TokenHash:               gen.Hash,
		TokenPrefix:             gen.DisplayPrefix,
		Scope:                   models.APITokenScopeReadOnly,
	})
	c.Assert(err, qt.IsNil)
	e.token = gen.Hash

	_, err = e.fs.UserMFASecretRegistry.Create(e.ctx, models.UserMFASecret{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: e.from, UserID: e.user.ID},
		SecretEncrypted:         "encrypted-secret",
	})
	c.Assert(err, qt.IsNil)

	settings, err := e.fs.SettingsRegistryFactory.CreateUserRegistry(appctx.WithUser(e.ctx, e.user))
	c.Assert(err, qt.IsNil)
	c.Assert(settings.Save(e.ctx, models.SettingsObject{AppearanceLanguage: new("cs")}), qt.IsNil)

	_, err = e.fs.LoginEventRegistry.Create(e.ctx, models.LoginEvent{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: e.from},
		UserID:              new(e.user.ID),
		Email:               e.user.Email,
		Outcome:             models.LoginOutcomeOK,
	})
	c.Assert(err, qt.IsNil)
}

func (e *moveEnv) storePhoto(c *qt.C) string {
	c.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	c.Assert(png.Encode(&buf, img), qt.IsNil)
	key, err := e.avatars.Store(e.ctx, e.user.ID, &buf)
	c.Assert(err, qt.IsNil)
	return key
}

func (e *moveEnv) blobExists(c *qt.C, key string) bool {
	c.Helper()
	b, err := blob.OpenBucket(e.ctx, e.location)
	c.Assert(err, qt.IsNil)
	defer b.Close()
	ok, err := b.Exists(e.ctx, key)
	c.Assert(err, qt.IsNil)
	return ok
}

// blobKeys lists the keys under prefix in the upload location.
func (e *moveEnv) blobKeys(c *qt.C, prefix string) []string {
	c.Helper()
	b, err := blob.OpenBucket(e.ctx, e.location)
	c.Assert(err, qt.IsNil)
	defer b.Close()
	var keys []string
	iter := b.List(&blob.ListOptions{Prefix: prefix})
	for {
		obj, err := iter.Next(e.ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		c.Assert(err, qt.IsNil)
		keys = append(keys, obj.Key)
	}
	return keys
}

// otherOwnersGroup is a group of the source tenant that someone else created
// and owns, so the user under test can be tied to it without creating it.
func (e *moveEnv) otherOwnersGroup(c *qt.C) *models.LocationGroup {
	c.Helper()
	owner, err := e.fs.UserRegistry.Create(e.ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: e.from},
		Email:               "owner@example.com",
		Name:                "Owner",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	slug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	group, err := e.fs.LocationGroupRegistry.Create(e.ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: e.from},
		Name:                "Shared",
		Slug:                slug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           owner.ID,
		GroupCurrency:       "USD",
	})
	c.Assert(err, qt.IsNil)
	return group
}

// asUser is a context carrying the user under test, which the registries use
// to stamp the author of what they create.
func (e *moveEnv) asUser() context.Context {
	return appctx.WithUser(e.ctx, e.user)
}

// authored stamps the user under test as the author of a row in group.
func (e *moveEnv) authored(group *models.LocationGroup) models.TenantGroupAwareEntityID {
	return models.TenantGroupAwareEntityID{TenantID: e.from, GroupID: group.ID, CreatedByUserID: e.user.ID}
}

func (e *moveEnv) seedCommodity(c *qt.C, group *models.LocationGroup) *models.Commodity {
	c.Helper()
	commodity, err := e.fs.CommodityRegistryFactory.CreateServiceRegistry().Create(e.asUser(), models.Commodity{
		TenantGroupAwareEntityID: e.authored(group),
		Name:                     "Drill",
		ShortName:                "drill",
		Type:                     models.CommodityTypeEquipment,
		Status:                   models.CommodityStatusInUse,
		Count:                    1,
	})
	c.Assert(err, qt.IsNil)
	return commodity
}

// An account created in the wrong tenant, with nothing in it, moves. It keeps
// its password, MFA, language and login history, loses its sessions and API
// tokens, and its photo is copied into the new tenant's storage.
func TestService_UpdateUser_MovesAnEmptyUser(t *testing.T) {
	c := qt.New(t)
	env := newMoveEnv(c)
	oldPhoto := env.storePhoto(c)

	moved, err := env.svc.UpdateUser(env.ctx, env.user.Email, admin.UserUpdateRequest{TenantID: new(env.to)})
	c.Assert(err, qt.IsNil)
	c.Assert(moved.TenantID, qt.Equals, env.to)
	c.Assert(moved.DefaultGroupID, qt.IsNil)

	stored, err := env.fs.UserRegistry.Get(env.ctx, env.user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.TenantID, qt.Equals, env.to)
	c.Assert(stored.CheckPassword(movePassword), qt.IsTrue)

	_, err = env.fs.UserMFASecretRegistry.GetByUser(env.ctx, env.to, env.user.ID)
	c.Assert(err, qt.IsNil)

	settings, err := env.fs.SettingsRegistryFactory.CreateUserRegistry(appctx.WithUser(env.ctx, stored))
	c.Assert(err, qt.IsNil)
	got, err := settings.Get(env.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(got.AppearanceLanguage, qt.IsNotNil)
	c.Assert(*got.AppearanceLanguage, qt.Equals, "cs")

	events, err := env.fs.LoginEventRegistry.ListByUser(env.ctx, env.to, env.user.ID, 0)
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)

	sessions, err := env.fs.RefreshTokenRegistry.GetByUserID(env.ctx, env.user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(sessions, qt.HasLen, 0)
	_, err = env.fs.APITokenRegistry.GetByTokenHash(env.ctx, env.token)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	c.Assert(stored.AvatarPath, qt.IsNotNil)
	c.Assert(strings.HasPrefix(*stored.AvatarPath, "t/"+env.to+"/avatars/"+env.user.ID+"/"), qt.IsTrue, qt.Commentf("got %s", *stored.AvatarPath))
	reader, _, err := env.avatars.Open(env.ctx, env.user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reader.Close(), qt.IsNil)
	c.Assert(env.blobExists(c, oldPhoto), qt.IsFalse)

	entries, err := env.fs.AuditLogRegistry.List(env.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(entries, qt.HasLen, 1)
	c.Assert(entries[0].Action, qt.Equals, "admin.move_user_tenant")
	c.Assert(entries[0].Success, qt.IsTrue)
}

// Other requested changes are applied together with the move.
func TestService_UpdateUser_MovesAndRenames(t *testing.T) {
	c := qt.New(t)
	env := newMoveEnv(c)

	moved, err := env.svc.UpdateUser(env.ctx, env.user.ID, admin.UserUpdateRequest{TenantID: new(env.to), Name: new("Right Tenant")})
	c.Assert(err, qt.IsNil)
	c.Assert(moved.TenantID, qt.Equals, env.to)
	c.Assert(moved.Name, qt.Equals, "Right Tenant")
}

// A user tied to the tenant by a membership or by anything they created is
// not moved. The error lists what blocks the move, and nothing changes: not
// the tenant, not the other requested fields, not the sessions or the photo.
func TestService_UpdateUser_RefusesAMoveWhileTheUserOwnsContent(t *testing.T) {
	tests := []struct {
		name string
		kind registry.OwnedContentKind
		seed func(c *qt.C, env *moveEnv)
	}{
		{"membership", registry.OwnedGroupMemberships, func(c *qt.C, env *moveEnv) {
			group := env.otherOwnersGroup(c)
			_, err := env.fs.GroupMembershipRegistry.Create(env.ctx, models.GroupMembership{
				TenantAwareEntityID: models.TenantAwareEntityID{TenantID: env.from},
				GroupID:             group.ID,
				MemberUserID:        env.user.ID,
				Role:                models.GroupRoleUser,
			})
			c.Assert(err, qt.IsNil)
		}},
		{"item", registry.OwnedCommodities, func(c *qt.C, env *moveEnv) {
			env.seedCommodity(c, env.otherOwnersGroup(c))
		}},
		{"file", registry.OwnedFiles, func(c *qt.C, env *moveEnv) {
			_, err := env.fs.FileRegistryFactory.CreateServiceRegistry().Create(env.asUser(), models.FileEntity{
				TenantGroupAwareEntityID: env.authored(env.otherOwnersGroup(c)),
				Title:                    "Invoice",
				Type:                     models.FileTypeDocument,
				File:                     &models.File{Path: "invoice", OriginalPath: "t/x/files/invoice.pdf", Ext: ".pdf", MIMEType: "application/pdf"},
			})
			c.Assert(err, qt.IsNil)
		}},
		{"loan", registry.OwnedCommodityLoans, func(c *qt.C, env *moveEnv) {
			group := env.otherOwnersGroup(c)
			commodity := env.seedCommodity(c, group)
			_, err := env.fs.CommodityLoanRegistryFactory.CreateServiceRegistry().Create(env.asUser(), models.CommodityLoan{
				TenantGroupAwareEntityID: env.authored(group),
				CommodityID:              commodity.ID,
				BorrowerName:             "Neighbor",
				LentAt:                   models.Date("2026-05-01"),
			})
			c.Assert(err, qt.IsNil)
		}},
		{"history entry", registry.OwnedCommodityEvents, func(c *qt.C, env *moveEnv) {
			group := env.otherOwnersGroup(c)
			commodity := env.seedCommodity(c, group)
			_, err := env.fs.CommodityEventRegistryFactory.CreateServiceRegistry().Create(env.asUser(), models.CommodityEvent{
				TenantGroupAwareEntityID: env.authored(group),
				CommodityID:              commodity.ID,
				Kind:                     models.CommodityEventKindUpdated,
				OccurredAt:               time.Now(),
			})
			c.Assert(err, qt.IsNil)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			env := newMoveEnv(c)
			photo := env.storePhoto(c)
			tt.seed(c, env)

			_, err := env.svc.UpdateUser(env.ctx, env.user.ID, admin.UserUpdateRequest{TenantID: new(env.to), Name: new("Should Not Persist")})
			c.Assert(err, qt.ErrorIs, registry.ErrUserTenantMoveBlocked)
			blocked, ok := errors.AsType[*registry.UserTenantMoveBlockedError](err)
			c.Assert(ok, qt.IsTrue)
			c.Assert(blocked.Owned.Count(tt.kind) >= 1, qt.IsTrue, qt.Commentf("owned: %s", blocked.Owned))

			stored, err := env.fs.UserRegistry.Get(env.ctx, env.user.ID)
			c.Assert(err, qt.IsNil)
			c.Assert(stored.TenantID, qt.Equals, env.from)
			c.Assert(stored.Name, qt.Equals, "Wrong Tenant")
			c.Assert(*stored.AvatarPath, qt.Equals, photo)
			c.Assert(env.blobExists(c, photo), qt.IsTrue)

			sessions, err := env.fs.RefreshTokenRegistry.GetByUserID(env.ctx, env.user.ID)
			c.Assert(err, qt.IsNil)
			c.Assert(sessions, qt.HasLen, 1)
			_, err = env.fs.APITokenRegistry.GetByTokenHash(env.ctx, env.token)
			c.Assert(err, qt.IsNil)
		})
	}
}

// The refusal reads as the list the operator acts on.
func TestService_UpdateUser_RefusalListsWhatBlocks(t *testing.T) {
	c := qt.New(t)
	env := newMoveEnv(c)
	group := env.otherOwnersGroup(c)
	_, err := env.fs.GroupMembershipRegistry.Create(env.ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: env.from},
		GroupID:             group.ID,
		MemberUserID:        env.user.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.IsNil)
	env.seedCommodity(c, group)
	env.seedCommodity(c, group)

	_, err = env.svc.UpdateUser(env.ctx, env.user.ID, admin.UserUpdateRequest{TenantID: new(env.to)})
	c.Assert(err, qt.ErrorMatches, `.*member of 1 group, 2 items.*`)
}

// A move needs a photo store when the user has a photo; without one it is
// refused before anything changes.
func TestService_UpdateUser_MoveWithAPhotoNeedsAvatarStorage(t *testing.T) {
	c := qt.New(t)
	env := newMoveEnv(c)
	env.storePhoto(c)
	svc := admin.NewServiceWithFactorySet(env.fs)

	_, err := svc.UpdateUser(env.ctx, env.user.ID, admin.UserUpdateRequest{TenantID: new(env.to)})
	c.Assert(err, qt.ErrorIs, registry.ErrInvalidConfig)

	stored, err := env.fs.UserRegistry.Get(env.ctx, env.user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.TenantID, qt.Equals, env.from)
}

// The target tenant has to exist.
func TestService_UpdateUser_MoveToAMissingTenant(t *testing.T) {
	c := qt.New(t)
	env := newMoveEnv(c)

	_, err := env.svc.UpdateUser(env.ctx, env.user.ID, admin.UserUpdateRequest{TenantID: new("no-such-tenant")})
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// Two users of one tenant cannot share an email, so a move into a tenant that
// has the email is refused.
func TestService_UpdateUser_MoveRefusesATakenEmail(t *testing.T) {
	c := qt.New(t)
	env := newMoveEnv(c)
	photo := env.storePhoto(c)
	_, err := env.fs.UserRegistry.Create(env.ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: env.to},
		Email:               env.user.Email,
		Name:                "Same Email",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)

	_, err = env.svc.UpdateUser(env.ctx, env.user.ID, admin.UserUpdateRequest{TenantID: new(env.to)})
	c.Assert(err, qt.ErrorIs, registry.ErrEmailAlreadyExists)

	stored, err := env.fs.UserRegistry.Get(env.ctx, env.user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.TenantID, qt.Equals, env.from)
	c.Assert(*stored.AvatarPath, qt.Equals, photo)
	c.Assert(env.blobExists(c, photo), qt.IsTrue)
	c.Assert(env.blobKeys(c, "t/"+env.to+"/avatars/"), qt.HasLen, 0)
}

// A suspended or inactive tenant takes no users.
func TestService_UpdateUser_MoveRefusesATenantThatIsNotActive(t *testing.T) {
	c := qt.New(t)
	env := newMoveEnv(c)
	target, err := env.fs.TenantRegistry.Get(env.ctx, env.to)
	c.Assert(err, qt.IsNil)
	target.Status = models.TenantStatusSuspended
	_, err = env.fs.TenantRegistry.Update(env.ctx, *target)
	c.Assert(err, qt.IsNil)

	_, err = env.svc.UpdateUser(env.ctx, env.user.ID, admin.UserUpdateRequest{TenantID: new(env.to)})
	c.Assert(err, qt.ErrorIs, registry.ErrTenantNotActive)

	stored, err := env.fs.UserRegistry.Get(env.ctx, env.user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.TenantID, qt.Equals, env.from)
}

// A photo the configured storage does not hold, as with a wrong
// --upload-location, stops the move before anything changes. The dry-run check
// reports the same.
func TestService_UpdateUser_MoveRefusesAPhotoTheStorageDoesNotHold(t *testing.T) {
	c := qt.New(t)
	env := newMoveEnv(c)
	env.storePhoto(c)
	env.svc.SetAvatarService(services.NewAvatarService(env.fs, "file://"+c.TempDir()+"?create_dir=1"))

	c.Assert(env.svc.CheckUserTenantMove(env.ctx, env.user.ID, env.to), qt.ErrorIs, registry.ErrNotFound)
	_, err := env.svc.UpdateUser(env.ctx, env.user.ID, admin.UserUpdateRequest{TenantID: new(env.to)})
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	stored, err := env.fs.UserRegistry.Get(env.ctx, env.user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.TenantID, qt.Equals, env.from)
	sessions, err := env.fs.RefreshTokenRegistry.GetByUserID(env.ctx, env.user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(sessions, qt.HasLen, 1)
}

// The check a dry run makes changes nothing and reports what the move would
// stop at.
func TestService_CheckUserTenantMove(t *testing.T) {
	c := qt.New(t)
	env := newMoveEnv(c)
	env.storePhoto(c)

	c.Assert(env.svc.CheckUserTenantMove(env.ctx, env.user.Email, env.to), qt.IsNil)
	c.Assert(env.svc.CheckUserTenantMove(env.ctx, env.user.Email, "no-such-tenant"), qt.ErrorIs, registry.ErrNotFound)

	env.seedCommodity(c, env.otherOwnersGroup(c))
	err := env.svc.CheckUserTenantMove(env.ctx, env.user.Email, env.to)
	c.Assert(err, qt.ErrorIs, registry.ErrUserTenantMoveBlocked)

	stored, err := env.fs.UserRegistry.Get(env.ctx, env.user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.TenantID, qt.Equals, env.from)
	c.Assert(env.blobKeys(c, "t/"+env.to+"/avatars/"), qt.HasLen, 0)
}
