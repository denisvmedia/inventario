package integration

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/denisvmedia/inventario/ptah/executor"
	"github.com/denisvmedia/inventario/ptah/migrator"
	"github.com/denisvmedia/inventario/ptah/schema/differ"
	"github.com/denisvmedia/inventario/ptah/schema/parser"
	"github.com/denisvmedia/inventario/ptah/schema/parser/parsertypes"
)

// TestResult represents the result of a single test scenario
type TestResult struct {
	Name        string        `json:"name"`
	Database    string        `json:"database"`
	Success     bool          `json:"success"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
	Description string        `json:"description"`
}

// TestReport represents the complete test report
type TestReport struct {
	StartTime    time.Time    `json:"start_time"`
	EndTime      time.Time    `json:"end_time"`
	TotalTests   int          `json:"total_tests"`
	PassedTests  int          `json:"passed_tests"`
	FailedTests  int          `json:"failed_tests"`
	Results      []TestResult `json:"results"`
	Summary      string       `json:"summary"`
}

// TestScenario represents a single test scenario
type TestScenario struct {
	Name        string
	Description string
	TestFunc    func(ctx context.Context, conn *executor.DatabaseConnection, fixtures fs.FS) error
}

// TestRunner manages and executes integration tests
type TestRunner struct {
	scenarios []TestScenario
	databases map[string]string // name -> connection URL
	fixtures  fs.FS
	report    *TestReport
	mu        sync.Mutex
}

// NewTestRunner creates a new test runner
func NewTestRunner(fixtures fs.FS) *TestRunner {
	return &TestRunner{
		scenarios: make([]TestScenario, 0),
		databases: make(map[string]string),
		fixtures:  fixtures,
		report: &TestReport{
			Results: make([]TestResult, 0),
		},
	}
}

// AddDatabase adds a database connection for testing
func (tr *TestRunner) AddDatabase(name, connectionURL string) {
	tr.databases[name] = connectionURL
}

// AddScenario adds a test scenario
func (tr *TestRunner) AddScenario(scenario TestScenario) {
	tr.scenarios = append(tr.scenarios, scenario)
}

// RunAll executes all test scenarios against all databases
func (tr *TestRunner) RunAll(ctx context.Context) error {
	tr.report.StartTime = time.Now()
	
	for dbName, dbURL := range tr.databases {
		for _, scenario := range tr.scenarios {
			result := tr.runSingleTest(ctx, scenario, dbName, dbURL)
			tr.mu.Lock()
			tr.report.Results = append(tr.report.Results, result)
			tr.report.TotalTests++
			if result.Success {
				tr.report.PassedTests++
			} else {
				tr.report.FailedTests++
			}
			tr.mu.Unlock()
		}
	}
	
	tr.report.EndTime = time.Now()
	tr.generateSummary()
	
	return nil
}

// runSingleTest executes a single test scenario against a specific database
func (tr *TestRunner) runSingleTest(ctx context.Context, scenario TestScenario, dbName, dbURL string) TestResult {
	start := time.Now()
	
	result := TestResult{
		Name:        fmt.Sprintf("%s_%s", scenario.Name, dbName),
		Database:    dbName,
		Description: scenario.Description,
	}
	
	// Connect to database
	conn, err := executor.ConnectToDatabase(dbURL)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to connect to database: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	defer conn.Close()
	
	// Clean database before test
	if err := tr.cleanDatabase(ctx, conn); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to clean database: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	
	// Run the test scenario
	if err := scenario.TestFunc(ctx, conn, tr.fixtures); err != nil {
		result.Success = false
		result.Error = err.Error()
	} else {
		result.Success = true
	}
	
	result.Duration = time.Since(start)
	return result
}

// cleanDatabase drops all tables and resets the database to a clean state
func (tr *TestRunner) cleanDatabase(ctx context.Context, conn *executor.DatabaseConnection) error {
	// Drop all tables to ensure clean state
	return conn.Writer().DropAllTables()
}

// VersionedEntityManager manages versioned entity fixtures for tests
type VersionedEntityManager struct {
	fixturesFS  fs.FS
	tempDir     string
	entitiesDir string
	version     int // Current migration version
}

// NewVersionedEntityManager creates a new versioned entity manager
func NewVersionedEntityManager(fixturesFS fs.FS) (*VersionedEntityManager, error) {
	tempDir, err := os.MkdirTemp("", "ptah_integration_test_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	entitiesDir := filepath.Join(tempDir, "entities")
	if err := os.MkdirAll(entitiesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create entities directory: %w", err)
	}

	return &VersionedEntityManager{
		fixturesFS:  fixturesFS,
		tempDir:     tempDir,
		entitiesDir: entitiesDir,
		version:     0,
	}, nil
}

// Cleanup removes the temporary directory
func (vem *VersionedEntityManager) Cleanup() error {
	return os.RemoveAll(vem.tempDir)
}

// GetEntitiesDir returns the path to the entities directory
func (vem *VersionedEntityManager) GetEntitiesDir() string {
	return vem.entitiesDir
}

// GetCurrentVersion returns the current migration version
func (vem *VersionedEntityManager) GetCurrentVersion() int {
	return vem.version
}

// LoadEntityVersion loads entities from a specific version directory
func (vem *VersionedEntityManager) LoadEntityVersion(versionDir string) error {
	// Clear current entities directory
	if err := os.RemoveAll(vem.entitiesDir); err != nil {
		return fmt.Errorf("failed to clear entities directory: %w", err)
	}
	if err := os.MkdirAll(vem.entitiesDir, 0755); err != nil {
		return fmt.Errorf("failed to recreate entities directory: %w", err)
	}

	// Copy entities from version directory
	// Use forward slashes for filesystem paths
	// Try both possible paths: with and without "fixtures/" prefix
	var versionPath string
	var entries []fs.DirEntry
	var err error

	// First try with "fixtures/" prefix (for embedded filesystem)
	versionPath = "fixtures/entities/" + versionDir
	entries, err = fs.ReadDir(vem.fixturesFS, versionPath)
	if err != nil {
		// If that fails, try without "fixtures/" prefix (for mounted filesystem)
		versionPath = "entities/" + versionDir
		entries, err = fs.ReadDir(vem.fixturesFS, versionPath)
		if err != nil {
			return fmt.Errorf("failed to read version directory %s (tried both fixtures/entities/%s and entities/%s): %w", versionDir, versionDir, versionDir, err)
		}
	}

	// Copy each file
	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip subdirectories
		}

		// Read file from fixtures
		filePath := versionPath + "/" + entry.Name()
		content, err := fs.ReadFile(vem.fixturesFS, filePath)
		if err != nil {
			return fmt.Errorf("failed to read fixture file %s: %w", filePath, err)
		}

		// Write to temp entities directory
		destPath := filepath.Join(vem.entitiesDir, entry.Name())
		if err := os.WriteFile(destPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write entity file %s: %w", destPath, err)
		}
	}

	return nil
}

// GenerateSchemaFromEntities parses the current entities and returns the schema
func (vem *VersionedEntityManager) GenerateSchemaFromEntities() (*parsertypes.PackageParseResult, error) {
	return parser.ParsePackageRecursively(vem.entitiesDir)
}

// GenerateMigrationSQL compares current entities with database and generates migration SQL
func (vem *VersionedEntityManager) GenerateMigrationSQL(ctx context.Context, conn *executor.DatabaseConnection) ([]string, error) {
	// Parse current entities
	generated, err := vem.GenerateSchemaFromEntities()
	if err != nil {
		return nil, fmt.Errorf("failed to parse entities: %w", err)
	}

	// Read current database schema
	dbSchema, err := conn.Reader().ReadSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to read database schema: %w", err)
	}

	// Compare schemas
	diff := differ.CompareSchemas(generated, dbSchema)

	// Generate migration SQL
	statements := diff.GenerateMigrationSQL(generated, conn.Info().Dialect)

	return statements, nil
}

// ApplyMigrationFromEntities generates and applies a migration from current entities
func (vem *VersionedEntityManager) ApplyMigrationFromEntities(ctx context.Context, conn *executor.DatabaseConnection, description string) error {
	// Generate migration SQL
	statements, err := vem.GenerateMigrationSQL(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to generate migration SQL: %w", err)
	}

	if len(statements) == 0 {
		// No changes needed - this is idempotent behavior
		return nil
	}

	// Only increment version if there are actual changes to apply
	vem.version++
	upSQL := ""
	for _, stmt := range statements {
		upSQL += stmt + "\n"
	}

	// For simplicity, we'll create a basic down migration that drops everything
	// In a real scenario, you'd want more sophisticated down migrations
	downSQL := "-- Auto-generated down migration\n-- Manual review required\n"

	migration := migrator.CreateMigrationFromSQL(vem.version, description, upSQL, downSQL)

	// Apply the migration
	migrator := migrator.NewMigrator(conn)
	migrator.Register(migration)

	return migrator.MigrateUp(ctx)
}

// MigrateToVersion loads entities from a version and applies the migration
func (vem *VersionedEntityManager) MigrateToVersion(ctx context.Context, conn *executor.DatabaseConnection, versionDir, description string) error {
	// Load entities for this version
	if err := vem.LoadEntityVersion(versionDir); err != nil {
		return fmt.Errorf("failed to load entity version %s: %w", versionDir, err)
	}

	// Apply migration
	return vem.ApplyMigrationFromEntities(ctx, conn, description)
}

// generateSummary creates a summary of the test results
func (tr *TestRunner) generateSummary() {
	duration := tr.report.EndTime.Sub(tr.report.StartTime)
	successRate := float64(tr.report.PassedTests) / float64(tr.report.TotalTests) * 100
	
	tr.report.Summary = fmt.Sprintf(
		"Executed %d tests in %v. %d passed, %d failed (%.1f%% success rate)",
		tr.report.TotalTests,
		duration.Round(time.Millisecond),
		tr.report.PassedTests,
		tr.report.FailedTests,
		successRate,
	)
}

// GetReport returns the current test report
func (tr *TestRunner) GetReport() *TestReport {
	return tr.report
}

// DatabaseHelper provides utility functions for database operations in tests
type DatabaseHelper struct {
	conn *executor.DatabaseConnection
}

// NewDatabaseHelper creates a new database helper
func NewDatabaseHelper(conn *executor.DatabaseConnection) *DatabaseHelper {
	return &DatabaseHelper{conn: conn}
}

// ApplyMigrations applies migrations from the given filesystem
func (dh *DatabaseHelper) ApplyMigrations(ctx context.Context, migrationsFS fs.FS) error {
	return migrator.RunMigrations(ctx, dh.conn, migrationsFS)
}

// GetCurrentVersion returns the current migration version
func (dh *DatabaseHelper) GetCurrentVersion(ctx context.Context, migrationsFS fs.FS) (int, error) {
	status, err := migrator.GetMigrationStatus(ctx, dh.conn, migrationsFS)
	if err != nil {
		return 0, err
	}
	return status.CurrentVersion, nil
}

// RollbackToVersion rolls back migrations to a specific version
func (dh *DatabaseHelper) RollbackToVersion(ctx context.Context, migrationsFS fs.FS, targetVersion int) error {
	m := migrator.NewMigrator(dh.conn)
	if err := migrator.RegisterMigrations(m, migrationsFS); err != nil {
		return err
	}
	return m.MigrateDown(ctx, targetVersion)
}

// TableExists checks if a table exists in the database
func (dh *DatabaseHelper) TableExists(tableName string) (bool, error) {
	schema, err := dh.conn.Reader().ReadSchema()
	if err != nil {
		return false, err
	}
	
	for _, table := range schema.Tables {
		if table.Name == tableName {
			return true, nil
		}
	}
	return false, nil
}

// ExecuteSQL executes raw SQL with proper transaction management
func (dh *DatabaseHelper) ExecuteSQL(sql string) error {
	// Start transaction
	if err := dh.conn.Writer().BeginTransaction(); err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Execute SQL
	if err := dh.conn.Writer().ExecuteSQL(sql); err != nil {
		_ = dh.conn.Writer().RollbackTransaction()
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	// Commit transaction
	if err := dh.conn.Writer().CommitTransaction(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// SetDryRun enables or disables dry run mode
func (dh *DatabaseHelper) SetDryRun(dryRun bool) {
	dh.conn.Writer().SetDryRun(dryRun)
}

// IsDryRun returns whether dry run mode is enabled
func (dh *DatabaseHelper) IsDryRun() bool {
	return dh.conn.Writer().IsDryRun()
}

// GetMigrationsFS returns the appropriate migrations filesystem for the database dialect
func GetMigrationsFS(fixtures fs.FS, conn *executor.DatabaseConnection, migrationType string) (fs.FS, error) {
	dialect := conn.Info().Dialect

	// Try database-specific migrations first
	var migrationPath string
	switch dialect {
	case "mysql":
		migrationPath = fmt.Sprintf("migrations/%s_mysql", migrationType)
	case "postgres":
		migrationPath = fmt.Sprintf("migrations/%s", migrationType) // PostgreSQL uses the default
	default:
		migrationPath = fmt.Sprintf("migrations/%s", migrationType) // Fallback to default
	}

	// Check if database-specific migrations exist
	if _, err := fs.Stat(fixtures, migrationPath); err == nil {
		return fs.Sub(fixtures, migrationPath)
	}

	// Fallback to default migrations
	defaultPath := fmt.Sprintf("migrations/%s", migrationType)
	return fs.Sub(fixtures, defaultPath)
}
