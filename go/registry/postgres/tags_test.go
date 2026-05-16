package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
	"github.com/denisvmedia/inventario/services"
)

// tagPGFixture wires what every postgres tag test needs:
//   - a shared *FactorySet for service-layer code (TagService) that resolves
//     the group from ctx;
//   - one user-aware *Set per group (registries with their groupID baked in,
//     mirroring what the apiserver builds per request);
//   - context values that carry user + group, ready to hand to TagService.
//
// The fixture is sized for two groups so cross-group RLS isolation can be
// exercised — the most load-bearing assertion in #1488 beyond the SQL itself.
type tagPGFixture struct {
	factorySet *registry.FactorySet
	groupASet  *registry.Set
	groupBSet  *registry.Set
	ctxA       context.Context
	ctxB       context.Context
	user       *models.User
	groupAID   string
	groupBID   string
	areaAID    string
	areaBID    string
	dbx        *sqlx.DB
}

// newTagPGFixture re-creates the schema, seeds tenant+user+two groups, and
// returns a fixture. setupTestRegistrySet drops + bootstraps before this
// returns, so each call is hermetic.
func newTagPGFixture(t *testing.T) tagPGFixture {
	t.Helper()
	c := qt.New(t)

	groupASet, _ := setupTestRegistrySet(t)

	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	factorySet := postgres.NewFactorySet(dbx)

	user := getTestUser(c, groupASet)

	// Group A is the one setupTestRegistrySet created. Discover its ID via
	// the service registry rather than threading it through the helper —
	// keeps the shared scaffold untouched.
	serviceSet := factorySet.CreateServiceRegistrySet()
	groups, err := serviceSet.LocationGroupRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(groups, qt.HasLen, 1)
	groupAID := groups[0].ID

	groupBSlug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	groupB, err := serviceSet.LocationGroupRegistry.Create(context.Background(), models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		Name:                "Test Group B",
		Slug:                groupBSlug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           user.ID,
		GroupCurrency:       models.Currency("USD"),
	})
	c.Assert(err, qt.IsNil)
	groupBID := groupB.ID

	groupBSet := postgres.NewRegistrySetWithUserAndGroupID(dbx, user.ID, user.TenantID, groupBID)

	ctxA := tagCtxFor(user, groupAID)
	ctxB := tagCtxFor(user, groupBID)

	areaAID := seedTagArea(c, groupASet, ctxA)
	areaBID := seedTagArea(c, groupBSet, ctxB)

	return tagPGFixture{
		factorySet: factorySet,
		groupASet:  groupASet,
		groupBSet:  groupBSet,
		ctxA:       ctxA,
		ctxB:       ctxB,
		user:       user,
		groupAID:   groupAID,
		groupBID:   groupBID,
		areaAID:    areaAID,
		areaBID:    areaBID,
		dbx:        dbx,
	}
}

func tagCtxFor(user *models.User, groupID string) context.Context {
	ctx := appctx.WithUser(context.Background(), user)
	return appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: groupID},
			TenantID: user.TenantID,
		},
	})
}

func seedTagArea(c *qt.C, set *registry.Set, ctx context.Context) string {
	c.Helper()
	loc, err := set.LocationRegistry.Create(ctx, models.Location{
		Name:    "Loc",
		Address: "addr",
	})
	c.Assert(err, qt.IsNil)
	area, err := set.AreaRegistry.Create(ctx, models.Area{
		Name:       "Area",
		LocationID: loc.GetID(),
	})
	c.Assert(err, qt.IsNil)
	return area.GetID()
}

func seedTagCommodity(c *qt.C, set *registry.Set, ctx context.Context, areaID, name string, tags ...string) string {
	c.Helper()
	cmd, err := set.CommodityRegistry.Create(ctx, models.Commodity{
		Name:                   name,
		ShortName:              name,
		Type:                   models.CommodityTypeOther,
		AreaID:                 areaID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(100.00),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.Zero,
		CurrentPrice:           decimal.NewFromFloat(90.00),
		Status:                 models.CommodityStatusInUse,
		PurchaseDate:           models.ToPDate("2024-01-01"),
		RegisteredDate:         models.ToPDate("2024-01-02"),
		LastModifiedDate:       models.ToPDate("2024-01-03"),
		Tags:                   tags,
	})
	c.Assert(err, qt.IsNil)
	return cmd.GetID()
}

