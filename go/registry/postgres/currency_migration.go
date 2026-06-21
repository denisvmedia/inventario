package postgres

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// inFlightUniqueIndexName is the partial unique index that enforces
// "at most one pending|running migration per group" at the schema level.
// Postgres-side unique violations on this index name map to
// registry.ErrMigrationInFlight; any other unique violation is
// re-raised as-is so it bubbles up as a 5xx (it would indicate a
// genuine bug, e.g. UUID collision).
const inFlightUniqueIndexName = "idx_currency_migrations_group_in_flight"

// CurrencyMigrationRegistryFactory creates context-aware
// CurrencyMigrationRegistry instances. The HMAC key for preview-token
// signing is held on the factory so user and service instances share
// the same signing context.
type CurrencyMigrationRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	hmacKey    []byte
}

type CurrencyMigrationRegistry struct {
	dbx             *sqlx.DB
	tableNames      store.TableNames
	tenantID        string
	groupID         string
	createdByUserID string
	service         bool
	hmacKey         []byte
}

var _ registry.CurrencyMigrationRegistry = (*CurrencyMigrationRegistry)(nil)
var _ registry.CurrencyMigrationRegistryFactory = (*CurrencyMigrationRegistryFactory)(nil)

// NewCurrencyMigrationRegistry creates a factory with a freshly-generated
// random HMAC key. Tokens issued by one process are not verifiable by
// another with this default — fine for PR 1, where no API endpoint reads
// the token; PR 2 swaps this for a config-driven key (the same key on
// every replica) via NewCurrencyMigrationRegistryWithKey.
func NewCurrencyMigrationRegistry(dbx *sqlx.DB) *CurrencyMigrationRegistryFactory {
	return NewCurrencyMigrationRegistryWithKey(dbx, generateRandomKey())
}

// NewCurrencyMigrationRegistryWithKey lets the caller supply the HMAC
// key. Used by tests (deterministic key) and by PR 2's apiserver bootstrap
// (key from config).
func NewCurrencyMigrationRegistryWithKey(dbx *sqlx.DB, key []byte) *CurrencyMigrationRegistryFactory {
	return &CurrencyMigrationRegistryFactory{
		dbx:        dbx,
		tableNames: store.DefaultTableNames,
		hmacKey:    append([]byte(nil), key...),
	}
}

func generateRandomKey() []byte {
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		// rand.Read on Linux uses getrandom(2) and never fails in
		// practice. A failure here is unrecoverable: the registry
		// can't sign tokens at all without a key.
		panic("failed to generate random HMAC key for currency migration registry: " + err.Error())
	}
	return k
}

// SetHMACKey overrides the signing key for preview tokens. Called once
// at bootstrap time when the operator has configured a stable key (so
// tokens survive restarts / are verifiable across replicas). Empty key
// is a no-op — the factory keeps the random per-process key chosen at
// construction.
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

	return &CurrencyMigrationRegistry{
		dbx:             f.dbx,
		tableNames:      f.tableNames,
		tenantID:        user.TenantID,
		groupID:         appctx.GroupIDFromContext(ctx),
		createdByUserID: user.ID,
		service:         false,
		hmacKey:         f.hmacKey,
	}, nil
}

func (f *CurrencyMigrationRegistryFactory) CreateServiceRegistry() registry.CurrencyMigrationRegistry {
	return &CurrencyMigrationRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		service:    true,
		hmacKey:    f.hmacKey,
	}
}

// NewProcessor returns a CurrencyMigrationProcessor wired against the
// same dbx + table names this factory uses. The processor owns TX2 of
// the migration lifecycle (#202 §4.5); the worker (in services/) holds
// the run loop + metrics and calls the processor once per claim.
//
// Returning the concrete type (rather than a registry-level interface)
// is intentional: TX2 is postgres-only by design — the advisory lock,
// the SET LOCAL role, the schema-level audit_logs INSERT all assume
// the postgres backend. Memory-backend callers don't need a worker
// (the feature is gated by FEATURE_CURRENCY_MIGRATION on top of a real
// database; the memory factory is wired at boot but ProcessRunningMigration
// is never reached).
func (f *CurrencyMigrationRegistryFactory) NewProcessor() *CurrencyMigrationProcessor {
	return NewCurrencyMigrationProcessorWithTableNames(f.dbx, f.tableNames)
}

// Registry[T] interface methods.

