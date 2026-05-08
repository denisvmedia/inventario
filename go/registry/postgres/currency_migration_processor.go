package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/internal/currency"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/internal/migrationops"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// CurrencyMigrationStuckThreshold is the §4.5 hard-coded threshold the
// worker uses to flip stuck `running` rows to `failed`. 10 minutes is
// long enough to cover legitimately slow runs on multi-thousand-row
// groups but short enough that a crashed-mid-TX2 worker is recovered
// well within a single business day.
const CurrencyMigrationStuckThreshold = 10 * time.Minute

// AuditActionCurrencyMigrationComplete is the audit_logs.action value
// the worker writes inside the same TX2 it uses to flip a migration
// row to `completed`. Co-located with the apiserver action constants so
// audit consumers can branch on a single namespace.
const (
	AuditActionCurrencyMigrationComplete = "currency_migration.complete"
	AuditActionCurrencyMigrationFail     = "currency_migration.fail"
)

// CurrencyMigrationProcessSummary is the aggregate produced by TX2 once
// a migration row commits as `completed`. The worker emits these values
// as Prometheus histogram observations.
type CurrencyMigrationProcessSummary struct {
	CommodityCount        int
	TotalBefore           decimal.Decimal
	TotalAfter            decimal.Decimal
	AcquisitionFillsCount int
	Duration              time.Duration
}

// CurrencyMigrationProcessor owns TX2 of the currency-migration
// lifecycle (#202 §4.5). It is intentionally postgres-only — the
// transaction takes the inventario_background_worker role, drops a
// pg_advisory_xact_lock keyed on group_id, runs the conversion via
// currency.ConversionService, persists per-row audit + price-changed
// events, flips the group's GroupCurrency, marks the migration
// row `completed`, and writes the audit_logs row, all atomically.
//
// Constructed at bootstrap time from the postgres FactorySet so the
// services-package worker only needs the small ProcessRunningMigration
// surface. Factory-style HMAC key handling stays inside the registry;
// the processor is plain DB plumbing.
type CurrencyMigrationProcessor struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	conv       *currency.ConversionService
}

// NewCurrencyMigrationProcessor wires the processor against the same
// dbx the registries use. Table names default to store.DefaultTableNames;
// tests that reroute tables can use NewCurrencyMigrationProcessorWithTableNames.
func NewCurrencyMigrationProcessor(dbx *sqlx.DB) *CurrencyMigrationProcessor {
	return NewCurrencyMigrationProcessorWithTableNames(dbx, store.DefaultTableNames)
}

// NewCurrencyMigrationProcessorWithTableNames is the test seam for
// renaming the underlying tables (parallels the conversion service
// constructor of the same shape).
func NewCurrencyMigrationProcessorWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *CurrencyMigrationProcessor {
	return &CurrencyMigrationProcessor{
		dbx:        dbx,
		tableNames: tableNames,
		conv:       currency.NewConversionServiceWithTableNames(tableNames),
	}
}