func seedTagFile(c *qt.C, set *registry.Set, ctx context.Context, name string, tags ...string) string {
	c.Helper()
	file, err := set.FileRegistry.Create(ctx, models.FileEntity{
		Title:    name,
		Type:     models.FileTypeImage,
		Category: models.FileCategoryImages,
		Tags:     tags,
		File: &models.File{
			Path:         name,
			OriginalPath: name + ".jpg",
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	})
	c.Assert(err, qt.IsNil)
	return file.GetID()
}

func mustCreateTag(c *qt.C, reg registry.TagRegistry, ctx context.Context, slug string) *models.Tag {
	c.Helper()
	tag, err := reg.Create(ctx, models.Tag{
		Slug:  slug,
		Label: slug,
		Color: models.DefaultTagColor,
	})
	c.Assert(err, qt.IsNil)
	return tag
}

// rawCommodityTagsText reads the literal JSONB column as text — needed to
// distinguish `[]` from `null`, which both round-trip to an empty Go slice
// after StructScan.
func rawCommodityTagsText(c *qt.C, dbx *sqlx.DB, id string) string {
	c.Helper()
	var raw *string
	err := dbx.QueryRowxContext(context.Background(),
		`SELECT tags::text FROM commodities WHERE id = $1`, id).Scan(&raw)
	c.Assert(err, qt.IsNil)
	if raw == nil {
		return "null"
	}
	return *raw
}

func TestTagRegistry_Postgres_RewriteSlugReferences(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	cmdID := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "fridge", "kitchen", "appliance")
	fileID := seedTagFile(c, fx.groupASet, fx.ctxA, "fridge-photo", "kitchen")

	commCount, fileCount, err := fx.groupASet.TagRegistry.RewriteSlugReferences(fx.ctxA, "kitchen", "kitchen-area")
	c.Assert(err, qt.IsNil)
	c.Assert(commCount, qt.Equals, 1)
	c.Assert(fileCount, qt.Equals, 1)

	cmd, err := fx.groupASet.CommodityRegistry.Get(fx.ctxA, cmdID)
	c.Assert(err, qt.IsNil)
	// jsonb_agg(DISTINCT ...) doesn't promise input order — assert by membership.
	c.Assert([]string(cmd.Tags), qt.Contains, "kitchen-area")
	c.Assert([]string(cmd.Tags), qt.Contains, "appliance")
	c.Assert([]string(cmd.Tags), qt.Not(qt.Contains), "kitchen")

	file, err := fx.groupASet.FileRegistry.Get(fx.ctxA, fileID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(file.Tags), qt.DeepEquals, []string{"kitchen-area"})
}

func TestTagRegistry_Postgres_RewriteSlugReferences_DedupOnRenameOntoExisting(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	// Row already contains both old and new slugs — DISTINCT in jsonb_agg
	// must collapse them to a single occurrence post-rewrite.
	cmdID := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "fridge", "kitchen", "kitchen-area", "appliance")

	commCount, _, err := fx.groupASet.TagRegistry.RewriteSlugReferences(fx.ctxA, "kitchen", "kitchen-area")
	c.Assert(err, qt.IsNil)
	c.Assert(commCount, qt.Equals, 1)

	cmd, err := fx.groupASet.CommodityRegistry.Get(fx.ctxA, cmdID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(cmd.Tags), qt.HasLen, 2)
	c.Assert([]string(cmd.Tags), qt.Contains, "kitchen-area")
	c.Assert([]string(cmd.Tags), qt.Contains, "appliance")
}

