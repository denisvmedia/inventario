package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

var (
	_ validation.Validatable            = (*CommodityLoan)(nil)
	_ validation.ValidatableWithContext = (*CommodityLoan)(nil)
	_ TenantGroupAwareIDable            = (*CommodityLoan)(nil)
)

// CommodityLoan tracks a single lend-out of a commodity to a free-form
// borrower. A commodity has at most one OPEN loan (`returned_at IS NULL`)
// at any time — enforced at the service layer, not via a SQL constraint
// (Postgres lacks per-row partial uniqueness without a unique partial
// index, and the service-level check is needed anyway because the FE
// must surface a meaningful 409 instead of a constraint name).
//
// Past loans stay in the table — they back the per-commodity history
// surface and the "this thing has been lent N times to M people" pill.
//
// The "currently lent out" status is **derived** at read time: a
// commodity is on loan iff there is any row with `returned_at IS NULL`.
// We deliberately do not denormalise that flag onto commodities — keeps
// the loan-write path a single-row INSERT/UPDATE, no cross-table
// transaction needed.
//
// Borrower is intentionally free-form (name + optional contact + note),
// not a separate `borrowers` entity. Until users actually have repeat
// borrowers, free-form is enough; the data model stays tiny and the FE
// stays a single text field. See the issue body's "out of scope" list.
//
// Enable RLS for multi-tenant isolation
//
//migrator:schema:rls:enable table="commodity_loans" comment="Enable RLS for multi-tenant commodity loan isolation"
//migrator:schema:rls:policy name="commodity_loan_isolation" table="commodity_loans" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures commodity loans can only be accessed and modified by their tenant and group with required contexts"
//migrator:schema:rls:policy name="commodity_loan_background_worker_access" table="commodity_loans" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all commodity loans for processing"
//migrator:schema:table name="commodity_loans"
type CommodityLoan struct {
	//migrator:embedded mode="inline"
	TenantGroupAwareEntityID

	// CommodityID — the lent item. ON DELETE CASCADE is added manually
	// to the generated migration: hard-deleting a commodity drops its
	// loan history (no orphan rows). Soft delete is not currently a
	// commodity capability, so this is the only path that touches loans.
	//migrator:schema:field name="commodity_id" type="TEXT" not_null="true" foreign="commodities(id)" foreign_key_name="fk_commodity_loan_commodity" on_delete="CASCADE"
	CommodityID string `json:"commodity_id" db:"commodity_id"`

	// BorrowerName is required and free-form. Capped at 200 chars to
	// match the soft cap the FE already enforces on similar text
	// fields and to leave room in DB indexes if we later add one.
	//migrator:schema:field name="borrower_name" type="TEXT" not_null="true"
	BorrowerName string `json:"borrower_name" db:"borrower_name"`

	// BorrowerContact is free-form (phone / email / @handle). No
	// validation — the field is for the user's own reference.
	//migrator:schema:field name="borrower_contact" type="TEXT"
	BorrowerContact string `json:"borrower_contact" db:"borrower_contact"`

	// BorrowerNote is a free-form aide-mémoire ("works in the office
	// downstairs"). Capped at 1000 chars.
	//migrator:schema:field name="borrower_note" type="TEXT"
	BorrowerNote string `json:"borrower_note" db:"borrower_note"`

	// LentAt is the date the item left. Required. Stored as TEXT in
	// YYYY-MM-DD format to match the project's other date fields
	// (purchase_date, registered_date, last_modified_date).
	//migrator:schema:field name="lent_at" type="TEXT" not_null="true"
	LentAt Date `json:"lent_at" db:"lent_at"`

	// DueBackAt is the optional expected-return date. Nullable for
	// open-ended loans ("when you're done with it").
	//migrator:schema:field name="due_back_at" type="TEXT"
	DueBackAt PDate `json:"due_back_at" db:"due_back_at"`

	// ReturnedAt closes out the loan. Nullable until the item comes
	// back. The "open vs returned" semantics derive from this field
	// alone — there is no explicit status enum.
	//migrator:schema:field name="returned_at" type="TEXT"
	ReturnedAt PDate `json:"returned_at" db:"returned_at"`

	// ReminderSentOverdue + ReminderSentDueSoon are idempotency flags
	// for the reminder worker (separate sub-issue, not exposed in the
	// base feature's UI). Columns ship with the base table so the
	// worker doesn't need a second migration to land. Set by the
	// worker only — zero on every Create.
	//migrator:schema:field name="reminder_sent_overdue" type="BOOLEAN" not_null="true" default="false"
	ReminderSentOverdue bool `json:"reminder_sent_overdue" db:"reminder_sent_overdue" userinput:"false"`

	//migrator:schema:field name="reminder_sent_due_soon" type="BOOLEAN" not_null="true" default="false"
	ReminderSentDueSoon bool `json:"reminder_sent_due_soon" db:"reminder_sent_due_soon" userinput:"false"`

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`

	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// CommodityLoanIndexes defines the postgres indexes for commodity_loans.
type CommodityLoanIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore).
	//migrator:schema:index name="idx_commodity_loans_uuid" fields="uuid" unique="true" table="commodity_loans"
	_ int

	// Index for tenant-based queries.
	//migrator:schema:index name="idx_commodity_loans_tenant_id" fields="tenant_id" table="commodity_loans"
	_ int

	// Composite index for tenant+group RLS-filtered queries.
	//migrator:schema:index name="idx_commodity_loans_tenant_group" fields="tenant_id,group_id" table="commodity_loans"
	_ int

	// Composite index for per-commodity history reads (the Lend tab).
	//migrator:schema:index name="idx_commodity_loans_commodity" fields="commodity_id,lent_at" table="commodity_loans"
	_ int

	// Partial index for the "currently lent out" group view — only
	// open rows. Cuts the index size to the working set (closed loans
	// dominate as time goes on).
	//migrator:schema:index name="idx_commodity_loans_active" fields="group_id,due_back_at" condition="returned_at IS NULL" table="commodity_loans"
	_ int

	// Partial index for the reminder worker's overdue scan — open
	// loans with a due date in the past.
	//migrator:schema:index name="idx_commodity_loans_due" fields="due_back_at" condition="returned_at IS NULL AND due_back_at IS NOT NULL" table="commodity_loans"
	_ int
}

// IsOpen reports whether the loan is currently active (no return logged).
func (l *CommodityLoan) IsOpen() bool {
	return l.ReturnedAt == nil || *l.ReturnedAt == ""
}

// IsOverdue reports whether the loan is open AND past its due date as of
// `now`. Open loans with no due date are never overdue. Callers that
// don't care about a specific clock can pass time.Now() — keeping the
// parameter explicit makes the worker testable with a frozen clock.
func (l *CommodityLoan) IsOverdue(now time.Time) bool {
	if !l.IsOpen() || l.DueBackAt == nil || *l.DueBackAt == "" {
		return false
	}
	due := l.DueBackAt.ToTime()
	if due.IsZero() {
		return false
	}
	return now.After(due)
}

func (*CommodityLoan) Validate() error {
	return ErrMustUseValidateWithContext
}

func (l *CommodityLoan) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, l,
		validation.Field(&l.CommodityID, rules.NotEmpty),
		validation.Field(&l.BorrowerName, rules.NotEmpty, validation.Length(1, 200)),
		validation.Field(&l.BorrowerContact, validation.Length(0, 200)),
		validation.Field(&l.BorrowerNote, validation.Length(0, 1000)),
		validation.Field(&l.LentAt, validation.Required),
		validation.Field(&l.DueBackAt),
		validation.Field(&l.ReturnedAt),
	)
}
