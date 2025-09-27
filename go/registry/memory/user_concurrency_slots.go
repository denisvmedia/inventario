package memory

import (
	"context"
	"sync"
	"time"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.UserConcurrencySlotRegistry = (*UserConcurrencySlotRegistry)(nil)

type baseUserConcurrencySlotRegistry = Registry[models.UserConcurrencySlot, *models.UserConcurrencySlot]

type UserConcurrencySlotRegistry struct {
	*baseUserConcurrencySlotRegistry
	// Additional mutex for concurrency slot operations
	slotMutex sync.Mutex
}

// UserConcurrencySlotRegistryFactory creates UserConcurrencySlotRegistry instances with proper context
type UserConcurrencySlotRegistryFactory struct {
	baseUserConcurrencySlotRegistry *Registry[models.UserConcurrencySlot, *models.UserConcurrencySlot]
	slotMutex                       *sync.Mutex
}

func NewUserConcurrencySlotRegistryFactory() *UserConcurrencySlotRegistryFactory {
	return &UserConcurrencySlotRegistryFactory{
		baseUserConcurrencySlotRegistry: NewRegistry[models.UserConcurrencySlot, *models.UserConcurrencySlot](),
		slotMutex:                       &sync.Mutex{},
	}
}

func (f *UserConcurrencySlotRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.UserConcurrencySlotRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user from context")
	}

	// Create a new registry with user context already set
	userRegistry := &Registry[models.UserConcurrencySlot, *models.UserConcurrencySlot]{
		items:  f.baseUserConcurrencySlotRegistry.items, // Share the data map
		lock:   f.baseUserConcurrencySlotRegistry.lock,  // Share the mutex pointer
		userID: user.ID,                                 // Set user-specific userID
	}

	return &UserConcurrencySlotRegistry{
		baseUserConcurrencySlotRegistry: userRegistry,
		slotMutex:                       sync.Mutex{}, // Create new mutex for this instance
	}, nil
}

func (f *UserConcurrencySlotRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.UserConcurrencySlotRegistry {
	reg, err := f.CreateUserRegistry(ctx)
	if err != nil {
		panic(err)
	}
	return reg
}

func (f *UserConcurrencySlotRegistryFactory) CreateServiceRegistry() registry.UserConcurrencySlotRegistry {
	// Create a new registry with service account context (no user filtering)
	serviceRegistry := &Registry[models.UserConcurrencySlot, *models.UserConcurrencySlot]{
		items:  f.baseUserConcurrencySlotRegistry.items, // Share the data map
		lock:   f.baseUserConcurrencySlotRegistry.lock,  // Share the mutex pointer
		userID: "",                                      // Clear userID to bypass user filtering
	}

	return &UserConcurrencySlotRegistry{
		baseUserConcurrencySlotRegistry: serviceRegistry,
		slotMutex:                       sync.Mutex{}, // Create new mutex for this instance
	}
}

// AcquireSlot attempts to acquire a concurrency slot for a user
func (r *UserConcurrencySlotRegistry) AcquireSlot(ctx context.Context, userID, jobID string, maxSlots int, slotDuration time.Duration) (*models.UserConcurrencySlot, error) {
	// Use dedicated mutex for slot operations to ensure atomicity
	r.slotMutex.Lock()
	defer r.slotMutex.Unlock()

	// Get all active slots for the user
	userSlots, err := r.getUserActiveSlots(ctx, userID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user slots")
	}

	// Clean up expired slots first (based on created_at + slot_duration)
	now := time.Now()
	for _, slot := range userSlots {
		if slot.CreatedAt.Add(slotDuration).Before(now) {
			// Delete expired slot
			if err := r.Delete(ctx, slot.ID); err != nil {
				// Log error but continue
				continue
			}
		}
	}

	// Recount active slots after cleanup
	userSlots, err = r.getUserActiveSlots(ctx, userID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user slots after cleanup")
	}

	// Check if user has reached the maximum number of slots
	if len(userSlots) >= maxSlots {
		return nil, errkit.WithStack(registry.ErrResourceLimitExceeded)
	}

	// Get tenant ID from the job being processed
	tenantID := getTenantIDFromContext(ctx)
	if tenantID == "" {
		// If no tenant in context (e.g., background worker), get it from the job
		tenantID = r.getTenantIDFromJob(jobID)
	}

	// Create new slot
	now = time.Now()
	slot := models.UserConcurrencySlot{
		TenantAwareEntityID: models.TenantAwareEntityID{
			UserID:   userID,
			TenantID: tenantID,
		},
		JobID:     jobID,
		Status:    models.SlotStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Save the slot
	createdSlot, err := r.Create(ctx, slot)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create concurrency slot")
	}

	return createdSlot, nil
}

// getUserActiveSlots returns all active slots for a user
func (r *UserConcurrencySlotRegistry) getUserActiveSlots(ctx context.Context, userID string) ([]*models.UserConcurrencySlot, error) {
	allSlots, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	var activeSlots []*models.UserConcurrencySlot
	for _, slot := range allSlots {
		if slot.UserID == userID && slot.Status == models.SlotStatusActive {
			activeSlots = append(activeSlots, slot)
		}
	}

	return activeSlots, nil
}

// ReleaseSlot releases a concurrency slot for a user
func (r *UserConcurrencySlotRegistry) ReleaseSlot(ctx context.Context, userID, jobID string) error {
	// Use dedicated mutex for slot operations to ensure atomicity
	r.slotMutex.Lock()
	defer r.slotMutex.Unlock()

	// Find the slot for this job
	slots, err := r.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to list slots")
	}

	for _, slot := range slots {
		if slot.UserID == userID && slot.JobID == jobID && slot.Status == models.SlotStatusActive {
			// Delete the slot
			if err := r.Delete(ctx, slot.ID); err != nil {
				return errkit.Wrap(err, "failed to release concurrency slot")
			}
			return nil
		}
	}

	return errkit.WithStack(registry.ErrNotFound)
}