func (r *CurrencyMigrationRegistry) Get(ctx context.Context, id string) (*models.CurrencyMigration, error) {
	var op models.CurrencyMigration
	reg := r.newSQLRegistry()
	if err := reg.ScanOneByField(ctx, store.Pair("id", id), &op); err != nil {
		return nil, errxtrace.Wrap("failed to get currency migration", err)
	}
	return &op, nil
}

func (r *CurrencyMigrationRegistry) List(ctx context.Context) ([]*models.CurrencyMigration, error) {
	var ops []*models.CurrencyMigration
	reg := r.newSQLRegistry()
	for op, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list currency migrations", err)
		}
		opCopy := op
		ops = append(ops, &opCopy)
	}
	return ops, nil
}

func (r *CurrencyMigrationRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()
	cnt, err := reg.Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count currency migrations", err)
	}
	return cnt, nil
}

// Create inserts a fresh pending row. Callers must populate
// (FromCurrency, ToCurrency, ExchangeRate); status defaults to pending.
// Postgres unique-violation on idx_currency_migrations_group_in_flight
// is mapped to registry.ErrMigrationInFlight.
func (r *CurrencyMigrationRegistry) Create(ctx context.Context, op models.CurrencyMigration) (*models.CurrencyMigration, error) {
	if op.Status == "" {
		op.Status = models.CurrencyMigrationStatusPending
	}
	if op.CreatedAt.IsZero() {
		op.CreatedAt = time.Now().UTC()
	}
	op.SetTenantID(r.tenantID)
	op.SetGroupID(r.groupID)
	op.SetCreatedByUserID(r.createdByUserID)

	if err := op.ValidateWithContext(ctx); err != nil {
		return nil, errxtrace.Wrap("validation failed", err)
	}

	reg := r.newSQLRegistry()
	created, err := reg.Create(ctx, op, nil)
	if err != nil {
		if isInFlightUniqueViolation(err) {
			return nil, registry.ErrMigrationInFlight
		}
		return nil, errxtrace.Wrap("failed to create currency migration", err)
	}
	return &created, nil
}

// Update is intentionally restricted: callers should use UpdateStatus
// for lifecycle transitions. We still implement it for the Registry[T]
// interface, but it short-circuits to the same UpdateStatus path so the
// "frozen except for lifecycle columns" invariant holds either way.
func (r *CurrencyMigrationRegistry) Update(ctx context.Context, op models.CurrencyMigration) (*models.CurrencyMigration, error) {
	if err := op.ValidateWithContext(ctx); err != nil {
		return nil, errxtrace.Wrap("validation failed", err)
	}
	patch := registry.CurrencyMigrationStatusPatch{
		Status:         op.Status,
		StartedAt:      op.StartedAt,
		CompletedAt:    op.CompletedAt,
		CommodityCount: &op.CommodityCount,
	}
	if op.ErrorMessage != "" {
		em := op.ErrorMessage
		patch.ErrorMessage = &em
	}
	if op.TotalBefore != nil {
		s := op.TotalBefore.String()
		patch.TotalBefore = &s
	}
	if op.TotalAfter != nil {
		s := op.TotalAfter.String()
		patch.TotalAfter = &s
	}
	if err := r.UpdateStatus(ctx, op.ID, patch); err != nil {
		return nil, err
	}
	return r.Get(ctx, op.ID)
}

func (r *CurrencyMigrationRegistry) Delete(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()
	if err := reg.Delete(ctx, id, nil); err != nil {
		return errxtrace.Wrap("failed to delete currency migration", err)
	}
	return nil
}

// CurrencyMigrationRegistry-specific methods.

func (r *CurrencyMigrationRegistry) LatestForGroup(ctx context.Context, groupID string) (*models.CurrencyMigration, error) {
	if groupID == "" {
		return nil, errxtrace.Wrap("group id is required", registry.ErrFieldRequired)
	}
	var op models.CurrencyMigration
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE group_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1`,
			r.tableNames.CurrencyMigrations(),
		)
		row := tx.QueryRowxContext(ctx, query, groupID)
		if err := row.StructScan(&op); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, sqlNoRows) {
			return nil, registry.ErrNotFound
		}
		return nil, errxtrace.Wrap("failed to load latest currency migration", err)
	}
	return &op, nil
}

func (r *CurrencyMigrationRegistry) InFlightForGroup(ctx context.Context, groupID string) (*models.CurrencyMigration, error) {
	if groupID == "" {
		return nil, errxtrace.Wrap("group id is required", registry.ErrFieldRequired)
	}
	var op models.CurrencyMigration
	found := false
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s
			   WHERE group_id = $1 AND status IN ('pending', 'running')
			   ORDER BY created_at DESC LIMIT 1`,
			r.tableNames.CurrencyMigrations(),
		)
		row := tx.QueryRowxContext(ctx, query, groupID)
		err := row.StructScan(&op)
		if err == nil {
			found = true
			return nil
		}
		if errors.Is(err, sqlNoRows) {
			return nil
		}
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to query in-flight currency migration", err)
	}
	if !found {
		return nil, nil
	}
	return &op, nil
}

