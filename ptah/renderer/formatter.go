package renderer

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/ptah/schema/parser/parsertypes"
)

// FormatSchema formats a database schema for display
func FormatSchema(schema *parsertypes.DatabaseSchema, info parsertypes.DatabaseInfo) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("=== DATABASE SCHEMA (%s) ===\n", strings.ToUpper(info.Dialect)))
	sb.WriteString(fmt.Sprintf("Version: %s\n", info.Version))
	sb.WriteString(fmt.Sprintf("Schema: %s\n", info.Schema))
	sb.WriteString("\n")

	// Summary
	sb.WriteString("SUMMARY:\n")
	sb.WriteString(fmt.Sprintf("- Tables: %d\n", len(schema.Tables)))
	sb.WriteString(fmt.Sprintf("- Enums: %d\n", len(schema.Enums)))
	sb.WriteString(fmt.Sprintf("- Indexes: %d\n", len(schema.Indexes)))
	sb.WriteString(fmt.Sprintf("- Constraints: %d\n", len(schema.Constraints)))
	sb.WriteString("\n")

	// Enums
	if len(schema.Enums) > 0 {
		sb.WriteString("=== ENUMS ===\n")
		for _, enum := range schema.Enums {
			sb.WriteString(fmt.Sprintf("- %s: [%s]\n", enum.Name, strings.Join(enum.Values, ", ")))
		}
		sb.WriteString("\n")
	}

	// Tables
	sb.WriteString("=== TABLES ===\n")
	for i, table := range schema.Tables {
		sb.WriteString(fmt.Sprintf("%d. %s (%s)\n", i+1, table.Name, table.Type))
		if table.Comment != "" {
			sb.WriteString(fmt.Sprintf("   Comment: %s\n", table.Comment))
		}

		// Columns
		sb.WriteString("   Columns:\n")
		for _, col := range table.Columns {
			sb.WriteString(formatColumn(col, "     "))
		}

		// Table constraints
		tableConstraints := getTableConstraints(schema.Constraints, table.Name)
		if len(tableConstraints) > 0 {
			sb.WriteString("   Constraints:\n")
			for _, constraint := range tableConstraints {
				sb.WriteString(formatConstraint(constraint, "     "))
			}
		}

		// Table indexes
		tableIndexes := getTableIndexes(schema.Indexes, table.Name)
		if len(tableIndexes) > 0 {
			sb.WriteString("   Indexes:\n")
			for _, index := range tableIndexes {
				sb.WriteString(formatIndex(index, "     "))
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// formatColumn formats a column for display
func formatColumn(col parsertypes.Column, indent string) string {
	var parts []string

	// Basic info
	typeInfo := col.DataType
	if col.UDTName != "" && col.UDTName != col.DataType {
		typeInfo = col.UDTName
	}
	if col.ColumnType != "" && col.ColumnType != col.DataType {
		typeInfo = col.ColumnType
	}

	// Add length/precision info
	if col.CharacterMaxLength != nil {
		typeInfo += fmt.Sprintf("(%d)", *col.CharacterMaxLength)
	} else if col.NumericPrecision != nil && col.NumericScale != nil {
		typeInfo += fmt.Sprintf("(%d,%d)", *col.NumericPrecision, *col.NumericScale)
	} else if col.NumericPrecision != nil {
		typeInfo += fmt.Sprintf("(%d)", *col.NumericPrecision)
	}

	parts = append(parts, fmt.Sprintf("%s %s", col.Name, typeInfo))

	// Constraints
	if col.IsPrimaryKey {
		parts = append(parts, "PRIMARY KEY")
	}
	if col.IsUnique {
		parts = append(parts, "UNIQUE")
	}
	if col.IsNullable == "NO" {
		parts = append(parts, "NOT NULL")
	}
	if col.IsAutoIncrement {
		parts = append(parts, "AUTO_INCREMENT")
	}

	// Default value
	if col.ColumnDefault != nil {
		parts = append(parts, fmt.Sprintf("DEFAULT %s", *col.ColumnDefault))
	}

	return fmt.Sprintf("%s- %s\n", indent, strings.Join(parts, " "))
}

// formatConstraint formats a constraint for display
func formatConstraint(constraint parsertypes.Constraint, indent string) string {
	switch constraint.Type {
	case "PRIMARY KEY":
		return fmt.Sprintf("%s- PRIMARY KEY (%s)\n", indent, constraint.ColumnName)
	case "FOREIGN KEY":
		fkInfo := fmt.Sprintf("%s -> %s(%s)", constraint.ColumnName,
			*constraint.ForeignTable, *constraint.ForeignColumn)
		if constraint.DeleteRule != nil && *constraint.DeleteRule != "" {
			fkInfo += fmt.Sprintf(" ON DELETE %s", *constraint.DeleteRule)
		}
		if constraint.UpdateRule != nil && *constraint.UpdateRule != "" {
			fkInfo += fmt.Sprintf(" ON UPDATE %s", *constraint.UpdateRule)
		}
		return fmt.Sprintf("%s- FOREIGN KEY %s\n", indent, fkInfo)
	case "UNIQUE":
		return fmt.Sprintf("%s- UNIQUE (%s)\n", indent, constraint.ColumnName)
	case "CHECK":
		checkInfo := constraint.ColumnName
		if constraint.CheckClause != nil {
			checkInfo += fmt.Sprintf(" CHECK %s", *constraint.CheckClause)
		}
		return fmt.Sprintf("%s- CHECK %s\n", indent, checkInfo)
	default:
		return fmt.Sprintf("%s- %s (%s)\n", indent, constraint.Type, constraint.ColumnName)
	}
}

// formatIndex formats an index for display
func formatIndex(index parsertypes.Index, indent string) string {
	indexType := "INDEX"
	if index.IsPrimary {
		indexType = "PRIMARY KEY"
	} else if index.IsUnique {
		indexType = "UNIQUE INDEX"
	}

	columns := strings.Join(index.Columns, ", ")
	return fmt.Sprintf("%s- %s %s (%s)\n", indent, indexType, index.Name, columns)
}

// getTableConstraints returns constraints for a specific table
func getTableConstraints(constraints []parsertypes.Constraint, tableName string) []parsertypes.Constraint {
	var result []parsertypes.Constraint
	for _, constraint := range constraints {
		if constraint.TableName == tableName {
			result = append(result, constraint)
		}
	}
	return result
}

// getTableIndexes returns indexes for a specific table
func getTableIndexes(indexes []parsertypes.Index, tableName string) []parsertypes.Index {
	var result []parsertypes.Index
	for _, index := range indexes {
		if index.TableName == tableName {
			result = append(result, index)
		}
	}
	return result
}