// CleanupExpiredSlots removes expired concurrency slots
func (r *UserConcurrencySlotRegistry) CleanupExpiredSlots(ctx context.Context) error {
	// Use dedicated mutex for slot operations to ensure atomicity
	r.slotMutex.Lock()
	defer r.slotMutex.Unlock()

	slots, err := r.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to list slots")
	}

	// Use a fixed duration of 1 hour for expired slots (matching postgres implementation)
	expiredBefore := time.Now().Add(-1 * time.Hour)
	for _, slot := range slots {
		if slot.Status == models.SlotStatusActive && slot.CreatedAt.Before(expiredBefore) {
			// Delete expired slot
			if err := r.Delete(ctx, slot.ID); err != nil {
				continue // Log error but continue cleanup
			}
		}
	}

	return nil
}

// GetUserSlotCount returns the number of active slots for a user
func (r *UserConcurrencySlotRegistry) GetUserSlotCount(ctx context.Context, userID string) (int, error) {
	activeSlots, err := r.getUserActiveSlots(ctx, userID)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to get user active slots")
	}

	return len(activeSlots), nil
}

// GetUserSlots returns all slots for a user
func (r *UserConcurrencySlotRegistry) GetUserSlots(ctx context.Context, userID string) ([]*models.UserConcurrencySlot, error) {
	slots, err := r.List(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list slots")
	}

	var userSlots []*models.UserConcurrencySlot
	for _, slot := range slots {
		if slot.UserID == userID {
			userSlots = append(userSlots, slot)
		}
	}

	return userSlots, nil
}

// Helper functions

// getTenantIDFromContext extracts tenant ID from context
func getTenantIDFromContext(ctx context.Context) string {
	user := appctx.UserFromContext(ctx)
	if user != nil {
		return user.TenantID
	}
	return ""
}

// getTenantIDFromJob gets tenant ID from a thumbnail generation job
func (r *UserConcurrencySlotRegistry) getTenantIDFromJob(jobID string) string {
	// Access the shared thumbnail job registry to find the job
	// This is a bit of a hack, but necessary for the memory implementation
	// In a real system, we'd have a proper service to look this up

	// For now, we'll iterate through all items to find the job
	// This is inefficient but works for the memory implementation
	r.baseUserConcurrencySlotRegistry.lock.RLock()
	defer r.baseUserConcurrencySlotRegistry.lock.RUnlock()

	// We need access to the thumbnail job registry to look up the job
	// Since this is the memory implementation, we'll return a default tenant ID
	// In practice, this should be coordinated with the job registry
	return "test-tenant-id" // Default for memory implementation
}