func (r *CurrencyMigrationRegistry) CompletedTodayForGroup(ctx context.Context, groupID string, now time.Time) (int, error) {
	if groupID == "" {
		return 0, errxtrace.Wrap("group id is required", registry.ErrFieldRequired)
	}
	startOfDay := utcMidnight(now)
	endOfDay := startOfDay.AddDate(0, 0, 1)

	var cnt int
	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT COUNT(*) FROM %s
			   WHERE group_id = $1 AND status = 'completed'
			     AND completed_at >= $2 AND completed_at < $3`,
			r.tableNames.CurrencyMigrations(),
		)
		return tx.QueryRowxContext(ctx, query, groupID, startOfDay, endOfDay).Scan(&cnt)
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to count completed currency migrations for today", err)
	}
	return cnt, nil
}

// UpdateStatus mutates only the worker-managed lifecycle columns. Other
// columns stay frozen at their original values.
func (r *CurrencyMigrationRegistry) UpdateStatus(ctx context.Context, id string, patch registry.CurrencyMigrationStatusPatch) error {
	if id == "" {
		return errxtrace.Wrap("id is required", registry.ErrFieldRequired)
	}
	if patch.Status == "" || !patch.Status.IsValid() {
		return errxtrace.Wrap("status must be set to a valid value", registry.ErrInvalidInput)
	}

	setParts := []string{"status = :status"}
	params := map[string]any{
		"status":          string(patch.Status),
		"entity_field_id": id,
	}
	if patch.StartedAt != nil {
		setParts = append(setParts, "started_at = :started_at")
		params["started_at"] = patch.StartedAt.UTC()
	}
	if patch.CompletedAt != nil {
		setParts = append(setParts, "completed_at = :completed_at")
		params["completed_at"] = patch.CompletedAt.UTC()
	}
	if patch.ErrorMessage != nil {
		setParts = append(setParts, "error_message = :error_message")
		params["error_message"] = *patch.ErrorMessage
	}
	if patch.CommodityCount != nil {
		setParts = append(setParts, "commodity_count = :commodity_count")
		params["commodity_count"] = *patch.CommodityCount
	}
	if patch.TotalBefore != nil {
		setParts = append(setParts, "total_before = :total_before")
		params["total_before"] = *patch.TotalBefore
	}
	if patch.TotalAfter != nil {
		setParts = append(setParts, "total_after = :total_after")
		params["total_after"] = *patch.TotalAfter
	}

	query := fmt.Sprintf(
		`UPDATE %s SET %s WHERE id = :entity_field_id`,
		r.tableNames.CurrencyMigrations(), strings.Join(setParts, ", "),
	)

	err := r.do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		_, err := sqlx.NamedExecContext(ctx, tx, query, params)
		return err
	})
	if err != nil {
		return errxtrace.Wrap("failed to update currency migration status", err)
	}
	return nil
}

func (r *CurrencyMigrationRegistry) WriteAuditRow(ctx context.Context, row models.CurrencyMigrationAuditRow) (*models.CurrencyMigrationAuditRow, error) {
	row.SetTenantID(r.tenantID)
	row.SetGroupID(r.groupID)
	row.SetCreatedByUserID(r.createdByUserID)
	if row.CreatedAt.IsZero() {
		row.CreatedAt = time.Now().UTC()
	}

	auditReg := store.NewGroupAwareSQLRegistry[models.CurrencyMigrationAuditRow](
		r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.CurrencyMigrationAudit(),
	)
	if r.service {
		auditReg = store.NewGroupServiceSQLRegistry[models.CurrencyMigrationAuditRow](
			r.dbx, r.tableNames.CurrencyMigrationAudit(),
		)
	}

	created, err := auditReg.Create(ctx, row, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to write currency migration audit row", err)
	}
	return &created, nil
}

func (r *CurrencyMigrationRegistry) ListAuditRows(ctx context.Context, migrationID string) ([]*models.CurrencyMigrationAuditRow, error) {
	if migrationID == "" {
		return nil, errxtrace.Wrap("migration id is required", registry.ErrFieldRequired)
	}
	var rows []*models.CurrencyMigrationAuditRow
	auditReg := store.NewGroupAwareSQLRegistry[models.CurrencyMigrationAuditRow](
		r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.CurrencyMigrationAudit(),
	)
	if r.service {
		auditReg = store.NewGroupServiceSQLRegistry[models.CurrencyMigrationAuditRow](
			r.dbx, r.tableNames.CurrencyMigrationAudit(),
		)
	}
	err := auditReg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE migration_id = $1 ORDER BY created_at ASC, id ASC`,
			r.tableNames.CurrencyMigrationAudit(),
		)
		results, err := tx.QueryxContext(ctx, query, migrationID)
		if err != nil {
			return err
		}
		defer results.Close()
		for results.Next() {
			var row models.CurrencyMigrationAuditRow
			if err := results.StructScan(&row); err != nil {
				return err
			}
			rows = append(rows, &row)
		}
		return results.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list currency migration audit rows", err)
	}
	return rows, nil
}

