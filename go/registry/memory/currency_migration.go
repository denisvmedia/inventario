package memory

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// CurrencyMigrationRegistryFactory is the in-memory factory mirror of
// the postgres one. The audit-row store and HMAC key are shared between
// every registry derived from a factory so that user / service /
// per-test instances see the same data.
type CurrencyMigrationRegistryFactory struct {
	baseRegistry *Registry[models.CurrencyMigration, *models.CurrencyMigration]
	auditLock    *sync.RWMutex
	auditRows    *[]*models.CurrencyMigrationAuditRow
	hmacKey      []byte
}

type CurrencyMigrationRegistry struct {
	*Registry[models.CurrencyMigration, *models.CurrencyMigration]
	tenantID  string
	groupID   string
	userID    string
	service   bool
	auditLock *sync.RWMutex
	auditRows *[]*models.CurrencyMigrationAuditRow
	hmacKey   []byte
}

var _ registry.CurrencyMigrationRegistry = (*CurrencyMigrationRegistry)(nil)
var _ registry.CurrencyMigrationRegistryFactory = (*CurrencyMigrationRegistryFactory)(nil)

func NewCurrencyMigrationRegistryFactory() *CurrencyMigrationRegistryFactory {
	return NewCurrencyMigrationRegistryFactoryWithKey(generateRandomKey())
}

func NewCurrencyMigrationRegistryFactoryWithKey(key []byte) *CurrencyMigrationRegistryFactory {
	rows := make([]*models.CurrencyMigrationAuditRow, 0)
	return &CurrencyMigrationRegistryFactory{
		baseRegistry: NewRegistry[models.CurrencyMigration, *models.CurrencyMigration](),
		auditLock:    &sync.RWMutex{},
		auditRows:    &rows,
		hmacKey:      append([]byte(nil), key...),
	}
}

func generateRandomKey() []byte {
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		panic("failed to generate random HMAC key for currency migration registry: " + err.Error())
	}
	return k
}

// SetHMACKey overrides the signing key for preview tokens. Mirrors the
// postgres factory's setter — bootstrap may call this once at startup
// with the operator-supplied key so memory-backed test deployments can
// also produce stable tokens. Empty key is a no-op.
func (f *CurrencyMigrationRegistryFactory) SetHMACKey(key []byte) {
	if len(key) == 0 {
		return
	}
	f.hmacKey = append([]byte(nil), key...)
}

func (f *CurrencyMigrationRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.CurrencyMigrationRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *CurrencyMigrationRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.CurrencyMigrationRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	groupID := appctx.GroupIDFromContext(ctx)

	scoped := &Registry[models.CurrencyMigration, *models.CurrencyMigration]{
		items:   f.baseRegistry.items,
		lock:    f.baseRegistry.lock,
		userID:  user.ID,
		groupID: groupID,
	}
	return &CurrencyMigrationRegistry{
		Registry:  scoped,
		tenantID:  user.TenantID,
		groupID:   groupID,
		userID:    user.ID,
		auditLock: f.auditLock,
		auditRows: f.auditRows,
		hmacKey:   f.hmacKey,
	}, nil
}

func (f *CurrencyMigrationRegistryFactory) CreateServiceRegistry() registry.CurrencyMigrationRegistry {
	scoped := &Registry[models.CurrencyMigration, *models.CurrencyMigration]{
		items:  f.baseRegistry.items,
		lock:   f.baseRegistry.lock,
		userID: "",
	}
	return &CurrencyMigrationRegistry{
		Registry:  scoped,
		service:   true,
		auditLock: f.auditLock,
		auditRows: f.auditRows,
		hmacKey:   f.hmacKey,
	}
}

// Create overrides the base Registry.Create to enforce the
// "at most one pending|running per group" invariant. The postgres
// registry relies on a partial unique index for this; memory mirrors
// the contract by checking under the registry-wide lock.
func (r *CurrencyMigrationRegistry) Create(ctx context.Context, m models.CurrencyMigration) (*models.CurrencyMigration, error) {
	if m.Status == "" {
		m.Status = models.CurrencyMigrationStatusPending
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	m.SetTenantID(r.tenantID)
	m.SetGroupID(r.groupID)
	m.SetCreatedByUserID(r.userID)

	if err := m.ValidateWithContext(ctx); err != nil {
		return nil, errxtrace.Wrap("validation failed", err)
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		existing := pair.Value
		if existing.GroupID == m.GroupID &&
			(existing.Status == models.CurrencyMigrationStatusPending ||
				existing.Status == models.CurrencyMigrationStatusRunning) {
			return nil, registry.ErrMigrationInFlight
		}
	}

	m.SetID(uuid.New().String())
	m.SetUUID(uuid.New().String())
	tmp := m
	r.items.Set(tmp.ID, &tmp)
	out := tmp
	return &out, nil
}

func (r *CurrencyMigrationRegistry) LatestForGroup(_ context.Context, groupID string) (*models.CurrencyMigration, error) {
	if groupID == "" {
		return nil, errxtrace.Wrap("group id is required", registry.ErrFieldRequired)
	}
	r.lock.RLock()
	defer r.lock.RUnlock()

	var latest *models.CurrencyMigration
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		op := pair.Value
		if op.GroupID != groupID {
			continue
		}
		if latest == nil || op.CreatedAt.After(latest.CreatedAt) ||
			(op.CreatedAt.Equal(latest.CreatedAt) && op.ID > latest.ID) {
			cp := *op
			latest = &cp
		}
	}
	if latest == nil {
		return nil, registry.ErrNotFound
	}
	return latest, nil
}

