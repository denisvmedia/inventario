package migrations_test

import (
	"context"
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/registry/migrations"
)

// TestRegister tests the Register function happy path
func TestRegister(t *testing.T) {
	c := qt.New(t)

	// Save the original migrators to restore after test
	originalMigrators := migrations.Migrators()
	defer func() {
		// Clear current migrators
		for name := range migrations.Migrators() {
			migrations.Unregister(name)
		}
		// Restore original migrators
		for name, fn := range originalMigrators {
			migrations.Register(name, fn)
		}
	}()

	// Clear migrators for clean test
	for name := range migrations.Migrators() {
		migrations.Unregister(name)
	}

	// Create test migrator function
	testMigratorFunc := func(dsn string) (migrations.Migrator, error) {
		return nil, nil
	}

	// Register migrator
	migrations.Register("test", testMigratorFunc)

	// Verify registration
	migrators := migrations.Migrators()
	_, exists := migrators["test"]
	c.Assert(exists, qt.IsTrue)
}

// TestRegisterPanic tests that Register panics on duplicate names
func TestRegisterPanic(t *testing.T) {
	c := qt.New(t)

	// Save the original migrators to restore after test
	originalMigrators := migrations.Migrators()
	defer func() {
		// Clear current migrators
		for name := range migrations.Migrators() {
			migrations.Unregister(name)
		}
		// Restore original migrators
		for name, fn := range originalMigrators {
			migrations.Register(name, fn)
		}
	}()

	// Clear migrators for clean test
	for name := range migrations.Migrators() {
		migrations.Unregister(name)
	}

	// Create test migrator function
	testMigratorFunc := func(dsn string) (migrations.Migrator, error) {
		return nil, nil
	}

	// Register first time
	migrations.Register("test", testMigratorFunc)

	// Register duplicate should panic
	c.Assert(func() { migrations.Register("test", testMigratorFunc) }, qt.PanicMatches, ".*duplicate.*")
}

// TestUnregister tests the Unregister function
func TestUnregister(t *testing.T) {
	tests := []struct {
		name           string
		registerFirst  bool
		unregisterName string
		shouldExist    bool
	}{
		{
			name:           "unregister existing migrator",
			registerFirst:  true,
			unregisterName: "test",
			shouldExist:    false,
		},
		{
			name:           "unregister non-existent migrator",
			registerFirst:  false,
			unregisterName: "nonexistent",
			shouldExist:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Save the original migrators to restore after test
			originalMigrators := migrations.Migrators()
			defer func() {
				// Clear current migrators
				for name := range migrations.Migrators() {
					migrations.Unregister(name)
				}
				// Restore original migrators
				for name, fn := range originalMigrators {
					migrations.Register(name, fn)
				}
			}()

			// Clear migrators for clean test
			for name := range migrations.Migrators() {
				migrations.Unregister(name)
			}

			// Register if needed
			if tt.registerFirst {
				testMigratorFunc := func(dsn string) (migrations.Migrator, error) {
					return nil, nil
				}
				migrations.Register("test", testMigratorFunc)
			}

			// Unregister
			migrations.Unregister(tt.unregisterName)

			// Verify result
			migrators := migrations.Migrators()
			_, exists := migrators[tt.unregisterName]
			c.Assert(exists, qt.Equals, tt.shouldExist)
		})
	}
}

// TestMigrators tests the Migrators function
func TestMigrators(t *testing.T) {
	c := qt.New(t)

	// Save the original migrators to restore after test
	originalMigrators := migrations.Migrators()
	defer func() {
		// Clear current migrators
		for name := range migrations.Migrators() {
			migrations.Unregister(name)
		}
		// Restore original migrators
		for name, fn := range originalMigrators {
			migrations.Register(name, fn)
		}
	}()

	// Clear migrators for clean test
	for name := range migrations.Migrators() {
		migrations.Unregister(name)
	}

	// Create test migrator functions
	testMigratorFunc1 := func(dsn string) (migrations.Migrator, error) {
		return nil, nil
	}
	testMigratorFunc2 := func(dsn string) (migrations.Migrator, error) {
		return nil, nil
	}

	// Register test migrators
	migrations.Register("test1", testMigratorFunc1)
	migrations.Register("test2", testMigratorFunc2)

	// Test Migrators
	m := migrations.Migrators()
	c.Assert(len(m), qt.Equals, 2)

	_, exists1 := m["test1"]
	c.Assert(exists1, qt.IsTrue)

	_, exists2 := m["test2"]
	c.Assert(exists2, qt.IsTrue)
}

