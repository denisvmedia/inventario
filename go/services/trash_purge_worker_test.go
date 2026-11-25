package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/services"
)

type purgeRound struct {
	items, files int
	err          error
}

// scriptedPurger returns one scripted round per PurgeExpired call and
// nothing once the script runs out.
type scriptedPurger struct {
	rounds []purgeRound
	calls  int
	batch  int
}

func (p *scriptedPurger) PurgeExpired(_ context.Context, _ time.Time, batch int) (items, files int, err error) {
	p.batch = batch
	p.calls++
	if p.calls > len(p.rounds) {
		return 0, 0, nil
	}
	r := p.rounds[p.calls-1]
	return r.items, r.files, r.err
}

func TestTrashPurgeWorker_Sweep(t *testing.T) {
	failure := errors.New("one bad row")
	const full = 100
	tests := []struct {
		name      string
		rounds    []purgeRound
		wantCalls int
	}{
		{
			name:      "a short round ends the sweep",
			rounds:    []purgeRound{{items: full}, {items: 3, files: 2}},
			wantCalls: 2,
		},
		{
			name:      "a full round of files asks for more",
			rounds:    []purgeRound{{files: full}, {}},
			wantCalls: 2,
		},
		{
			name: "a failing entry does not stop the rounds that still delete",
			rounds: []purgeRound{
				{items: full - 1, err: failure},
				{items: full - 1, err: failure},
				{items: 40, files: 7, err: failure},
				{err: failure},
			},
			wantCalls: 4,
		},
		{
			name:      "a round that only fails ends the sweep",
			rounds:    []purgeRound{{err: failure}, {items: full}},
			wantCalls: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			purger := &scriptedPurger{rounds: tc.rounds}
			services.NewTrashPurgeWorker(purger).Sweep(context.Background())
			c.Assert(purger.batch, qt.Equals, full)
			c.Assert(purger.calls, qt.Equals, tc.wantCalls)
		})
	}
}