func (r *CurrencyMigrationRegistry) InFlightForGroup(_ context.Context, groupID string) (*models.CurrencyMigration, error) {
	if groupID == "" {
		return nil, errxtrace.Wrap("group id is required", registry.ErrFieldRequired)
	}
	r.lock.RLock()
	defer r.lock.RUnlock()

	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		op := pair.Value
		if op.GroupID == groupID &&
			(op.Status == models.CurrencyMigrationStatusPending || op.Status == models.CurrencyMigrationStatusRunning) {
			cp := *op
			return &cp, nil
		}
	}
	return nil, nil
}

func (r *CurrencyMigrationRegistry) CompletedTodayForGroup(_ context.Context, groupID string, now time.Time) (int, error) {
	if groupID == "" {
		return 0, errxtrace.Wrap("group id is required", registry.ErrFieldRequired)
	}
	startOfDay := utcMidnight(now)
	endOfDay := startOfDay.AddDate(0, 0, 1)

	r.lock.RLock()
	defer r.lock.RUnlock()

	cnt := 0
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		op := pair.Value
		if op.GroupID != groupID || op.Status != models.CurrencyMigrationStatusCompleted {
			continue
		}
		if op.CompletedAt == nil {
			continue
		}
		ca := op.CompletedAt.UTC()
		if !ca.Before(startOfDay) && ca.Before(endOfDay) {
			cnt++
		}
	}
	return cnt, nil
}

