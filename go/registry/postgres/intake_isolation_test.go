package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// TestIntake_Postgres_GroupIsolation: under RLS a group neither sees nor
// changes another group's sessions, sources and products, cannot link
// another group's file, and the client key and the external identity are
// unique per group only.
func TestIntake_Postgres_GroupIsolation(t *testing.T) {
	c := qt.New(t)
	fx := newLoanPGFixture(t)

	file := func(ctx context.Context, set *registry.Set, name string) *models.FileEntity {
		f, err := set.FileRegistry.Create(ctx, models.FileEntity{
			Title: name, Type: models.FileTypeDocument, Category: models.FileCategoryDocuments,
			File: &models.File{Path: name, OriginalPath: "files/" + name, Ext: ".pdf", MIMEType: "application/pdf", SizeBytes: 10},
		})
		c.Assert(err, qt.IsNil)
		return f
	}
	regA, err := fx.factorySet.IntakeRegistryFactory.CreateUserRegistry(fx.ctxA)
	c.Assert(err, qt.IsNil)
	regB, err := fx.factorySet.IntakeRegistryFactory.CreateUserRegistry(fx.ctxB)
	c.Assert(err, qt.IsNil)

	fileA := file(fx.ctxA, fx.groupASet, "a.pdf")
	doc := registry.IntakeSourceInput{FileIDs: []string{fileA.ID}, ExternalSystem: "paperless-ngx", ExternalInstance: "https://docs", ExternalDocumentID: "7"}
	sessA, created, err := regA.CreateSession(fx.ctxA, models.IntakeSession{ClientKey: "same-key"}, []registry.IntakeSourceInput{doc}, 0)
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)
	srcs, err := regA.ListSources(fx.ctxA, sessA.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(srcs, qt.HasLen, 1)

	// Scan through the service registry so A has a product.
	svc := fx.factorySet.IntakeRegistryFactory.CreateServiceRegistry()
	claimed, err := svc.ClaimNextSource(context.Background(), time.Now().Add(time.Second))
	c.Assert(err, qt.IsNil)
	c.Assert(svc.StoreScan(context.Background(), claimed.ID, claimed.Attempts, registry.IntakeScan{Candidates: []models.IntakeCandidate{{
		Fields: models.IntakeFields{"name": {Value: "x", Confidence: 1}}, Selected: true,
	}}}, time.Now()), qt.IsNil)
	candsA, err := regA.ListCandidates(fx.ctxA, sessA.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(candsA, qt.HasLen, 1)

	// Group B sees and changes nothing of A.
	_, err = regB.Get(fx.ctxB, sessA.ID)
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	list, total, err := regB.ListSessions(fx.ctxB, 0, 10)
	c.Assert(err, qt.IsNil)
	c.Check(list, qt.HasLen, 0)
	c.Check(total, qt.Equals, 0)
	got, err := regB.ListSources(fx.ctxB, sessA.ID)
	c.Assert(err, qt.IsNil)
	c.Check(got, qt.HasLen, 0)
	gotC, err := regB.ListCandidates(fx.ctxB, sessA.ID)
	c.Assert(err, qt.IsNil)
	c.Check(gotC, qt.HasLen, 0)
	_, err = regB.GetCandidate(fx.ctxB, candsA[0].ID)
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = regB.ReviewCandidate(fx.ctxB, candsA[0].ID, registry.IntakeCandidateReview{Selected: new(false)})
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = regB.SetCandidateOutcome(fx.ctxB, candsA[0].ID, models.IntakeCandidateSkipped, nil, "")
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	n, err := regB.CancelSession(fx.ctxB, sessA.ID)
	c.Assert(err, qt.IsNil)
	c.Check(n, qt.Equals, 0)
	_, err = regB.RescanSource(fx.ctxB, srcs[0].ID, time.Now())
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	c.Check(regB.Delete(fx.ctxB, sessA.ID), qt.ErrorIs, registry.ErrNotFound)
	_, err = regB.GetSourceByExternalID(fx.ctxB, "paperless-ngx", "https://docs", "7")
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = regB.GetSessionByClientKey(fx.ctxB, "same-key")
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)

	// B cannot link A's file into its own session.
	_, _, err = regB.CreateSession(fx.ctxB, models.IntakeSession{ClientKey: "steal"}, []registry.IntakeSourceInput{{FileIDs: []string{fileA.ID}}}, 0)
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)

	// The same client key and the same external document are fine in B.
	fileB := file(fx.ctxB, fx.groupBSet, "b.pdf")
	docB := doc
	docB.FileIDs = []string{fileB.ID}
	sessB, created, err := regB.CreateSession(fx.ctxB, models.IntakeSession{ClientKey: "same-key"}, []registry.IntakeSourceInput{docB}, 0)
	c.Assert(err, qt.IsNil)
	c.Check(created, qt.IsTrue)
	c.Check(sessB.ID, qt.Not(qt.Equals), sessA.ID)

	// A's session is untouched.
	candsA2, err := regA.ListCandidates(fx.ctxA, sessA.ID)
	c.Assert(err, qt.IsNil)
	c.Check(candsA2[0].Selected, qt.IsTrue)
	c.Check(candsA2[0].Outcome, qt.Equals, models.IntakeCandidatePending)
}
