package models

import "time"

// Commodity scan audit values used by both the service that writes the
// row and the registry tests that read it back. Keeping them as typed
// constants prevents accidental drift between the writer and the
// retention reader.
const (
	// CommodityScanStatusOK signals a successful provider call whose
	// extracted result was returned to the client.
	CommodityScanStatusOK = "ok"
	// CommodityScanStatusError signals an upstream/provider error that
	// could not be classified more specifically.
	CommodityScanStatusError = "error"
	// CommodityScanStatusRateLimited signals the per-user rate limiter
	// rejected the request before the provider was called.
	CommodityScanStatusRateLimited = "rate_limited"
	// CommodityScanStatusTimeout signals the provider call exceeded
	// the configured deadline.
	CommodityScanStatusTimeout = "timeout"
	// CommodityScanStatusDisabled signals the provider was configured
	// as "none" — the request never went out.
	CommodityScanStatusDisabled = "disabled"
	// CommodityScanStatusValidation signals the request was rejected
	// pre-provider by the service-layer validator (too many photos,
	// unsupported MIME, oversize part, no photos, body cap hit).
	// Distinct from "error" so the rate limiter can exclude these
	// rows — a user who sends malformed requests shouldn't burn the
	// per-user budget that's meant to bound vendor cost on real
	// provider attempts.
	CommodityScanStatusValidation = "validation"
)

// CommodityScanAudit records each invocation of the AI vision scan
// endpoint (issue #1720). It carries enough metadata to drive cost +
// abuse dashboards (which provider, how many photos, how many tokens)
// and to back the per-user rate limiter (CountRecentForUser).
//
// RLS is enabled and the policies mirror refresh_tokens.go: the row is
// only visible to the (tenant_id, user_id) owner, with a separate
// background-worker policy that grants the worker role unrestricted
// access for retention sweeps and analytics.
//
// Enable RLS for multi-tenant + per-user isolation:
//
//migrator:schema:rls:enable table="commodity_scan_audits" comment="Enable RLS for multi-tenant commodity scan audit isolation"
//migrator:schema:rls:policy name="commodity_scan_audit_isolation" table="commodity_scan_audits" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" comment="Ensures commodity scan audit rows can only be accessed and modified by the owning user within their tenant"
//migrator:schema:rls:policy name="commodity_scan_audit_background_worker_access" table="commodity_scan_audits" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all commodity scan audit rows for retention/analytics"
//migrator:schema:table name="commodity_scan_audits"
type CommodityScanAudit struct {
	//migrator:embedded mode="inline"
	TenantUserAwareEntityID

	// Provider is the aivision provider name that handled the scan
	// (e.g. "anthropic", "openai", "mock"). For rate-limit / disabled
	// rows this is the configured provider name even though no call
	// went out, so the audit table tells the same operational story
	// regardless of where the request short-circuited.
	//migrator:schema:field name="provider" type="VARCHAR(32)" not_null="true"
	Provider string `json:"provider" db:"provider"`

	// Model is the specific model id the provider was configured with
	// (e.g. "claude-sonnet-4-6", "gpt-4o"). Empty when the call was
	// short-circuited before a model was resolved.
	//migrator:schema:field name="model" type="VARCHAR(64)" not_null="true"
	Model string `json:"model" db:"model"`

	// PhotoCount is the number of photos in the request, after the
	// handler's per-call limit was applied. 0 for short-circuited rows.
	//migrator:schema:field name="photo_count" type="SMALLINT" not_null="true"
	PhotoCount int16 `json:"photo_count" db:"photo_count"`

	// TotalPhotoBytes is the sum of per-photo sizes (uncompressed
	// reach the provider; we don't re-encode). Used by abuse dashboards.
	//migrator:schema:field name="total_photo_bytes" type="INTEGER" not_null="true"
	TotalPhotoBytes int32 `json:"total_photo_bytes" db:"total_photo_bytes"`

	// Status is one of the CommodityScanStatus* constants. Always set.
	//migrator:schema:field name="status" type="VARCHAR(16)" not_null="true"
	Status string `json:"status" db:"status"`

	// ErrorCode is the structured error code surfaced to the client
	// (e.g. "commodity_scan.rate_limited"). Empty on success.
	//migrator:schema:field name="error_code" type="VARCHAR(64)"
	ErrorCode string `json:"error_code,omitempty" db:"error_code"`

	// LatencyMS is the wall-clock service-side duration in ms.
	//migrator:schema:field name="latency_ms" type="INTEGER" not_null="true"
	LatencyMS int32 `json:"latency_ms" db:"latency_ms"`

	// TokensUsed is the provider-reported token usage when available;
	// zero otherwise.
	//migrator:schema:field name="tokens_used" type="INTEGER" not_null="true" default="0"
	TokensUsed int32 `json:"tokens_used" db:"tokens_used"`

	// ResultJSON is the marshalled ScanResult on success. Empty on
	// every non-OK status. Kept JSONB so cost-analytics queries can
	// project specific fields without a downstream join.
	//migrator:schema:field name="result_json" type="JSONB"
	ResultJSON []byte `json:"result_json,omitempty" db:"result_json"`

	// CreatedAt is the row creation time, set by the registry.
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// CommodityScanAuditIndexes defines PostgreSQL indexes for the
// commodity_scan_audits table. The composite (user_id, created_at)
// index backs the rate-limit count query.
type CommodityScanAuditIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore)
	//migrator:schema:index name="idx_commodity_scan_audits_uuid" fields="uuid" unique="true" table="commodity_scan_audits"
	_ int

	// Composite index for the per-user rate-limit window query.
	//migrator:schema:index name="idx_commodity_scan_audits_user_created" fields="user_id,created_at" table="commodity_scan_audits"
	_ int

	// Index for tenant-level dashboards (cost per tenant).
	//migrator:schema:index name="idx_commodity_scan_audits_tenant_created" fields="tenant_id,created_at" table="commodity_scan_audits"
	_ int
}
