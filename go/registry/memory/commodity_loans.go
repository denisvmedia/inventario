package memory

import (
	"context"
	"sort"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// CommodityLoanRegistryFactory creates CommodityLoanRegistry instances
// with proper context. Stores the base registry so all per-request
// registries share the same backing map (mirrors the tag/export pattern).
type CommodityLoanRegistryFactory struct {
	base *Registry[models.CommodityLoan, *models.CommodityLoan]
}

// CommodityLoanRegistry is the context-aware in-memory registry of loans.
type CommodityLoanRegistry struct {
	*Registry[models.CommodityLoan, *models.CommodityLoan]

	userID string
}

var (
	_ registry.CommodityLoanRegistry        = (*CommodityLoanRegistry)(nil)
	_ registry.CommodityLoanRegistryFactory = (*CommodityLoanRegistryFactory)(nil)
)

func NewCommodityLoanRegistryFactory() *CommodityLoanRegistryFactory {
	return &CommodityLoanRegistryFactory{
		base: NewRegistry[models.CommodityLoan, *models.CommodityLoan](),
	}
}

func (f *CommodityLoanRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.CommodityLoanRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *CommodityLoanRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.CommodityLoanRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}

	groupID := appctx.GroupIDFromContext(ctx)
	userRegistry := &Registry[models.CommodityLoan, *models.CommodityLoan]{
		items:   f.base.items,
		lock:    f.base.lock,
		userID:  user.ID,
		groupID: groupID,
	}

	return &CommodityLoanRegistry{
		Registry: userRegistry,
		userID:   user.ID,
	}, nil
}

func (f *CommodityLoanRegistryFactory) CreateServiceRegistry() registry.CommodityLoanRegistry {
	serviceRegistry := &Registry[models.CommodityLoan, *models.CommodityLoan]{
		items:  f.base.items,
		lock:   f.base.lock,
		userID: "",
	}

	return &CommodityLoanRegistry{
		Registry: serviceRegistry,
		userID:   "",
	}
}

func (r *CommodityLoanRegistry) Create(ctx context.Context, loan models.CommodityLoan) (*models.CommodityLoan, error) {
	now := time.Now()
	loan.CreatedAt = now
	loan.UpdatedAt = now
	created, err := r.Registry.CreateWithUser(ctx, loan)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create loan", err)
	}
	return created, nil
}

func (r *CommodityLoanRegistry) Update(ctx context.Context, loan models.CommodityLoan) (*models.CommodityLoan, error) {
	loan.UpdatedAt = time.Now()
	updated, err := r.Registry.UpdateWithUser(ctx, loan)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update loan", err)
	}
	return updated, nil
}

