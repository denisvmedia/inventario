package goschema

import (
	"fmt"
	"strings"
)

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
func GetDependencyInfo(r *Database) string {
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
