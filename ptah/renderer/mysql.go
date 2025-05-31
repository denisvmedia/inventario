package renderer

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/ptah/schema/ast"
)

// MySQLRenderer provides MySQL-specific SQL rendering
type MySQLRenderer struct {
	*BaseRenderer
}

// NewMySQLRenderer creates a new MySQL renderer
func NewMySQLRenderer() *MySQLRenderer {
	return &MySQLRenderer{
		BaseRenderer: NewBaseRenderer("mysql"),
	}
}

// VisitEnum renders enum handling for MySQL (inline ENUM types)
func (r *MySQLRenderer) VisitEnum(_node *ast.EnumNode) error {
	// MySQL doesn't have separate enum types like PostgreSQL
	// Enums are defined inline in column definitions
	// So this method doesn't render anything for MySQL
	return nil
}

// renderAutoIncrement renders MySQL auto increment
func (r *MySQLRenderer) renderAutoIncrement() string {
	return "AUTO_INCREMENT"
}

// processFieldType processes field type for MySQL, handling enums and type conversions
func (r *MySQLRenderer) processFieldType(fieldType string, enumValues []string) string {
	// Check if this is an enum type and convert to MySQL ENUM syntax
	if len(enumValues) > 0 {
		quotedValues := make([]string, len(enumValues))
		for i, value := range enumValues {
			quotedValues[i] = fmt.Sprintf("'%s'", value)
		}
		return fmt.Sprintf("ENUM(%s)", strings.Join(quotedValues, ", "))
	}

	// Handle MySQL-specific type mappings
	switch strings.ToUpper(fieldType) {
	case "SERIAL":
		return "INT"
	case "BOOLEAN":
		return "BOOLEAN"
	default:
		return fieldType
	}
}

// convertDefaultFunction converts default functions to MySQL-compatible syntax
func (r *MySQLRenderer) convertDefaultFunction(function string) string {
	switch strings.ToUpper(function) {
	case "NOW()":
		return "CURRENT_TIMESTAMP"
	default:
		return function
	}
}

// convertDefaultValue converts default values to MySQL-compatible syntax
func (r *MySQLRenderer) convertDefaultValue(value, columnType string) string {
	// Handle boolean values
	if strings.ToUpper(columnType) == "BOOLEAN" {
		switch strings.ToLower(value) {
		case "true", "'true'":
			return "TRUE"
		case "false", "'false'":
			return "FALSE"
		}
	}

	// For other types, quote the value if it's not already quoted
	if !strings.HasPrefix(value, "'") && !strings.HasSuffix(value, "'") {
		return fmt.Sprintf("'%s'", value)
	}
	return value
}

// Enhanced column rendering with MySQL-specific enum handling
func (r *MySQLRenderer) renderColumnWithEnums(column *ast.ColumnNode, enumValues []string) (string, error) {
	var parts []string

	// Handle MySQL-specific type conversions (especially enums)
	columnType := r.processFieldType(column.Type, enumValues)

	// Column name and type
	parts = append(parts, fmt.Sprintf("  %s %s", column.Name, columnType))

	// Column constraints
	if column.Primary {
		parts = append(parts, "PRIMARY KEY")
	} else {
		if column.Unique {
			parts = append(parts, "UNIQUE")
		}
		if !column.Nullable {
			parts = append(parts, "NOT NULL")
		}
	}

	// Auto increment
	if column.AutoInc {
		parts = append(parts, r.renderAutoIncrement())
	}

	// Default value
	if column.Default != nil {
		if column.Default.Function != "" {
			// Handle MySQL-specific function mappings
			defaultFunc := r.convertDefaultFunction(column.Default.Function)
			parts = append(parts, fmt.Sprintf("DEFAULT %s", defaultFunc))
		} else if column.Default.Value != "" {
			// Handle MySQL-specific value mappings
			defaultValue := r.convertDefaultValue(column.Default.Value, columnType)
			parts = append(parts, fmt.Sprintf("DEFAULT %s", defaultValue))
		}
	}

	// Check constraint
	if column.Check != "" {
		parts = append(parts, fmt.Sprintf("CHECK (%s)", column.Check))
	}

	return strings.Join(parts, " "), nil
}