// DeleteAuditRowsByGroup removes every audit row for the given (tenant,
// group). The audit rows only cascade from the migration row
// (currency_migration_audit_rows.migration_id ON DELETE CASCADE), not
// from the group (group_id is NO ACTION), so the group-deletion cleanup
// path must clear them explicitly. Idempotent: a parameterized DELETE
// that matches zero rows returns (0, nil).
func (r *CurrencyMigrationRegistry) DeleteAuditRowsByGroup(ctx context.Context, tenantID, groupID string) (int, error) {
	if tenantID == "" || groupID == "" {
		return 0, errxtrace.Wrap("tenant id and group id are required", registry.ErrFieldRequired)
	}

	auditReg := store.NewGroupAwareSQLRegistry[models.CurrencyMigrationAuditRow](
		r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.CurrencyMigrationAudit(),
	)
	if r.service {
		auditReg = store.NewGroupServiceSQLRegistry[models.CurrencyMigrationAuditRow](
			r.dbx, r.tableNames.CurrencyMigrationAudit(),
		)
	}

	var deleted int64
	err := auditReg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`DELETE FROM %s WHERE tenant_id = $1 AND group_id = $2`,
			r.tableNames.CurrencyMigrationAudit(),
		)
		res, err := tx.ExecContext(ctx, query, tenantID, groupID)
		if err != nil {
			return err
		}
		deleted, err = res.RowsAffected()
		return err
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to delete currency migration audit rows by group", err)
	}
	return int(deleted), nil
}

// ClaimNextPending atomically picks one pending row, flips it to running
// (TX1), sets started_at, and returns the updated row. Uses
// SELECT ... FOR UPDATE SKIP LOCKED to serialise multiple workers on the
// same DB without blocking. Service registry only — user mode has no
// reason to call this.
func (r *CurrencyMigrationRegistry) ClaimNextPending(ctx context.Context) (*models.CurrencyMigration, error) {
	if !r.service {
		return nil, errxtrace.Wrap("ClaimNextPending requires a service registry", registry.ErrUserContextRequired)
	}
	var op models.CurrencyMigration
	found := false
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		selectQuery := fmt.Sprintf(
			`SELECT id FROM %s
			   WHERE status = 'pending'
			   ORDER BY created_at ASC, id ASC
			   FOR UPDATE SKIP LOCKED LIMIT 1`,
			r.tableNames.CurrencyMigrations(),
		)
		var pickedID string
		row := tx.QueryRowxContext(ctx, selectQuery)
		if err := row.Scan(&pickedID); err != nil {
			if errors.Is(err, sqlNoRows) {
				return nil
			}
			return err
		}

		now := time.Now().UTC()
		updateQuery := fmt.Sprintf(
			`UPDATE %s SET status = 'running', started_at = $2 WHERE id = $1`,
			r.tableNames.CurrencyMigrations(),
		)
		if _, err := tx.ExecContext(ctx, updateQuery, pickedID, now); err != nil {
			return err
		}

		fetchQuery := fmt.Sprintf(`SELECT * FROM %s WHERE id = $1`, r.tableNames.CurrencyMigrations())
		if err := tx.QueryRowxContext(ctx, fetchQuery, pickedID).StructScan(&op); err != nil {
			return err
		}
		found = true
		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to claim next pending currency migration", err)
	}
	if !found {
		return nil, registry.ErrNotFound
	}
	return &op, nil
}