func TestTagRegistry_Postgres_RewriteSlugReferences_CrossGroupIsolation(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	cmdA := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "in-A", "kitchen")
	cmdB := seedTagCommodity(c, fx.groupBSet, fx.ctxB, fx.areaBID, "in-B", "kitchen")
	fileB := seedTagFile(c, fx.groupBSet, fx.ctxB, "in-B-photo", "kitchen")

	// Rewrite scoped to group A only.
	commCount, fileCount, err := fx.groupASet.TagRegistry.RewriteSlugReferences(fx.ctxA, "kitchen", "kitchen-area")
	c.Assert(err, qt.IsNil)
	c.Assert(commCount, qt.Equals, 1)
	c.Assert(fileCount, qt.Equals, 0)

	gotA, err := fx.groupASet.CommodityRegistry.Get(fx.ctxA, cmdA)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(gotA.Tags), qt.DeepEquals, []string{"kitchen-area"})

	gotB, err := fx.groupBSet.CommodityRegistry.Get(fx.ctxB, cmdB)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(gotB.Tags), qt.DeepEquals, []string{"kitchen"},
		qt.Commentf("group B's commodity must remain untouched after group-A rewrite"))

	gotFileB, err := fx.groupBSet.FileRegistry.Get(fx.ctxB, fileB)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(gotFileB.Tags), qt.DeepEquals, []string{"kitchen"})
}

func TestTagRegistry_Postgres_RewriteSlugReferences_LeavesUnrelatedRowsAlone(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	emptyID := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "no-tags")
	otherID := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "other", "appliance")
	taggedID := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "fridge", "kitchen")

	commCount, _, err := fx.groupASet.TagRegistry.RewriteSlugReferences(fx.ctxA, "kitchen", "kitchen-area")
	c.Assert(err, qt.IsNil)
	c.Assert(commCount, qt.Equals, 1, qt.Commentf("only the row containing the old slug is touched"))

	gotEmpty, err := fx.groupASet.CommodityRegistry.Get(fx.ctxA, emptyID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(gotEmpty.Tags), qt.HasLen, 0)

	gotOther, err := fx.groupASet.CommodityRegistry.Get(fx.ctxA, otherID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(gotOther.Tags), qt.DeepEquals, []string{"appliance"})

	gotTagged, err := fx.groupASet.CommodityRegistry.Get(fx.ctxA, taggedID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(gotTagged.Tags), qt.DeepEquals, []string{"kitchen-area"})
}

func TestTagRegistry_Postgres_RewriteSlugReferences_NoOpWhenSlugUnchanged(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	cmdID := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "fridge", "kitchen")

	commCount, fileCount, err := fx.groupASet.TagRegistry.RewriteSlugReferences(fx.ctxA, "kitchen", "kitchen")
	c.Assert(err, qt.IsNil)
	c.Assert(commCount, qt.Equals, 0)
	c.Assert(fileCount, qt.Equals, 0)

	cmd, err := fx.groupASet.CommodityRegistry.Get(fx.ctxA, cmdID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(cmd.Tags), qt.DeepEquals, []string{"kitchen"})
}

func TestTagRegistry_Postgres_StripSlugReferences(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	cmdID := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "fridge", "kitchen", "appliance")
	fileID := seedTagFile(c, fx.groupASet, fx.ctxA, "fridge-photo", "kitchen")

	commCount, fileCount, err := fx.groupASet.TagRegistry.StripSlugReferences(fx.ctxA, "kitchen")
	c.Assert(err, qt.IsNil)
	c.Assert(commCount, qt.Equals, 1)
	c.Assert(fileCount, qt.Equals, 1)

	cmd, err := fx.groupASet.CommodityRegistry.Get(fx.ctxA, cmdID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(cmd.Tags), qt.DeepEquals, []string{"appliance"})

	file, err := fx.groupASet.FileRegistry.Get(fx.ctxA, fileID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(file.Tags), qt.HasLen, 0)
}

func TestTagRegistry_Postgres_StripSlugReferences_EmptyArrayNotNull(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	// Single-tag row → after strip the JSONB value must persist as `[]`,
	// not `null`. Mismatch would break downstream code that calls
	// jsonb_array_length on the column.
	cmdID := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "fridge", "kitchen")

	_, _, err := fx.groupASet.TagRegistry.StripSlugReferences(fx.ctxA, "kitchen")
	c.Assert(err, qt.IsNil)

	c.Assert(rawCommodityTagsText(c, fx.dbx, cmdID), qt.Equals, "[]")
}

