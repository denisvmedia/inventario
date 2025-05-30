// Package parser provides functionality for parsing Go packages to extract database schema information.
//
// This package implements a recursive parser that walks through Go source files to discover
// entity definitions, table directives, field mappings, indexes, enums, and embedded fields.
// It builds a complete database schema representation that can be used for migration generation.
//
// The parser handles:
//   - Recursive directory traversal to find all Go files
//   - Extraction of database entities from struct definitions
//   - Dependency analysis for foreign key relationships
//   - Topological sorting to ensure proper table creation order
//   - Deduplication of entities found in multiple files
//   - Generation of ordered CREATE TABLE statements
//
// Key features:
//   - Skips test files and vendor directories automatically
//   - Resolves circular dependencies with warnings
//   - Supports embedded fields with relation modes
//   - Provides debugging information for dependency analysis
//
// Example usage:
//
//	result, err := builder.ParsePackageRecursively("./entities")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	statements := result.GetOrderedCreateStatements("postgresql")
//	for _, stmt := range statements {
//		fmt.Println(stmt)
//	}
package parser

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/denisvmedia/inventario/ptah/renderer/generators"
	"github.com/denisvmedia/inventario/ptah/schema/parser/parsertypes"
	"github.com/denisvmedia/inventario/ptah/schema/types"
)

// ParsePackageRecursively parses all Go files in the given root directory and its subdirectories
// to find all entity definitions and build a complete database schema.
//
// This function performs a comprehensive analysis of the Go codebase to extract database
// schema information. It walks through the directory tree recursively, parsing each Go file
// to discover entity definitions, and then processes the results to build a coherent
// database schema with proper dependency ordering.
//
// The parsing process includes:
//   - Recursive directory traversal starting from rootDir
//   - Filtering to include only .go files (excluding tests and vendor)
//   - Extraction of tables, fields, indexes, enums, and embedded fields
//   - Deduplication of entities found in multiple files
//   - Dependency analysis based on foreign key relationships
//   - Topological sorting to determine proper table creation order
//
// Parameters:
//   - rootDir: The root directory to start parsing from (e.g., "./entities", "./models")
//
// Returns:
//   - *PackageParseResult: Complete schema information with dependency ordering
//   - error: Any error encountered during parsing or file system operations
//
// Example:
//
//	result, err := ParsePackageRecursively("./internal/entities")
//	if err != nil {
//		return fmt.Errorf("failed to parse entities: %w", err)
//	}
//
//	// Generate migration statements in proper order
//	statements := GetOrderedCreateStatements(result, "postgresql")
func ParsePackageRecursively(rootDir string) (*parsertypes.PackageParseResult, error) {
	result := &parsertypes.PackageParseResult{
		Tables:         []types.TableDirective{},
		Fields:         []types.SchemaField{},
		Indexes:        []types.SchemaIndex{},
		Enums:          []types.GlobalEnum{},
		EmbeddedFields: []types.EmbeddedField{},
		Dependencies:   make(map[string][]string),
	}

	// Walk through all directories recursively
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-Go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip test files
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip vendor directories
		if strings.Contains(path, "vendor/") {
			return nil
		}

		// Parse the file
		embeddedFields, fields, indexes, tables, enums := ParseFile(path)

		// Add to result
		result.EmbeddedFields = append(result.EmbeddedFields, embeddedFields...)
		result.Fields = append(result.Fields, fields...)
		result.Indexes = append(result.Indexes, indexes...)
		result.Tables = append(result.Tables, tables...)
		result.Enums = append(result.Enums, enums...)

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Deduplicate entities (same table/field defined in multiple files)
	deduplicate(result)

	// Build dependency graph for foreign key ordering
	buildDependencyGraph(result)

	// Sort tables by dependency order
	sortTablesByDependencies(result)

	return result, nil
}