func (r *CurrencyMigrationRegistry) UpdateStatus(_ context.Context, id string, patch registry.CurrencyMigrationStatusPatch) error {
	if id == "" {
		return errxtrace.Wrap("id is required", registry.ErrFieldRequired)
	}
	if patch.Status == "" || !patch.Status.IsValid() {
		return errxtrace.Wrap("status must be set to a valid value", registry.ErrInvalidInput)
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	op, ok := r.items.Get(id)
	if !ok {
		return registry.ErrNotFound
	}
	tmp := *op
	tmp.Status = patch.Status
	if patch.StartedAt != nil {
		t := patch.StartedAt.UTC()
		tmp.StartedAt = &t
	}
	if patch.CompletedAt != nil {
		t := patch.CompletedAt.UTC()
		tmp.CompletedAt = &t
	}
	if patch.ErrorMessage != nil {
		tmp.ErrorMessage = *patch.ErrorMessage
	}
	if patch.CommodityCount != nil {
		tmp.CommodityCount = *patch.CommodityCount
	}
	// total_before / total_after are intentionally treated as opaque
	// strings here — the registry interface uses *string so that a
	// "set to nil" caller can be distinguished from "leave alone". The
	// memory store deliberately mirrors the postgres semantics rather
	// than reaching into decimal types.
	if patch.TotalBefore != nil {
		tmp.TotalBefore = decimalFromString(*patch.TotalBefore)
	}
	if patch.TotalAfter != nil {
		tmp.TotalAfter = decimalFromString(*patch.TotalAfter)
	}
	r.items.Set(id, &tmp)
	return nil
}

func (r *CurrencyMigrationRegistry) WriteAuditRow(_ context.Context, row models.CurrencyMigrationAuditRow) (*models.CurrencyMigrationAuditRow, error) {
	row.SetTenantID(r.tenantID)
	row.SetGroupID(r.groupID)
	row.SetCreatedByUserID(r.userID)
	if row.CreatedAt.IsZero() {
		row.CreatedAt = time.Now().UTC()
	}
	row.SetID(uuid.New().String())
	row.SetUUID(uuid.New().String())

	r.auditLock.Lock()
	defer r.auditLock.Unlock()
	cp := row
	*r.auditRows = append(*r.auditRows, &cp)
	out := cp
	return &out, nil
}

func (r *CurrencyMigrationRegistry) ListAuditRows(_ context.Context, migrationID string) ([]*models.CurrencyMigrationAuditRow, error) {
	if migrationID == "" {
		return nil, errxtrace.Wrap("migration id is required", registry.ErrFieldRequired)
	}
	r.auditLock.RLock()
	defer r.auditLock.RUnlock()

	var out []*models.CurrencyMigrationAuditRow
	for _, row := range *r.auditRows {
		if row.MigrationID == migrationID {
			cp := *row
			out = append(out, &cp)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// DeleteAuditRowsByGroup removes every audit row for the given (tenant,
// group) from the bespoke audit slice. Mirrors the postgres registry's
// explicit cleanup: the audit rows do not cascade from the group, so the
// group-deletion path clears them directly. Idempotent: zero matches
// returns (0, nil).
func (r *CurrencyMigrationRegistry) DeleteAuditRowsByGroup(_ context.Context, tenantID, groupID string) (int, error) {
	if tenantID == "" || groupID == "" {
		return 0, errxtrace.Wrap("tenant id and group id are required", registry.ErrFieldRequired)
	}

	r.auditLock.Lock()
	defer r.auditLock.Unlock()

	kept := make([]*models.CurrencyMigrationAuditRow, 0, len(*r.auditRows))
	deleted := 0
	for _, row := range *r.auditRows {
		if row.TenantID == tenantID && row.GroupID == groupID {
			deleted++
			continue
		}
		kept = append(kept, row)
	}
	*r.auditRows = kept
	return deleted, nil
}

func (r *CurrencyMigrationRegistry) ClaimNextPending(_ context.Context) (*models.CurrencyMigration, error) {
	if !r.service {
		return nil, errxtrace.Wrap("ClaimNextPending requires a service registry", registry.ErrUserContextRequired)
	}
	r.lock.Lock()
	defer r.lock.Unlock()

	type candidate struct {
		op *models.CurrencyMigration
	}
	var picks []candidate
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		if pair.Value.Status == models.CurrencyMigrationStatusPending {
			picks = append(picks, candidate{op: pair.Value})
		}
	}
	if len(picks) == 0 {
		return nil, registry.ErrNotFound
	}
	sort.SliceStable(picks, func(i, j int) bool {
		if picks[i].op.CreatedAt.Equal(picks[j].op.CreatedAt) {
			return picks[i].op.ID < picks[j].op.ID
		}
		return picks[i].op.CreatedAt.Before(picks[j].op.CreatedAt)
	})

	now := time.Now().UTC()
	picked := picks[0].op
	tmp := *picked
	tmp.Status = models.CurrencyMigrationStatusRunning
	tmp.StartedAt = &now
	r.items.Set(tmp.ID, &tmp)
	out := tmp
	return &out, nil
}

func (r *CurrencyMigrationRegistry) SweepStuckRunning(_ context.Context, now time.Time, threshold time.Duration) ([]*models.CurrencyMigration, error) {
	if !r.service {
		return nil, errxtrace.Wrap("SweepStuckRunning requires a service registry", registry.ErrUserContextRequired)
	}
	cutoff := now.UTC().Add(-threshold)
	completed := now.UTC()

	r.lock.Lock()
	defer r.lock.Unlock()

	var swept []*models.CurrencyMigration
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		op := pair.Value
		if op.Status != models.CurrencyMigrationStatusRunning {
			continue
		}
		if op.StartedAt == nil || !op.StartedAt.Before(cutoff) {
			continue
		}
		tmp := *op
		tmp.Status = models.CurrencyMigrationStatusFailed
		tmp.CompletedAt = &completed
		if strings.TrimSpace(tmp.ErrorMessage) == "" {
			tmp.ErrorMessage = "worker crashed or stalled"
		}
		r.items.Set(tmp.ID, &tmp)
		out := tmp
		swept = append(swept, &out)
	}
	return swept, nil
}

func (r *CurrencyMigrationRegistry) IssuePreviewToken(inputs registry.PreviewTokenInputs) (string, error) {
	if len(r.hmacKey) == 0 {
		return "", errxtrace.Wrap("no HMAC key configured for preview token", registry.ErrInvalidConfig)
	}
	payload, err := json.Marshal(inputs)
	if err != nil {
		return "", errxtrace.Wrap("failed to marshal preview token payload", err)
	}
	mac := hmac.New(sha256.New, r.hmacKey)
	mac.Write(payload)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func (r *CurrencyMigrationRegistry) VerifyPreviewToken(token string, now time.Time) (registry.PreviewTokenInputs, error) {
	var zero registry.PreviewTokenInputs
	if len(r.hmacKey) == 0 {
		return zero, errxtrace.Wrap("no HMAC key configured for preview token", registry.ErrInvalidConfig)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return zero, registry.ErrPreviewTokenInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return zero, registry.ErrPreviewTokenInvalid
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return zero, registry.ErrPreviewTokenInvalid
	}
	mac := hmac.New(sha256.New, r.hmacKey)
	mac.Write(payload)
	expected := mac.Sum(nil)
	if !hmac.Equal(sig, expected) {
		return zero, registry.ErrPreviewTokenInvalid
	}
	var inputs registry.PreviewTokenInputs
	if err := json.Unmarshal(payload, &inputs); err != nil {
		return zero, registry.ErrPreviewTokenInvalid
	}
	if !inputs.ExpiresAt.IsZero() && now.UTC().After(inputs.ExpiresAt.UTC()) {
		return zero, registry.ErrPreviewTokenExpired
	}
	return inputs, nil
}

func utcMidnight(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// decimalFromString parses s as a decimal. Empty / invalid input is
// returned as nil (treated as "leave the field unset"). Used by the
// memory UpdateStatus path to mirror the postgres registry's
// pointer-to-string contract for total_before / total_after.
func decimalFromString(s string) *decimal.Decimal {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return nil
	}
	return &d
}