// ProcessRunningMigration runs TX2 for `op` (which the worker has
// already flipped to `running` via TX1).
//
// On success: the migration row is updated to `completed` with the
// captured totals, the group's GroupCurrency is flipped to op.ToCurrency
// and its currency_migration_id is cleared, every per-commodity audit
// row + price-changed CommodityEvent is written, and one
// audit_logs.currency_migration.complete row is inserted — all in a
// single tx. The returned summary is ready to feed into Prometheus.
//
// On any error the tx is rolled back. The migration row stays in
// `running` (so SweepStuckRunning recovers it after the threshold) and
// no commodity changes leak. The error is returned wrapped so the
// caller can log and continue.
func (p *CurrencyMigrationProcessor) ProcessRunningMigration(ctx context.Context, op *models.CurrencyMigration) (CurrencyMigrationProcessSummary, error) {
	var summary CurrencyMigrationProcessSummary
	if op == nil {
		return summary, errxtrace.Wrap("op is required", registry.ErrFieldRequired)
	}
	if op.TenantID == "" || op.GroupID == "" {
		return summary, errxtrace.Wrap("tenant id and group id are required on the running migration", registry.ErrFieldRequired)
	}
	if op.ID == "" {
		return summary, errxtrace.Wrap("migration id is required", registry.ErrFieldRequired)
	}

	startedAt := time.Now()

	err := store.DoAsBackgroundWorker(ctx, p.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		// Set tenant + group context so any application-role read inside
		// the tx (e.g. a future RLS-enabled join) is scoped correctly.
		// The worker role bypasses RLS but we keep the GUCs in sync so
		// helper functions (get_current_group_id) see the right value.
		if err := setLocalTenantAndGroup(ctx, tx, op.TenantID, op.GroupID); err != nil {
			return err
		}

		// pg_advisory_xact_lock serialises this group's TX2 with any
		// concurrent worker process that picks up the same row by
		// mistake (FOR UPDATE SKIP LOCKED in TX1 already prevents the
		// claim collision; the advisory lock is defence in depth for
		// the work phase). Two-arg form: hashtext(namespace) +
		// hashtext(group_id) live in the lock manager's 2x int4
		// keyspace, so the pair acts as a per-group lock without
		// needing an int8 PK.
		if _, err := tx.ExecContext(ctx,
			"SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))",
			"currency_migration", op.GroupID,
		); err != nil {
			return errxtrace.Wrap("failed to take per-group advisory lock", err)
		}

		// Build the per-row callbacks the conversion service consumes.
		// FillAcquisition delegates to migrationops.SetAcquisition so the
		// write-once guard inside the helper still runs (defence in
		// depth on top of the schema CHECK constraint). Audit writes go
		// through a tx-local TxExecutor so they commit atomically with
		// the commodity row update.
		commoditiesTable := string(p.tableNames.Commodities())
		auditExec := store.NewTxRegistry[models.CurrencyMigrationAuditRow](tx, p.tableNames.CurrencyMigrationAudit())
		eventsExec := store.NewTxRegistry[models.CommodityEvent](tx, p.tableNames.CommodityEvents())

		fillAcquisition := func(ctx context.Context, tx *sqlx.Tx, commodityID string, price decimal.Decimal, cur models.Currency) error {
			return migrationops.SetAcquisition(ctx, tx, commoditiesTable, commodityID, price, cur)
		}
		writeAudit := func(ctx context.Context, row models.CurrencyMigrationAuditRow) error {
			row.SetTenantID(op.TenantID)
			row.SetGroupID(op.GroupID)
			row.SetCreatedByUserID(op.CreatedByUserID)
			if row.CreatedAt.IsZero() {
				row.CreatedAt = time.Now().UTC()
			}
			// TxExecutor.Insert does not auto-generate id/uuid the way
			// the standard repository Create path does — the worker
			// is bypassing that path so the audit write commits
			// atomically with the commodity update. Pre-stamp both
			// columns explicitly.
			row.ID = generateAuditLogID()
			row.UUID = generateAuditLogID()
			return auditExec.Insert(ctx, row)
		}

		runResult, err := p.conv.ConvertGroup(ctx, tx, currency.RunOptions{
			TenantID:        op.TenantID,
			GroupID:         op.GroupID,
			FromCurrency:    op.FromCurrency,
			ToCurrency:      op.ToCurrency,
			Rate:            op.ExchangeRate,
			MigrationID:     op.ID,
			FillAcquisition: fillAcquisition,
			Audit:           writeAudit,
		})
		if err != nil {
			return errxtrace.Wrap("conversion failed", err)
		}

		// Emit one price_changed CommodityEvent per mutated row inside
		// the same tx so the timeline lands atomically with the price
		// changes. Skip rows whose price didn't actually move (Case C
		// where rate is 1.0, or Case B in a third currency with no
		// rate-side delta — though that's impossible by construction
		// since rate>0 always shifts ConvertedOriginalPrice).
		now := time.Now().UTC()
		for i := range runResult.Diffs {
			diff := &runResult.Diffs[i]
			if !priceImagesDiffer(diff.Before, diff.After) {
				continue
			}
			event := buildPriceChangedEvent(op, diff, now)
			// TxExecutor.Insert does not auto-generate id/uuid;
			// pre-stamp both so the INSERT carries them.
			event.ID = generateAuditLogID()
			event.UUID = generateAuditLogID()
			if err := eventsExec.Insert(ctx, event); err != nil {
				return errxtrace.Wrap("failed to write commodity_events row", err)
			}
		}

		// Flip the group's GroupCurrency to the new currency and clear
		// the currency_migration_id lock signal. Both updates land in
		// the same tx as the migration row update, so the FE sees the
		// lock release and the new group currency together.
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf(
				`UPDATE %s
				    SET group_currency = $1,
				        currency_migration_id = NULL,
				        updated_at = $2
				  WHERE id = $3`,
				p.tableNames.LocationGroups(),
			),
			string(op.ToCurrency), now, op.GroupID,
		); err != nil {
			return errxtrace.Wrap("failed to update location_groups for completed migration", err)
		}

		// Update the migration row to completed.
		totalBefore := runResult.TotalCurrentBefore
		totalAfter := runResult.TotalCurrentAfter
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf(
				`UPDATE %s
				    SET status = 'completed',
				        completed_at = $1,
				        commodity_count = $2,
				        total_before = $3,
				        total_after = $4
				  WHERE id = $5`,
				p.tableNames.CurrencyMigrations(),
			),
			now, runResult.CommodityCount, totalBefore.String(), totalAfter.String(), op.ID,
		); err != nil {
			return errxtrace.Wrap("failed to mark currency migration completed", err)
		}

		// Audit-log row keyed to the migration. tenant_id / user_id are
		// what start handler stamped at INSERT time; entity_type +
		// entity_id let downstream queries pivot on the migration.
		if err := insertCurrencyMigrationAuditLog(ctx, tx, p.tableNames, auditLogParams{
			Action:  AuditActionCurrencyMigrationComplete,
			Op:      op,
			Success: true,
			ErrMsg:  "",
			Now:     now,
			Summary: &runResult,
			Reason:  "",
		}); err != nil {
			return errxtrace.Wrap("failed to write audit_logs row for completed migration", err)
		}

		summary = CurrencyMigrationProcessSummary{
			CommodityCount:        runResult.CommodityCount,
			TotalBefore:           totalBefore,
			TotalAfter:            totalAfter,
			AcquisitionFillsCount: runResult.AcquisitionFillsCount,
		}
		return nil
	})

	summary.Duration = time.Since(startedAt)
	if err != nil {
		return CurrencyMigrationProcessSummary{Duration: summary.Duration}, err
	}
	return summary, nil
}

