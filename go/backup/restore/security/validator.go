package security

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/registry"
)

// Package-level errors
var (
	ErrNoUserContext = errors.New("unauthorized: no user context")
)

// SecurityValidator defines the interface for security validation during restore operations
type SecurityValidator interface {
	ValidateEntityOwnership(ctx context.Context, entityID string, userID string) error
	ValidateRelationshipIntegrity(ctx context.Context, fileType string, parentEntityType string) error
	ValidateImportScope(ctx context.Context, entityID string, importSession string, sessionEntities map[string]bool) error
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
	factorySet *registry.FactorySet
	logger     *slog.Logger
}

// NewRestoreSecurityValidator creates a new RestoreSecurityValidator
func NewRestoreSecurityValidator(factorySet *registry.FactorySet, logger *slog.Logger) *RestoreSecurityValidator {
	return &RestoreSecurityValidator{
		factorySet: factorySet,
		logger:     logger,
	}
}

// ValidateEntityOwnership validates that the current user owns the specified entity
func (v *RestoreSecurityValidator) ValidateEntityOwnership(ctx context.Context, entityID string, userID string) error {
	// Use service account registries to bypass user filtering and see all entities
	commodityRegistry := v.factorySet.CommodityRegistryFactory.CreateServiceRegistry()
	areaRegistry := v.factorySet.AreaRegistryFactory.CreateServiceRegistry()
	locationRegistry := v.factorySet.LocationRegistryFactory.CreateServiceRegistry()

	// Check commodity ownership
	if commodity, err := commodityRegistry.Get(ctx, entityID); err == nil {
		if commodity.UserID != userID {
			v.LogUnauthorizedAttempt(ctx, UnauthorizedAttempt{
				UserID:         userID,
				TargetEntityID: entityID,
				EntityType:     "commodity",
				Operation:      "restore_link_files",
				AttemptType:    "cross_user_access",
				Timestamp:      time.Now(),
			})
			return errors.New("unauthorized: cannot link to entity owned by different user")
		}
		return nil
	}

	// Check area ownership
	if area, err := areaRegistry.Get(ctx, entityID); err == nil {
		if area.UserID != userID {
			v.LogUnauthorizedAttempt(ctx, UnauthorizedAttempt{
				UserID:         userID,
				TargetEntityID: entityID,
				EntityType:     "area",
				Operation:      "restore_link_files",
				AttemptType:    "cross_user_access",
				Timestamp:      time.Now(),
			})
			return errors.New("unauthorized: cannot link to entity owned by different user")
		}
		return nil
	}

	// Check location ownership
	if location, err := locationRegistry.Get(ctx, entityID); err == nil {
		if location.UserID != userID {
			v.LogUnauthorizedAttempt(ctx, UnauthorizedAttempt{
				UserID:         userID,
				TargetEntityID: entityID,
				EntityType:     "location",
				Operation:      "restore_link_files",
				AttemptType:    "cross_user_access",
				Timestamp:      time.Now(),
			})
			return errors.New("unauthorized: cannot link to entity owned by different user")
		}
		return nil
	}

	// Entity not found - log the attempt but allow file upload (will be orphaned)
	v.LogUnauthorizedAttempt(ctx, UnauthorizedAttempt{
		UserID:         userID,
		TargetEntityID: entityID,
		EntityType:     "unknown",
		Operation:      "restore_link_files",
		AttemptType:    "non_existent_entity_access",
		Timestamp:      time.Now(),
	})
	return errors.New("entity not found: file will be uploaded as orphaned")
}

// ValidateRelationshipIntegrity validates that file types can be linked to appropriate entity types
func (v *RestoreSecurityValidator) ValidateRelationshipIntegrity(ctx context.Context, fileType string, parentEntityType string) error {
	allowedRelationships := map[string][]string{
		"invoice": {"commodity"},
		"image":   {"commodity"},
		"manual":  {"commodity"},
	}

	allowedParents, exists := allowedRelationships[fileType]
	if !exists {
		return fmt.Errorf("unknown file type: %s", fileType)
	}

	for _, allowedParent := range allowedParents {
		if allowedParent == parentEntityType {
			return nil // Valid relationship
		}
	}

	return fmt.Errorf("invalid relationship: %s files cannot be linked to %s entities", fileType, parentEntityType)
}

// ValidateImportScope validates that entities being linked are within the import scope
func (v *RestoreSecurityValidator) ValidateImportScope(ctx context.Context, entityID string, importSession string, sessionEntities map[string]bool) error {
	// Check if entity was created in this import session
	if sessionEntities[entityID] {
		return nil // OK - entity created in this import
	}

	// Check if user already owns this entity
	currentUser := appctx.UserFromContext(ctx)
	if currentUser == nil {
		return ErrNoUserContext
	}

	err := v.ValidateEntityOwnership(ctx, entityID, currentUser.ID)
	if err != nil {
		return fmt.Errorf("unauthorized: cannot link to entity outside import scope: %w", err)
	}

	return nil // OK - user owns existing entity
}

// LogUnauthorizedAttempt logs details about unauthorized access attempts
func (v *RestoreSecurityValidator) LogUnauthorizedAttempt(ctx context.Context, attempt UnauthorizedAttempt) {
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