// VisitAlterTable renders MySQL-specific ALTER TABLE statements
func (r *MySQLRenderer) VisitAlterTable(node *ast.AlterTableNode) error {
	r.WriteLine("-- ALTER statements: --")

	for _, operation := range node.Operations {
		switch op := operation.(type) {
		case *ast.AddColumnOperation:
			line, err := r.renderColumn(op.Column)
			if err != nil {
				return fmt.Errorf("error rendering add column: %w", err)
			}
			// Remove the leading spaces from column rendering for ALTER
			line = strings.TrimPrefix(line, "  ")
			r.WriteLinef("ALTER TABLE %s ADD COLUMN %s;", node.Name, line)

		case *ast.DropColumnOperation:
			r.WriteLinef("ALTER TABLE %s DROP COLUMN %s;", node.Name, op.ColumnName)

		case *ast.ModifyColumnOperation:
			// MySQL uses MODIFY COLUMN syntax
			line, err := r.renderColumn(op.Column)
			if err != nil {
				return fmt.Errorf("error rendering modify column: %w", err)
			}
			// Remove the leading spaces from column rendering for ALTER
			line = strings.TrimPrefix(line, "  ")
			r.WriteLinef("ALTER TABLE %s MODIFY COLUMN %s;", node.Name, line)

		default:
			return fmt.Errorf("unknown alter operation type: %T", operation)
		}
	}

	r.WriteLine("")
	return nil
}

