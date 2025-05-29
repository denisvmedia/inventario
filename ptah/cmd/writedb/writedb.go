package writedb

import (
	"fmt"
	"path/filepath"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/ptah/executor"
	"github.com/denisvmedia/inventario/ptah/schema/builder"
)

var writeDBCmd = &cobra.Command{
	Use:   "write-db",
	Short: "Write schema to database",
	Long: `Write the generated schema from Go entities to the specified database.
	
This command parses Go entities and writes the resulting schema directly to the database.
It will skip existing tables and provide warnings about conflicts.`,
	RunE: writeDBCommand,
}

const (
	rootDirFlag = "root-dir"
	dbURLFlag   = "db-url"
)

var writeDBFlags = map[string]cobraflags.Flag{
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

func NewWriteDBCommand() *cobra.Command {
	cobraflags.RegisterMap(writeDBCmd, writeDBFlags)
	return writeDBCmd
}

func writeDBCommand(_ *cobra.Command, _ []string) error {
	rootDir := writeDBFlags[rootDirFlag].GetString()
	dbURL := writeDBFlags[dbURLFlag].GetString()

	if dbURL == "" {
		return fmt.Errorf("database URL is required")
	}

	fmt.Printf("Writing schema from %s to database %s\n", rootDir, executor.FormatDatabaseURL(dbURL))
	fmt.Println("=== WRITE SCHEMA TO DATABASE ===")
	fmt.Println()

	// 1. Parse Go entities
	absPath, err := filepath.Abs(rootDir)
	if err != nil {
		return fmt.Errorf("error resolving path: %w", err)
	}

	result, err := builder.ParsePackageRecursively(absPath)
	if err != nil {
		return fmt.Errorf("error parsing Go entities: %w", err)
	}

	fmt.Printf("Parsed %d tables, %d enums from Go entities\n", len(result.Tables), len(result.Enums))

	// 2. Connect to database
	conn, err := executor.ConnectToDatabase(dbURL)
	if err != nil {
		return fmt.Errorf("error connecting to database: %w", err)
	}
	defer conn.Close()

	fmt.Printf("Connected to %s database successfully!\n", conn.Info.Dialect)

	// 3. Check if schema already exists
	existingTables, err := conn.Writer.CheckSchemaExists(result)
	if err != nil {
		return fmt.Errorf("error checking existing schema: %w", err)
	}

	if len(existingTables) > 0 {
		fmt.Printf("⚠️  WARNING: The following tables already exist: %v\n", existingTables)
		fmt.Println("This operation will skip existing tables.")
		fmt.Println("Use 'compare' command to see differences, or 'migrate' to generate update SQL.")
		fmt.Println()
	}

	// 4. Write schema
	fmt.Println("Writing schema to database...")
	err = conn.Writer.WriteSchema(result)
	if err != nil {
		return fmt.Errorf("error writing schema: %w", err)
	}

	fmt.Println("✅ Schema written successfully!")
	return nil
}
