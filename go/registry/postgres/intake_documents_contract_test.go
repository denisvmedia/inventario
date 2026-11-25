package postgres_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/registrytest"
)

func TestIntakeDocumentContract_Postgres(t *testing.T) {
	registrytest.RunIntakeDocumentContract(t, newPostgresIntakeDocumentEnv)
}

// newPostgresIntakeDocumentEnv returns the registries of the test group.
func newPostgresIntakeDocumentEnv(t *testing.T) registrytest.IntakeDocumentEnv {
	c := qt.New(t)
	fx := newFileLinkPGFixture(t)
	location := createTestLocation(c, fx.set)
	area := createTestArea(c, fx.set, location.ID)
	return registrytest.IntakeDocumentEnv{
		Ctx:       fx.ctx,
		Documents: fx.set.IntakeDocumentRegistry,
		Service:   fx.factorySet.IntakeDocumentRegistryFactory.CreateServiceRegistry(),
		Drafts:    fx.set.CommodityDraftRegistry,
		AreaID:    area.ID,
	}
}

// TestIntakeDocuments_Postgres_GroupIsolation: under RLS a group neither sees
// nor changes another group's delivered documents, the identity is unique
// per group only, and another group's source is not found by its identity.
func TestIntakeDocuments_Postgres_GroupIsolation(t *testing.T) {
	c := qt.New(t)
	fx := newLoanPGFixture(t)

	docsA := fx.groupASet.IntakeDocumentRegistry
	docsB, err := fx.factorySet.IntakeDocumentRegistryFactory.CreateUserRegistry(fx.ctxB)
	c.Assert(err, qt.IsNil)

	doc := models.IntakeDocument{ExternalSystem: "paperless-ngx", ExternalDocumentID: "7", PayloadHash: "h"}
	a, created, err := docsA.CreateOrGet(fx.ctxA, doc)
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)

	_, err = docsB.Get(fx.ctxB, a.ID)
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = docsB.GetByExternalID(fx.ctxB, "paperless-ngx", "", "7")
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = docsB.RecordProgress(fx.ctxB, a.ID, registry.IntakeDocumentProgress{DraftID: new("x")})
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	c.Check(docsB.Delete(fx.ctxB, a.ID), qt.IsNotNil)
	listB, err := docsB.List(fx.ctxB)
	c.Assert(err, qt.IsNil)
	c.Check(listB, qt.HasLen, 0)

	// The same identity in group B is a document of its own.
	b, created, err := docsB.CreateOrGet(fx.ctxB, doc)
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)
	c.Assert(b.ID, qt.Not(qt.Equals), a.ID)

	// A source with the identity in group A is not found from group B.
	intakeA, err := fx.factorySet.IntakeRegistryFactory.CreateUserRegistry(fx.ctxA)
	c.Assert(err, qt.IsNil)
	intakeB, err := fx.factorySet.IntakeRegistryFactory.CreateUserRegistry(fx.ctxB)
	c.Assert(err, qt.IsNil)
	_, _, err = intakeA.CreateSession(fx.ctxA, models.IntakeSession{ClientKey: "external:a", Origin: models.IntakeOriginExternal},
		[]registry.IntakeSourceInput{{ExternalSystem: "paperless-ngx", ExternalDocumentID: "7"}}, 0)
	c.Assert(err, qt.IsNil)
	_, err = intakeB.GetSourceByExternalID(fx.ctxB, "paperless-ngx", "", "7")
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	found, err := intakeA.GetSourceByExternalID(fx.ctxA, "paperless-ngx", "", "7")
	c.Assert(err, qt.IsNil)
	c.Check(found.GroupID, qt.Equals, a.GroupID)
}
