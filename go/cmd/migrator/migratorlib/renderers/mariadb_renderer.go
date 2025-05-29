package renderers

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/ptah/schema/ast"
)

// MariaDBRenderer provides MariaDB-specific SQL rendering
// MariaDB is largely compatible with MySQL, so it inherits most functionality
type MariaDBRenderer struct {
	*MySQLRenderer
}

// NewMariaDBRenderer creates a new MariaDB renderer
func NewMariaDBRenderer() *MariaDBRenderer {
	return &MariaDBRenderer{
		MySQLRenderer: NewMySQLRenderer(),
	}
}

// VisitEnum renders enum handling for MariaDB (inline ENUM types like MySQL)
func (r *MariaDBRenderer) VisitEnum(node *ast.EnumNode) error {
	// MariaDB doesn't have separate enum types like PostgreSQL
	// Enums are defined inline in column definitions like MySQL
	// So this method doesn't render anything for MariaDB
	return nil
}

// renderColumn overrides MySQL column rendering with MariaDB-specific handling
func (r *MariaDBRenderer) renderColumn(column *ast.ColumnNode) (string, error) {
	// MariaDB uses the same column syntax as MySQL, so we can delegate
	return r.MySQLRenderer.renderColumn(column)
}

// renderAutoIncrement renders MariaDB auto increment (same as MySQL)
func (r *MariaDBRenderer) renderAutoIncrement() string {
	return "AUTO_INCREMENT" // MariaDB uses same syntax as MySQL
}

// VisitCreateTable renders MariaDB-specific CREATE TABLE statements
func (r *MariaDBRenderer) VisitCreateTable(node *ast.CreateTableNode) error {
	// Table comment
	if node.Comment != "" {
		r.WriteLinef("-- MARIADB TABLE: %s (%s) --", node.Name, node.Comment)
	} else {
		r.WriteLinef("-- MARIADB TABLE: %s --", node.Name)
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

	// Close table definition with MariaDB-specific options
	if len(node.Options) > 0 {
		options := r.renderTableOptions(node.Options)
		if options != "" {
			r.Write(" ")
			r.Write(options)
		}
	}

	r.WriteLine("")
	// Only one newline instead of two for better spacing
	return nil
}

// renderTableOptions renders MariaDB table options (same as MySQL)
func (r *MariaDBRenderer) renderTableOptions(options map[string]string) string {
	var parts []string
	for key, value := range options {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(parts, " ")
}

// VisitAlterTable renders MariaDB-specific ALTER TABLE statements
func (r *MariaDBRenderer) VisitAlterTable(node *ast.AlterTableNode) error {
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
			// MariaDB uses MODIFY COLUMN syntax like MySQL
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

// RenderSchema renders a complete schema for MariaDB
func (r *MariaDBRenderer) RenderSchema(statements *ast.StatementList) (string, error) {
	r.Reset()

	// MariaDB doesn't need separate enum definitions like MySQL, so we render everything in order
	for _, stmt := range statements.Statements {
		// Skip enum nodes as MariaDB handles enums inline
		if _, ok := stmt.(*ast.EnumNode); ok {
			continue
		}

		if err := stmt.Accept(r); err != nil {
			return "", fmt.Errorf("error rendering statement: %w", err)
		}
	}

	return r.GetOutput(), nil
}

// renderColumnWithEnums renders a column with enum support for MariaDB
func (r *MariaDBRenderer) renderColumnWithEnums(column *ast.ColumnNode, enumValues []string) (string, error) {
	var parts []string

	// Handle enum types inline for MariaDB
	columnType := column.Type
	if len(enumValues) > 0 {
		// Convert to MariaDB ENUM syntax
		quotedValues := make([]string, len(enumValues))
		for i, value := range enumValues {
			quotedValues[i] = fmt.Sprintf("'%s'", value)
		}
		columnType = fmt.Sprintf("ENUM(%s)", strings.Join(quotedValues, ", "))
	}

	// Column name and type
	parts = append(parts, fmt.Sprintf("  %s %s", column.Name, columnType))

	// Column constraints - MariaDB order: PRIMARY KEY, then NOT NULL, then UNIQUE
	if column.Primary {
		parts = append(parts, "PRIMARY KEY")
		if column.AutoInc {
			parts = append(parts, r.renderAutoIncrement())
		}
	} else {
		if !column.Nullable {
			parts = append(parts, "NOT NULL")
		}
		if column.Unique {
			parts = append(parts, "UNIQUE")
		}
		if column.AutoInc {
			parts = append(parts, r.renderAutoIncrement())
		}
	}

	// Default values
	if column.Default != nil {
		if column.Default.Function != "" {
			parts = append(parts, fmt.Sprintf("DEFAULT %s", column.Default.Function))
		} else if column.Default.Value != "" {
			parts = append(parts, fmt.Sprintf("DEFAULT '%s'", column.Default.Value))
		}
	}

	// Check constraints
	if column.Check != "" {
		parts = append(parts, fmt.Sprintf("CHECK (%s)", column.Check))
	}

	// Comments
	if column.Comment != "" {
		parts = append(parts, fmt.Sprintf("COMMENT '%s'", column.Comment))
	}

	return strings.Join(parts, " "), nil
}

// VisitCreateTableWithEnums renders CREATE TABLE with enum support for MariaDB
func (r *MariaDBRenderer) VisitCreateTableWithEnums(node *ast.CreateTableNode, enums map[string][]string) error {
	// Table comment
	if node.Comment != "" {
		r.WriteLinef("-- MARIADB TABLE: %s (%s) --", node.Name, node.Comment)
	} else {
		r.WriteLinef("-- MARIADB TABLE: %s --", node.Name)
	}

	// CREATE TABLE statement
	r.WriteLinef("CREATE TABLE %s (", node.Name)

	var lines []string

	// Render columns with enum support
	for _, column := range node.Columns {
		var enumValues []string
		if enums != nil {
			enumValues = enums[column.Type] // Check if column type is an enum
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
	r.Write(strings.Join(lines, ",\n"))

	// Close table definition with MariaDB-specific options
	if len(node.Options) > 0 {
		options := r.renderTableOptions(node.Options)
		if options != "" {
			r.WriteLinef("\n); %s", options)
		} else {
			r.WriteLine("\n);")
		}
	} else {
		r.WriteLine("\n);")
	}

	r.WriteLine("")
	return nil
}
