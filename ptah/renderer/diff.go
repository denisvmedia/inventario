package renderer

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/ptah/schema/differ"
)

// FormatSchemaDiff formats a schema difference for display
func FormatSchemaDiff(diff *differ.SchemaDiff) string {
	var sb strings.Builder

	if !diff.HasChanges() {
		sb.WriteString("=== NO SCHEMA CHANGES DETECTED ===\n")
		sb.WriteString("The database schema matches your entity definitions.\n")
		return sb.String()
	}

	sb.WriteString("=== SCHEMA DIFFERENCES DETECTED ===\n\n")

	// Summary
	totalChanges := len(diff.TablesAdded) + len(diff.TablesRemoved) + len(diff.TablesModified) +
		len(diff.EnumsAdded) + len(diff.EnumsRemoved) + len(diff.EnumsModified) +
		len(diff.IndexesAdded) + len(diff.IndexesRemoved)

	sb.WriteString(fmt.Sprintf("SUMMARY: %d changes detected\n", totalChanges))
	sb.WriteString(fmt.Sprintf("- Tables: +%d -%d ~%d\n", len(diff.TablesAdded), len(diff.TablesRemoved), len(diff.TablesModified)))
	sb.WriteString(fmt.Sprintf("- Enums: +%d -%d ~%d\n", len(diff.EnumsAdded), len(diff.EnumsRemoved), len(diff.EnumsModified)))
	sb.WriteString(fmt.Sprintf("- Indexes: +%d -%d\n", len(diff.IndexesAdded), len(diff.IndexesRemoved)))
	sb.WriteString("\n")

	// Tables
	if len(diff.TablesAdded) > 0 {
		sb.WriteString("📋 TABLES TO ADD:\n")
		for _, table := range diff.TablesAdded {
			sb.WriteString(fmt.Sprintf("  + %s\n", table))
		}
		sb.WriteString("\n")
	}

	if len(diff.TablesRemoved) > 0 {
		sb.WriteString("🗑️  TABLES TO REMOVE:\n")
		for _, table := range diff.TablesRemoved {
			sb.WriteString(fmt.Sprintf("  - %s (⚠️  DATA WILL BE LOST!)\n", table))
		}
		sb.WriteString("\n")
	}

	if len(diff.TablesModified) > 0 {
		sb.WriteString("🔧 TABLES TO MODIFY:\n")
		for _, tableDiff := range diff.TablesModified {
			sb.WriteString(fmt.Sprintf("  ~ %s\n", tableDiff.TableName))

			for _, col := range tableDiff.ColumnsAdded {
				sb.WriteString(fmt.Sprintf("    + Column: %s\n", col))
			}

			for _, col := range tableDiff.ColumnsRemoved {
				sb.WriteString(fmt.Sprintf("    - Column: %s (⚠️  DATA WILL BE LOST!)\n", col))
			}

			for _, colDiff := range tableDiff.ColumnsModified {
				sb.WriteString(fmt.Sprintf("    ~ Column: %s\n", colDiff.ColumnName))
				for changeType, change := range colDiff.Changes {
					sb.WriteString(fmt.Sprintf("      %s: %s\n", changeType, change))
				}
			}
		}
		sb.WriteString("\n")
	}

	// Enums
	if len(diff.EnumsAdded) > 0 {
		sb.WriteString("🏷️  ENUMS TO ADD:\n")
		for _, enum := range diff.EnumsAdded {
			sb.WriteString(fmt.Sprintf("  + %s\n", enum))
		}
		sb.WriteString("\n")
	}

	if len(diff.EnumsRemoved) > 0 {
		sb.WriteString("🗑️  ENUMS TO REMOVE:\n")
		for _, enum := range diff.EnumsRemoved {
			sb.WriteString(fmt.Sprintf("  - %s (⚠️  MAKE SURE NO TABLES USE THIS!)\n", enum))
		}
		sb.WriteString("\n")
	}

	if len(diff.EnumsModified) > 0 {
		sb.WriteString("🔧 ENUMS TO MODIFY:\n")
		for _, enumDiff := range diff.EnumsModified {
			sb.WriteString(fmt.Sprintf("  ~ %s\n", enumDiff.EnumName))

			for _, value := range enumDiff.ValuesAdded {
				sb.WriteString(fmt.Sprintf("    + Value: %s\n", value))
			}

			for _, value := range enumDiff.ValuesRemoved {
				sb.WriteString(fmt.Sprintf("    - Value: %s (⚠️  NOT SUPPORTED IN POSTGRESQL!)\n", value))
			}
		}
		sb.WriteString("\n")
	}

	// Indexes
	if len(diff.IndexesAdded) > 0 {
		sb.WriteString("📇 INDEXES TO ADD:\n")
		for _, index := range diff.IndexesAdded {
			sb.WriteString(fmt.Sprintf("  + %s\n", index))
		}
		sb.WriteString("\n")
	}

	if len(diff.IndexesRemoved) > 0 {
		sb.WriteString("🗑️  INDEXES TO REMOVE:\n")
		for _, index := range diff.IndexesRemoved {
			sb.WriteString(fmt.Sprintf("  - %s\n", index))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