// ListByCommodity returns loans for a single commodity, most-recent-first
// (lent_at desc, created_at desc as tiebreaker).
func (r *CommodityLoanRegistry) ListByCommodity(ctx context.Context, commodityID string) ([]*models.CommodityLoan, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*models.CommodityLoan, 0, len(all))
	for _, l := range all {
		if l.CommodityID == commodityID {
			out = append(out, l)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].LentAt != out[j].LentAt {
			return out[i].LentAt > out[j].LentAt
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// GetOpenForCommodity returns the (at most one) open loan for the given
// commodity. Returns ErrNotFound if no open loan exists. If multiple
// open loans somehow exist (e.g. memory backend mid-test), returns the
// most recent — matching what a SELECT ... ORDER BY lent_at DESC LIMIT 1
// would do in postgres.
func (r *CommodityLoanRegistry) GetOpenForCommodity(ctx context.Context, commodityID string) (*models.CommodityLoan, error) {
	loans, err := r.ListByCommodity(ctx, commodityID)
	if err != nil {
		return nil, err
	}
	for _, l := range loans {
		if l.IsOpen() {
			return l, nil
		}
	}
	return nil, registry.ErrNotFound
}

func (r *CommodityLoanRegistry) ListPaginated(ctx context.Context, offset, limit int, opts registry.LoanListOptions) ([]*models.CommodityLoan, int, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, 0, err
	}

	state := opts.State
	if state == "" {
		state = registry.LoanStateAll
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	filtered := all[:0:0]
	for _, l := range all {
		switch state {
		case registry.LoanStateAll:
			filtered = append(filtered, l)
		case registry.LoanStateOpen:
			if l.IsOpen() {
				filtered = append(filtered, l)
			}
		case registry.LoanStateOverdue:
			if l.IsOverdue(now) {
				filtered = append(filtered, l)
			}
		case registry.LoanStateReturned:
			if !l.IsOpen() {
				filtered = append(filtered, l)
			}
		}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].LentAt != filtered[j].LentAt {
			return filtered[i].LentAt > filtered[j].LentAt
		}
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := len(filtered)
	if offset < 0 {
		offset = 0
	}
	if limit < 0 {
		limit = 0
	}
	start := min(offset, total)
	end := min(start+limit, total)
	return filtered[start:end], total, nil
}

// ListPendingReminders mirrors the postgres path's filter shape using
// the in-memory loan slice. Walks every loan and emits the open rows
// whose due_back_at falls into the requested window and whose matching
// reminder_sent_* flag is still false. The result is ordered by loan ID
// for deterministic iteration in tests.
func (r *CommodityLoanRegistry) ListPendingReminders(ctx context.Context, kind registry.LoanReminderKind, now time.Time, dueSoonDays int) ([]*models.CommodityLoan, error) {
	if !kind.IsValid() {
		return nil, registry.ErrInvalidInput
	}
	if r.userID != "" {
		// Match the postgres precondition: this method is worker-only,
		// not a per-user surface. Rejecting on memory too means a
		// miswired registry set (where a user-mode handler accidentally
		// reaches the worker path) fails identically in tests and prod.
		return nil, errxtrace.Wrap("ListPendingReminders requires service-mode registry", registry.ErrInvalidInput)
	}
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	n := now.UTC()
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	out := make([]*models.CommodityLoan, 0, len(all))
	for _, l := range all {
		if l == nil || !l.IsOpen() || l.DueBackAt == nil || string(*l.DueBackAt) == "" {
			continue
		}
		due := l.DueBackAt.ToTime()
		if due.IsZero() {
			continue
		}
		switch kind {
		case registry.LoanReminderKindOverdue:
			if l.ReminderSentOverdue {
				continue
			}
			if !due.Before(today) {
				continue
			}
		case registry.LoanReminderKindDueSoon:
			if l.ReminderSentDueSoon {
				continue
			}
			if due.Before(today) {
				continue
			}
			limit := today.AddDate(0, 0, dueSoonDays)
			if due.After(limit) {
				continue
			}
		}
		out = append(out, l)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// MarkReminderSent flips the in-memory flag. The (false, nil) return
// covers both "already true" (another sweep would have flipped first)
// and "not found" so the service layer treats the outcomes identically.
func (r *CommodityLoanRegistry) MarkReminderSent(ctx context.Context, loanID string, kind registry.LoanReminderKind) (bool, error) {
	if !kind.IsValid() {
		return false, registry.ErrInvalidInput
	}
	if r.userID != "" {
		return false, errxtrace.Wrap("MarkReminderSent requires service-mode registry", registry.ErrInvalidInput)
	}
	loan, err := r.Get(ctx, loanID)
	if err != nil {
		return false, nil //nolint:nilerr // already-gone is a stable no-op for the worker.
	}
	switch kind {
	case registry.LoanReminderKindOverdue:
		if loan.ReminderSentOverdue {
			return false, nil
		}
		loan.ReminderSentOverdue = true
	case registry.LoanReminderKindDueSoon:
		if loan.ReminderSentDueSoon {
			return false, nil
		}
		loan.ReminderSentDueSoon = true
	}
	if _, err := r.Update(ctx, *loan); err != nil {
		return false, errxtrace.Wrap("failed to flip reminder flag", err)
	}
	return true, nil
}

func (r *CommodityLoanRegistry) CountOpenByCommodity(ctx context.Context, commodityIDs []string) (map[string]int, error) {
	out := make(map[string]int, len(commodityIDs))
	for _, id := range commodityIDs {
		out[id] = 0
	}
	if len(commodityIDs) == 0 {
		return out, nil
	}
	wanted := make(map[string]struct{}, len(commodityIDs))
	for _, id := range commodityIDs {
		wanted[id] = struct{}{}
	}

	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, l := range all {
		if !l.IsOpen() {
			continue
		}
		if _, ok := wanted[l.CommodityID]; !ok {
			continue
		}
		out[l.CommodityID]++
	}
	return out, nil
}
