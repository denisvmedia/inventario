package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"
)

// OperationSlot represents a resource allocation slot for a specific operation
//
//migrator:schema:table name="operation_slots"
type OperationSlot struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID

	// SlotID is the numeric identifier for this slot within the user/operation scope
	//migrator:schema:field name="slot_id" type="INTEGER" not_null="true"
	SlotID int `json:"slot_id" db:"slot_id"`

	// OperationName identifies the type of operation this slot is allocated for
	//migrator:schema:field name="operation_name" type="TEXT" not_null="true" default="upload"
	OperationName string `json:"operation_name" db:"operation_name"`

	// CreatedAt is when the slot was allocated
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true"
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// ExpiresAt is when the slot will automatically be released
	//migrator:schema:field name="expires_at" type="TIMESTAMP" not_null="true"
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
}

// PostgreSQL-specific indexes for operation slots
type OperationSlotIndexes struct {
	// Primary lookup index for user/operation queries
	//migrator:schema:index name="idx_operation_slots_user_operation" fields="tenant_id,user_id,operation_name,expires_at" table="operation_slots"
	_ int

	// Cleanup index for expired slot removal
	//migrator:schema:index name="idx_operation_slots_cleanup" fields="expires_at" table="operation_slots"
	_ int

	// Unique constraint per user/operation/slot
	//migrator:schema:index name="idx_operation_slots_unique" fields="tenant_id,user_id,operation_name,slot_id" table="operation_slots" unique="true"
	_ int

	// Operation-specific queries
	//migrator:schema:index name="idx_operation_slots_operation" fields="operation_name,expires_at" table="operation_slots"
	_ int
}

// Validate validates the operation slot using the legacy validation interface
func (*OperationSlot) Validate() error {
	return ErrMustUseValidateWithContext
}

// ValidateWithContext validates the operation slot with context
func (o *OperationSlot) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&o.SlotID, validation.Required, validation.Min(1)),
		validation.Field(&o.OperationName, validation.Required, validation.Length(1, 100)),
		validation.Field(&o.CreatedAt, validation.Required),
		validation.Field(&o.ExpiresAt, validation.Required),
	)

	// Ensure expires_at is after created_at
	if !o.CreatedAt.IsZero() && !o.ExpiresAt.IsZero() {
		fields = append(fields,
			validation.Field(&o.ExpiresAt, validation.By(func(value any) error {
				if expiresAt, ok := value.(time.Time); ok {
					if expiresAt.Before(o.CreatedAt) || expiresAt.Equal(o.CreatedAt) {
						return validation.NewError("expires_at_invalid", "expires_at must be after created_at")
					}
				}
				return nil
			})),
		)
	}

	return validation.ValidateStructWithContext(ctx, o, fields...)
}

// IsExpired returns true if the slot has expired
func (o *OperationSlot) IsExpired() bool {
	return time.Now().After(o.ExpiresAt)
}

// TimeUntilExpiry returns the duration until the slot expires
func (o *OperationSlot) TimeUntilExpiry() time.Duration {
	if o.IsExpired() {
		return 0
	}
	return time.Until(o.ExpiresAt)
}

// UploadStatus represents the current upload status for a user and operation
type UploadStatus struct {
	OperationName     string `json:"operation_name"`
	ActiveUploads     int    `json:"active_uploads"`
	MaxUploads        int    `json:"max_uploads"`
	AvailableUploads  int    `json:"available_uploads"`
	CanStartUpload    bool   `json:"can_start_upload"`
	RetryAfterSeconds *int   `json:"retry_after_seconds,omitempty"`
}

// OperationSlotConfig contains configuration for a specific operation type
type OperationSlotConfig struct {
	MaxSlotsPerUser int           `json:"max_slots_per_user"`
	SlotTimeout     time.Duration `json:"slot_timeout"`
	RetryInterval   time.Duration `json:"retry_interval"`
}

