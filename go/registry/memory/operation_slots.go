package memory

import (
	"context"
	"sync"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// OperationSlotRegistry implements registry.OperationSlotRegistry for in-memory storage
type OperationSlotRegistry struct {
	mu        sync.RWMutex
	slots     map[string]*models.OperationSlot     // id -> slot
	userSlots map[string]map[string]map[int]string // userID -> operationName -> slotID -> id
	userID    string
	tenantID  string
	service   bool
}

// NewOperationSlotRegistry creates a new memory-based operation slot registry
func NewOperationSlotRegistry(service bool, userID, tenantID string) *OperationSlotRegistry {
	return &OperationSlotRegistry{
		slots:     make(map[string]*models.OperationSlot),
		userSlots: make(map[string]map[string]map[int]string),
		userID:    userID,
		tenantID:  tenantID,
		service:   service,
	}
}

// Create creates a new operation slot
func (r *OperationSlotRegistry) Create(ctx context.Context, slot models.OperationSlot) (*models.OperationSlot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Set user/tenant context if not service registry
	if !r.service {
		slot.UserID = r.userID
		slot.TenantID = r.tenantID
	}

	// Generate ID if not set
	if slot.ID == "" {
		slot.ID = uuid.New().String()
	}

	// Validate the slot
	if err := slot.ValidateWithContext(ctx); err != nil {
		return nil, errxtrace.Wrap("invalid operation slot", err)
	}

	// Initialize nested maps if needed
	if r.userSlots[slot.UserID] == nil {
		r.userSlots[slot.UserID] = make(map[string]map[int]string)
	}
	if r.userSlots[slot.UserID][slot.OperationName] == nil {
		r.userSlots[slot.UserID][slot.OperationName] = make(map[int]string)
	}

	// Check for duplicate slot ID
	if existingID, exists := r.userSlots[slot.UserID][slot.OperationName][slot.SlotID]; exists {
		if existingSlot, ok := r.slots[existingID]; ok && !existingSlot.IsExpired() {
			return nil, errxtrace.Wrap("slot ID already exists for user/operation", registry.ErrAlreadyExists)
		}
		// Clean up expired slot
		delete(r.slots, existingID)
	}

	// Store the slot
	r.slots[slot.ID] = &slot
	r.userSlots[slot.UserID][slot.OperationName][slot.SlotID] = slot.ID

	return &slot, nil
}

// Get retrieves an operation slot by ID
func (r *OperationSlotRegistry) Get(ctx context.Context, id string) (*models.OperationSlot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	slot, exists := r.slots[id]
	if !exists {
		return nil, errxtrace.Wrap("operation slot not found", registry.ErrNotFound)
	}

	// Filter by user if not service registry
	if !r.service && slot.UserID != r.userID {
		return nil, errxtrace.Wrap("operation slot not found", registry.ErrNotFound)
	}

	return slot, nil
}

// GetSlot retrieves a specific slot for a user and operation
func (r *OperationSlotRegistry) GetSlot(ctx context.Context, userID, operationName string, slotID int) (*models.OperationSlot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.userSlots[userID] == nil ||
		r.userSlots[userID][operationName] == nil {
		return nil, errxtrace.Wrap("operation slot not found", registry.ErrNotFound)
	}

	slotEntityID, exists := r.userSlots[userID][operationName][slotID]
	if !exists {
		return nil, errxtrace.Wrap("operation slot not found", registry.ErrNotFound)
	}

	slot, exists := r.slots[slotEntityID]
	if !exists {
		return nil, errxtrace.Wrap("operation slot not found", registry.ErrNotFound)
	}

	// Check if expired
	if slot.IsExpired() {
		return nil, errxtrace.Wrap("operation slot expired", registry.ErrNotFound)
	}

	return slot, nil
}

// ReleaseSlot removes a specific slot for a user and operation
func (r *OperationSlotRegistry) ReleaseSlot(ctx context.Context, userID, operationName string, slotID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.userSlots[userID] == nil ||
		r.userSlots[userID][operationName] == nil {
		return errxtrace.Wrap("operation slot not found", registry.ErrNotFound)
	}

	slotEntityID, exists := r.userSlots[userID][operationName][slotID]
	if !exists {
		return errxtrace.Wrap("operation slot not found", registry.ErrNotFound)
	}

	// Remove from both maps
	delete(r.slots, slotEntityID)
	delete(r.userSlots[userID][operationName], slotID)

	return nil
}

// GetActiveSlotCount returns the number of active (non-expired) slots for a user and operation
func (r *OperationSlotRegistry) GetActiveSlotCount(ctx context.Context, userID, operationName string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.userSlots[userID] == nil ||
		r.userSlots[userID][operationName] == nil {
		return 0, nil
	}

	count := 0
	now := time.Now()

	for _, slotEntityID := range r.userSlots[userID][operationName] {
		if slot, exists := r.slots[slotEntityID]; exists && now.Before(slot.ExpiresAt) {
			count++
		}
	}

	return count, nil
}

// GetNextSlotID returns the next available slot ID for a user and operation
func (r *OperationSlotRegistry) GetNextSlotID(ctx context.Context, userID, operationName string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.userSlots[userID] == nil ||
		r.userSlots[userID][operationName] == nil {
		return 1, nil
	}

	maxSlotID := 0
	for slotID := range r.userSlots[userID][operationName] {
		if slotID > maxSlotID {
			maxSlotID = slotID
		}
	}

	return maxSlotID + 1, nil
}

// CleanupExpiredSlots removes all expired slots and returns the count of deleted slots
func (r *OperationSlotRegistry) CleanupExpiredSlots(ctx context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	deletedCount := 0
	now := time.Now()

	// Collect expired slot IDs
	var expiredIDs []string
	for id, slot := range r.slots {
		if now.After(slot.ExpiresAt) {
			expiredIDs = append(expiredIDs, id)
		}
	}

	// Remove expired slots
	for _, id := range expiredIDs {
		slot := r.slots[id]
		delete(r.slots, id)

		// Remove from user slots map
		if r.userSlots[slot.UserID] != nil &&
			r.userSlots[slot.UserID][slot.OperationName] != nil {
			delete(r.userSlots[slot.UserID][slot.OperationName], slot.SlotID)
		}

		deletedCount++
	}

	return deletedCount, nil
}

// GetOperationStats returns statistics about slot usage across all operations
func (r *OperationSlotRegistry) GetOperationStats(ctx context.Context) (map[string]models.OperationStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]models.OperationStats)
	operationCounts := make(map[string]int)
	operationUsers := make(map[string]map[string]bool)
	now := time.Now()

	// Count active slots per operation
	for _, slot := range r.slots {
		if now.Before(slot.ExpiresAt) {
			if operationUsers[slot.OperationName] == nil {
				operationUsers[slot.OperationName] = make(map[string]bool)
			}
			operationUsers[slot.OperationName][slot.UserID] = true
			operationCounts[slot.OperationName]++
		}
	}

	// Build statistics
	for operationName, activeSlots := range operationCounts {
		totalUsers := len(operationUsers[operationName])

		stats[operationName] = models.OperationStats{
			OperationName:  operationName,
			ActiveSlots:    activeSlots,
			TotalUsers:     totalUsers,
			MaxSlots:       0, // Will be set by service layer
			AvgUtilization: 0, // Will be calculated by service layer
		}
	}

	return stats, nil
}

