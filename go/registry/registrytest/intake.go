package registrytest

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// IntakeEnv is what RunIntakeContract needs from a backend: the user
// registry of one empty group with the context that carries the group scope,
// the service registry the worker uses, and the group's file and draft
// registries.
type IntakeEnv struct {
	Ctx     context.Context
	Intake  registry.IntakeRegistry
	Service registry.IntakeRegistry
	Files   registry.FileRegistry
	Drafts  registry.CommodityDraftRegistry
}

// intakeTestMaxAttempts and intakeTestStuckCode are what the contract passes
// to SweepStuckSources.
const (
	intakeTestMaxAttempts = 3
	intakeTestStuckCode   = "provider_error"
)

// RunIntakeContract checks the intake lifecycle a backend's IntakeRegistry
// implements (#2810): idempotent create by client key with the source files
// linked, the scan queue the worker drives, review and outcomes of detected
// products, cancel, resume, rescan and delete. newEnv is called once per
// subtest and must return an empty group.
func RunIntakeContract(t *testing.T, newEnv func(t *testing.T) IntakeEnv) {
	tests := []struct {
		name string
		run  func(t *testing.T, env IntakeEnv)
	}{
		{"create is idempotent and links the files", intakeContractCreate},
		{"create keeps the queue within its limit", intakeContractQueueLimit},
		{"external identity is unique in the group", intakeContractExternalIdentity},
		{"a delivered source is stored scanned and skips the queue", intakeContractDelivered},
		{"the worker claims, stores and finishes scans", intakeContractWorkerLifecycle},
		{"two workers never claim the same source", intakeContractConcurrentClaim},
		{"the sweep requeues a stuck scan and drops its late result", intakeContractStuckSweep},
		{"the sweep fails a source that keeps getting stuck", intakeContractStuckSweepLimit},
		{"cancel stops the queue and resume retries only unfinished sources", intakeContractCancelResume},
		{"cancel and resume during a scan keep a single claim", intakeContractCancelResumeInScan},
		{"review and outcomes of a candidate", intakeContractReviewOutcomes},
		{"rescan replaces the candidates", intakeContractRescan},
		{"rescan is refused for a draft the outcome missed", intakeContractRescanUnrecordedDraft},
		{"delete takes sources, candidates and links but not files", intakeContractDelete},
		{"sessions list newest first", intakeContractListOrder},
		{"open sessions are the ones with work left", intakeContractOpenSessions},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, newEnv(t))
		})
	}
}

func intakeContractCreate(t *testing.T, env IntakeEnv) {
	c := qt.New(t)
	receipt := intakeFile(c, env, "receipt.pdf", models.FileCategoryDocuments)
	photo := intakeFile(c, env, "photo.jpg", models.FileCategoryImages)

	session, created, err := env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "batch-1"}, []registry.IntakeSourceInput{
		{FileIDs: []string{receipt.ID}},
		{FileIDs: []string{photo.ID}, OneItem: true},
	}, 0)
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)
	c.Assert(session.ID, qt.Not(qt.Equals), "")
	c.Assert(session.Origin, qt.Equals, models.IntakeOriginUpload)

	again, created, err := env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "batch-1"}, []registry.IntakeSourceInput{
		{FileIDs: []string{receipt.ID}},
	}, 0)
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsFalse)
	c.Assert(again.ID, qt.Equals, session.ID)

	byKey, err := env.Intake.GetSessionByClientKey(env.Ctx, "batch-1")
	c.Assert(err, qt.IsNil)
	c.Assert(byKey.ID, qt.Equals, session.ID)

	sources, err := env.Intake.ListSources(env.Ctx, session.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(sources, qt.HasLen, 2, qt.Commentf("a repeated create adds no sources"))
	c.Assert(sources[0].SortOrder, qt.Equals, 0)
	c.Assert(sources[0].OneItem, qt.IsFalse)
	c.Assert(sources[0].Status, qt.Equals, models.IntakeSourcePending)
	c.Assert(sources[1].OneItem, qt.IsTrue)

	onReceipt, err := env.Files.ListByLinkedEntity(env.Ctx, models.FileLinkIntake, sources[0].ID)
	c.Assert(err, qt.IsNil)
	c.Assert(onReceipt, qt.HasLen, 1)
	c.Assert(onReceipt[0].ID, qt.Equals, receipt.ID)
	link := onReceipt[0].LinkTo(models.FileLinkIntake, sources[0].ID)
	c.Assert(link, qt.IsNotNil)
	c.Assert(link.Meta, qt.Equals, models.FileLinkMetaDocuments)

	onPhoto, err := env.Files.ListByLinkedEntity(env.Ctx, models.FileLinkIntake, sources[1].ID)
	c.Assert(err, qt.IsNil)
	c.Assert(onPhoto, qt.HasLen, 1)
	c.Assert(onPhoto[0].LinkTo(models.FileLinkIntake, sources[1].ID).Meta, qt.Equals, models.FileLinkMetaImages)

	_, _, err = env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "batch-2"}, []registry.IntakeSourceInput{
		{FileIDs: []string{"missing-file"}},
	}, 0)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = env.Intake.GetSessionByClientKey(env.Ctx, "batch-2")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound, qt.Commentf("a refused create leaves no session"))
}