// OperationStats provides statistics about slot usage for an operation
type OperationStats struct {
	OperationName  string  `json:"operation_name"`
	ActiveSlots    int     `json:"active_slots"`
	TotalUsers     int     `json:"total_users"`
	MaxSlots       int     `json:"max_slots"`
	AvgUtilization float64 `json:"avg_utilization"`
}

// SlotManagerConfig contains the complete configuration for the slot manager
type SlotManagerConfig struct {
	Operations map[string]OperationSlotConfig `json:"operations"`

	// Global defaults
	DefaultMaxSlotsPerUser int           `env:"DEFAULT_MAX_SLOTS_PER_USER" default:"3"`
	DefaultSlotTimeout     time.Duration `env:"DEFAULT_SLOT_TIMEOUT" default:"30s"`
	DefaultRetryInterval   time.Duration `env:"DEFAULT_SLOT_RETRY_INTERVAL" default:"3s"`
	CleanupInterval        time.Duration `env:"SLOT_CLEANUP_INTERVAL" default:"60s"`
}

// LoadSlotManagerConfigFromEnv loads slot manager configuration from environment variables
func LoadSlotManagerConfigFromEnv() SlotManagerConfig {
	config := SlotManagerConfig{
		Operations:             make(map[string]OperationSlotConfig),
		DefaultMaxSlotsPerUser: 3,
		DefaultSlotTimeout:     30 * time.Second,
		DefaultRetryInterval:   3 * time.Second,
		CleanupInterval:        60 * time.Second,
	}

	// Define default operation configurations
	config.Operations["image_upload"] = OperationSlotConfig{
		MaxSlotsPerUser: 5,
		SlotTimeout:     60 * time.Second,
		RetryInterval:   5 * time.Second,
	}

	config.Operations["document_upload"] = OperationSlotConfig{
		MaxSlotsPerUser: 3,
		SlotTimeout:     120 * time.Second,
		RetryInterval:   10 * time.Second,
	}

	config.Operations["file_upload"] = OperationSlotConfig{
		MaxSlotsPerUser: 3,
		SlotTimeout:     120 * time.Second,
		RetryInterval:   10 * time.Second,
	}

	// TODO: Load from environment variables using a config library
	// For now, return defaults
	return config
}

// GetOperationConfig returns the configuration for a specific operation
func (c *SlotManagerConfig) GetOperationConfig(operationName string) OperationSlotConfig {
	if config, exists := c.Operations[operationName]; exists {
		return config
	}

	// Return default configuration if operation not found
	return OperationSlotConfig{
		MaxSlotsPerUser: c.DefaultMaxSlotsPerUser,
		SlotTimeout:     c.DefaultSlotTimeout,
		RetryInterval:   c.DefaultRetryInterval,
	}
}

// DefaultSlotManagerConfig returns a configuration with sensible defaults
func DefaultSlotManagerConfig() SlotManagerConfig {
	return SlotManagerConfig{
		Operations: map[string]OperationSlotConfig{
			"image_upload": {
				MaxSlotsPerUser: 5,
				SlotTimeout:     30 * time.Second,
				RetryInterval:   3 * time.Second,
			},
			"document_upload": {
				MaxSlotsPerUser: 3,
				SlotTimeout:     60 * time.Second,
				RetryInterval:   5 * time.Second,
			},
			"file_upload": {
				MaxSlotsPerUser: 3,
				SlotTimeout:     30 * time.Second,
				RetryInterval:   3 * time.Second,
			},
			"export_generation": {
				MaxSlotsPerUser: 1,
				SlotTimeout:     300 * time.Second, // 5 minutes
				RetryInterval:   10 * time.Second,
			},
			"thumbnail_generation": {
				MaxSlotsPerUser: 10,
				SlotTimeout:     120 * time.Second, // 2 minutes
				RetryInterval:   2 * time.Second,
			},
		},
		DefaultMaxSlotsPerUser: 3,
		DefaultSlotTimeout:     30 * time.Second,
		DefaultRetryInterval:   3 * time.Second,
		CleanupInterval:        60 * time.Second,
	}
}