// GetUserSlotStats returns slot usage statistics for a specific user
func (r *OperationSlotRegistry) GetUserSlotStats(ctx context.Context, userID string) (map[string]int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]int)
	now := time.Now()

	if r.userSlots[userID] != nil {
		for operationName, slots := range r.userSlots[userID] {
			count := 0
			for _, slotEntityID := range slots {
				if slot, exists := r.slots[slotEntityID]; exists && now.Before(slot.ExpiresAt) {
					count++
				}
			}
			if count > 0 {
				stats[operationName] = count
			}
		}
	}

	return stats, nil
}

// GetExpiredSlots returns all expired slots (for testing/debugging)
func (r *OperationSlotRegistry) GetExpiredSlots(ctx context.Context) ([]models.OperationSlot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var expiredSlots []models.OperationSlot
	now := time.Now()

	for _, slot := range r.slots {
		if now.After(slot.ExpiresAt) {
			expiredSlots = append(expiredSlots, *slot)
		}
	}

	return expiredSlots, nil
}

// List returns all operation slots
func (r *OperationSlotRegistry) List(ctx context.Context) ([]*models.OperationSlot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var slots []*models.OperationSlot
	for _, slot := range r.slots {
		// Filter by user if not service registry
		if !r.service && slot.UserID != r.userID {
			continue
		}
		slots = append(slots, slot)
	}

	return slots, nil
}

