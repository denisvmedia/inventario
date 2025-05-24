package migrations

import (
	"context"
)

// MemoryMigrator implements the Migrator interface for Memory
type MemoryMigrator struct{}

// NewMemoryMigrator creates a new MemoryMigrator
func NewMemoryMigrator(_ string) (Migrator, error) {
	return &MemoryMigrator{}, nil
}

// RunMigrations is a no-op for Memory as it doesn't need migrations
func (m *MemoryMigrator) RunMigrations(_ context.Context) error {
	// No-op for memory database
	return nil
}

// CheckMigrationsApplied always returns true for Memory as it doesn't need migrations
func (m *MemoryMigrator) CheckMigrationsApplied(_ context.Context) (bool, error) {
	// Always return true for memory database
	return true, nil
}
