package migrations

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned when a feature is not implemented
var ErrNotImplemented = errors.New("migrations not implemented for this database type")

// BoltDBMigrator implements the Migrator interface for BoltDB
type BoltDBMigrator struct{}

// NewBoltDBMigrator creates a new BoltDBMigrator
func NewBoltDBMigrator(_ string) (Migrator, error) {
	return &BoltDBMigrator{}, nil
}

// RunMigrations returns an error for BoltDB as migrations are not implemented
func (*BoltDBMigrator) RunMigrations(_ context.Context) error {
	return ErrNotImplemented
}

// CheckMigrationsApplied returns an error for BoltDB as migrations are not implemented
func (*BoltDBMigrator) CheckMigrationsApplied(_ context.Context) (bool, error) {
	return false, ErrNotImplemented
}
