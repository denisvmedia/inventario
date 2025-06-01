package mysqllike

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/renderer/dialects/internal/bufwriter"
	"github.com/denisvmedia/inventario/ptah/core/renderer/types"
)

var (
	_ types.RenderVisitor = (*Renderer)(nil)
)

// Renderer provides MySQL-like-specific SQL rendering
type Renderer struct {
	// currentEnums stores enum names available in the current rendering context
	currentEnums []string
	dialect      string
	dialectUpper string
	w            *bufwriter.Writer
}

// New creates a new MySQL-like renderer
func New(dialect string, buf *bufwriter.Writer) *Renderer {
	return &Renderer{
		w:            buf,
		dialect:      dialect,
		dialectUpper: strings.ToUpper(dialect),
	}
}

func (r *Renderer) VisitDropIndex(node *ast.DropIndexNode) error {
	// Build DROP INDEX statement for MySQL/MariaDB
	var parts []string
	parts = append(parts, "DROP INDEX")

	// Note: MySQL 8.0.1+ and MariaDB 10.1.4+ support IF EXISTS with DROP INDEX
	// For compatibility with older versions, we'll skip IF EXISTS for now
	// if node.IfExists {
	//     parts = append(parts, "IF EXISTS")
	// }

	parts = append(parts, node.Name)

	// MySQL/MariaDB requires table name in DROP INDEX
	if node.Table != "" {
		parts = append(parts, "ON", node.Table)
	}

	sql := strings.Join(parts, " ") + ";"

	// Add comment if provided
	if node.Comment != "" {
		r.w.WriteLinef("-- %s", node.Comment)
	}

	r.w.WriteLine(sql)
	return nil
}

func (r *Renderer) VisitCreateType(node *ast.CreateTypeNode) error {
	// MySQL/MariaDB doesn't support separate type definitions
	// Enums are handled inline in column definitions
	if node.Comment != "" {
		r.w.WriteLinef("-- %s", node.Comment)
	}
	r.w.WriteLinef("-- %s does not support CREATE TYPE - enums are handled inline in column definitions", r.dialectUpper)
	return nil
}

func (r *Renderer) VisitAlterType(node *ast.AlterTypeNode) error {
	// MySQL/MariaDB doesn't support ALTER TYPE operations
	// Type changes are handled through ALTER TABLE MODIFY COLUMN
	r.w.WriteLinef("-- %s does not support ALTER TYPE - type changes are handled through ALTER TABLE MODIFY COLUMN", r.dialectUpper)
	return nil
}

func (r *Renderer) Dialect() string {
	return r.dialect
}

func (r *Renderer) Reset() {
	r.w.Reset()
}

func (r *Renderer) Output() string {
	return r.w.Output()
}

// Render renders an AST node to SQL and returns the result
func (r *Renderer) Render(node ast.Node) (string, error) {
	r.Reset()
	if err := node.Accept(r); err != nil {
		return "", err
	}
	return r.Output(), nil
}

// GetDialect returns the database dialect (alias for Dialect for compatibility)
func (r *Renderer) GetDialect() string {
	return r.Dialect()
}

// GetOutput returns the current generated SQL output (alias for Output for compatibility)
func (r *Renderer) GetOutput() string {
	return r.Output()
}

// VisitCreateTable renders MariaDB-specific CREATE TABLE statements
func (r *Renderer) VisitCreateTable(node *ast.CreateTableNode) error {
	// Table comment
	if node.Comment != "" {
		r.w.WriteLinef("-- %s TABLE: %s (%s) --", r.dialectUpper, node.Name, node.Comment)
	} else {
		r.w.WriteLinef("-- %s TABLE: %s --", r.dialectUpper, node.Name)
	}

	// CREATE TABLE statement
	r.w.WriteLinef("CREATE TABLE %s (", node.Name)

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
			r.w.WriteLine(line) // Last line without comma
		} else {
			r.w.WriteLinef("%s,", line)
		}
	}

	r.w.Write(")")

	// Close table definition with MariaDB-specific options
	if len(node.Options) > 0 {
		options := r.renderTableOptions(node.Options)
		if options != "" {
			r.w.Write(" ")
			r.w.Write(options)
		}
	}

	r.w.WriteLine(";")
	r.w.WriteLine("")

	// Only one newline instead of two for better spacing
	return nil
}

