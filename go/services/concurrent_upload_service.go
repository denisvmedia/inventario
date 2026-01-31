package services

import (
	"context"
	"sync"

	"github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ConcurrentUploadService provides simple concurrent upload limiting
type ConcurrentUploadService interface {
	// Check if user can start an upload for the given operation
	CanStartUpload(ctx context.Context, userID string, operationName string) (bool, error)

	// Start an upload (increment counter)
	StartUpload(ctx context.Context, userID string, operationName string) error

	// Finish an upload (decrement counter)
	FinishUpload(ctx context.Context, userID string, operationName string) error

	// Get current upload status
	GetUploadStatus(ctx context.Context, userID string, operationName string) (*models.UploadStatus, error)

	// Get operation configuration
	GetOperationConfig(operationName string) models.OperationSlotConfig
}

// MemoryConcurrentUploadService implements ConcurrentUploadService using in-memory storage
type MemoryConcurrentUploadService struct {
	mu           sync.RWMutex
	uploadCounts map[string]map[string]int // userID -> operationName -> count
	config       models.SlotManagerConfig
}

// NewMemoryConcurrentUploadService creates a new memory-based concurrent upload service
func NewMemoryConcurrentUploadService(config models.SlotManagerConfig) *MemoryConcurrentUploadService {
	return &MemoryConcurrentUploadService{
		uploadCounts: make(map[string]map[string]int),
		config:       config,
	}
}

// CanStartUpload checks if user can start an upload for the given operation
func (s *MemoryConcurrentUploadService) CanStartUpload(ctx context.Context, userID string, operationName string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	opConfig := s.GetOperationConfig(operationName)
	currentCount := s.getCurrentCount(userID, operationName)

	return currentCount < opConfig.MaxSlotsPerUser, nil
}

// StartUpload increments the upload counter for the user and operation
func (s *MemoryConcurrentUploadService) StartUpload(ctx context.Context, userID string, operationName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	opConfig := s.GetOperationConfig(operationName)
	currentCount := s.getCurrentCount(userID, operationName)

	if currentCount >= opConfig.MaxSlotsPerUser {
		return stacktrace.Wrap("maximum concurrent uploads reached", registry.ErrTooManyRequests)
	}

	// Initialize maps if needed
	if s.uploadCounts[userID] == nil {
		s.uploadCounts[userID] = make(map[string]int)
	}

	s.uploadCounts[userID][operationName]++
	return nil
}

// FinishUpload decrements the upload counter for the user and operation
func (s *MemoryConcurrentUploadService) FinishUpload(ctx context.Context, userID string, operationName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.uploadCounts[userID] == nil {
		return nil // Nothing to decrement
	}

	if s.uploadCounts[userID][operationName] > 0 {
		s.uploadCounts[userID][operationName]--

		// Clean up empty maps
		if s.uploadCounts[userID][operationName] == 0 {
			delete(s.uploadCounts[userID], operationName)
			if len(s.uploadCounts[userID]) == 0 {
				delete(s.uploadCounts, userID)
			}
		}
	}

	return nil
}

// GetUploadStatus returns the current upload status for a user and operation
func (s *MemoryConcurrentUploadService) GetUploadStatus(ctx context.Context, userID string, operationName string) (*models.UploadStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	opConfig := s.GetOperationConfig(operationName)
	activeUploads := s.getCurrentCount(userID, operationName)
	availableUploads := opConfig.MaxSlotsPerUser - activeUploads
	if availableUploads < 0 {
		availableUploads = 0
	}

	var retryAfter *int
	if activeUploads >= opConfig.MaxSlotsPerUser {
		retrySeconds := int(opConfig.RetryInterval.Seconds())
		retryAfter = &retrySeconds
	}

	return &models.UploadStatus{
		OperationName:     operationName,
		ActiveUploads:     activeUploads,
		MaxUploads:        opConfig.MaxSlotsPerUser,
		AvailableUploads:  availableUploads,
		CanStartUpload:    activeUploads < opConfig.MaxSlotsPerUser,
		RetryAfterSeconds: retryAfter,
	}, nil
}

// GetOperationConfig returns the configuration for a specific operation
func (s *MemoryConcurrentUploadService) GetOperationConfig(operationName string) models.OperationSlotConfig {
	return s.config.GetOperationConfig(operationName)
}

// getCurrentCount returns the current upload count for a user and operation (must be called with lock held)
func (s *MemoryConcurrentUploadService) getCurrentCount(userID string, operationName string) int {
	if s.uploadCounts[userID] == nil {
		return 0
	}
	return s.uploadCounts[userID][operationName]
}

// NewConcurrentUploadService creates a new concurrent upload service
func NewConcurrentUploadService(config models.SlotManagerConfig) ConcurrentUploadService {
	// For now, always use memory implementation
	// In the future, we could add PostgreSQL implementation if needed
	return NewMemoryConcurrentUploadService(config)
}
