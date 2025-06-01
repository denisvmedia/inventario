package compare

import (
	"fmt"
	"path/filepath"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/ptah/core/goschema"
	"github.com/denisvmedia/inventario/ptah/dbschema"
	"github.com/denisvmedia/inventario/ptah/renderer"
	"github.com/denisvmedia/inventario/ptah/schema/differ"
)

var compareCmd = &cobra.Command{
	Use:   "compare",
	Short: "Compare generated schema with database",
	Long: `Compare the schema generated from Go entities with the current database schema.
	
This command shows differences between what your Go entities define and what
currently exists in the database, helping you identify what needs to be migrated.`,
	RunE: compareCommand,
}

const (
	rootDirFlag = "root-dir"
	dbURLFlag   = "db-url"
)

var compareFlags = map[string]cobraflags.Flag{
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

func NewCompareCommand() *cobra.Command {
	cobraflags.RegisterMap(compareCmd, compareFlags)
	return compareCmd
}

func compareCommand(_ *cobra.Command, _ []string) error {
	rootDir := compareFlags[rootDirFlag].GetString()
	dbURL := compareFlags[dbURLFlag].GetString()

	if dbURL == "" {
		return fmt.Errorf("database URL is required")
	}

	fmt.Printf("Comparing schema from %s with database %s\n", rootDir, dbschema.FormatDatabaseURL(dbURL))
	fmt.Println("=== SCHEMA COMPARISON ===")
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
	diff := differ.CompareSchemas(result, dbSchema)

	// 4. Display differences
	output := renderer.FormatSchemaDiff(diff)
	fmt.Print(output)

	return nil
}
