package dropschema

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/ptah/executor"
	"github.com/denisvmedia/inventario/ptah/schema/builder"
)

var dropSchemaCmd = &cobra.Command{
	Use:   "drop-schema",
	Short: "Drop tables/enums from Go entities (DANGEROUS!)",
	Long: `Drop all tables and enums defined in Go entities from the database.
	
⚠️  WARNING: This is a destructive operation that will permanently delete data!
This command will only drop tables and enums that are defined in your Go entities,
not everything in the database.`,
	RunE: dropSchemaCommand,
}

const (
	rootDirFlag = "root-dir"
	dbURLFlag   = "db-url"
)

var dropSchemaFlags = map[string]cobraflags.Flag{
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

func NewDropSchemaCommand() *cobra.Command {
	cobraflags.RegisterMap(dropSchemaCmd, dropSchemaFlags)
	return dropSchemaCmd
}

func dropSchemaCommand(_ *cobra.Command, _ []string) error {
	rootDir := dropSchemaFlags[rootDirFlag].GetString()
	dbURL := dropSchemaFlags[dbURLFlag].GetString()

	if dbURL == "" {
		return fmt.Errorf("database URL is required")
	}

	fmt.Printf("Dropping schema from %s based on entities in %s\n", executor.FormatDatabaseURL(dbURL), rootDir)
	fmt.Println("=== DROP SCHEMA FROM DATABASE ===")
	fmt.Println()

	// 1. Parse Go entities to know what to drop
	absPath, err := filepath.Abs(rootDir)
	if err != nil {
		return fmt.Errorf("error resolving path: %w", err)
	}

	result, err := builder.ParsePackageRecursively(absPath)
	if err != nil {
		return fmt.Errorf("error parsing Go entities: %w", err)
	}

	fmt.Printf("Found %d tables, %d enums to drop\n", len(result.Tables), len(result.Enums))

	// 2. Connect to database
	conn, err := executor.ConnectToDatabase(dbURL)
	if err != nil {
		return fmt.Errorf("error connecting to database: %w", err)
	}
	defer conn.Close()

	fmt.Printf("Connected to %s database successfully!\n", conn.Info.Dialect)
	fmt.Println()

	// 3. Show warning and ask for confirmation
	fmt.Println("⚠️  WARNING: This operation will permanently delete all tables and enums!")
	fmt.Println("⚠️  This action cannot be undone!")
	fmt.Printf("⚠️  Tables to be dropped: %v\n", func() []string {
		names := make([]string, len(result.Tables))
		for i, table := range result.Tables {
			names[i] = table.Name
		}
		return names
	}())
	if len(result.Enums) > 0 {
		fmt.Printf("⚠️  Enums to be dropped: %v\n", func() []string {
			names := make([]string, len(result.Enums))
			for i, enum := range result.Enums {
				names[i] = enum.Name
			}
			return names
		}())
	}
	fmt.Println()
	fmt.Print("Type 'YES' to confirm: ")

	confirmation, err := readLine()
	if err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	if confirmation != "YES" {
		fmt.Println("Operation cancelled.")
		return nil
	}

	// 4. Drop schema
	fmt.Println("Dropping schema from database...")
	err = conn.Writer.DropSchema(result)
	if err != nil {
		return fmt.Errorf("error dropping schema: %w", err)
	}

	fmt.Println("✅ Schema dropped successfully!")
	return nil
}

// readLine reads a complete line from stdin, including spaces
func readLine() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	// Remove the trailing newline
	return strings.TrimSpace(line), nil
}