// VisitAlterTable renders MariaDB-specific ALTER TABLE statements
func (r *Renderer) VisitAlterTable(node *ast.AlterTableNode) error {
	return r.visitAlterTableWithEnums(node, nil)
}

// VisitColumn is called when visiting individual columns (used by other visitors)
func (r *Renderer) VisitColumn(node *ast.ColumnNode) error {
	// This is typically called from within other visitors
	// The actual rendering is done by RenderColumn
	return nil
}

// VisitConstraint is called when visiting individual constraints (used by other visitors)
func (r *Renderer) VisitConstraint(node *ast.ConstraintNode) error {
	// This is typically called from within other visitors
	// The actual rendering is done by RenderConstraint
	return nil
}

// VisitIndex renders a CREATE INDEX statement for MySQL
func (r *Renderer) VisitIndex(node *ast.IndexNode) error {
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

	r.w.WriteLinef("%s;", strings.Join(parts, " "))
	return nil
}

// VisitEnum renders enum handling for MariaDB (inline ENUM types like MySQL)
func (r *Renderer) VisitEnum(node *ast.EnumNode) error {
	// MariaDB doesn't have separate enum types like PostgreSQL
	// Enums are defined inline in column definitions like MySQL
	// So this method doesn't render anything for MariaDB
	return nil
}

// VisitComment renders a comment
func (r *Renderer) VisitComment(node *ast.CommentNode) error {
	r.w.WriteLinef("-- %s --", node.Text)
	return nil
}

// VisitDropTable renders MariaDB-specific DROP TABLE statements
func (r *Renderer) VisitDropTable(node *ast.DropTableNode) error {
	// Build DROP TABLE statement with MariaDB-specific features
	var parts []string
	parts = append(parts, "DROP TABLE")

	if node.IfExists {
		parts = append(parts, "IF EXISTS")
	}

	parts = append(parts, node.Name)

	// MariaDB doesn't support CASCADE for DROP TABLE like PostgreSQL
	// Ignore the Cascade flag for MariaDB

	sql := strings.Join(parts, " ") + ";"

	// Add comment if provided
	if node.Comment != "" {
		r.w.WriteLinef("-- %s", node.Comment)
	}

	r.w.WriteLine(sql)
	return nil
}

// VisitDropType renders DROP TYPE statements for MariaDB
func (r *Renderer) VisitDropType(node *ast.DropTypeNode) error {
	// MariaDB doesn't have separate enum types like PostgreSQL
	// This operation is not applicable for MariaDB, so we just add a comment
	if node.Comment != "" {
		r.w.WriteLinef("-- %s", node.Comment)
	}
	r.w.WriteLinef("-- MariaDB does not support DROP TYPE - enums are handled inline in column definitions")
	return nil
}

// RenderColumn renders a column definition
func (r *Renderer) renderColumn(column *ast.ColumnNode) (string, error) {
	var parts []string

	// Column name and type
	parts = append(parts, fmt.Sprintf("  %s %s", column.Name, column.Type))

	// Column constraints
	if column.Primary {
		parts = append(parts, "PRIMARY KEY")
	} else {
		if !column.Nullable {
			parts = append(parts, "NOT NULL")
		}
		if column.Unique {
			parts = append(parts, "UNIQUE")
		}
	}

	// Auto increment (dialect-specific)
	if column.AutoInc {
		parts = append(parts, r.renderAutoIncrement())
	}

	// Default value
	switch {
	case column.Default == nil:
		// No default value
	case column.Default.Value != "":
		parts = append(parts, fmt.Sprintf("DEFAULT '%s'", column.Default.Value)) // TODO: escape!
	case column.Default.Expression != "":
		parts = append(parts, fmt.Sprintf("DEFAULT %s", column.Default.Expression))
	}

	// Check constraint
	if column.Check != "" {
		parts = append(parts, fmt.Sprintf("CHECK (%s)", column.Check))
	}

	return strings.Join(parts, " "), nil
}

// renderAutoIncrement renders auto increment (dialect-specific, override in subclasses)
func (r *Renderer) renderAutoIncrement() string {
	return "AUTO_INCREMENT" // Default MySQL/MariaDB style
}