// deduplicate removes duplicate entities that may be defined in multiple files.
//
// During recursive parsing, the same entity might be encountered multiple times
// if it's defined in different files or referenced across packages. This method
// ensures that each unique entity appears only once in the final result.
//
// The deduplication process handles:
//   - Tables: Deduplicated by table name
//   - Fields: Deduplicated by struct name + field name combination
//   - Indexes: Deduplicated by struct name + index name combination
//   - Enums: Deduplicated by enum name
//   - Embedded Fields: Deduplicated by struct name + embedded type name combination
//
// This method modifies the PackageParseResult in-place, replacing the original
// slices with deduplicated versions. The order of entities may change during
// this process, but dependency ordering is handled separately.
func deduplicate(r *parsertypes.PackageParseResult) {
	// Deduplicate tables by name
	tableMap := make(map[string]types.TableDirective)
	for _, table := range r.Tables {
		tableMap[table.Name] = table
	}
	r.Tables = make([]types.TableDirective, 0, len(tableMap))
	for _, table := range tableMap {
		r.Tables = append(r.Tables, table)
	}

	// Deduplicate fields by struct name and field name
	fieldMap := make(map[string]types.SchemaField)
	for _, field := range r.Fields {
		key := field.StructName + "." + field.Name
		fieldMap[key] = field
	}
	r.Fields = make([]types.SchemaField, 0, len(fieldMap))
	for _, field := range fieldMap {
		r.Fields = append(r.Fields, field)
	}

	// Deduplicate indexes by struct name and index name
	indexMap := make(map[string]types.SchemaIndex)
	for _, index := range r.Indexes {
		key := index.StructName + "." + index.Name
		indexMap[key] = index
	}
	r.Indexes = make([]types.SchemaIndex, 0, len(indexMap))
	for _, index := range indexMap {
		r.Indexes = append(r.Indexes, index)
	}

	// Deduplicate enums by name
	enumMap := make(map[string]types.GlobalEnum)
	for _, enum := range r.Enums {
		enumMap[enum.Name] = enum
	}
	r.Enums = make([]types.GlobalEnum, 0, len(enumMap))
	for _, enum := range enumMap {
		r.Enums = append(r.Enums, enum)
	}

	// Deduplicate embedded fields by struct name and embedded type name
	embeddedMap := make(map[string]types.EmbeddedField)
	for _, embedded := range r.EmbeddedFields {
		key := embedded.StructName + "." + embedded.EmbeddedTypeName
		embeddedMap[key] = embedded
	}
	r.EmbeddedFields = make([]types.EmbeddedField, 0, len(embeddedMap))
	for _, embedded := range embeddedMap {
		r.EmbeddedFields = append(r.EmbeddedFields, embedded)
	}
}

// buildDependencyGraph analyzes foreign key relationships to build a dependency graph.
//
// This method examines all fields and embedded fields to identify foreign key relationships
// and builds a dependency graph that maps each table to the tables it depends on. This
// information is crucial for determining the correct order of table creation to satisfy
// foreign key constraints.
//
// The analysis process:
//  1. Initializes empty dependency lists for all tables
//  2. Scans all fields for foreign key references (field.Foreign attribute)
//  3. Scans embedded fields with relation mode for references (embedded.Ref attribute)
//  4. Extracts referenced table names from foreign key specifications
//  5. Maps each table to its list of dependencies
//
// Foreign key format examples:
//   - "users(id)" -> depends on "users" table
//   - "categories(uuid)" -> depends on "categories" table
//
// The resulting dependency graph is stored in the Dependencies field and used by
// sortTablesByDependencies() to perform topological sorting.
func buildDependencyGraph(r *parsertypes.PackageParseResult) {
	// Initialize dependencies map for all tables
	for _, table := range r.Tables {
		r.Dependencies[table.Name] = []string{}
	}

	// Analyze foreign key relationships
	for _, field := range r.Fields {
		if field.Foreign != "" {
			// Parse foreign key reference (e.g., "users(id)" -> "users")
			refTable := strings.Split(field.Foreign, "(")[0]

			// Find the table that contains this field
			for _, table := range r.Tables {
				if table.StructName == field.StructName {
					// Add dependency: table depends on refTable
					if !slices.Contains(r.Dependencies[table.Name], refTable) {
						r.Dependencies[table.Name] = append(r.Dependencies[table.Name], refTable)
					}
					break
				}
			}
		}
	}

	// Analyze embedded field relationships (relation mode)
	for _, embedded := range r.EmbeddedFields {
		if embedded.Mode == "relation" && embedded.Ref != "" {
			// Parse embedded relation reference (e.g., "users(id)" -> "users")
			refTable := strings.Split(embedded.Ref, "(")[0]

			// Find the table that contains this embedded field
			for _, table := range r.Tables {
				if table.StructName == embedded.StructName {
					// Add dependency: table depends on refTable
					if !slices.Contains(r.Dependencies[table.Name], refTable) {
						r.Dependencies[table.Name] = append(r.Dependencies[table.Name], refTable)
					}
					break
				}
			}
		}
	}
}