func TestTagRegistry_Postgres_StripSlugReferences_CrossGroupIsolation(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	cmdA := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "in-A", "kitchen", "appliance")
	cmdB := seedTagCommodity(c, fx.groupBSet, fx.ctxB, fx.areaBID, "in-B", "kitchen")

	commCount, _, err := fx.groupASet.TagRegistry.StripSlugReferences(fx.ctxA, "kitchen")
	c.Assert(err, qt.IsNil)
	c.Assert(commCount, qt.Equals, 1)

	gotA, err := fx.groupASet.CommodityRegistry.Get(fx.ctxA, cmdA)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(gotA.Tags), qt.DeepEquals, []string{"appliance"})

	gotB, err := fx.groupBSet.CommodityRegistry.Get(fx.ctxB, cmdB)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(gotB.Tags), qt.DeepEquals, []string{"kitchen"})
}

// TestTagService_Postgres_RenameTag_RefusesPreemptivelyOnSlugClash asserts
// the slug-clash check fires before any JSONB UPDATE — partial rewrite
// would be the worst-of-both-worlds outcome.
func TestTagService_Postgres_RenameTag_RefusesPreemptivelyOnSlugClash(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	srcTag := mustCreateTag(c, fx.groupASet.TagRegistry, fx.ctxA, "kitchen")
	_ = mustCreateTag(c, fx.groupASet.TagRegistry, fx.ctxA, "kitchen-area")
	cmdID := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "fridge", "kitchen")

	svc := services.NewTagService(fx.factorySet)
	_, err := svc.RenameTag(fx.ctxA, srcTag.ID, "Kitchen Area", "kitchen-area", "")
	c.Assert(err, qt.IsNotNil)
	c.Assert(errors.Is(err, registry.ErrAlreadyExists), qt.IsTrue,
		qt.Commentf("expected ErrAlreadyExists, got %v", err))

	cmd, err := fx.groupASet.CommodityRegistry.Get(fx.ctxA, cmdID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(cmd.Tags), qt.DeepEquals, []string{"kitchen"},
		qt.Commentf("JSONB must be untouched when rename is refused — no partial rewrite"))
}

// TestTagService_Postgres_RenameTag_ParallelDifferentSourceSlugs covers two
// renames operating on distinct tags in the same group: they share no
// row-level state, so both must succeed and both rewrites must land.
func TestTagService_Postgres_RenameTag_ParallelDifferentSourceSlugs(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	tagA := mustCreateTag(c, fx.groupASet.TagRegistry, fx.ctxA, "alpha")
	tagB := mustCreateTag(c, fx.groupASet.TagRegistry, fx.ctxA, "beta")
	cmdA := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "with-alpha", "alpha")
	cmdB := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "with-beta", "beta")

	svc := services.NewTagService(fx.factorySet)

	var wg sync.WaitGroup
	var errA, errB error
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, errA = svc.RenameTag(fx.ctxA, tagA.ID, "Alpha 2", "alpha-2", "")
	}()
	go func() {
		defer wg.Done()
		<-start
		_, errB = svc.RenameTag(fx.ctxA, tagB.ID, "Beta 2", "beta-2", "")
	}()
	close(start)
	wg.Wait()

	c.Assert(errA, qt.IsNil)
	c.Assert(errB, qt.IsNil)

	gotA, err := fx.groupASet.CommodityRegistry.Get(fx.ctxA, cmdA)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(gotA.Tags), qt.DeepEquals, []string{"alpha-2"})

	gotB, err := fx.groupASet.CommodityRegistry.Get(fx.ctxA, cmdB)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(gotB.Tags), qt.DeepEquals, []string{"beta-2"})

	tagAFinal, err := fx.groupASet.TagRegistry.Get(fx.ctxA, tagA.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(tagAFinal.Slug, qt.Equals, "alpha-2")
	tagBFinal, err := fx.groupASet.TagRegistry.Get(fx.ctxA, tagB.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(tagBFinal.Slug, qt.Equals, "beta-2")
}

// TestTagService_Postgres_RenameTag_ParallelSameSourceSlug is the racey one:
// two parallel renames target the same tag id but pick different new slugs.
// The invariant the issue calls out — "JSONB references match the surviving
// tags row's slug" — is what we assert. If the implementation can't deliver
// it, this test fails and exposes a real concurrency gap; we don't soften
// the assertion to keep the bar honest.
func TestTagService_Postgres_RenameTag_ParallelSameSourceSlug(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	srcTag := mustCreateTag(c, fx.groupASet.TagRegistry, fx.ctxA, "kitchen")
	cmdID := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "fridge", "kitchen")

	svc := services.NewTagService(fx.factorySet)

	var wg sync.WaitGroup
	var errA, errB error
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, errA = svc.RenameTag(fx.ctxA, srcTag.ID, "K1", "k1", "")
	}()
	go func() {
		defer wg.Done()
		<-start
		_, errB = svc.RenameTag(fx.ctxA, srcTag.ID, "K2", "k2", "")
	}()
	close(start)
	wg.Wait()

	finalTag, err := fx.groupASet.TagRegistry.Get(fx.ctxA, srcTag.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(finalTag.Slug, qt.Matches, "k[12]",
		qt.Commentf("surviving slug must be one of the two targets, got %q (errA=%v errB=%v)",
			finalTag.Slug, errA, errB))

	cmd, err := fx.groupASet.CommodityRegistry.Get(fx.ctxA, cmdID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(cmd.Tags), qt.DeepEquals, []string{finalTag.Slug},
		qt.Commentf("JSONB ref %v must match surviving tag slug %q (errA=%v errB=%v)",
			cmd.Tags, finalTag.Slug, errA, errB))
}

