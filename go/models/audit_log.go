package models

import "time"

// AuditLog records security-relevant events for compliance and debugging.
//
//migrator:schema:table name="audit_logs"
type AuditLog struct {
	// ID is the unique identifier for the audit log entry.
	//migrator:schema:field name="id" type="TEXT" primary="true"
	ID string `json:"id" db:"id"`

	// Timestamp is when the event occurred.
	//migrator:schema:field name="timestamp" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	Timestamp time.Time `json:"timestamp" db:"timestamp"`

	// UserID is the ID of the user who performed the action (nullable for unauthenticated events).
	//migrator:schema:field name="user_id" type="TEXT"
	UserID *string `json:"user_id,omitempty" db:"user_id"`

	// TenantID is the ID of the tenant the action was performed in (nullable for system events).
	//migrator:schema:field name="tenant_id" type="TEXT"
	TenantID *string `json:"tenant_id,omitempty" db:"tenant_id"`

	// Action describes the type of event (e.g. "login", "logout", "password_change").
	//migrator:schema:field name="action" type="TEXT" not_null="true"
	Action string `json:"action" db:"action"`

	// EntityType is the type of entity affected by the action (e.g. "user", "commodity").
	//migrator:schema:field name="entity_type" type="TEXT"
	EntityType *string `json:"entity_type,omitempty" db:"entity_type"`

	// EntityID is the ID of the affected entity.
	//migrator:schema:field name="entity_id" type="TEXT"
	EntityID *string `json:"entity_id,omitempty" db:"entity_id"`

	// IPAddress is the client IP address from which the action originated.
	//migrator:schema:field name="ip_address" type="TEXT"
	IPAddress string `json:"ip_address" db:"ip_address"`

	// UserAgent is the HTTP User-Agent header from the client request.
	//migrator:schema:field name="user_agent" type="TEXT"
	UserAgent string `json:"user_agent" db:"user_agent"`

	// Success indicates whether the action succeeded.
	//migrator:schema:field name="success" type="BOOLEAN" not_null="true" default="true"
	Success bool `json:"success" db:"success"`

	// ErrorMessage contains an optional error description for failed actions.
	//migrator:schema:field name="error_message" type="TEXT"
	ErrorMessage *string `json:"error_message,omitempty" db:"error_message"`
}

// GetID returns the audit log entry's unique identifier.
func (a *AuditLog) GetID() string {
	return a.ID
}

// SetID sets the audit log entry's unique identifier.
func (a *AuditLog) SetID(id string) {
	a.ID = id
}

// AuditLogIndexes defines PostgreSQL indexes for the audit_logs table.
type AuditLogIndexes struct {
	// Index for user-based queries
	//migrator:schema:index name="audit_logs_user_id_idx" fields="user_id" table="audit_logs"
	_ int

	// Index for tenant-based queries
	//migrator:schema:index name="audit_logs_tenant_id_idx" fields="tenant_id" table="audit_logs"
	_ int

	// Index for timestamp ordering and range queries
	//migrator:schema:index name="audit_logs_timestamp_idx" fields="timestamp" table="audit_logs"
	_ int

	// Index for action-type filtering
	//migrator:schema:index name="audit_logs_action_idx" fields="action" table="audit_logs"
	_ int

	// Composite index for entity lookups
	//migrator:schema:index name="audit_logs_entity_idx" fields="entity_type,entity_id" table="audit_logs"
	_ int
}