// Update updates an operation slot
func (r *OperationSlotRegistry) Update(ctx context.Context, slot models.OperationSlot) (*models.OperationSlot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.slots[slot.ID]; !exists {
		return nil, errxtrace.Wrap("operation slot not found", registry.ErrNotFound)
	}

	// Set user/tenant context if not service registry
	if !r.service {
		slot.UserID = r.userID
		slot.TenantID = r.tenantID
	}

	// Validate the slot
	if err := slot.ValidateWithContext(ctx); err != nil {
		return nil, errxtrace.Wrap("invalid operation slot", err)
	}

	r.slots[slot.ID] = &slot
	return &slot, nil
}

// Delete deletes an operation slot
func (r *OperationSlotRegistry) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	slot, exists := r.slots[id]
	if !exists {
		return errxtrace.Wrap("operation slot not found", registry.ErrNotFound)
	}

	// Filter by user if not service registry
	if !r.service && slot.UserID != r.userID {
		return errxtrace.Wrap("operation slot not found", registry.ErrNotFound)
	}

	// Remove from user slots mapping
	if r.userSlots[slot.UserID] != nil &&
		r.userSlots[slot.UserID][slot.OperationName] != nil {
		delete(r.userSlots[slot.UserID][slot.OperationName], slot.SlotID)
	}

	delete(r.slots, id)
	return nil
}

// Count returns the number of operation slots
func (r *OperationSlotRegistry) Count(ctx context.Context) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.service {
		return len(r.slots), nil
	}

	// Count only user's slots
	count := 0
	for _, slot := range r.slots {
		if slot.UserID == r.userID {
			count++
		}
	}

	return count, nil
}

// OperationSlotRegistryFactory implements registry.OperationSlotRegistryFactory for memory
type OperationSlotRegistryFactory struct {
	registry *OperationSlotRegistry
}

// NewOperationSlotRegistryFactory creates a new memory operation slot registry factory
func NewOperationSlotRegistryFactory() *OperationSlotRegistryFactory {
	return &OperationSlotRegistryFactory{
		registry: NewOperationSlotRegistry(false, "", ""),
	}
}

// CreateUserRegistry creates a new registry with user context from the provided context
func (f *OperationSlotRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.OperationSlotRegistry, error) {
	user := appctx.UserFromContext(ctx)
	if user == nil {
		return nil, errxtrace.Wrap("user context required", registry.ErrInvalidInput)
	}

	// For memory implementation, we use the same registry instance but filter by user context
	return f.registry, nil
}

// MustCreateUserRegistry creates a new registry with user context, panics on error
func (f *OperationSlotRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.OperationSlotRegistry {
	reg, err := f.CreateUserRegistry(ctx)
	if err != nil {
		panic(err)
	}
	return reg
}

// CreateServiceRegistry creates a new registry with service account context
func (f *OperationSlotRegistryFactory) CreateServiceRegistry() registry.OperationSlotRegistry {
	return f.registry
}