// renderTableOptions renders MariaDB table options (same as MySQL)
func (r *Renderer) renderTableOptions(options map[string]string) string {
	var parts []string
	for key, value := range options {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(parts, " ")
}

// renderConstraint renders a table-level constraint
func (r *Renderer) renderConstraint(constraint *ast.ConstraintNode) (string, error) {
	switch constraint.Type {
	case ast.PrimaryKeyConstraint:
		return fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(constraint.Columns, ", ")), nil
	case ast.UniqueConstraint:
		if constraint.Name != "" {
			return fmt.Sprintf("  CONSTRAINT %s UNIQUE (%s)", constraint.Name, strings.Join(constraint.Columns, ", ")), nil
		}
		return fmt.Sprintf("  UNIQUE (%s)", strings.Join(constraint.Columns, ", ")), nil
	case ast.ForeignKeyConstraint:
		return r.renderForeignKeyConstraint(constraint)
	case ast.CheckConstraint:
		if constraint.Name != "" {
			return fmt.Sprintf("  CONSTRAINT %s CHECK (%s)", constraint.Name, constraint.Expression), nil
		}
		return fmt.Sprintf("  CHECK (%s)", constraint.Expression), nil
	default:
		return "", fmt.Errorf("unknown constraint type: %v", constraint.Type)
	}
}

// renderForeignKeyConstraint renders a foreign key constraint
func (r *Renderer) renderForeignKeyConstraint(constraint *ast.ConstraintNode) (string, error) {
	if constraint.Reference == nil {
		return "", fmt.Errorf("foreign key constraint missing reference")
	}

	ref := constraint.Reference
	result := fmt.Sprintf("  CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s(%s)",
		constraint.Name,
		strings.Join(constraint.Columns, ", "),
		ref.Table,
		ref.Column)

	if ref.OnDelete != "" {
		result += fmt.Sprintf(" ON DELETE %s", ref.OnDelete)
	}

	if ref.OnUpdate != "" {
		result += fmt.Sprintf(" ON UPDATE %s", ref.OnUpdate)
	}

	return result, nil
}

// renderColumnWithEnums renders a column with enum support for MariaDB
func (r *Renderer) renderColumnWithEnums(column *ast.ColumnNode, enumValues []string) (string, error) {
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
		if column.Default.Expression != "" {
			parts = append(parts, fmt.Sprintf("DEFAULT %s", column.Default.Expression))
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

// VisitAlterTableWithEnums renders MariaDB-specific ALTER TABLE statements with enum support
func (r *Renderer) visitAlterTableWithEnums(node *ast.AlterTableNode, enums map[string][]string) error {
	r.w.WriteLine("-- ALTER statements: --")

	for _, operation := range node.Operations {
		switch op := operation.(type) {
		case *ast.AddColumnOperation:
			// Get enum values for this column type
			var enumValues []string
			if enums != nil {
				enumValues = enums[op.Column.Type]
			}

			line, err := r.renderColumnWithEnums(op.Column, enumValues)
			if err != nil {
				return fmt.Errorf("error rendering add column: %w", err)
			}
			// Remove the leading spaces from column rendering for ALTER
			line = strings.TrimPrefix(line, "  ")
			r.w.WriteLinef("ALTER TABLE %s ADD COLUMN %s;", node.Name, line)

		case *ast.DropColumnOperation:
			r.w.WriteLinef("ALTER TABLE %s DROP COLUMN %s;", node.Name, op.ColumnName)

		case *ast.ModifyColumnOperation:
			// Get enum values for this column type
			var enumValues []string
			if enums != nil {
				enumValues = enums[op.Column.Type]
			}

			// MariaDB uses MODIFY COLUMN syntax like MySQL
			line, err := r.renderColumnWithEnums(op.Column, enumValues)
			if err != nil {
				return fmt.Errorf("error rendering modify column: %w", err)
			}
			// Remove the leading spaces from column rendering for ALTER
			line = strings.TrimPrefix(line, "  ")
			r.w.WriteLinef("ALTER TABLE %s MODIFY COLUMN %s;", node.Name, line)

		default:
			return fmt.Errorf("unknown alter operation type: %T", operation)
		}
	}

	r.w.WriteLine("")
	return nil
}
