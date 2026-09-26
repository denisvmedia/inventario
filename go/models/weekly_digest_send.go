package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"
)

var (
	_ validation.Validatable            = (*WeeklyDigestSend)(nil)
	_ validation.ValidatableWithContext = (*WeeklyDigestSend)(nil)
	_ TenantUserAwareIDable             = (*WeeklyDigestSend)(nil)
)

// Row-level security for weekly_digest_sends, the same shape the other
// per-user tenant tables carry: the owning user inside their tenant for
// inventario_app, and a bypass for the worker that writes these rows while no
// user context exists on the connection.
//
//ptah:schema:rls:enable table="weekly_digest_sends" comment="Enable RLS for per-user weekly digest send isolation"
//ptah:schema:rls:policy name="weekly_digest_send_isolation" table="weekly_digest_sends" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" comment="Ensures a digest send record is visible only to the user it was sent to, within their tenant"
//ptah:schema:rls:policy name="weekly_digest_send_background_worker_access" table="weekly_digest_sends" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows the digest worker to record sends across every tenant, where no user context exists"

// WeeklyDigestSend is the idempotency row the weekly digest worker writes when
// it has enqueued a digest. The (user_id, week_start) pair is unique, so a
// second tick in the same week — a restart, an overlapping schedule, a second
// replica — cannot send a user two digests for one week.
//
// The row is written before the email is enqueued rather than after. The two
// orderings trade a duplicate email against a missed one, and for a weekly
// summary a miss is the cheaper failure: nothing in it is actionable that the
// app does not also show, while two identical digests look like a bug to the
// reader. This is the opposite choice from the reminder workers, which write
// after a successful enqueue because a missed warranty alert has a deadline
// behind it.
//
//ptah:schema:table name="weekly_digest_sends"
type WeeklyDigestSend struct {
	//ptah:embedded mode="inline"
	TenantUserAwareEntityID

	// WeekStart is the Monday of the week the digest covers, as a date in UTC.
	// It is the idempotency key, so it has to be derived the same way every
	// tick — see services.WeekStart.
	//ptah:schema:field name="week_start" type="DATE" not_null="true"
	WeekStart time.Time `json:"week_start" db:"week_start"`

	// SentAt is when the worker claimed the week, not when the mail left: the
	// email queue is asynchronous and records delivery itself.
	//ptah:schema:field name="sent_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	SentAt time.Time `json:"sent_at" db:"sent_at"`
}

// WeeklyDigestSendIndexes defines the indexes for weekly_digest_sends.
type WeeklyDigestSendIndexes struct {
	// The idempotency key. Unique, because the insert is what claims the week:
	// a losing insert is how a second worker learns it has nothing to send.
	//ptah:schema:index name="idx_weekly_digest_sends_user_week" fields="user_id,week_start" unique="true" table="weekly_digest_sends"
	_ int

	// Lets the retention sweep find old rows without scanning the table.
	//ptah:schema:index name="idx_weekly_digest_sends_week_start" fields="week_start" table="weekly_digest_sends"
	_ int
}

func (*WeeklyDigestSend) Validate() error {
	return ErrMustUseValidateWithContext
}

func (w *WeeklyDigestSend) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, w,
		validation.Field(&w.TenantUserAwareEntityID),
		validation.Field(&w.WeekStart, validation.Required),
	)
}
