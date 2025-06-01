package goschema

import (
	"os"
	"path/filepath"
	"strings"
)

// ParseDir parses all Go files in the given root directory and its subdirectories
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
//	result, err := ParseDir("./internal/entities")
//	if err != nil {
//		return fmt.Errorf("failed to parse entities: %w", err)
//	}
//
//	// Generate migration statements in proper order
//	statements := GetOrderedCreateStatements(result, "postgresql")
func ParseDir(rootDir string) (*Database, error) {
	result := &Database{
		Tables:         []Table{},
		Fields:         []Field{},
		Indexes:        []Index{},
		Enums:          []Enum{},
		EmbeddedFields: []EmbeddedField{},
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

	// deduplicate entities (same table/field defined in multiple files)
	deduplicate(result)

	// Build dependency graph for foreign key ordering
	buildDependencyGraph(result)

	// Sort tables by dependency order
	sortTablesByDependencies(result)

	return result, nil
}