func intakeContractQueueLimit(t *testing.T, env IntakeEnv) {
	c := qt.New(t)
	_, created, err := env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "first"}, []registry.IntakeSourceInput{{}, {}}, 3)
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)

	_, _, err = env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "second"}, []registry.IntakeSourceInput{{}, {}}, 3)
	c.Assert(err, qt.ErrorIs, registry.ErrIntakeQueueFull)
	_, err = env.Intake.GetSessionByClientKey(env.Ctx, "second")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound, qt.Commentf("a refused create leaves no session"))

	// A replay is not a new batch: it returns the session whatever the
	// queue holds.
	_, created, err = env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "first"}, []registry.IntakeSourceInput{{}, {}}, 1)
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsFalse)

	// Sessions created at once share the room: with one place left, one of
	// them gets it.
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		made int
	)
	for i := range 4 {
		wg.Go(func() {
			_, created, err := env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "race-" + strconv.Itoa(i)}, []registry.IntakeSourceInput{{}}, 3)
			if err == nil && created {
				mu.Lock()
				made++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	c.Assert(made, qt.Equals, 1)
	queued, err := env.Intake.CountQueued(env.Ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(queued, qt.Equals, 3)
}

func intakeContractExternalIdentity(t *testing.T, env IntakeEnv) {
	c := qt.New(t)
	doc := registry.IntakeSourceInput{ExternalSystem: "paperless-ngx", ExternalInstance: "https://docs.example", ExternalDocumentID: "42"}

	session, _, err := env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "doc-42", Origin: models.IntakeOriginExternal}, []registry.IntakeSourceInput{doc}, 0)
	c.Assert(err, qt.IsNil)
	c.Assert(session.Origin, qt.Equals, models.IntakeOriginExternal)

	found, err := env.Intake.GetSourceByExternalID(env.Ctx, "paperless-ngx", "https://docs.example", "42")
	c.Assert(err, qt.IsNil)
	c.Assert(found.SessionID, qt.Equals, session.ID)
	_, err = env.Intake.GetSourceByExternalID(env.Ctx, "paperless-ngx", "https://other.example", "42")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	_, _, err = env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "doc-42-again"}, []registry.IntakeSourceInput{doc}, 0)
	c.Assert(err, qt.ErrorIs, registry.ErrAlreadyExists)

	// Two uploads carry no external identity and never collide.
	_, _, err = env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "u1"}, []registry.IntakeSourceInput{{}, {}}, 0)
	c.Assert(err, qt.IsNil)

	// A document id names the system it belongs to.
	_, _, err = env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "doc-no-system"},
		[]registry.IntakeSourceInput{{ExternalDocumentID: "43"}}, 0)
	c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
	_, err = env.Intake.GetSessionByClientKey(env.Ctx, "doc-no-system")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// The identity is unique per group only, so the service registry, which
	// sees every group, cannot look it up.
	_, err = env.Service.GetSourceByExternalID(env.Ctx, "paperless-ngx", "https://docs.example", "42")
	c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
}

