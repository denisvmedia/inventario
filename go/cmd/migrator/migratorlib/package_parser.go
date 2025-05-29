package migratorlib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/denisvmedia/inventario/internal/log"
	"github.com/denisvmedia/inventario/ptah/schema/meta"
)

// PackageParseResult contains all parsed entities from the entire project
type PackageParseResult struct {
	Tables         []meta.TableDirective
	Fields         []meta.SchemaField
	Indexes        []meta.SchemaIndex
	Enums          []meta.GlobalEnum
	EmbeddedFields []meta.EmbeddedField
	Dependencies   map[string][]string // table -> list of tables it depends on
}

// ParsePackageRecursively parses all Go files in the given root directory and its subdirectories
// to find all entity definitions and build a complete database schema
func ParsePackageRecursively(rootDir string) (*PackageParseResult, error) {
	result := &PackageParseResult{
		Tables:         []meta.TableDirective{},
		Fields:         []meta.SchemaField{},
		Indexes:        []meta.SchemaIndex{},
		Enums:          []meta.GlobalEnum{},
		EmbeddedFields: []meta.EmbeddedField{},
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
	result.deduplicate()

	// Build dependency graph for foreign key ordering
	result.buildDependencyGraph()

	// Sort tables by dependency order
	result.sortTablesByDependencies()

	return result, nil
}

// deduplicate removes duplicate entities that may be defined in multiple files
func (r *PackageParseResult) deduplicate() {
	// Deduplicate tables by name
	tableMap := make(map[string]meta.TableDirective)
	for _, table := range r.Tables {
		tableMap[table.Name] = table
	}
	r.Tables = make([]meta.TableDirective, 0, len(tableMap))
	for _, table := range tableMap {
		r.Tables = append(r.Tables, table)
	}

	// Deduplicate fields by struct name and field name
	fieldMap := make(map[string]meta.SchemaField)
	for _, field := range r.Fields {
		key := field.StructName + "." + field.Name
		fieldMap[key] = field
	}
	r.Fields = make([]meta.SchemaField, 0, len(fieldMap))
	for _, field := range fieldMap {
		r.Fields = append(r.Fields, field)
	}

	// Deduplicate indexes by struct name and index name
	indexMap := make(map[string]meta.SchemaIndex)
	for _, index := range r.Indexes {
		key := index.StructName + "." + index.Name
		indexMap[key] = index
	}
	r.Indexes = make([]meta.SchemaIndex, 0, len(indexMap))
	for _, index := range indexMap {
		r.Indexes = append(r.Indexes, index)
	}

	// Deduplicate enums by name
	enumMap := make(map[string]meta.GlobalEnum)
	for _, enum := range r.Enums {
		enumMap[enum.Name] = enum
	}
	r.Enums = make([]meta.GlobalEnum, 0, len(enumMap))
	for _, enum := range enumMap {
		r.Enums = append(r.Enums, enum)
	}

	// Deduplicate embedded fields by struct name and embedded type name
	embeddedMap := make(map[string]meta.EmbeddedField)
	for _, embedded := range r.EmbeddedFields {
		key := embedded.StructName + "." + embedded.EmbeddedTypeName
		embeddedMap[key] = embedded
	}
	r.EmbeddedFields = make([]meta.EmbeddedField, 0, len(embeddedMap))
	for _, embedded := range embeddedMap {
		r.EmbeddedFields = append(r.EmbeddedFields, embedded)
	}
}

// buildDependencyGraph analyzes foreign key relationships to build a dependency graph
func (r *PackageParseResult) buildDependencyGraph() {
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
					if !contains(r.Dependencies[table.Name], refTable) {
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
					if !contains(r.Dependencies[table.Name], refTable) {
						r.Dependencies[table.Name] = append(r.Dependencies[table.Name], refTable)
					}
					break
				}
			}
		}
	}
}

// sortTablesByDependencies performs topological sort to order tables by their dependencies
func (r *PackageParseResult) sortTablesByDependencies() {
	// Create a map for quick table lookup
	tableMap := make(map[string]meta.TableDirective)
	for _, table := range r.Tables {
		tableMap[table.Name] = table
	}

	// Perform topological sort using Kahn's algorithm
	sorted := []meta.TableDirective{}
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
		log.Warnf("Circular dependency detected in foreign key relationships. Some tables may not be ordered correctly.")
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

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetOrderedCreateStatements returns CREATE TABLE statements in dependency order
func (r *PackageParseResult) GetOrderedCreateStatements(dialect string) []string {
	statements := []string{}

	for _, table := range r.Tables {
		sql := GenerateCreateTableWithEmbedded(table, r.Fields, r.Indexes, r.Enums, r.EmbeddedFields, dialect)
		statements = append(statements, sql)
	}

	return statements
}

// GetDependencyInfo returns human-readable dependency information for debugging
func (r *PackageParseResult) GetDependencyInfo() string {
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
