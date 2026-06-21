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

// SecurityValidator defines the interface for security validation during restore operations
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

// RestoreSecurityValidator implements SecurityValidator for restore operations
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
