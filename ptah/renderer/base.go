package renderer

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/ptah/core/ast"
)

// BaseRenderer provides common SQL rendering functionality
type BaseRenderer struct {
	dialect string
	output  strings.Builder
}

// NewBaseRenderer creates a new base renderer
func NewBaseRenderer(dialect string) *BaseRenderer {
	return &BaseRenderer{
		dialect: dialect,
	}
}

// GetOutput returns the generated SQL
func (r *BaseRenderer) GetOutput() string {
	return r.output.String()
}

// Reset clears the output buffer
func (r *BaseRenderer) Reset() {
	r.output.Reset()
}

// Write writes a string to the output
func (r *BaseRenderer) Write(s string) {
	r.output.WriteString(s)
}

// Writef writes a formatted string to the output
func (r *BaseRenderer) Writef(format string, args ...any) {
	fmt.Fprintf(&r.output, format, args...)
}

// WriteLine writes a string followed by a newline
func (r *BaseRenderer) WriteLine(s string) {
	r.output.WriteString(s)
	r.output.WriteString("\n")
}

// WriteLinef writes a formatted string followed by a newline
func (r *BaseRenderer) WriteLinef(format string, args ...any) {
	fmt.Fprintf(&r.output, format, args...)
	r.output.WriteString("\n")
}

// VisitComment renders a comment
func (r *BaseRenderer) VisitComment(node *ast.CommentNode) error {
	r.WriteLinef("-- %s --", node.Text)
	return nil
}

// VisitCreateTable renders a CREATE TABLE statement
func (r *BaseRenderer) VisitCreateTable(node *ast.CreateTableNode) error {
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
	r.WriteLine("")

	return nil
}

// renderColumn renders a column definition
func (r *BaseRenderer) renderColumn(column *ast.ColumnNode) (string, error) {
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

// renderConstraint renders a table-level constraint
func (r *BaseRenderer) renderConstraint(constraint *ast.ConstraintNode) (string, error) {
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
func (r *BaseRenderer) renderForeignKeyConstraint(constraint *ast.ConstraintNode) (string, error) {
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

// renderTableOptions renders table options (can be overridden by dialects)
func (r *BaseRenderer) renderTableOptions(options map[string]string) string {
	var parts []string
	for key, value := range options {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(parts, " ")
}

// renderAutoIncrement renders auto increment (dialect-specific, override in subclasses)
func (r *BaseRenderer) renderAutoIncrement() string {
	return "AUTO_INCREMENT" // Default MySQL/MariaDB style
}

// VisitAlterTable renders ALTER TABLE statements
func (r *BaseRenderer) VisitAlterTable(node *ast.AlterTableNode) error {
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
			line, err := r.renderColumn(op.Column)
			if err != nil {
				return fmt.Errorf("error rendering modify column: %w", err)
			}
			// Remove the leading spaces from column rendering for ALTER
			line = strings.TrimPrefix(line, "  ")
			r.WriteLinef("ALTER TABLE %s ALTER COLUMN %s;", node.Name, line)

		default:
			return fmt.Errorf("unknown alter operation type: %T", operation)
		}
	}

	r.WriteLine("")
	return nil
}

// VisitColumn is called when visiting individual columns (used by other visitors)
func (r *BaseRenderer) VisitColumn(node *ast.ColumnNode) error {
	// This is typically called from within other visitors
	// The actual rendering is done by renderColumn
	return nil
}

// VisitConstraint is called when visiting individual constraints (used by other visitors)
func (r *BaseRenderer) VisitConstraint(node *ast.ConstraintNode) error {
	// This is typically called from within other visitors
	// The actual rendering is done by renderConstraint
	return nil
}

// VisitIndex renders a CREATE INDEX statement
func (r *BaseRenderer) VisitIndex(node *ast.IndexNode) error {
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

// VisitEnum renders enum creation (base implementation does nothing)
func (r *BaseRenderer) VisitEnum(node *ast.EnumNode) error {
	// Base implementation does nothing - enums are dialect-specific
	// PostgreSQL will override this, MySQL/MariaDB handle enums differently
	return nil
}

// Render renders an AST node to SQL
func (r *BaseRenderer) Render(node ast.Node) (string, error) {
	r.Reset()
	if err := node.Accept(r); err != nil {
		return "", err
	}
	return r.GetOutput(), nil
}
