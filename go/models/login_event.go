package models

import "time"

// LoginOutcome is the result of a credential check attempt recorded in
// login_events (issue #1379). Stored as TEXT so the enum can grow
// without a DB CHECK migration each time.
type LoginOutcome string

const (
	// LoginOutcomeOK indicates a successful authentication.
	LoginOutcomeOK LoginOutcome = "ok"
	// LoginOutcomeBadPassword covers both "wrong password" and "unknown email"
	// — collapsing them keeps the surface useful for the user (they see
	// "someone tried my email") without leaking enumeration on the unknown
	// branch.
	LoginOutcomeBadPassword LoginOutcome = "bad_password"
	// LoginOutcomeAccountLocked is recorded when the rate limiter rejected
	// the attempt before the password check happened.
	LoginOutcomeAccountLocked LoginOutcome = "account_locked"
	// LoginOutcomeAccountDisabled is recorded when the credentials matched
	// but IsActive=false (terminated account).
	LoginOutcomeAccountDisabled LoginOutcome = "account_disabled"
	// LoginOutcomeEmailNotVerified is reserved for the post-#1372 path
	// where unverified accounts cannot log in; recorded so the user can
	// see the attempt even though no session was issued.
	LoginOutcomeEmailNotVerified LoginOutcome = "email_not_verified"
	// LoginOutcomeMFARequired marks a successful step-1 password check
	// that did not issue tokens because the user has TOTP enabled. The
	// step-2 attempt is recorded separately as LoginOutcomeOK or
	// LoginOutcomeBadMFA (#1380 / #1645).
	LoginOutcomeMFARequired LoginOutcome = "mfa_required"
	// LoginOutcomeBadMFA is recorded when step-2 of the MFA flow rejected
	// the supplied TOTP / backup code (#1380 / #1645).
	LoginOutcomeBadMFA LoginOutcome = "bad_mfa"
	// LoginOutcomeMFAAdminReset is recorded when an operator wiped the
	// user's MFA enrollment via the CLI (`inventario users mfa-reset`).
	// It shows up in the user's login history so they know the second
	// factor was removed out-of-band rather than by them (#1645).
	LoginOutcomeMFAAdminReset LoginOutcome = "mfa_admin_reset"
)

// LoginMethod is the credential family that produced the event. "password"
// is the original value; OAuth methods land with #1394.
type LoginMethod string

const (
	// LoginMethodPassword is the email + password flow.
	LoginMethodPassword LoginMethod = "password"
	// LoginMethodOAuthGoogle is the Google OAuth sign-in flow (#1394).
	// Recorded on both the start-of-session callback and any subsequent
	// link-from-settings attempt so the history page reads consistently
	// with the password flow.
	LoginMethodOAuthGoogle LoginMethod = "oauth_google"
	// LoginMethodOAuthGitHub is the GitHub OAuth sign-in flow (#1394).
	LoginMethodOAuthGitHub LoginMethod = "oauth_github"
)

// LoginEvent is the append-only audit trail of credential-check attempts
// (issue #1379). Every code path in apiserver/auth.go that reaches a
// credential check writes one row regardless of outcome. Retention is
// bounded by the login_event_retention_worker (90 days default).
//
// IP storage policy mirrors refresh_tokens: truncated /24 (IPv4) or /56
// (IPv6) — never the raw client address.
//
//migrator:schema:table name="login_events"
//migrator:schema:rls:enable table="login_events" comment="Enable RLS for multi-tenant login event isolation"
//migrator:schema:rls:policy name="login_event_tenant_isolation" table="login_events" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != ''" comment="Login events are tenant-isolated; per-user filtering happens in application logic so a user only sees their own attempts"
//migrator:schema:rls:policy name="login_event_background_worker_access" table="login_events" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows the retention worker and login flow to insert/sweep events outside any user context"
type LoginEvent struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID

	// UserID references the user the attempt resolved to. NULL when the
	// email did not match any user (unknown-email bad_password case) so
	// the row still survives the retention sweep keyed on user_id.
	//migrator:schema:field name="user_id" type="TEXT" foreign="users(id)" foreign_key_name="fk_login_event_user"
	UserID *string `json:"user_id,omitempty" db:"user_id"`

	// Email is the address the client typed. Always recorded — even when
	// UserID is set — so a user changing their email later still sees
	// "someone tried <my-old-email>" in their history. Empty allowed for
	// future code paths that authenticate without an email (none today).
	//migrator:schema:field name="email" type="TEXT" not_null="true"
	Email string `json:"email" db:"email"`

	// Outcome is the resolved LoginOutcome at the moment the row was
	// written. Stored TEXT (no DB CHECK).
	//migrator:schema:field name="outcome" type="TEXT" not_null="true"
	Outcome LoginOutcome `json:"outcome" db:"outcome"`

	// Method is the credential family. "password" for v1; OAuth lands
	// with #1394 / #1395.
	//migrator:schema:field name="method" type="TEXT" not_null="true" default="password"
	Method LoginMethod `json:"method" db:"method"`

	// IPAddress holds the truncated client IP (/24 IPv4, /56 IPv6) per
	// the privacy policy in #1378. Empty when the request had no
	// resolvable address.
	//migrator:schema:field name="ip_address" type="VARCHAR(64)"
	IPAddress string `json:"ip_address,omitempty" db:"ip_address"`

	// UserAgent is the raw Sec-CH-UA / User-Agent header. Parsing happens
	// client-side per #1378 option 2 (server-side parsing would age
	// poorly as UA strings drift).
	//migrator:schema:field name="user_agent" type="TEXT"
	UserAgent string `json:"user_agent,omitempty" db:"user_agent"`

	// CreatedAt is the wall-clock instant the row was written. The
	// list endpoint orders newest-first by this column.
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`
}

// LoginEventIndexes carries the migrator hints for the table's indexes.
type LoginEventIndexes struct {
	// Unique index for the immutable UUID (deduplication key for
	// import/restore).
	//migrator:schema:index name="idx_login_events_uuid" fields="uuid" unique="true" table="login_events"
	_ int

	// Read pattern: "the last N events for me", newest first.
	//migrator:schema:index name="idx_login_events_user_created_at" fields="user_id,created_at" table="login_events"
	_ int

	// Tenant isolation index — same shape every other tenant-scoped
	// table uses; lets the RLS qual short-circuit on a single column.
	//migrator:schema:index name="idx_login_events_tenant_id" fields="tenant_id" table="login_events"
	_ int

	// Retention sweep predicate ("delete WHERE created_at < cutoff").
	// A plain (created_at) index is enough — the table is append-only,
	// the sweep runs daily, and we don't need composite ordering here.
	//migrator:schema:index name="idx_login_events_created_at" fields="created_at" table="login_events"
	_ int
}