// TestMigratorNames tests the MigratorNames function
func TestMigratorNames(t *testing.T) {
	c := qt.New(t)

	// Save the original migrators to restore after test
	originalMigrators := migrations.Migrators()
	defer func() {
		// Clear current migrators
		for name := range migrations.Migrators() {
			migrations.Unregister(name)
		}
		// Restore original migrators
		for name, fn := range originalMigrators {
			migrations.Register(name, fn)
		}
	}()

	// Clear migrators for clean test
	for name := range migrations.Migrators() {
		migrations.Unregister(name)
	}

	// Create test migrator functions
	testMigratorFunc1 := func(dsn string) (migrations.Migrator, error) {
		return nil, nil
	}
	testMigratorFunc2 := func(dsn string) (migrations.Migrator, error) {
		return nil, nil
	}

	// Register test migrators
	migrations.Register("test1", testMigratorFunc1)
	migrations.Register("test2", testMigratorFunc2)

	// Test MigratorNames
	names := migrations.MigratorNames()
	c.Assert(len(names), qt.Equals, 2)
	c.Assert(names, qt.Contains, "test1")
	c.Assert(names, qt.Contains, "test2")
}

// TestGetMigratorHappyPath tests the GetMigrator function happy path
func TestGetMigratorHappyPath(t *testing.T) {
	c := qt.New(t)

	// Save the original migrators to restore after test
	originalMigrators := migrations.Migrators()
	defer func() {
		// Clear current migrators
		for name := range migrations.Migrators() {
			migrations.Unregister(name)
		}
		// Restore original migrators
		for name, fn := range originalMigrators {
			migrations.Register(name, fn)
		}
	}()

	// Clear migrators for clean test
	for name := range migrations.Migrators() {
		migrations.Unregister(name)
	}

	// Create a test migrator function
	testMigratorFunc := func(dsn string) (migrations.Migrator, error) {
		return nil, nil
	}

	// Register a test migrator
	migrations.Register("test", testMigratorFunc)

	// Test GetMigrator with valid DSN
	fn, ok := migrations.GetMigrator("test://localhost")
	c.Assert(ok, qt.IsTrue)
	c.Assert(fn, qt.IsNotNil)
}

// TestGetMigratorUnhappyPath tests the GetMigrator function error cases
func TestGetMigratorUnhappyPath(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
	}{
		{
			name: "invalid scheme",
			dsn:  "invalid://localhost",
		},
		{
			name: "invalid DSN format",
			dsn:  ":",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Save the original migrators to restore after test
			originalMigrators := migrations.Migrators()
			defer func() {
				// Clear current migrators
				for name := range migrations.Migrators() {
					migrations.Unregister(name)
				}
				// Restore original migrators
				for name, fn := range originalMigrators {
					migrations.Register(name, fn)
				}
			}()

			// Clear migrators for clean test
			for name := range migrations.Migrators() {
				migrations.Unregister(name)
			}

			// Test GetMigrator with invalid input
			fn, ok := migrations.GetMigrator(tt.dsn)
			c.Assert(ok, qt.IsFalse)
			c.Assert(fn, qt.IsNil)
		})
	}
}

// mockMigrator is a mock implementation of the Migrator interface for testing
type mockMigrator struct {
	runMigrationsFunc          func(ctx context.Context) error
	checkMigrationsAppliedFunc func(ctx context.Context) (bool, error)
}

func (m *mockMigrator) RunMigrations(ctx context.Context) error {
	return m.runMigrationsFunc(ctx)
}

func (m *mockMigrator) CheckMigrationsApplied(ctx context.Context) (bool, error) {
	return m.checkMigrationsAppliedFunc(ctx)
}

// TestRunMigrationsHappyPath tests the RunMigrations function happy path
func TestRunMigrationsHappyPath(t *testing.T) {
	c := qt.New(t)

	// Save the original migrators to restore after test
	originalMigrators := migrations.Migrators()
	defer func() {
		// Clear current migrators
		for name := range migrations.Migrators() {
			migrations.Unregister(name)
		}
		// Restore original migrators
		for name, fn := range originalMigrators {
			migrations.Register(name, fn)
		}
	}()

	// Clear migrators for clean test
	for name := range migrations.Migrators() {
		migrations.Unregister(name)
	}

	// Register successful migrator
	migrations.Register("success", func(dsn string) (migrations.Migrator, error) {
		return &mockMigrator{
			runMigrationsFunc: func(ctx context.Context) error {
				return nil
			},
			checkMigrationsAppliedFunc: func(ctx context.Context) (bool, error) {
				return true, nil
			},
		}, nil
	})

	// Test successful migration
	err := migrations.RunMigrations(context.Background(), "success://localhost")
	c.Assert(err, qt.IsNil)
}