// SweepStuckRunning flips long-stuck running rows to failed and clears
// the matching location_groups.currency_migration_id lock. Service-mode
// only — no app role can write across groups.
func (r *CurrencyMigrationRegistry) SweepStuckRunning(ctx context.Context, now time.Time, threshold time.Duration) ([]*models.CurrencyMigration, error) {
	if !r.service {
		return nil, errxtrace.Wrap("SweepStuckRunning requires a service registry", registry.ErrUserContextRequired)
	}
	cutoff := now.UTC().Add(-threshold)

	var swept []*models.CurrencyMigration
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		// Mark stuck runs failed (sets completed_at + error_message).
		updateQuery := fmt.Sprintf(
			`UPDATE %s
			    SET status = 'failed',
			        completed_at = $2,
			        error_message = COALESCE(NULLIF(error_message, ''), 'worker crashed or stalled')
			  WHERE status = 'running' AND started_at IS NOT NULL AND started_at < $1
			  RETURNING *`,
			r.tableNames.CurrencyMigrations(),
		)
		rows, err := tx.QueryxContext(ctx, updateQuery, cutoff, now.UTC())
		if err != nil {
			return err
		}
		var sweptIDs []string
		for rows.Next() {
			var op models.CurrencyMigration
			if scanErr := rows.StructScan(&op); scanErr != nil {
				rows.Close()
				return scanErr
			}
			sweptIDs = append(sweptIDs, op.ID)
			swept = append(swept, &op)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()

		// Clear the corresponding lock signal on each affected group.
		// Done in one shot so the lock UX flips back as soon as the
		// sweep commits.
		if len(sweptIDs) > 0 {
			lockQuery := fmt.Sprintf(
				`UPDATE %s SET currency_migration_id = NULL WHERE currency_migration_id = ANY($1)`,
				r.tableNames.LocationGroups(),
			)
			if _, err := tx.ExecContext(ctx, lockQuery, sweptIDs); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to sweep stuck currency migrations", err)
	}
	return swept, nil
}

// HMAC token signing.

// IssuePreviewToken signs `inputs` and returns "<b64-payload>.<b64-mac>".
// Stateless: no DB write, no in-memory cache. Verification re-derives
// the signature from the same key.
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

// HashGroupState returns the canonical state hash hex string the start
// handler will compare against the recomputed live state. Exposed as a
// helper so the apiserver and tests share one implementation.
func HashGroupState(commodityCount int, sumCurrentPrice string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d|%s", commodityCount, sumCurrentPrice)
	return hex.EncodeToString(h.Sum(nil))
}

// Helpers.

// sqlNoRows is the sentinel returned by sqlx when StructScan finds
// nothing. Aliased so the helper code below reads cleanly.
var sqlNoRows = sql.ErrNoRows

func (r *CurrencyMigrationRegistry) newSQLRegistry() *store.RLSGroupRepository[models.CurrencyMigration, *models.CurrencyMigration] {
	if r.service {
		return store.NewGroupServiceSQLRegistry[models.CurrencyMigration](r.dbx, r.tableNames.CurrencyMigrations())
	}
	return store.NewGroupAwareSQLRegistry[models.CurrencyMigration](
		r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.CurrencyMigrations(),
	)
}

func (r *CurrencyMigrationRegistry) do(ctx context.Context, fn func(context.Context, *sqlx.Tx) error) error {
	return r.newSQLRegistry().Do(ctx, fn)
}

func utcMidnight(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// isInFlightUniqueViolation reports whether err is a Postgres
// unique-violation on the partial index that enforces "at most one
// pending|running migration per group". Other unique violations (e.g.
// uuid collision) are NOT mapped — they would mask programmer errors.
func isInFlightUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	type sqlStater interface {
		SQLState() string
	}
	type constrainter interface {
		ConstraintName() string
	}
	var s sqlStater
	if !errors.As(err, &s) || s.SQLState() != "23505" {
		// Fall back to substring match against the index name when
		// the underlying error doesn't expose SQLState (unlikely with
		// pgx, but defence in depth).
		return strings.Contains(err.Error(), inFlightUniqueIndexName)
	}
	var c constrainter
	if errors.As(err, &c) && c.ConstraintName() == inFlightUniqueIndexName {
		return true
	}
	return strings.Contains(err.Error(), inFlightUniqueIndexName)
}
