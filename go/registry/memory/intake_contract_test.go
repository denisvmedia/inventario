package memory_test

import (
	"context"
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/registry/registrytest"
)

func TestIntakeContract_Memory(t *testing.T) {
	registrytest.RunIntakeContract(t, newMemoryIntakeEnv)
}

// newMemoryIntakeEnv returns the registries of one empty group.
func newMemoryIntakeEnv(t *testing.T) registrytest.IntakeEnv {
	c := qt.New(t)

	ctx := appctx.WithUser(t.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
	})
	ctx = appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "group-1"},
			TenantID: "tenant-1",
		},
		Slug:          "group-1",
		GroupCurrency: "USD",
	})

	fs := memory.NewFactorySet()
	set, err := fs.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)
	return registrytest.IntakeEnv{
		Ctx:     ctx,
		Intake:  set.IntakeRegistry,
		Service: fs.IntakeRegistryFactory.CreateServiceRegistry(),
		Files:   set.FileRegistry,
		Drafts:  set.CommodityDraftRegistry,
	}
}

// The purgers remove intake sessions as the postgres ones do: account
// deletion takes the sessions the user created and leaves their files as
// standalone files, the group purge takes the group's, the tenant purge the
// rest.
func TestIntakePurges_Memory(t *testing.T) {
	c := qt.New(t)

	fs := memory.NewFactorySet()
	scope := func(userID, groupID string) context.Context {
		ctx := appctx.WithUser(t.Context(), &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: userID}, TenantID: "tenant-1"},
		})
		return appctx.WithGroup(ctx, &models.LocationGroup{
			TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: groupID}, TenantID: "tenant-1"},
			Slug:                groupID,
			GroupCurrency:       "USD",
		})
	}
	type stored struct {
		ctx     context.Context
		session string
		file    string
	}
	seed := func(userID, groupID string) stored {
		ctx := scope(userID, groupID)
		set, err := fs.CreateUserRegistrySet(ctx)
		c.Assert(err, qt.IsNil)
		file, err := set.FileRegistry.Create(ctx, models.FileEntity{
			Title:    "receipt",
			Type:     models.FileTypeDocument,
			Category: models.FileCategoryDocuments,
			File:     &models.File{Path: "receipt", OriginalPath: "files/receipt", Ext: ".pdf", MIMEType: "application/pdf"},
		})
		c.Assert(err, qt.IsNil)
		session, _, err := set.IntakeRegistry.CreateSession(ctx,
			models.IntakeSession{ClientKey: userID + "-" + groupID, Origin: models.IntakeOriginUpload},
			[]registry.IntakeSourceInput{{FileIDs: []string{file.ID}}}, 0)
		c.Assert(err, qt.IsNil)
		return stored{ctx: ctx, session: session.ID, file: file.ID}
	}
	service := fs.IntakeRegistryFactory.CreateServiceRegistry()
	exists := func(s stored) bool {
		_, err := service.Get(t.Context(), s.session)
		if errors.Is(err, registry.ErrNotFound) {
			return false
		}
		c.Assert(err, qt.IsNil)
		return true
	}

	leaving := seed("user-1", "group-a")
	staying := seed("user-2", "group-a")
	other := seed("user-2", "group-b")

	c.Assert(fs.UserPurger.PurgeUserDependents(t.Context(), "tenant-1", "user-1"), qt.IsNil)
	c.Check(exists(leaving), qt.IsFalse)
	c.Check(exists(staying), qt.IsTrue)
	files, err := fs.CreateUserRegistrySet(leaving.ctx)
	c.Assert(err, qt.IsNil)
	file, err := files.FileRegistry.Get(leaving.ctx, leaving.file)
	c.Assert(err, qt.IsNil)
	c.Check(file.Links, qt.HasLen, 0)

	c.Assert(fs.GroupPurger.PurgeGroupDependents(t.Context(), "tenant-1", "group-a"), qt.IsNil)
	c.Check(exists(staying), qt.IsFalse)
	c.Check(exists(other), qt.IsTrue)

	c.Assert(fs.TenantPurger.PurgeTenantDependents(t.Context(), "tenant-1"), qt.IsNil)
	c.Check(exists(other), qt.IsFalse)
}