// sortTablesByDependencies performs topological sort to order tables by their dependencies.
//
// This method implements Kahn's algorithm for topological sorting to determine the correct
// order for creating database tables. Tables with no dependencies are created first,
// followed by tables that depend on them, ensuring that foreign key constraints can be
// satisfied during migration execution.
//
// Algorithm steps:
//  1. Calculate in-degrees (number of dependencies) for each table
//  2. Initialize queue with tables that have no dependencies (in-degree 0)
//  3. Process queue: remove table, add to sorted result, reduce in-degrees of dependent tables
//  4. Continue until all tables are processed or circular dependency is detected
//
// Circular dependency handling:
//   - If circular dependencies are detected, a warning is logged
//   - Remaining tables are appended to the end of the sorted list
//   - This allows migration to proceed, but manual intervention may be needed
//
// The method modifies the Tables slice in-place, reordering it according to dependency
// requirements. This ensures that CREATE TABLE statements can be executed in the
// returned order without foreign key constraint violations.
func sortTablesByDependencies(r *parsertypes.PackageParseResult) {
	// Create a map for quick table lookup
	tableMap := make(map[string]types.TableDirective)
	for _, table := range r.Tables {
		tableMap[table.Name] = table
	}

	// Perform topological sort using Kahn's algorithm
	sorted := []types.TableDirective{}
	inDegree := make(map[string]int)

	// Calculate in-degrees (how many dependencies each table has)
	for tableName := range r.Dependencies {
		inDegree[tableName] = 0
	}
	for tableName, deps := range r.Dependencies {
		inDegree[tableName] = len(deps)
	}

	// Find tables with no dependencies (in-degree 0)
	queue := []string{}
	for tableName, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, tableName)
		}
	}

	// Process queue
	for len(queue) > 0 {
		// Remove first element from queue
		current := queue[0]
		queue = queue[1:]

		// Add to sorted result if table exists
		if table, exists := tableMap[current]; exists {
			sorted = append(sorted, table)
		}

		// Reduce in-degree of tables that depend on the current table
		for tableName, deps := range r.Dependencies {
			for _, dep := range deps {
				if dep == current {
					inDegree[tableName]--
					if inDegree[tableName] == 0 {
						queue = append(queue, tableName)
					}
				}
			}
		}
	}

	// Check for circular dependencies
	if len(sorted) != len(r.Tables) {
		slog.Warn("Circular dependency detected in foreign key relationships. Some tables may not be ordered correctly.")
		// Add remaining tables to the end
		for _, table := range r.Tables {
			found := false
			for _, sortedTable := range sorted {
				if sortedTable.Name == table.Name {
					found = true
					break
				}
			}
			if !found {
				sorted = append(sorted, table)
			}
		}
	}

	// Update the tables slice with sorted order
	r.Tables = sorted
}

// GetDependencyInfo returns human-readable dependency information for debugging.
//
// This method generates a formatted string that displays the complete dependency
// graph and table creation order. It's useful for debugging dependency issues,
// understanding the schema structure, and verifying that the topological sort
// has produced the expected results.
//
// The output includes:
//  1. Table Dependencies section: Shows each table and what it depends on
//  2. Table Creation Order section: Shows the final order for table creation
//
// Tables with no dependencies are clearly marked, making it easy to identify
// root tables in the dependency graph. This information is particularly valuable
// when troubleshooting circular dependencies or unexpected table ordering.
//
// Returns a formatted string containing dependency and ordering information.
//
// Example output:
//
//	Table Dependencies:
//	==================
//	users: (no dependencies)
//	categories: (no dependencies)
//	products: depends on [categories users]
//	orders: depends on [users products]
//
//	Table Creation Order:
//	====================
//	1. users
//	2. categories
//	3. products
//	4. orders
func GetDependencyInfo(r *parsertypes.PackageParseResult) string {
	var info strings.Builder
	info.WriteString("Table Dependencies:\n")
	info.WriteString("==================\n")

	for _, table := range r.Tables {
		deps := r.Dependencies[table.Name]
		if len(deps) == 0 {
			info.WriteString(fmt.Sprintf("%s: (no dependencies)\n", table.Name))
		} else {
			info.WriteString(fmt.Sprintf("%s: depends on %v\n", table.Name, deps))
		}
	}

	info.WriteString("\nTable Creation Order:\n")
	info.WriteString("====================\n")
	for i, table := range r.Tables {
		info.WriteString(fmt.Sprintf("%d. %s\n", i+1, table.Name))
	}

	return info.String()
}

func GetOrderedCreateStatements(r *parsertypes.PackageParseResult, dialect string) []string {
	statements := []string{}

	for _, table := range r.Tables {
		sql := generators.GenerateCreateTableWithEmbedded(table, r.Fields, r.Indexes, r.Enums, r.EmbeddedFields, dialect)
		statements = append(statements, sql)
	}

	return statements
}