// TestTagService_Postgres_DeleteTag_ForceUnderConcurrentInsert covers the
// atomicity invariant from the issue: under a force-delete racing with a
// concurrent commodity insert that references the same slug, no commodity
// may end up referencing a slug whose tags row no longer exists.
//
// We accept either of the two consistent end-states the issue lists:
//   - insert wins → tag row survives, commodity points at it;
//   - delete wins + insert auto-recreated → fresh tag row exists with the
//     same slug, commodity points at the fresh row.
//
// What we refuse: a JSONB reference to a slug with no matching row.
func TestTagService_Postgres_DeleteTag_ForceUnderConcurrentInsert(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	srcTag := mustCreateTag(c, fx.groupASet.TagRegistry, fx.ctxA, "kitchen")
	_ = seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "existing", "kitchen")

	svc := services.NewTagService(fx.factorySet)

	var wg sync.WaitGroup
	var errDel, errIns error
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, errDel = svc.DeleteTag(fx.ctxA, srcTag.ID, true)
	}()
	go func() {
		defer wg.Done()
		<-start
		// Mirrors the apiserver write path: normalize-and-ensure runs first
		// (will auto-recreate if the tag was already deleted), then the
		// commodity row is persisted with the resolved slug list.
		slugs, ensErr := svc.NormalizeAndEnsureSlugs(fx.ctxA, []string{"kitchen"})
		if ensErr != nil {
			errIns = ensErr
			return
		}
		seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "racer", slugs...)
	}()
	close(start)
	wg.Wait()

	// Whatever interleaving won, the invariant holds at the end.
	allCmds, err := fx.groupASet.CommodityRegistry.List(fx.ctxA)
	c.Assert(err, qt.IsNil)
	for _, cmd := range allCmds {
		for _, slug := range cmd.Tags {
			_, lookupErr := fx.groupASet.TagRegistry.GetBySlug(fx.ctxA, slug)
			c.Assert(lookupErr, qt.IsNil,
				qt.Commentf("orphan reference: commodity %s -> tag slug %q with no matching row (errDel=%v errIns=%v)",
					cmd.ID, slug, errDel, errIns))
		}
	}
}