func intakeContractDelivered(t *testing.T, env IntakeEnv) {
	c := qt.New(t)
	receipt := intakeFile(c, env, "invoice.pdf", models.FileCategoryDocuments)
	delivered := registry.IntakeSourceInput{
		FileIDs:            []string{receipt.ID},
		OneItem:            true,
		ExternalSystem:     "paperless-ngx",
		ExternalDocumentID: "117",
		Delivered: &registry.IntakeScan{Candidates: []models.IntakeCandidate{{
			Fields:         models.IntakeFields{"name": {Value: "Invoice 117", Confidence: 1}},
			Selected:       true,
			AcceptedFields: []string{"name"},
			Quantity:       1,
		}}},
	}
	// A queue limit of one, filled by an upload, still takes the delivery.
	_, _, err := env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "upload"}, []registry.IntakeSourceInput{{}}, 1)
	c.Assert(err, qt.IsNil)
	session, created, err := env.Intake.CreateSession(env.Ctx,
		models.IntakeSession{ClientKey: "external:117", Origin: models.IntakeOriginExternal},
		[]registry.IntakeSourceInput{delivered}, 1)
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)

	sources, err := env.Intake.ListSources(env.Ctx, session.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(sources, qt.HasLen, 1)
	c.Assert(sources[0].Status, qt.Equals, models.IntakeSourceScanned)
	c.Assert(sources[0].ScannedAt, qt.IsNotNil)
	c.Assert(sources[0].ExternalDocumentID, qt.Equals, "117")
	linked, err := env.Files.ListByLinkedEntity(env.Ctx, models.FileLinkIntake, sources[0].ID)
	c.Assert(err, qt.IsNil)
	c.Assert(linked, qt.HasLen, 1)

	candidates, err := env.Intake.ListCandidates(env.Ctx, session.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(candidates, qt.HasLen, 1)
	c.Assert(candidates[0].Fields["name"].Value, qt.Equals, "Invoice 117")
	c.Assert(candidates[0].Outcome, qt.Equals, models.IntakeCandidatePending)
	c.Assert(candidates[0].SourceID, qt.Equals, sources[0].ID)

	queued, err := env.Intake.CountQueued(env.Ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(queued, qt.Equals, 1)
	// The worker takes the upload, never the delivery.
	claimed, err := env.Service.ClaimNextSource(env.Ctx, time.Now().Add(time.Second))
	c.Assert(err, qt.IsNil)
	c.Assert(claimed.SessionID, qt.Not(qt.Equals), session.ID)
	_, err = env.Service.ClaimNextSource(env.Ctx, time.Now().Add(time.Second))
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func intakeContractWorkerLifecycle(t *testing.T, env IntakeEnv) {
	c := qt.New(t)
	session, _, err := env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "batch"}, []registry.IntakeSourceInput{{}, {}, {}}, 0)
	c.Assert(err, qt.IsNil)
	now := time.Now()

	queued, err := env.Intake.CountQueued(env.Ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(queued, qt.Equals, 3)

	first, err := env.Service.ClaimNextSource(env.Ctx, now.Add(time.Second))
	c.Assert(err, qt.IsNil)
	c.Assert(first.Status, qt.Equals, models.IntakeSourceScanning)
	c.Assert(first.Attempts, qt.Equals, 1)
	c.Assert(first.SortOrder, qt.Equals, 0, qt.Commentf("the first dropped source is scanned first"))

	err = env.Service.StoreScan(env.Ctx, first.ID, first.Attempts, registry.IntakeScan{
		Candidates: []models.IntakeCandidate{
			{Fields: models.IntakeFields{"name": {Value: "Kettle", Confidence: 0.9}, "original_price": {Value: 39.9, Confidence: 0.8}}, Selected: true, AcceptedFields: []string{"name", "original_price"}, Quantity: 1},
			{Fields: models.IntakeFields{"name": {Value: "Toaster", Confidence: 0.7}}, Selected: true, AcceptedFields: []string{"name"}, Quantity: 2},
		},
		Warnings: []models.IntakeWarning{{Code: "low_confidence", Field: "serial_number"}},
	}, now)
	c.Assert(err, qt.IsNil)
	err = env.Service.StoreScan(env.Ctx, first.ID, first.Attempts, registry.IntakeScan{}, now)
	c.Assert(err, qt.ErrorIs, registry.ErrIntakeSourceChanged, qt.Commentf("a stored scan is not stored twice"))

	stored, err := env.Intake.GetSource(env.Ctx, first.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.Status, qt.Equals, models.IntakeSourceScanned)
	c.Assert(stored.ScannedAt, qt.IsNotNil)
	c.Assert(stored.Warnings, qt.HasLen, 1)

	candidates, err := env.Intake.ListCandidates(env.Ctx, session.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(candidates, qt.HasLen, 2)
	c.Assert(candidates[0].Fields["name"].Value, qt.Equals, "Kettle")
	c.Assert(candidates[0].Fields["original_price"].Value, qt.Equals, 39.9)
	c.Assert(candidates[0].Outcome, qt.Equals, models.IntakeCandidatePending)
	c.Assert(candidates[1].Quantity, qt.Equals, 2)
	c.Assert(candidates[1].SessionID, qt.Equals, session.ID)
	c.Assert(candidates[1].SourceID, qt.Equals, first.ID)

	second, err := env.Service.ClaimNextSource(env.Ctx, now.Add(time.Second))
	c.Assert(err, qt.IsNil)
	c.Assert(second.SortOrder, qt.Equals, 1)
	// A rate-limited wait does not count against the limit, and the count
	// of attempts still grows.
	err = env.Service.RequeueSource(env.Ctx, second.ID, second.Attempts, now.Add(time.Hour), models.IntakeWaitRateLimited)
	c.Assert(err, qt.IsNil)
	waiting, err := env.Intake.GetSource(env.Ctx, second.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(waiting.Status, qt.Equals, models.IntakeSourcePending)
	c.Assert(waiting.Attempts, qt.Equals, 1)
	c.Assert(waiting.CountedAttempts(), qt.Equals, 0)
	c.Assert(waiting.WaitReason, qt.Equals, models.IntakeWaitRateLimited)

	third, err := env.Service.ClaimNextSource(env.Ctx, now.Add(time.Second))
	c.Assert(err, qt.IsNil)
	c.Assert(third.SortOrder, qt.Equals, 2, qt.Commentf("a source that waits is passed over"))
	err = env.Service.FailSource(env.Ctx, third.ID, third.Attempts, "unreadable")
	c.Assert(err, qt.IsNil)
	failed, err := env.Intake.GetSource(env.Ctx, third.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(failed.Status, qt.Equals, models.IntakeSourceFailed)
	c.Assert(failed.ErrorCode, qt.Equals, "unreadable")

	_, err = env.Service.ClaimNextSource(env.Ctx, now.Add(time.Second))
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound, qt.Commentf("nothing else is due"))
	later, err := env.Service.ClaimNextSource(env.Ctx, now.Add(2*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(later.ID, qt.Equals, second.ID)
	c.Assert(later.Attempts, qt.Equals, 2, qt.Commentf("a new claim never reuses the attempt of an earlier one"))
	c.Assert(later.CountedAttempts(), qt.Equals, 1)
	err = env.Service.StoreScan(env.Ctx, second.ID, second.Attempts, registry.IntakeScan{}, now)
	c.Assert(err, qt.ErrorIs, registry.ErrIntakeSourceChanged, qt.Commentf("the earlier claim's result is dropped"))
}

func intakeContractConcurrentClaim(t *testing.T, env IntakeEnv) {
	c := qt.New(t)
	_, _, err := env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "batch"}, []registry.IntakeSourceInput{{}, {}, {}, {}}, 0)
	c.Assert(err, qt.IsNil)
	at := time.Now().Add(time.Second)

	var (
		mu      sync.Mutex
		claimed = map[string]int{}
		wg      sync.WaitGroup
	)
	for range 8 {
		wg.Go(func() {
			src, err := env.Service.ClaimNextSource(env.Ctx, at)
			if err != nil {
				return
			}
			mu.Lock()
			claimed[src.ID]++
			mu.Unlock()
		})
	}
	wg.Wait()
	c.Assert(claimed, qt.HasLen, 4)
	for id, n := range claimed {
		c.Assert(n, qt.Equals, 1, qt.Commentf("source %s claimed %d times", id, n))
	}
}

func intakeContractStuckSweep(t *testing.T, env IntakeEnv) {
	c := qt.New(t)
	_, _, err := env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "batch"}, []registry.IntakeSourceInput{{}}, 0)
	c.Assert(err, qt.IsNil)
	now := time.Now()
	stuck, err := env.Service.ClaimNextSource(env.Ctx, now.Add(time.Second))
	c.Assert(err, qt.IsNil)

	swept, err := env.Service.SweepStuckSources(env.Ctx, now.Add(-time.Minute), now, intakeTestMaxAttempts, intakeTestStuckCode)
	c.Assert(err, qt.IsNil)
	c.Assert(swept, qt.Equals, 0, qt.Commentf("a recent claim is not stuck"))
	swept, err = env.Service.SweepStuckSources(env.Ctx, now.Add(time.Minute), now, intakeTestMaxAttempts, intakeTestStuckCode)
	c.Assert(err, qt.IsNil)
	c.Assert(swept, qt.Equals, 1)
	requeued, err := env.Intake.GetSource(env.Ctx, stuck.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(requeued.Status, qt.Equals, models.IntakeSourcePending)
	c.Assert(requeued.WaitReason, qt.Equals, models.IntakeWaitRetry)

	again, err := env.Service.ClaimNextSource(env.Ctx, now.Add(time.Second))
	c.Assert(err, qt.IsNil)
	c.Assert(again.ID, qt.Equals, stuck.ID)
	c.Assert(again.Attempts, qt.Equals, 2)

	err = env.Service.StoreScan(env.Ctx, stuck.ID, stuck.Attempts, registry.IntakeScan{}, now)
	c.Assert(err, qt.ErrorIs, registry.ErrIntakeSourceChanged)
	err = env.Service.RequeueSource(env.Ctx, stuck.ID, stuck.Attempts, now, models.IntakeWaitRetry)
	c.Assert(err, qt.ErrorIs, registry.ErrIntakeSourceChanged)
	err = env.Service.FailSource(env.Ctx, stuck.ID, stuck.Attempts, "timeout")
	c.Assert(err, qt.ErrorIs, registry.ErrIntakeSourceChanged)
	c.Assert(env.Service.StoreScan(env.Ctx, again.ID, again.Attempts, registry.IntakeScan{}, now), qt.IsNil)
}

func intakeContractStuckSweepLimit(t *testing.T, env IntakeEnv) {
	c := qt.New(t)
	session, _, err := env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "batch"}, []registry.IntakeSourceInput{{}}, 0)
	c.Assert(err, qt.IsNil)
	now := time.Now()
	for attempt := 1; attempt <= intakeTestMaxAttempts; attempt++ {
		claimed, err := env.Service.ClaimNextSource(env.Ctx, now.Add(time.Second))
		c.Assert(err, qt.IsNil, qt.Commentf("attempt %d", attempt))
		c.Assert(claimed.Attempts, qt.Equals, attempt)
		swept, err := env.Service.SweepStuckSources(env.Ctx, now.Add(time.Minute), now, intakeTestMaxAttempts, intakeTestStuckCode)
		c.Assert(err, qt.IsNil)
		c.Assert(swept, qt.Equals, 1)
	}
	failed, err := env.Intake.ListSources(env.Ctx, session.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(failed[0].Status, qt.Equals, models.IntakeSourceFailed, qt.Commentf("the third stuck scan is the last"))
	c.Assert(failed[0].ErrorCode, qt.Equals, intakeTestStuckCode)
	c.Assert(failed[0].ClaimedAt, qt.IsNil)
	_, err = env.Service.ClaimNextSource(env.Ctx, now.Add(time.Second))
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// A resume gives the source as many attempts again.
	resumed, err := env.Intake.ResumeSession(env.Ctx, session.ID, now)
	c.Assert(err, qt.IsNil)
	c.Assert(resumed, qt.Equals, 1)
	again, err := env.Service.ClaimNextSource(env.Ctx, now.Add(time.Second))
	c.Assert(err, qt.IsNil)
	c.Assert(again.Attempts, qt.Equals, intakeTestMaxAttempts+1)
	c.Assert(again.CountedAttempts(), qt.Equals, 1)
}

func intakeContractCancelResume(t *testing.T, env IntakeEnv) {
	c := qt.New(t)
	session, _, err := env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "batch"}, []registry.IntakeSourceInput{{}, {}, {}}, 0)
	c.Assert(err, qt.IsNil)
	now := time.Now()

	done, err := env.Service.ClaimNextSource(env.Ctx, now.Add(time.Second))
	c.Assert(err, qt.IsNil)
	c.Assert(env.Service.StoreScan(env.Ctx, done.ID, done.Attempts, registry.IntakeScan{
		Candidates: []models.IntakeCandidate{{Fields: models.IntakeFields{"name": {Value: "Lamp", Confidence: 1}}, Selected: true, Quantity: 1}},
	}, now), qt.IsNil)
	inFlight, err := env.Service.ClaimNextSource(env.Ctx, now.Add(time.Second))
	c.Assert(err, qt.IsNil)

	canceled, err := env.Intake.CancelSession(env.Ctx, session.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(canceled, qt.Equals, 2, qt.Commentf("the pending and the scanning source"))
	queued, err := env.Intake.CountQueued(env.Ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(queued, qt.Equals, 0)
	_, err = env.Service.ClaimNextSource(env.Ctx, now.Add(time.Second))
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// The scan that was running when the user canceled still lands: it
	// was paid for.
	c.Assert(env.Service.StoreScan(env.Ctx, inFlight.ID, inFlight.Attempts, registry.IntakeScan{
		Candidates: []models.IntakeCandidate{{Fields: models.IntakeFields{"name": {Value: "Desk", Confidence: 1}}, Selected: true, Quantity: 1}},
	}, now), qt.IsNil)

	resumed, err := env.Intake.ResumeSession(env.Ctx, session.ID, now)
	c.Assert(err, qt.IsNil)
	c.Assert(resumed, qt.Equals, 1, qt.Commentf("only the source that was never scanned"))
	sources, err := env.Intake.ListSources(env.Ctx, session.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(sources[0].Status, qt.Equals, models.IntakeSourceScanned)
	c.Assert(sources[1].Status, qt.Equals, models.IntakeSourceScanned)
	c.Assert(sources[2].Status, qt.Equals, models.IntakeSourcePending)
	c.Assert(sources[2].Attempts, qt.Equals, 0)
	candidates, err := env.Intake.ListCandidates(env.Ctx, session.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(candidates, qt.HasLen, 2)
}

func intakeContractCancelResumeInScan(t *testing.T, env IntakeEnv) {
	c := qt.New(t)
	session, _, err := env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "batch"}, []registry.IntakeSourceInput{{}, {}}, 0)
	c.Assert(err, qt.IsNil)
	now := time.Now()
	first, err := env.Service.ClaimNextSource(env.Ctx, now.Add(time.Second))
	c.Assert(err, qt.IsNil)

	// Cancel and resume while the scan runs: the scan stays the only one.
	_, err = env.Intake.CancelSession(env.Ctx, session.ID)
	c.Assert(err, qt.IsNil)
	_, err = env.Intake.RescanSource(env.Ctx, first.ID, now)
	c.Assert(err, qt.ErrorIs, registry.ErrIntakeSourceBusy, qt.Commentf("the canceled scan still runs"))
	resumed, err := env.Intake.ResumeSession(env.Ctx, session.ID, now)
	c.Assert(err, qt.IsNil)
	c.Assert(resumed, qt.Equals, 2)
	back, err := env.Intake.GetSource(env.Ctx, first.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(back.Status, qt.Equals, models.IntakeSourceScanning)
	c.Assert(back.Attempts, qt.Equals, first.Attempts)
	other, err := env.Service.ClaimNextSource(env.Ctx, now.Add(time.Second))
	c.Assert(err, qt.IsNil)
	c.Assert(other.ID, qt.Not(qt.Equals), first.ID, qt.Commentf("the running scan is not claimed again"))
	c.Assert(env.Service.StoreScan(env.Ctx, first.ID, first.Attempts, registry.IntakeScan{}, now), qt.IsNil)

	// A canceled scan that fails loses its claim; a resume then queues the
	// source again under a new attempt, and the old attempt is refused.
	_, err = env.Intake.CancelSession(env.Ctx, session.ID)
	c.Assert(err, qt.IsNil)
	err = env.Service.FailSource(env.Ctx, other.ID, other.Attempts, "timeout")
	c.Assert(err, qt.ErrorIs, registry.ErrIntakeSourceChanged)
	released, err := env.Intake.GetSource(env.Ctx, other.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(released.Status, qt.Equals, models.IntakeSourceCanceled)
	c.Assert(released.ClaimedAt, qt.IsNil)
	resumed, err = env.Intake.ResumeSession(env.Ctx, session.ID, now)
	c.Assert(err, qt.IsNil)
	c.Assert(resumed, qt.Equals, 1)
	retried, err := env.Service.ClaimNextSource(env.Ctx, now.Add(time.Second))
	c.Assert(err, qt.IsNil)
	c.Assert(retried.ID, qt.Equals, other.ID)
	c.Assert(retried.Attempts, qt.Equals, other.Attempts+1)
	c.Assert(env.Service.StoreScan(env.Ctx, other.ID, other.Attempts, registry.IntakeScan{}, now), qt.ErrorIs, registry.ErrIntakeSourceChanged)
	c.Assert(env.Service.StoreScan(env.Ctx, retried.ID, retried.Attempts, registry.IntakeScan{}, now), qt.IsNil)
}

func intakeContractReviewOutcomes(t *testing.T, env IntakeEnv) {
	c := qt.New(t)
	session, candidates := scannedSession(c, env, "Kettle", "Toaster")

	skip := false
	reviewed, err := env.Intake.ReviewCandidate(env.Ctx, candidates[1].ID, registry.IntakeCandidateReview{
		Selected:       &skip,
		AcceptedFields: &[]string{"name"},
		Quantity:       new(3),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(reviewed.Selected, qt.IsFalse)
	c.Assert([]string(reviewed.AcceptedFields), qt.DeepEquals, []string{"name"})
	c.Assert(reviewed.Quantity, qt.Equals, 3)

	skipped, err := env.Intake.SetCandidateOutcome(env.Ctx, candidates[1].ID, models.IntakeCandidateSkipped, nil, "")
	c.Assert(err, qt.IsNil)
	c.Assert(skipped.Outcome, qt.Equals, models.IntakeCandidateSkipped)
	keep := true
	back, err := env.Intake.ReviewCandidate(env.Ctx, candidates[1].ID, registry.IntakeCandidateReview{Selected: &keep})
	c.Assert(err, qt.IsNil)
	c.Assert(back.Outcome, qt.Equals, models.IntakeCandidatePending, qt.Commentf("a kept product is no longer skipped"))

	failed, err := env.Intake.SetCandidateOutcome(env.Ctx, candidates[0].ID, models.IntakeCandidateFailed, new("draft-1"), "invalid")
	c.Assert(err, qt.IsNil)
	c.Assert(failed.ErrorCode, qt.Equals, "invalid")
	c.Assert(failed.DraftID, qt.IsNil, qt.Commentf("a failed outcome stores no draft"))

	created, err := env.Intake.SetCandidateOutcome(env.Ctx, candidates[0].ID, models.IntakeCandidateCreated, new("draft-1"), "ignored")
	c.Assert(err, qt.IsNil)
	c.Assert(created.Outcome, qt.Equals, models.IntakeCandidateCreated)
	c.Assert(*created.DraftID, qt.Equals, "draft-1")
	c.Assert(created.ErrorCode, qt.Equals, "")

	_, err = env.Intake.ReviewCandidate(env.Ctx, candidates[0].ID, registry.IntakeCandidateReview{Selected: &skip})
	c.Assert(err, qt.ErrorIs, registry.ErrIntakeCandidateFinished)

	got, err := env.Intake.GetCandidate(env.Ctx, candidates[0].ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Selected, qt.IsTrue)
	_, err = env.Intake.GetCandidate(env.Ctx, "missing")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	_, err = env.Intake.RescanSource(env.Ctx, candidates[0].SourceID, time.Now())
	c.Assert(err, qt.ErrorIs, registry.ErrIntakeSourceHasDrafts)
	c.Assert(session.ID, qt.Equals, got.SessionID)
}

func intakeContractRescan(t *testing.T, env IntakeEnv) {
	c := qt.New(t)
	session, candidates := scannedSession(c, env, "Kettle")
	sourceID := candidates[0].SourceID

	rescanned, err := env.Intake.RescanSource(env.Ctx, sourceID, time.Now())
	c.Assert(err, qt.IsNil)
	c.Assert(rescanned.Status, qt.Equals, models.IntakeSourcePending)
	c.Assert(rescanned.Attempts, qt.Equals, 1, qt.Commentf("the count of attempts never goes down"))
	c.Assert(rescanned.CountedAttempts(), qt.Equals, 0)
	left, err := env.Intake.ListCandidates(env.Ctx, session.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(left, qt.HasLen, 0)

	_, err = env.Intake.RescanSource(env.Ctx, sourceID, time.Now())
	c.Assert(err, qt.ErrorIs, registry.ErrIntakeSourceBusy)
}

func intakeContractRescanUnrecordedDraft(t *testing.T, env IntakeEnv) {
	c := qt.New(t)
	_, candidates := scannedSession(c, env, "Kettle")
	sourceID := candidates[0].SourceID

	// A draft made for the candidate whose outcome was never recorded.
	draft, _, err := env.Drafts.CreateOrGet(env.Ctx, models.CommodityDraft{
		ClientKey: candidates[0].DraftClientKey(),
		Values:    models.CommodityDraftValues{"name": "Kettle"},
	})
	c.Assert(err, qt.IsNil)
	_, err = env.Intake.RescanSource(env.Ctx, sourceID, time.Now())
	c.Assert(err, qt.ErrorIs, registry.ErrIntakeSourceHasDrafts)

	// Once the draft is being discarded, it no longer holds the rescan.
	_, err = env.Drafts.MarkDiscarding(env.Ctx, draft.ID)
	c.Assert(err, qt.IsNil)
	_, err = env.Intake.RescanSource(env.Ctx, sourceID, time.Now())
	c.Assert(err, qt.IsNil)
}

func intakeContractDelete(t *testing.T, env IntakeEnv) {
	c := qt.New(t)
	receipt := intakeFile(c, env, "receipt.pdf", models.FileCategoryDocuments)
	session, _, err := env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "batch"}, []registry.IntakeSourceInput{
		{FileIDs: []string{receipt.ID}},
	}, 0)
	c.Assert(err, qt.IsNil)
	sources, err := env.Intake.ListSources(env.Ctx, session.ID)
	c.Assert(err, qt.IsNil)

	c.Assert(env.Intake.Delete(env.Ctx, session.ID), qt.IsNil)
	_, err = env.Intake.Get(env.Ctx, session.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = env.Intake.GetSource(env.Ctx, sources[0].ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	file, err := env.Files.Get(env.Ctx, receipt.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(file.Links, qt.HasLen, 0)
	c.Assert(env.Intake.Delete(env.Ctx, session.ID), qt.ErrorIs, registry.ErrNotFound)
}

func intakeContractListOrder(t *testing.T, env IntakeEnv) {
	c := qt.New(t)
	for _, key := range []string{"a", "b", "c"} {
		_, _, err := env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: key}, nil, 0)
		c.Assert(err, qt.IsNil)
		time.Sleep(5 * time.Millisecond)
	}
	page, total, err := env.Intake.ListSessions(env.Ctx, 0, 2)
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 3)
	c.Assert(page, qt.HasLen, 2)
	c.Assert(page[0].ClientKey, qt.Equals, "c")
	c.Assert(page[1].ClientKey, qt.Equals, "b")
	count, err := env.Intake.Count(env.Ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 3)
}

func intakeContractOpenSessions(t *testing.T, env IntakeEnv) {
	c := qt.New(t)
	pause := func() { time.Sleep(5 * time.Millisecond) }

	// Done: one draft made, the other product left out.
	_, done := scannedSessionWithKey(c, env, "done", "Kettle", "Toaster")
	_, err := env.Intake.SetCandidateOutcome(env.Ctx, done[0].ID, models.IntakeCandidateCreated, new("draft-1"), "")
	c.Assert(err, qt.IsNil)
	_, err = env.Intake.SetCandidateOutcome(env.Ctx, done[1].ID, models.IntakeCandidateSkipped, nil, "")
	c.Assert(err, qt.IsNil)
	pause()
	// A product without an outcome.
	scannedSessionWithKey(c, env, "review", "Lamp")
	pause()
	// A kept product whose draft could not be made.
	_, retry := scannedSessionWithKey(c, env, "retry", "Chair")
	_, err = env.Intake.SetCandidateOutcome(env.Ctx, retry[0].ID, models.IntakeCandidateFailed, nil, "invalid_values")
	c.Assert(err, qt.IsNil)
	pause()
	// The only file could not be scanned.
	_, _, err = env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "failed"}, []registry.IntakeSourceInput{{}}, 0)
	c.Assert(err, qt.IsNil)
	src, err := env.Service.ClaimNextSource(env.Ctx, time.Now().Add(time.Second))
	c.Assert(err, qt.IsNil)
	c.Assert(env.Service.FailSource(env.Ctx, src.ID, src.Attempts, "file_unreadable"), qt.IsNil)
	pause()
	// Nothing to scan and nothing to decide.
	_, _, err = env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "empty"}, nil, 0)
	c.Assert(err, qt.IsNil)
	pause()
	// A file waiting for its scan.
	_, _, err = env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: "queued"}, []registry.IntakeSourceInput{{}}, 0)
	c.Assert(err, qt.IsNil)

	open, err := env.Intake.ListOpenSessions(env.Ctx)
	c.Assert(err, qt.IsNil)
	keys := make([]string, 0, len(open))
	for _, sess := range open {
		keys = append(keys, sess.ClientKey)
	}
	c.Assert(keys, qt.DeepEquals, []string{"queued", "failed", "retry", "review"})

	// The inbox of a group reads them; the service registry, which sees
	// every group, is refused.
	_, err = env.Service.ListOpenSessions(env.Ctx)
	c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
}

