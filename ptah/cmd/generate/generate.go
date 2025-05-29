package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/ptah/executor"
	"github.com/denisvmedia/inventario/ptah/schema/builder"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate schema from Go entities",
	Long: `Generate database schema from Go entities in the specified directory.
	
This command scans the directory recursively for Go files with migrator directives
and generates SQL schema for the specified database dialect(s).`,
	RunE: generateCommand,
}

const (
	rootDirFlag = "root-dir"
	dialectFlag = "dialect"
)

var generateFlags = map[string]cobraflags.Flag{
	rootDirFlag: &cobraflags.StringFlag{
		Name:  rootDirFlag,
		Value: "./",
		Usage: "Root directory to scan for Go entities",
	},
	dialectFlag: &cobraflags.StringFlag{
		Name:  dialectFlag,
		Value: "",
		Usage: "Database dialect (postgres, mysql, mariadb). If empty, generates for all dialects",
	},
}

func NewGenerateCommand() *cobra.Command {
	cobraflags.RegisterMap(generateCmd, generateFlags)
	return generateCmd
}

func generateCommand(_ *cobra.Command, _ []string) error {
	rootDir := generateFlags[rootDirFlag].GetString()
	dialect := generateFlags[dialectFlag].GetString()

	// Convert to absolute path
	absPath, err := filepath.Abs(rootDir)
	if err != nil {
		return fmt.Errorf("error resolving path: %w", err)
	}

	// Check if directory exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", absPath)
	}

	fmt.Printf("Scanning directory: %s\n", absPath)
	fmt.Println("=" + strings.Repeat("=", len(absPath)+19))
	fmt.Println()

	// Parse the entire package recursively
	result, err := builder.ParsePackageRecursively(absPath)
	if err != nil {
		return fmt.Errorf("error parsing package: %w", err)
	}

	// Print summary
	fmt.Printf("Found %d tables, %d fields, %d indexes, %d enums, %d embedded fields\n",
		len(result.Tables), len(result.Fields), len(result.Indexes), len(result.Enums), len(result.EmbeddedFields))
	fmt.Println()

	// Print dependency information
	fmt.Println(result.GetDependencyInfo())
	fmt.Println()

	// Determine which dialects to generate
	dialects := []string{"postgres", "mysql", "mariadb"}
	if dialect != "" {
		dialects = []string{dialect}
	}

	// Generate SQL for each dialect
	for _, d := range dialects {
		fmt.Printf("=== %s SCHEMA ===\n", strings.ToUpper(d))
		fmt.Println()

		// Generate enum statements first (only once per dialect)
		if len(result.Enums) > 0 {
			fmt.Println("-- ENUMS --")
			for _, enum := range result.Enums {
				if d == "postgres" {
					fmt.Printf("CREATE TYPE %s AS ENUM (%s);\n", enum.Name,
						strings.Join(func() []string {
							quoted := make([]string, len(enum.Values))
							for i, v := range enum.Values {
								quoted[i] = "'" + v + "'"
							}
							return quoted
						}(), ", "))
				} else {
					fmt.Printf("-- Enum %s: %v (handled in table definitions)\n", enum.Name, enum.Values)
				}
			}
			fmt.Println()
		}

		// Generate table statements
		statements := executor.GetOrderedCreateStatements(result, d)

		for i, statement := range statements {
			fmt.Printf("-- Table %d/%d\n", i+1, len(result.Tables))
			fmt.Println(statement)
			fmt.Println()
		}

		fmt.Println()
	}

	return nil
}