// TestTagRegistry_Postgres_SearchScoped verifies the per-scope strict
// filter on the autocomplete (Search) endpoint added for #1628. The
// scoped expression must match what GetUsage returns for the same slug.
func TestTagRegistry_Postgres_SearchScoped(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	// Seed four tags, vary usage by scope.
	mustCreateTag(c, fx.groupASet.TagRegistry, fx.ctxA, "kitchen")
	mustCreateTag(c, fx.groupASet.TagRegistry, fx.ctxA, "invoice")
	mustCreateTag(c, fx.groupASet.TagRegistry, fx.ctxA, "warranty")
	mustCreateTag(c, fx.groupASet.TagRegistry, fx.ctxA, "unused")

	seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "fridge", "kitchen", "warranty")
	seedTagFile(c, fx.groupASet, fx.ctxA, "fridge-receipt", "invoice", "warranty")

	gotCommodity, err := fx.groupASet.TagRegistry.Search(fx.ctxA, "", 10, registry.TagScopeCommodity)
	c.Assert(err, qt.IsNil)
	gotCommoditySlugs := make([]string, 0, len(gotCommodity))
	for _, t := range gotCommodity {
		gotCommoditySlugs = append(gotCommoditySlugs, t.Slug)
	}
	c.Assert(gotCommoditySlugs, qt.Contains, "kitchen")
	c.Assert(gotCommoditySlugs, qt.Contains, "warranty")
	c.Assert(gotCommoditySlugs, qt.Not(qt.Contains), "invoice")
	c.Assert(gotCommoditySlugs, qt.Not(qt.Contains), "unused")

	gotFile, err := fx.groupASet.TagRegistry.Search(fx.ctxA, "", 10, registry.TagScopeFile)
	c.Assert(err, qt.IsNil)
	gotFileSlugs := make([]string, 0, len(gotFile))
	for _, t := range gotFile {
		gotFileSlugs = append(gotFileSlugs, t.Slug)
	}
	c.Assert(gotFileSlugs, qt.Contains, "invoice")
	c.Assert(gotFileSlugs, qt.Contains, "warranty")
	c.Assert(gotFileSlugs, qt.Not(qt.Contains), "kitchen")
	c.Assert(gotFileSlugs, qt.Not(qt.Contains), "unused")

	// TagScopeAny includes every tag, including the unused one.
	gotAny, err := fx.groupASet.TagRegistry.Search(fx.ctxA, "", 10, registry.TagScopeAny)
	c.Assert(err, qt.IsNil)
	c.Assert(gotAny, qt.HasLen, 4)
}

// TestTagRegistry_Postgres_ListPaginatedScoped mirrors SearchScoped for
// the paginated listing endpoint. Asserts both filter + total count.
func TestTagRegistry_Postgres_ListPaginatedScoped(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	mustCreateTag(c, fx.groupASet.TagRegistry, fx.ctxA, "kitchen")
	mustCreateTag(c, fx.groupASet.TagRegistry, fx.ctxA, "invoice")
	mustCreateTag(c, fx.groupASet.TagRegistry, fx.ctxA, "warranty")
	mustCreateTag(c, fx.groupASet.TagRegistry, fx.ctxA, "unused")

	seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "fridge", "kitchen", "warranty")
	seedTagFile(c, fx.groupASet, fx.ctxA, "fridge-receipt", "invoice", "warranty")

	got, total, err := fx.groupASet.TagRegistry.ListPaginated(fx.ctxA, 0, 50, registry.TagListOptions{
		Scope: registry.TagScopeCommodity,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 2)
	slugs := make([]string, 0, len(got))
	for _, t := range got {
		slugs = append(slugs, t.Slug)
	}
	c.Assert(slugs, qt.Contains, "kitchen")
	c.Assert(slugs, qt.Contains, "warranty")

	got, total, err = fx.groupASet.TagRegistry.ListPaginated(fx.ctxA, 0, 50, registry.TagListOptions{
		Scope: registry.TagScopeFile,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 2)
	slugs = slugs[:0]
	for _, t := range got {
		slugs = append(slugs, t.Slug)
	}
	c.Assert(slugs, qt.Contains, "invoice")
	c.Assert(slugs, qt.Contains, "warranty")

	got, total, err = fx.groupASet.TagRegistry.ListPaginated(fx.ctxA, 0, 50, registry.TagListOptions{})
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 4)
	c.Assert(got, qt.HasLen, 4)
}
