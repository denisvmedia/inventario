package features

import (
	"fmt"

	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	var featuresCmd = &cobra.Command{
		Use:   "features",
		Short: "Show database backend information",
		Long: `Display information about database backends supported by Inventario.

This command helps you understand the capabilities and limitations of different
database backends supported by Inventario.

BACKEND RECOMMENDATIONS:

  PostgreSQL (postgres://)
    - Recommended for production deployments
    - Full feature support with optimal performance
    - Best choice for complex queries and large datasets
    - Supports all advanced features including:
      * Full-text search with ranking
      * JSONB operators for complex queries
      * Advanced indexing (GIN, GiST, partial)
      * Database triggers and stored procedures
      * Efficient bulk operations
      * ACID transactions
      * Array operations

  Memory (memory://)
    - In-memory storage for testing and development
    - Simplified implementations for basic functionality
    - Data lost on restart
    - Only suitable for testing and temporary usage

USAGE EXAMPLES:

  # Show database information
  inventario features

  # Use with PostgreSQL for production
  inventario run --db-dsn="postgres://user:pass@localhost/db"
`,
		RunE: featuresCommand,
	}

	return featuresCmd
}

func featuresCommand(_ *cobra.Command, _ []string) error {
	fmt.Println("Inventario Database Backend Information")
	fmt.Println("=======================================")
	fmt.Println()

	fmt.Println("PostgreSQL (postgres://)")
	fmt.Println("------------------------")
	fmt.Println("✓ Full-text search with ranking")
	fmt.Println("✓ JSONB operators for complex queries")
	fmt.Println("✓ Advanced indexing (GIN, GiST, partial)")
	fmt.Println("✓ Database triggers")
	fmt.Println("✓ Stored procedures and functions")
	fmt.Println("✓ Efficient bulk operations")
	fmt.Println("✓ ACID transactions")
	fmt.Println("✓ Array operations")
	fmt.Println("✓ Recommended for production")
	fmt.Println()

	fmt.Println("Memory (memory://)")
	fmt.Println("------------------")
	fmt.Println("✓ Basic CRUD operations")
	fmt.Println("✓ Simple text search")
	fmt.Println("✓ In-memory filtering")
	fmt.Println("✗ Advanced database features")
	fmt.Println("✗ Data persistence")
	fmt.Println("✗ Production use")
	fmt.Println()

	fmt.Println("Database Recommendations:")
	fmt.Println("-------------------------")
	fmt.Println("PostgreSQL: Best performance and full feature support (recommended)")
	fmt.Println("Memory:     Testing only, minimal features, data not persisted")

	fmt.Println()
	fmt.Println("For optimal performance and full feature support, use PostgreSQL:")
	fmt.Println("  inventario run --db-dsn=\"postgres://user:password@localhost/inventario\"")

	return nil
}