// scannedSession creates a session with one source and stores a scan with
// one candidate per name.
func scannedSession(c *qt.C, env IntakeEnv, names ...string) (*models.IntakeSession, []*models.IntakeCandidate) {
	c.Helper()
	return scannedSessionWithKey(c, env, "scanned", names...)
}

// scannedSessionWithKey is scannedSession for a session with the client key.
// No other source of the group may wait for a scan.
func scannedSessionWithKey(c *qt.C, env IntakeEnv, key string, names ...string) (*models.IntakeSession, []*models.IntakeCandidate) {
	c.Helper()
	session, _, err := env.Intake.CreateSession(env.Ctx, models.IntakeSession{ClientKey: key}, []registry.IntakeSourceInput{{}}, 0)
	c.Assert(err, qt.IsNil)
	now := time.Now()
	src, err := env.Service.ClaimNextSource(env.Ctx, now.Add(time.Second))
	c.Assert(err, qt.IsNil)
	scan := registry.IntakeScan{}
	for _, name := range names {
		scan.Candidates = append(scan.Candidates, models.IntakeCandidate{
			Fields:   models.IntakeFields{"name": {Value: name, Confidence: 0.9}},
			Selected: true, AcceptedFields: []string{"name"}, Quantity: 1,
		})
	}
	c.Assert(env.Service.StoreScan(env.Ctx, src.ID, src.Attempts, scan, now), qt.IsNil)
	candidates, err := env.Intake.ListCandidates(env.Ctx, session.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(candidates, qt.HasLen, len(names))
	return session, candidates
}

func intakeFile(c *qt.C, env IntakeEnv, name string, category models.FileCategory) *models.FileEntity {
	c.Helper()
	mime, ext, typ := "application/pdf", ".pdf", models.FileTypeDocument
	if category == models.FileCategoryImages {
		mime, ext, typ = "image/jpeg", ".jpg", models.FileTypeImage
	}
	file, err := env.Files.Create(env.Ctx, models.FileEntity{
		Title:    name,
		Type:     typ,
		Category: category,
		File: &models.File{
			Path:         name,
			OriginalPath: "files/" + name,
			Ext:          ext,
			MIMEType:     mime,
		},
	})
	c.Assert(err, qt.IsNil)
	return file
}
