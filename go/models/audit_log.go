package models

import "time"

// AuditLog records security-relevant events for compliance and debugging.
//
//ptah:schema:table name="audit_logs"
type AuditLog struct {
	// ID is the unique identifier for the audit log entry.
	//ptah:schema:field name="id" type="TEXT" primary="true"
	ID string `json:"id" db:"id"`
	// UUID is the immutable public identifier, stable across restores.
	//ptah:schema:field name="uuid" type="TEXT" not_null="true" default_expr="(gen_random_uuid())::text"
	UUID string `json:"uuid" db:"uuid" userinput:"false"`

	// Timestamp is when the event occurred.
	//ptah:schema:field name="timestamp" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	Timestamp time.Time `json:"timestamp" db:"timestamp"`

	// UserID is the actor id. The column is polymorphic since #1785/Phase 3:
	//   - For tenant-plane actions (register, login, user-CRUD via tenant JWT), it's a users.id.
	//   - For back-office-plane admin CRUD (Phase 3 onward), it's a backoffice_users.id.
	// No FK because of the polymorphism. A future schema change (#1785 follow-up) will add actor_type.
	//ptah:schema:field name="user_id" type="TEXT"
	UserID *string `json:"user_id,omitempty" db:"user_id"`

	// TenantID is the ID of the tenant the action was performed in (nullable for system events).
	//ptah:schema:field name="tenant_id" type="TEXT"
	TenantID *string `json:"tenant_id,omitempty" db:"tenant_id"`

	// Action describes the type of event (e.g. "login", "logout", "password_change").
	//ptah:schema:field name="action" type="TEXT" not_null="true"
	Action string `json:"action" db:"action"`

	// EntityType is the type of entity affected by the action (e.g. "user", "commodity").
	//ptah:schema:field name="entity_type" type="TEXT"
	EntityType *string `json:"entity_type,omitempty" db:"entity_type"`

	// EntityID is the ID of the affected entity.
	//ptah:schema:field name="entity_id" type="TEXT"
	EntityID *string `json:"entity_id,omitempty" db:"entity_id"`

	// IPAddress is the client IP address from which the action originated.
	//ptah:schema:field name="ip_address" type="TEXT"
	IPAddress string `json:"ip_address" db:"ip_address"`

	// UserAgent is the HTTP User-Agent header from the client request.
	//ptah:schema:field name="user_agent" type="TEXT"
	UserAgent string `json:"user_agent" db:"user_agent"`

	// Success indicates whether the action succeeded.
	//ptah:schema:field name="success" type="BOOLEAN" not_null="true" default="true"
	Success bool `json:"success" db:"success"`

	// ErrorMessage contains an optional error description for failed actions.
	//ptah:schema:field name="error_message" type="TEXT"
	ErrorMessage *string `json:"error_message,omitempty" db:"error_message"`

	// ImpersonatedBy is the operator-of-record when the action was performed
	// inside an impersonation session (#1745 foundation, #1750 primitive).
	//ptah:schema:field name="impersonated_by" type="TEXT"
	ImpersonatedBy *string `json:"impersonated_by,omitempty" db:"impersonated_by"`
}

// GetID returns the audit log entry's unique identifier.
func (a *AuditLog) GetID() string {
	return a.ID
}

// SetID sets the audit log entry's unique identifier.
func (a *AuditLog) SetID(id string) {
	a.ID = id
}

// GetUUID returns the audit log entry's immutable UUID.
func (a *AuditLog) GetUUID() string {
	return a.UUID
}

// SetUUID sets the audit log entry's immutable UUID.
func (a *AuditLog) SetUUID(uuid string) {
	a.UUID = uuid
}

// AuditLogIndexes defines PostgreSQL indexes for the audit_logs table.
type AuditLogIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore)
	//ptah:schema:index name="idx_audit_logs_uuid" fields="uuid" unique="true" table="audit_logs"
	_ int

	// Index for user-based queries
	//ptah:schema:index name="audit_logs_user_id_idx" fields="user_id" table="audit_logs"
	_ int

	// Index for tenant-based queries
	//ptah:schema:index name="audit_logs_tenant_id_idx" fields="tenant_id" table="audit_logs"
	_ int

	// Index for timestamp ordering and range queries.
	//
	// The column name is quoted because `timestamp` is a keyword: PostgreSQL
	// reports this index key as `"timestamp"` through pg_get_indexdef, and Ptah
	// compares the annotation against that string verbatim. Spelled bare, the
	// drift check plans a DROP/CREATE of an identical index on every run. Ptah
	// strips the quotes again when it renders CREATE INDEX, so a schema built
	// from scratch still emits `("timestamp")`.
	//ptah:schema:index name="audit_logs_timestamp_idx" fields="\"timestamp\"" table="audit_logs"
	_ int

	// Index for action-type filtering
	//ptah:schema:index name="audit_logs_action_idx" fields="action" table="audit_logs"
	_ int

	// Composite index for entity lookups
	//ptah:schema:index name="audit_logs_entity_idx" fields="entity_type,entity_id" table="audit_logs"
	_ int
}
