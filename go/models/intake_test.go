package models_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

func TestIntakeSessionSummary_State(t *testing.T) {
	src := func(status models.IntakeSourceStatus) *models.IntakeSource {
		return &models.IntakeSource{Status: status}
	}
	cand := func(selected bool, outcome models.IntakeCandidateOutcome) *models.IntakeCandidate {
		return &models.IntakeCandidate{Selected: selected, Outcome: outcome}
	}
	tests := []struct {
		name       string
		sources    []*models.IntakeSource
		candidates []*models.IntakeCandidate
		state      models.IntakeSessionState
		attention  bool
	}{
		{
			name:    "queued",
			sources: []*models.IntakeSource{src(models.IntakeSourcePending), src(models.IntakeSourceScanned)},
			state:   models.IntakeSessionProcessing,
		},
		{
			name:       "scanned and undecided",
			sources:    []*models.IntakeSource{src(models.IntakeSourceScanned)},
			candidates: []*models.IntakeCandidate{cand(true, models.IntakeCandidatePending)},
			state:      models.IntakeSessionReview,
			attention:  true,
		},
		{
			name:       "every product left out but not yet skipped",
			sources:    []*models.IntakeSource{src(models.IntakeSourceScanned)},
			candidates: []*models.IntakeCandidate{cand(false, models.IntakeCandidatePending)},
			state:      models.IntakeSessionReview,
			attention:  true,
		},
		{
			name:       "a draft failed",
			sources:    []*models.IntakeSource{src(models.IntakeSourceScanned)},
			candidates: []*models.IntakeCandidate{cand(true, models.IntakeCandidateCreated), cand(true, models.IntakeCandidateFailed)},
			state:      models.IntakeSessionReview,
			attention:  true,
		},
		{
			name:       "done",
			sources:    []*models.IntakeSource{src(models.IntakeSourceScanned)},
			candidates: []*models.IntakeCandidate{cand(true, models.IntakeCandidateCreated), cand(false, models.IntakeCandidateSkipped)},
			state:      models.IntakeSessionDone,
		},
		{
			name:    "done with a file that failed",
			sources: []*models.IntakeSource{src(models.IntakeSourceFailed), src(models.IntakeSourceCanceled)},
			state:   models.IntakeSessionDone,
			// The failed file still asks for a look.
			attention: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			var s models.IntakeSessionSummary
			for _, source := range tc.sources {
				s.AddSource(source, 1)
			}
			for _, candidate := range tc.candidates {
				s.AddCandidate(candidate)
			}
			c.Check(s.State(), qt.Equals, tc.state)
			c.Check(s.NeedsAttention(), qt.Equals, tc.attention)
			c.Check(s.Files, qt.Equals, len(tc.sources))
		})
	}
}

func TestIntakeFields_RoundTrip(t *testing.T) {
	c := qt.New(t)
	in := models.IntakeFields{"name": {Value: "Kettle", Confidence: 0.9}, "tags": {Value: []any{"kitchen"}, Confidence: 0.5}}
	raw, err := in.Value()
	c.Assert(err, qt.IsNil)
	var out models.IntakeFields
	c.Assert(out.Scan(raw), qt.IsNil)
	c.Check(out, qt.DeepEquals, in)

	var empty models.IntakeFields
	c.Assert(empty.Scan(nil), qt.IsNil)
	c.Check(empty, qt.DeepEquals, models.IntakeFields{})
	nilRaw, err := models.IntakeFields(nil).Value()
	c.Assert(err, qt.IsNil)
	c.Check(nilRaw, qt.DeepEquals, []byte("{}"))
	c.Check(empty.Scan(42), qt.IsNotNil)
}

func TestIntakeCandidate_Helpers(t *testing.T) {
	c := qt.New(t)
	cand := &models.IntakeCandidate{Selected: true, Outcome: models.IntakeCandidateFailed, AcceptedFields: []string{"name"}}
	cand.ID = "cand-1"
	c.Check(cand.DraftClientKey(), qt.Equals, "intake:cand-1")
	c.Check(cand.IsUnfinished(), qt.IsTrue)
	c.Check(cand.Accepts("name"), qt.IsTrue)
	c.Check(cand.Accepts("brand"), qt.IsFalse)
	cand.Outcome = models.IntakeCandidateCreated
	c.Check(cand.IsUnfinished(), qt.IsFalse)
}
