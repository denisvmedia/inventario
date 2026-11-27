package services_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/services"
)

type purgeRound struct {
	items, files int
	err          error
	// groups, when set, is what the round reports instead of items and
	// files in one group.
	groups []services.GroupTrashOutcome
}

func purgeIDs(prefix string, n int) []string {
	ids := make([]string, 0, n)
	for i := range n {
		ids = append(ids, fmt.Sprintf("%s-%d", prefix, i))
	}
	return ids
}

// scriptedPurger returns one scripted round per PurgeExpired call and
// nothing once the script runs out.
type scriptedPurger struct {
	rounds []purgeRound
	calls  int
	batch  int
}

func (p *scriptedPurger) PurgeExpired(_ context.Context, _ time.Time, batch int) ([]services.GroupTrashOutcome, error) {
	p.batch = batch
	p.calls++
	if p.calls > len(p.rounds) {
		return nil, nil
	}
	r := p.rounds[p.calls-1]
	if r.groups != nil || r.items+r.files == 0 {
		return r.groups, r.err
	}
	round := fmt.Sprintf("round-%d", p.calls)
	return []services.GroupTrashOutcome{{
		TenantID: "tenant-1",
		GroupID:  "group-1",
		TrashOutcome: services.TrashOutcome{
			ItemIDs: purgeIDs(round+"-item", r.items),
			FileIDs: purgeIDs(round+"-file", r.files),
		},
	}}, r.err
}

// recordingAudit keeps the audit events it is given.
type recordingAudit struct {
	events []services.AuthEvent
}

func (a *recordingAudit) LogAuth(_ context.Context, ev services.AuthEvent) {
	a.events = append(a.events, ev)
}

func (*recordingAudit) LogAdmin(context.Context, services.AdminEvent) {}

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

// A sweep writes one audit row for each group it deleted something in, with
// what it deleted there across all its rounds, and none for a sweep that
// deleted nothing (#2893).
func TestTrashPurgeWorker_SweepWritesOneAuditRowPerGroup(t *testing.T) {
	c := qt.New(t)
	const full = 100
	groupA := func(items, files []string) services.GroupTrashOutcome {
		return services.GroupTrashOutcome{TenantID: "tenant-1", GroupID: "group-a",
			TrashOutcome: services.TrashOutcome{ItemIDs: items, FileIDs: files}}
	}
	groupB := services.GroupTrashOutcome{TenantID: "tenant-2", GroupID: "group-b",
		TrashOutcome: services.TrashOutcome{FileIDs: []string{"file-b"}}}
	purger := &scriptedPurger{rounds: []purgeRound{
		{groups: []services.GroupTrashOutcome{groupA(purgeIDs("item-a", full), []string{"file-a1"}), groupB}},
		{groups: []services.GroupTrashOutcome{groupA([]string{"item-last"}, []string{"file-a2"})}},
	}}
	audit := &recordingAudit{}
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)

	services.NewTrashPurgeWorker(purger,
		services.WithTrashPurgeAudit(audit),
		services.WithTrashPurgeClock(func() time.Time { return now }),
	).Sweep(context.Background())

	c.Assert(purger.calls, qt.Equals, 2)
	c.Assert(audit.events, qt.HasLen, 2)
	for _, ev := range audit.events {
		c.Check(ev.Action, qt.Equals, services.AuditActionTrashPurgeExpired)
		c.Check(ev.UserID, qt.IsNil, qt.Commentf("the retention deleted the entries, not a member"))
		c.Check(*ev.EntityType, qt.Equals, "location_group")
		c.Check(ev.Success, qt.IsTrue)
		c.Check(ev.Extra["cutoff"], qt.Equals, now.Add(-services.DefaultTrashRetention).Format(time.RFC3339))
	}

	a := audit.events[0]
	c.Assert(*a.TenantID, qt.Equals, "tenant-1")
	c.Assert(*a.EntityID, qt.Equals, "group-a")
	c.Assert(a.Extra["items"], qt.Equals, full+1)
	c.Assert(a.Extra["item_ids"], qt.DeepEquals, append(purgeIDs("item-a", full), "item-last"))
	c.Assert(a.Extra["files"], qt.Equals, 2)
	c.Assert(a.Extra["file_ids"], qt.DeepEquals, []string{"file-a1", "file-a2"})

	b := audit.events[1]
	c.Assert(*b.TenantID, qt.Equals, "tenant-2")
	c.Assert(*b.EntityID, qt.Equals, "group-b")
	c.Assert(b.Extra["items"], qt.Equals, 0)
	c.Assert(b.Extra["item_ids"], qt.DeepEquals, []string{})
	c.Assert(b.Extra["file_ids"], qt.DeepEquals, []string{"file-b"})

	c.Run("a sweep that deletes nothing writes no row", func(c *qt.C) {
		audit := &recordingAudit{}
		services.NewTrashPurgeWorker(&scriptedPurger{}, services.WithTrashPurgeAudit(audit)).Sweep(context.Background())
		c.Assert(audit.events, qt.HasLen, 0)
	})
}

// An audit row lists at most TrashAuditIDLimit IDs of each kind and says so
// when it cut a list; the counts stay exact.
func TestTrashOutcome_AuditExtraCapsTheIDLists(t *testing.T) {
	c := qt.New(t)
	ids := purgeIDs("item", services.TrashAuditIDLimit+5)
	extra := services.TrashOutcome{ItemIDs: ids}.AuditExtra()
	c.Assert(extra["items"], qt.Equals, services.TrashAuditIDLimit+5)
	c.Assert(extra["item_ids"], qt.HasLen, services.TrashAuditIDLimit)
	c.Assert(extra["ids_truncated"], qt.IsTrue)

	extra = services.TrashOutcome{ItemIDs: ids[:3]}.AuditExtra()
	_, cut := extra["ids_truncated"]
	c.Assert(cut, qt.IsFalse)
}
