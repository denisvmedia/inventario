package services

import (
	"context"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ThumbnailConcurrencyService manages per-user concurrency limits for thumbnail generation
type ThumbnailConcurrencyService struct {
	factorySet      *registry.FactorySet
	maxSlotsPerUser int
	slotDuration    time.Duration
}

// NewThumbnailConcurrencyService creates a new thumbnail concurrency service
func NewThumbnailConcurrencyService(factorySet *registry.FactorySet, maxSlotsPerUser int, slotDuration time.Duration) *ThumbnailConcurrencyService {
	return &ThumbnailConcurrencyService{
		factorySet:      factorySet,
		maxSlotsPerUser: maxSlotsPerUser,
		slotDuration:    slotDuration,
	}
}

// AcquireSlot attempts to acquire a concurrency slot for a user
func (s *ThumbnailConcurrencyService) AcquireSlot(ctx context.Context, userID, jobID string) (*models.UserConcurrencySlot, error) {
	slotRegistry := s.factorySet.UserConcurrencySlotRegistryFactory.CreateServiceRegistry()

	slot, err := slotRegistry.AcquireSlot(ctx, userID, jobID, s.maxSlotsPerUser, s.slotDuration)
	if err != nil {
		return nil, errxtrace.Wrap("failed to acquire concurrency slot", err)
	}

	return slot, nil
}

// ReleaseSlot releases a concurrency slot for a user
func (s *ThumbnailConcurrencyService) ReleaseSlot(ctx context.Context, userID, jobID string) error {
	slotRegistry := s.factorySet.UserConcurrencySlotRegistryFactory.CreateServiceRegistry()

	err := slotRegistry.ReleaseSlot(ctx, userID, jobID)
	if err != nil {
		return errxtrace.Wrap("failed to release concurrency slot", err)
	}

	return nil
}

// GetUserSlots returns all active slots for a user
func (s *ThumbnailConcurrencyService) GetUserSlots(ctx context.Context, userID string) ([]*models.UserConcurrencySlot, error) {
	slotRegistry := s.factorySet.UserConcurrencySlotRegistryFactory.CreateServiceRegistry()

	slots, err := slotRegistry.GetUserSlots(ctx, userID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user slots", err)
	}

	return slots, nil
}

// CleanupExpiredSlots removes expired slots
func (s *ThumbnailConcurrencyService) CleanupExpiredSlots(ctx context.Context) error {
	slotRegistry := s.factorySet.UserConcurrencySlotRegistryFactory.CreateServiceRegistry()

	err := slotRegistry.CleanupExpiredSlots(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to cleanup expired slots", err)
	}

	return nil
}