// WriteSweepFailureAuditLog inserts one audit_logs.currency_migration.fail
// row per swept migration. Called by the worker after SweepStuckRunning
// returns the recovered rows so the audit trail captures the recovery
// event even though the original tx rolled back. The reason string is
// the error_message persisted on the migration row (defaults to
// "worker crashed or stalled" if empty).
func (p *CurrencyMigrationProcessor) WriteSweepFailureAuditLog(ctx context.Context, op *models.CurrencyMigration) error {
	if op == nil {
		return nil
	}
	return store.DoAsBackgroundWorker(ctx, p.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		return insertCurrencyMigrationAuditLog(ctx, tx, p.tableNames, auditLogParams{
			Action:  AuditActionCurrencyMigrationFail,
			Op:      op,
			Success: false,
			ErrMsg:  op.ErrorMessage,
			Now:     time.Now().UTC(),
			Reason:  "recovery_sweep",
		})
	})
}

// auditLogParams bundles the inputs to insertCurrencyMigrationAuditLog
// so the helper signature stays compact.
type auditLogParams struct {
	Action  string
	Op      *models.CurrencyMigration
	Success bool
	ErrMsg  string
	Now     time.Time
	Summary *currency.RunResult // nil for failure rows
	Reason  string              // free-form context, embedded in user_agent
}

// insertCurrencyMigrationAuditLog writes a single row into audit_logs
// inside the supplied tx. The tenant/user fields fall back to the
// migration row's stamps so the audit log is never headless.
func insertCurrencyMigrationAuditLog(ctx context.Context, tx *sqlx.Tx, tableNames store.TableNames, p auditLogParams) error {
	entry := models.AuditLog{
		Timestamp: p.Now,
		Action:    p.Action,
		Success:   p.Success,
	}
	if p.Op.CreatedByUserID != "" {
		userID := p.Op.CreatedByUserID
		entry.UserID = &userID
	}
	if p.Op.TenantID != "" {
		tenantID := p.Op.TenantID
		entry.TenantID = &tenantID
	}
	entityType := "currency_migration"
	entry.EntityType = &entityType
	if p.Op.ID != "" {
		entityID := p.Op.ID
		entry.EntityID = &entityID
	}
	if p.ErrMsg != "" {
		em := p.ErrMsg
		entry.ErrorMessage = &em
	}
	// Encode summary (success path) or recovery breadcrumb (failure
	// path) as a JSON blob in user_agent. The audit_logs schema doesn't
	// have a generic context column today; stuffing the breadcrumb into
	// user_agent keeps the row self-describing without a schema bump.
	breadcrumb := map[string]any{
		"group_id":      p.Op.GroupID,
		"migration_id":  p.Op.ID,
		"from_currency": string(p.Op.FromCurrency),
		"to_currency":   string(p.Op.ToCurrency),
	}
	if p.Reason != "" {
		breadcrumb["reason"] = p.Reason
	}
	if p.Summary != nil {
		breadcrumb["commodity_count"] = p.Summary.CommodityCount
		breadcrumb["acquisition_fills"] = p.Summary.AcquisitionFillsCount
		breadcrumb["total_before"] = p.Summary.TotalCurrentBefore.String()
		breadcrumb["total_after"] = p.Summary.TotalCurrentAfter.String()
	}
	if encoded, err := json.Marshal(breadcrumb); err == nil {
		entry.UserAgent = string(encoded)
	}

	// audit_logs goes through NonRLSRepository.Create in the normal
	// path, which stamps id+uuid via generateID(). We're bypassing that
	// path so the writes commit in TX2 — pre-stamp both columns
	// explicitly so the INSERT carries them.
	entry.ID = generateAuditLogID()
	entry.UUID = generateAuditLogID()

	exec := store.NewTxRegistry[models.AuditLog](tx, tableNames.AuditLogs())
	if err := exec.Insert(ctx, entry); err != nil {
		return errxtrace.Wrap("failed to insert audit_logs row", err)
	}
	return nil
}

