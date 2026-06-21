package security

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// Package-level errors
var (
	ErrNoUserContext      = errors.New("unauthorized: no user context")
	ErrOwnershipViolation = errors.New("commodity belongs to a different user")
)

// SecurityValidator is the audit-logging surface for restore operations: it
// records unauthorized entity-access attempts. Ownership and import scope are
// enforced by the RLS layer and the restore processor
// (validateCommodityOwnershipInDB), not here — this interface only logs.
type SecurityValidator interface {
	LogUnauthorizedAttempt(ctx context.Context, attempt UnauthorizedAttempt)
}

// UnauthorizedAttempt represents details about an unauthorized access attempt
type UnauthorizedAttempt struct {
	UserID         string
	TargetEntityID string
	EntityType     string
	Operation      string
	AttemptType    string
	Timestamp      time.Time
	RequestDetails map[string]any
}

// RestoreSecurityValidator is the slog-backed SecurityValidator the restore
// processor uses to record unauthorized-access attempts.
type RestoreSecurityValidator struct {
	logger *slog.Logger
}

// NewRestoreSecurityValidator creates a new RestoreSecurityValidator
func NewRestoreSecurityValidator(logger *slog.Logger) *RestoreSecurityValidator {
	return &RestoreSecurityValidator{
		logger: logger,
	}
}

// LogUnauthorizedAttempt logs details about unauthorized access attempts
func (v *RestoreSecurityValidator) LogUnauthorizedAttempt(_ context.Context, attempt UnauthorizedAttempt) {
	v.logger.Warn("Unauthorized entity access attempt",
		"user_id", attempt.UserID,
		"target_entity_id", attempt.TargetEntityID,
		"entity_type", attempt.EntityType,
		"operation", attempt.Operation,
		"attempt_type", attempt.AttemptType,
		"timestamp", attempt.Timestamp,
		"request_details", attempt.RequestDetails,
	)

	// TODO: Consider additional security measures:
	// - Rate limiting for repeated attempts
	// - Alerting for suspicious patterns
	// - Temporary account restrictions
}
