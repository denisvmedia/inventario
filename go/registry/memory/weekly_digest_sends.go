package memory

import (
	"context"
	"time"

	"github.com/google/uuid"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

var _ registry.WeeklyDigestSendRegistry = (*WeeklyDigestSendRegistry)(nil)

type baseWeeklyDigestSendRegistry = Registry[models.WeeklyDigestSend, *models.WeeklyDigestSend]

// WeeklyDigestSendRegistry is the in-memory idempotency store for the weekly
// digest worker (#1391), mirroring the postgres one: the (user_id, week_start)
// pair can be claimed once, and old claims can be swept.
type WeeklyDigestSendRegistry struct {
	*baseWeeklyDigestSendRegistry
}

func NewWeeklyDigestSendRegistry() *WeeklyDigestSendRegistry {
	return &WeeklyDigestSendRegistry{
		baseWeeklyDigestSendRegistry: NewRegistry[models.WeeklyDigestSend, *models.WeeklyDigestSend](),
	}
}

// ClaimWeek records this user's week as sent and reports whether this call is
// the one that recorded it.
//
// The scan and the insert share one lock acquisition, which is the whole point:
// two goroutines that each scanned and then inserted would both find nothing
// and both write, and the unique index this stands in for would not exist to
// stop them. That is why this does not delegate to the base Create, which would
// take the lock again.
func (r *WeeklyDigestSendRegistry) ClaimWeek(_ context.Context, send models.WeeklyDigestSend) (bool, error) {
	if send.SentAt.IsZero() {
		send.SentAt = time.Now()
	}
	week := send.WeekStart.UTC().Truncate(24 * time.Hour)

	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		v := pair.Value
		if v.UserID == send.UserID && v.WeekStart.UTC().Truncate(24*time.Hour).Equal(week) {
			return false, nil
		}
	}
	row := send
	row.WeekStart = week
	row.ID = uuid.New().String()
	if row.UUID == "" {
		row.UUID = uuid.New().String()
	}
	r.items.Set(row.ID, &row)
	return true, nil
}

// ReleaseWeek drops one user's claim on a week.
func (r *WeeklyDigestSendRegistry) ReleaseWeek(_ context.Context, userID string, weekStart time.Time) error {
	week := weekStart.UTC().Truncate(24 * time.Hour)

	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		v := pair.Value
		if v.UserID == userID && v.WeekStart.UTC().Truncate(24*time.Hour).Equal(week) {
			r.items.Delete(pair.Key)
			return nil
		}
	}
	return nil
}

// DeleteSentBefore removes claims for weeks starting before the given date and
// returns how many went.
func (r *WeeklyDigestSendRegistry) DeleteSentBefore(_ context.Context, weekStart time.Time) (int, error) {
	cutoff := weekStart.UTC().Truncate(24 * time.Hour)

	r.lock.Lock()
	defer r.lock.Unlock()
	var stale []string
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		if pair.Value.WeekStart.UTC().Truncate(24 * time.Hour).Before(cutoff) {
			stale = append(stale, pair.Key)
		}
	}
	for _, key := range stale {
		r.items.Delete(key)
	}
	return len(stale), nil
}