// generateAuditLogID returns a fresh UUID4 string. Mirrors store.generateID's
// implementation; lives here (vs the unexported store helper) so this file
// stays self-contained without leaking a new export.
func generateAuditLogID() string {
	return uuid.New().String()
}

// setLocalTenantAndGroup sets the per-tx app.current_tenant_id and
// app.current_group_id GUCs so any helper function inside the tx
// resolves the correct scope. Mirrors the per-request middleware path
// that the apiserver uses; the worker has no http.Request to drive
// it from, so we set the values from the migration row directly.
func setLocalTenantAndGroup(ctx context.Context, tx *sqlx.Tx, tenantID, groupID string) error {
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL app.current_tenant_id = '%s'", escapeSQLIdent(tenantID))); err != nil {
		return errxtrace.Wrap("failed to set tenant context for currency migration tx", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL app.current_group_id = '%s'", escapeSQLIdent(groupID))); err != nil {
		return errxtrace.Wrap("failed to set group context for currency migration tx", err)
	}
	return nil
}

// escapeSQLIdent escapes single quotes for the SET LOCAL value above.
// Same pattern store.setUserContext / setTenantContext use for their
// session GUCs — the values we feed in come from the migration row's
// own UUID columns so they shouldn't carry quotes, but defence in
// depth costs nothing.
func escapeSQLIdent(s string) string {
	return sqlEscape(s)
}

func sqlEscape(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			out = append(out, '\'', '\'')
			continue
		}
		out = append(out, s[i])
	}
	return string(out)
}

// priceImagesDiffer reports whether the four price-related fields
// changed across the conversion. Used to skip emitting commodity_events
// for no-op rows (e.g. zero-priced commodities under any rate).
func priceImagesDiffer(before, after currency.RowImage) bool {
	if !before.OriginalPrice.Equal(after.OriginalPrice) {
		return true
	}
	if before.OriginalPriceCurrency != after.OriginalPriceCurrency {
		return true
	}
	if !before.ConvertedOriginalPrice.Equal(after.ConvertedOriginalPrice) {
		return true
	}
	if !before.CurrentPrice.Equal(after.CurrentPrice) {
		return true
	}
	return false
}

// buildPriceChangedEvent returns the CommodityEvent row for one
// migrated commodity. Mirrors CommodityEventService's price_changed
// payload shape so the FE timeline can render it identically to a
// user-driven price edit.
func buildPriceChangedEvent(op *models.CurrencyMigration, diff *currency.PerRowDiff, now time.Time) models.CommodityEvent {
	event := models.CommodityEvent{
		CommodityID: diff.CommodityID,
		Kind:        models.CommodityEventKindPriceChanged,
		OccurredAt:  now,
		Before: models.CommodityEventPayload{
			"original_price":           diff.Before.OriginalPrice.String(),
			"original_price_currency":  string(diff.Before.OriginalPriceCurrency),
			"converted_original_price": diff.Before.ConvertedOriginalPrice.String(),
			"current_price":            diff.Before.CurrentPrice.String(),
		},
		After: models.CommodityEventPayload{
			"original_price":           diff.After.OriginalPrice.String(),
			"original_price_currency":  string(diff.After.OriginalPriceCurrency),
			"converted_original_price": diff.After.ConvertedOriginalPrice.String(),
			"current_price":            diff.After.CurrentPrice.String(),
			"_source":                  "currency_migration",
			"_migration_id":            op.ID,
		},
	}
	event.SetTenantID(op.TenantID)
	event.SetGroupID(op.GroupID)
	event.SetCreatedByUserID(op.CreatedByUserID)
	return event
}