// renderTableOptions renders MySQL table options in a specific order
func (r *MySQLRenderer) renderTableOptions(options map[string]string) string {
	var parts []string

	// Define the order of options - ENGINE should come first
	orderedKeys := []string{"ENGINE", "CHARSET", "CHARACTER SET", "COLLATE", "COMMENT"}

	// Add options in the specified order
	for _, key := range orderedKeys {
		// Check for the key in a case-insensitive manner
		var value string
		var exists bool
		for optKey, optValue := range options {
			if strings.EqualFold(optKey, key) {
				value = optValue
				exists = true
				break
			}
		}

		if exists {
			switch strings.ToUpper(key) {
			case "ENGINE":
				parts = append(parts, fmt.Sprintf("ENGINE=%s", value))
			case "CHARSET":
				parts = append(parts, fmt.Sprintf("charset=%s", value))
			case "CHARACTER SET":
				parts = append(parts, fmt.Sprintf("CHARACTER SET=%s", value))
			case "COLLATE":
				parts = append(parts, fmt.Sprintf("COLLATE=%s", value))
			case "COMMENT":
				parts = append(parts, fmt.Sprintf("COMMENT='%s'", value))
			}
		}
	}

	// Add any remaining options that weren't in the ordered list
	for key, value := range options {
		found := false
		for _, orderedKey := range orderedKeys {
			if strings.EqualFold(key, orderedKey) {
				found = true
				break
			}
		}
		if !found {
			parts = append(parts, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return strings.Join(parts, " ")
}

// VisitIndex renders a CREATE INDEX statement for MySQL
func (r *MySQLRenderer) VisitIndex(node *ast.IndexNode) error {
	var parts []string

	parts = append(parts, "CREATE")

	if node.Unique {
		parts = append(parts, "UNIQUE")
	}

	parts = append(parts, "INDEX")
	parts = append(parts, node.Name)
	parts = append(parts, "ON")
	parts = append(parts, node.Table)
	parts = append(parts, fmt.Sprintf("(%s)", strings.Join(node.Columns, ", ")))

	r.WriteLinef("%s;", strings.Join(parts, " "))
	return nil
}

// VisitCreateTable renders CREATE TABLE with MySQL-specific spacing
func (r *MySQLRenderer) VisitCreateTable(node *ast.CreateTableNode) error {
	// Table comment
	if node.Comment != "" {
		r.WriteLinef("-- %s TABLE: %s (%s) --", strings.ToUpper(r.dialect), node.Name, node.Comment)
	} else {
		r.WriteLinef("-- %s TABLE: %s --", strings.ToUpper(r.dialect), node.Name)
	}

	// CREATE TABLE statement
	r.WriteLinef("CREATE TABLE %s (", node.Name)

	var lines []string

	// Render columns
	for _, column := range node.Columns {
		line, err := r.renderColumn(column)
		if err != nil {
			return fmt.Errorf("error rendering column %s: %w", column.Name, err)
		}
		lines = append(lines, line)
	}

	// Render table-level constraints
	for _, constraint := range node.Constraints {
		line, err := r.renderConstraint(constraint)
		if err != nil {
			return fmt.Errorf("error rendering constraint: %w", err)
		}
		if line != "" {
			lines = append(lines, line)
		}
	}

	// Join all lines
	for i, line := range lines {
		if i == len(lines)-1 {
			r.WriteLine(line) // Last line without comma
		} else {
			r.WriteLinef("%s,", line)
		}
	}

	r.Write(");")

	// Table options (like ENGINE for MySQL)
	if len(node.Options) > 0 {
		r.Write(" ")
		r.Write(r.renderTableOptions(node.Options))
	}

	r.WriteLine("")
	// Only one newline instead of two for better spacing
	return nil
}

// RenderSchema renders a complete schema for MySQL
func (r *MySQLRenderer) RenderSchema(statements *ast.StatementList) (string, error) {
	r.Reset()

	// MySQL doesn't need separate enum definitions, so we render everything in order
	for _, stmt := range statements.Statements {
		// Skip enum nodes as MySQL handles enums inline
		if _, ok := stmt.(*ast.EnumNode); ok {
			continue
		}

		if err := stmt.Accept(r); err != nil {
			return "", fmt.Errorf("error rendering statement: %w", err)
		}
	}

	return r.GetOutput(), nil
}

// VisitCreateTableWithEnums provides enhanced CREATE TABLE rendering with enum support
func (r *MySQLRenderer) VisitCreateTableWithEnums(node *ast.CreateTableNode, enums map[string][]string) error {
	// Table comment
	if node.Comment != "" {
		r.WriteLinef("-- %s TABLE: %s (%s) --", strings.ToUpper(r.dialect), node.Name, node.Comment)
	} else {
		r.WriteLinef("-- %s TABLE: %s --", strings.ToUpper(r.dialect), node.Name)
	}

	// CREATE TABLE statement
	r.WriteLinef("CREATE TABLE %s (", node.Name)

	var lines []string

	// Render columns with enum support
	for _, column := range node.Columns {
		// Always get enum values, the method handles nil enums internally
		enumValues := r.getEnumValues(column.Type, enums)

		// Only use enum values if the type is actually an enum
		if !r.isEnumType(column.Type, enums) {
			enumValues = nil
		}

		line, err := r.renderColumnWithEnums(column, enumValues)
		if err != nil {
			return fmt.Errorf("error rendering column %s: %w", column.Name, err)
		}
		lines = append(lines, line)
	}

	// Render table-level constraints
	for _, constraint := range node.Constraints {
		line, err := r.renderConstraint(constraint)
		if err != nil {
			return fmt.Errorf("error rendering constraint: %w", err)
		}
		if line != "" {
			lines = append(lines, line)
		}
	}

	// Join all lines
	for i, line := range lines {
		if i == len(lines)-1 {
			r.WriteLine(line) // Last line without comma
		} else {
			r.WriteLinef("%s,", line)
		}
	}

	r.Write(");")

	// Table options
	if len(node.Options) > 0 {
		r.Write(" ")
		r.Write(r.renderTableOptions(node.Options))
	}

	r.WriteLine("")
	// Only one newline instead of two for better spacing
	return nil
}

// Helper method to check if a type is an enum
func (r *MySQLRenderer) isEnumType(fieldType string, enums map[string][]string) bool {
	if enums == nil {
		return false
	}
	_, exists := enums[fieldType]
	return exists
}

// Helper method to get enum values for a type
func (r *MySQLRenderer) getEnumValues(fieldType string, enums map[string][]string) []string {
	if enums == nil {
		return nil
	}
	return enums[fieldType]
}
