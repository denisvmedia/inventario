package readdb

import (
	"fmt"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/ptah/dbschema"
	"github.com/denisvmedia/inventario/ptah/renderer"
)

var readDBCmd = &cobra.Command{
	Use:   "read-db",
	Short: "Read schema from database",
	Long: `Read and display the current schema from the specified database.
	
This command connects to the database and reads the existing schema,
displaying tables, columns, indexes, and constraints in a formatted output.`,
	RunE: readDBCommand,
}

const (
	dbURLFlag = "db-url"
)

var readDBFlags = map[string]cobraflags.Flag{
	dbURLFlag: &cobraflags.StringFlag{
		Name:  dbURLFlag,
		Value: "",
		Usage: "Database URL (required). Example: postgres://user:pass@localhost/db",
	},
}

func NewReadDBCommand() *cobra.Command {
	cobraflags.RegisterMap(readDBCmd, readDBFlags)
	return readDBCmd
}

func readDBCommand(_ *cobra.Command, _ []string) error {
	dbURL := readDBFlags[dbURLFlag].GetString()

	if dbURL == "" {
		return fmt.Errorf("database URL is required")
	}

	fmt.Printf("Reading schema from database: %s\n", dbschema.FormatDatabaseURL(dbURL))
	fmt.Println("=== DATABASE SCHEMA ===")
	fmt.Println()

	// Connect to the database
	conn, err := dbschema.ConnectToDatabase(dbURL)
	if err != nil {
		fmt.Printf("Error connecting to database: %v\n", err)
		fmt.Println()
		fmt.Println("Make sure:")
		fmt.Println("1. The database URL is correct")
		fmt.Println("2. The database server is running")
		fmt.Println("3. You have the correct permissions")
		fmt.Println("4. The database exists")
		return err
	}
	defer conn.Close()

	fmt.Printf("Connected to %s database successfully!\n", conn.Info().Dialect)
	fmt.Println()

	// Read the schema
	schema, err := conn.Reader().ReadSchema()
	if err != nil {
		return fmt.Errorf("error reading schema: %w", err)
	}

	// Format and display the schema
	output := renderer.FormatSchema(schema, conn.Info())
	fmt.Print(output)

	return nil
}
