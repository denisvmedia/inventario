package features

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/registry"
)

var featuresCmd = &cobra.Command{
	Use:   "features",
	Short: "Show database feature matrix",
	Long: `Display a matrix showing which features are supported by each database backend.

This command helps you understand the capabilities and limitations of different
database backends supported by Inventario.

FEATURE DESCRIPTIONS:

  FullTextSearch     - Advanced full-text search with ranking and relevance scoring
  JSONBOperators     - JSONB query operators for complex JSON field queries  
  AdvancedIndexing   - GIN, GiST, and partial indexes for optimized queries
  Triggers           - Database triggers for automatic data maintenance
  StoredProcedures   - Stored procedures and functions for complex operations
  BulkOperations     - Efficient bulk insert/update operations
  Transactions       - ACID transaction support for data consistency
  ArrayOperations    - Array data types and specialized array operations

BACKEND RECOMMENDATIONS:

  PostgreSQL (postgres://)
    - Recommended for production deployments
    - Full feature support with optimal performance
    - Best choice for complex queries and large datasets

  MySQL/MariaDB (mysql://)
    - Good alternative to PostgreSQL
    - Most features supported except JSONB operators
    - Suitable for production with some limitations

  BoltDB (boltdb://)
    - Embedded database for single-user deployments
    - Basic features only, no advanced querying
    - Good for development and small-scale usage

  Memory (memory://)
    - In-memory storage for testing and development
    - Minimal features, data lost on restart
    - Only suitable for testing and temporary usage

USAGE EXAMPLES:

  # Show feature matrix
  inventario features

  # Use with specific database to see what features are available
  inventario run --db-dsn="postgres://user:pass@localhost/db"
  inventario run --db-dsn="boltdb://./data.db"`,
	RunE: featuresCommand,
}

func NewFeaturesCommand() *cobra.Command {
	return featuresCmd
}

func featuresCommand(_ *cobra.Command, _ []string) error {
	fmt.Println("Inventario Database Feature Matrix") //nolint:forbidigo // CLI output is OK
	fmt.Println("==================================") //nolint:forbidigo // CLI output is OK
	fmt.Println()                                     //nolint:forbidigo // CLI output is OK

	// Print the feature matrix
	registry.PrintFeatureMatrix()

	fmt.Println()                                                             //nolint:forbidigo // CLI output is OK
	fmt.Println("Legend:")                                                    //nolint:forbidigo // CLI output is OK
	fmt.Println("  ✓ = Feature supported")                                    //nolint:forbidigo // CLI output is OK
	fmt.Println("  ✗ = Feature not supported (fallback implementation used)") //nolint:forbidigo // CLI output is OK
	fmt.Println()                                                             //nolint:forbidigo // CLI output is OK

	// Print detailed feature descriptions
	fmt.Println("Feature Descriptions:") //nolint:forbidigo // CLI output is OK
	fmt.Println("--------------------")  //nolint:forbidigo // CLI output is OK

	features := registry.GetFeatureMatrix()
	for _, feature := range features {
		fmt.Printf("%-18s - %s\n", feature.Feature, feature.Description) //nolint:forbidigo // CLI output is OK
	}

	fmt.Println()                                                                        //nolint:forbidigo // CLI output is OK
	fmt.Println("Database Recommendations:")                                             //nolint:forbidigo // CLI output is OK
	fmt.Println("-------------------------")                                             //nolint:forbidigo // CLI output is OK
	fmt.Println("PostgreSQL: Best performance and full feature support (recommended)")   //nolint:forbidigo // CLI output is OK
	fmt.Println("MySQL:      Good alternative with most features supported")             //nolint:forbidigo // CLI output is OK
	fmt.Println("BoltDB:     Basic features only, suitable for single-user deployments") //nolint:forbidigo // CLI output is OK
	fmt.Println("Memory:     Testing only, minimal features, data not persisted")        //nolint:forbidigo // CLI output is OK

	fmt.Println()                                                                              //nolint:forbidigo // CLI output is OK
	fmt.Println("For optimal performance and full feature support, use PostgreSQL:")           //nolint:forbidigo // CLI output is OK
	fmt.Println("  inventario run --db-dsn=\"postgres://user:password@localhost/inventario\"") //nolint:forbidigo // CLI output is OK

	return nil
}
