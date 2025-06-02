package migrate

import (
	"fmt"
	"path/filepath"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/core/renderer"
	"github.com/denisvmedia/inventario/ptah/dbschema"
	"github.com/denisvmedia/inventario/ptah/migration/planner"
	"github.com/denisvmedia/inventario/ptah/migration/schemadiff"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Generate migration SQL from differences",
	Long: `Generate migration SQL statements based on differences between Go entities and database schema.
	
This command compares your Go entities with the current database schema and generates
the SQL statements needed to update the database to match your entities.`,
	RunE: migrateCommand,
}

const (
	rootDirFlag = "root-dir"
	dbURLFlag   = "db-url"
)

var migrateFlags = map[string]cobraflags.Flag{
	rootDirFlag: &cobraflags.StringFlag{
		Name:  rootDirFlag,
		Value: "./",
		Usage: "Root directory to scan for Go entities",
	},
	dbURLFlag: &cobraflags.StringFlag{
		Name:  dbURLFlag,
		Value: "",
		Usage: "Database URL (required). Example: postgres://user:pass@localhost/db",
	},
}

func NewMigrateCommand() *cobra.Command {
	cobraflags.RegisterMap(migrateCmd, migrateFlags)
	return migrateCmd
}

func migrateCommand(_ *cobra.Command, _ []string) error {
	rootDir := migrateFlags[rootDirFlag].GetString()
	dbURL := migrateFlags[dbURLFlag].GetString()

	if dbURL == "" {
		return fmt.Errorf("database URL is required")
	}

	fmt.Printf("Generating migration from %s to database %s\n", rootDir, dbschema.FormatDatabaseURL(dbURL))
	fmt.Println("=== GENERATE MIGRATION SQL ===")
	fmt.Println()

	// 1. Parse Go entities
	absPath, err := filepath.Abs(rootDir)
	if err != nil {
		return fmt.Errorf("error resolving path: %w", err)
	}

	result, err := goschema.ParseDir(absPath)
	if err != nil {
		return fmt.Errorf("error parsing Go entities: %w", err)
	}

	// 2. Connect to database and read schema
	conn, err := dbschema.ConnectToDatabase(dbURL)
	if err != nil {
		return fmt.Errorf("error connecting to database: %w", err)
	}
	defer conn.Close()

	dbSchema, err := conn.Reader().ReadSchema()
	if err != nil {
		return fmt.Errorf("error reading database schema: %w", err)
	}

	// 3. Compare schemas
	diff := schemadiff.Compare(result, dbSchema)

	// 4. Display differences summary
	astNodes := planner.GenerateSchemaDiffAST(diff, result, conn.Info().Dialect)
	fmt.Print(astNodes)

	if !diff.HasChanges() {
		return nil
	}

	// 5. Generate migration SQL
	fmt.Println("=== MIGRATION SQL ===")
	fmt.Println()

	statements, err := renderer.RenderSQL(conn.Info().Dialect, astNodes...)
	if err != nil {
		return fmt.Errorf("error rendering SQL: %w", err)
	}

	fmt.Println("-- Migration generated from schema differences")
	fmt.Printf("-- Generated on: %s\n", "now") // You could add actual timestamp
	fmt.Printf("-- Source: %s\n", rootDir)
	fmt.Printf("-- Target: %s\n", dbschema.FormatDatabaseURL(dbURL))
	fmt.Println()

	for _, statement := range statements {
		fmt.Println(statement)
	}

	fmt.Println()
	fmt.Printf("Generated %d migration statements.\n", len(statements))
	fmt.Println("⚠️  Review the SQL carefully before executing!")

	return nil
}