// TestRunMigrationsUnhappyPath tests the RunMigrations function error cases
func TestRunMigrationsUnhappyPath(t *testing.T) {
	tests := []struct {
		name        string
		dsn         string
		setupFunc   func()
		expectError bool
	}{
		{
			name:        "unknown database type",
			dsn:         "unknown://localhost",
			setupFunc:   func() {},
			expectError: true,
		},
		{
			name: "migrator creation error",
			dsn:  "error://localhost",
			setupFunc: func() {
				migrations.Register("error", func(dsn string) (migrations.Migrator, error) {
					return nil, errors.New("migrator creation error")
				})
			},
			expectError: true,
		},
		{
			name: "migration execution error",
			dsn:  "test://localhost",
			setupFunc: func() {
				migrations.Register("test", func(dsn string) (migrations.Migrator, error) {
					return &mockMigrator{
						runMigrationsFunc: func(ctx context.Context) error {
							return errors.New("migration error")
						},
						checkMigrationsAppliedFunc: func(ctx context.Context) (bool, error) {
							return false, nil
						},
					}, nil
				})
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Save the original migrators to restore after test
			originalMigrators := migrations.Migrators()
			defer func() {
				// Clear current migrators
				for name := range migrations.Migrators() {
					migrations.Unregister(name)
				}
				// Restore original migrators
				for name, fn := range originalMigrators {
					migrations.Register(name, fn)
				}
			}()

			// Clear migrators for clean test
			for name := range migrations.Migrators() {
				migrations.Unregister(name)
			}

			// Setup test case
			tt.setupFunc()

			// Test RunMigrations
			err := migrations.RunMigrations(context.Background(), tt.dsn)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

// TestMemoryMigrator tests the memory implementation of the Migrator interface
func TestMemoryMigrator(t *testing.T) {
	c := qt.New(t)

	// Create a memory migrator
	migrator, err := migrations.NewMemoryMigrator("memory://localhost")
	c.Assert(err, qt.IsNil)
	c.Assert(migrator, qt.IsNotNil)

	// Test RunMigrations (should be no-op)
	err = migrator.RunMigrations(context.Background())
	c.Assert(err, qt.IsNil)

	// Test CheckMigrationsApplied (should always return true)
	applied, err := migrator.CheckMigrationsApplied(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(applied, qt.IsTrue)
}

// TestBoltDBMigrator tests the BoltDB implementation of the Migrator interface
func TestBoltDBMigrator(t *testing.T) {
	c := qt.New(t)

	// Create a BoltDB migrator
	migrator, err := migrations.NewBoltDBMigrator("boltdb://localhost")
	c.Assert(err, qt.IsNil)
	c.Assert(migrator, qt.IsNotNil)

	// Test RunMigrations (should return ErrNotImplemented)
	err = migrator.RunMigrations(context.Background())
	c.Assert(err, qt.IsNotNil)
	c.Assert(errors.Is(err, migrations.ErrNotImplemented), qt.IsTrue)

	// Test CheckMigrationsApplied (should return ErrNotImplemented)
	applied, err := migrator.CheckMigrationsApplied(context.Background())
	c.Assert(err, qt.IsNotNil)
	c.Assert(errors.Is(err, migrations.ErrNotImplemented), qt.IsTrue)
	c.Assert(applied, qt.IsFalse)
}

// TestRegisterMigrators tests the RegisterMigrators function
func TestRegisterMigrators(t *testing.T) {
	c := qt.New(t)

	// Save the original migrators to restore after test
	originalMigrators := migrations.Migrators()
	defer func() {
		// Clear current migrators
		for name := range migrations.Migrators() {
			migrations.Unregister(name)
		}
		// Restore original migrators
		for name, fn := range originalMigrators {
			migrations.Register(name, fn)
		}
	}()

	// Clear migrators for clean test
	for name := range migrations.Migrators() {
		migrations.Unregister(name)
	}

	// Call RegisterMigrators
	migrations.RegisterMigrators()

	// Check that all expected migrators are registered
	expectedTypes := []string{"memory", "boltdb", "postgresql"}
	migrators := migrations.Migrators()

	for _, dbType := range expectedTypes {
		_, exists := migrators[dbType]
		c.Assert(exists, qt.IsTrue, qt.Commentf("RegisterMigrators did not register %s migrator", dbType))
	}
}

// TestPostgreSQLMigrator tests the PostgreSQL implementation of the Migrator interface
func TestPostgreSQLMigrator(t *testing.T) {
	c := qt.New(t)

	// This test only verifies that the PostgreSQL migrator can be created
	// It doesn't test the actual migration functionality, which would require a database connection

	// Create a PostgreSQL migrator
	migrator, err := migrations.NewPostgreSQLMigrator("postgresql://localhost:5432/testdb")
	c.Assert(err, qt.IsNil)
	c.Assert(migrator, qt.IsNotNil)

	// Verify the migrator implements the Migrator interface
	var _ migrations.Migrator = migrator
}

// TestCheckMigrationsAppliedHappyPath tests the CheckMigrationsApplied function happy path
func TestCheckMigrationsAppliedHappyPath(t *testing.T) {
	tests := []struct {
		name            string
		migratorName    string
		expectedApplied bool
	}{
		{
			name:            "migrations not applied",
			migratorName:    "notapplied",
			expectedApplied: false,
		},
		{
			name:            "migrations applied",
			migratorName:    "applied",
			expectedApplied: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Save the original migrators to restore after test
			originalMigrators := migrations.Migrators()
			defer func() {
				// Clear current migrators
				for name := range migrations.Migrators() {
					migrations.Unregister(name)
				}
				// Restore original migrators
				for name, fn := range originalMigrators {
					migrations.Register(name, fn)
				}
			}()

			// Clear migrators for clean test
			for name := range migrations.Migrators() {
				migrations.Unregister(name)
			}

			// Register test migrator
			migrations.Register(tt.migratorName, func(dsn string) (migrations.Migrator, error) {
				return &mockMigrator{
					runMigrationsFunc: func(ctx context.Context) error {
						return nil
					},
					checkMigrationsAppliedFunc: func(ctx context.Context) (bool, error) {
						return tt.expectedApplied, nil
					},
				}, nil
			})

			// Test CheckMigrationsApplied
			applied, err := migrations.CheckMigrationsApplied(context.Background(), tt.migratorName+"://localhost")
			c.Assert(err, qt.IsNil)
			c.Assert(applied, qt.Equals, tt.expectedApplied)
		})
	}
}

// TestCheckMigrationsAppliedUnhappyPath tests the CheckMigrationsApplied function error cases
func TestCheckMigrationsAppliedUnhappyPath(t *testing.T) {
	tests := []struct {
		name      string
		dsn       string
		setupFunc func()
	}{
		{
			name:      "unknown database type",
			dsn:       "unknown://localhost",
			setupFunc: func() {},
		},
		{
			name: "migrator creation error",
			dsn:  "error://localhost",
			setupFunc: func() {
				migrations.Register("error", func(dsn string) (migrations.Migrator, error) {
					return nil, errors.New("migrator creation error")
				})
			},
		},
		{
			name: "check error",
			dsn:  "test://localhost",
			setupFunc: func() {
				migrations.Register("test", func(dsn string) (migrations.Migrator, error) {
					return &mockMigrator{
						runMigrationsFunc: func(ctx context.Context) error {
							return nil
						},
						checkMigrationsAppliedFunc: func(ctx context.Context) (bool, error) {
							return false, errors.New("check error")
						},
					}, nil
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Save the original migrators to restore after test
			originalMigrators := migrations.Migrators()
			defer func() {
				// Clear current migrators
				for name := range migrations.Migrators() {
					migrations.Unregister(name)
				}
				// Restore original migrators
				for name, fn := range originalMigrators {
					migrations.Register(name, fn)
				}
			}()

			// Clear migrators for clean test
			for name := range migrations.Migrators() {
				migrations.Unregister(name)
			}

			// Setup test case
			tt.setupFunc()

			// Test CheckMigrationsApplied
			applied, err := migrations.CheckMigrationsApplied(context.Background(), tt.dsn)
			c.Assert(err, qt.IsNotNil)
			c.Assert(applied, qt.IsFalse)
		})
	}
}
