package renderer

import (
	"fmt"
	"strings"

	"github.com/denisvmedia/inventario/ptah/schema/ast"
)

// PostgreSQLRenderer provides PostgreSQL-specific SQL rendering
type PostgreSQLRenderer struct {
	*BaseRenderer
}

// Ensure PostgreSQLRenderer implements the Visitor interface
var _ ast.Visitor = (*PostgreSQLRenderer)(nil)

// NewPostgreSQLRenderer creates a new PostgreSQL renderer
func NewPostgreSQLRenderer() *PostgreSQLRenderer {
	return &PostgreSQLRenderer{
		BaseRenderer: NewBaseRenderer("postgres"),
	}
}

// VisitEnum renders CREATE TYPE ... AS ENUM for PostgreSQL
func (r *PostgreSQLRenderer) VisitEnum(node *ast.EnumNode) error {
	values := make([]string, len(node.Values))
	for i, value := range node.Values {
		values[i] = fmt.Sprintf("'%s'", value)
	}

	r.WriteLinef("CREATE TYPE %s AS ENUM (%s);", node.Name, strings.Join(values, ", "))
	return nil
}

// renderAutoIncrement renders PostgreSQL auto increment
func (r *PostgreSQLRenderer) renderAutoIncrement() string {
	// PostgreSQL uses SERIAL/BIGSERIAL types instead of AUTO_INCREMENT
	// This is typically handled at the type level, so we return empty string
	return ""
}

// VisitAlterTable renders PostgreSQL-specific ALTER TABLE statements
func (r *PostgreSQLRenderer) VisitAlterTable(node *ast.AlterTableNode) error {
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
			// PostgreSQL uses different syntax for modifying columns
			r.renderPostgreSQLModifyColumn(node.Name, op.Column)

		default:
			return fmt.Errorf("unknown alter operation type: %T", operation)
		}
	}

	r.WriteLine("")
	return nil
}

// renderPostgreSQLModifyColumn renders PostgreSQL-specific column modifications
func (r *PostgreSQLRenderer) renderPostgreSQLModifyColumn(tableName string, column *ast.ColumnNode) {
	// PostgreSQL requires separate ALTER statements for different column properties

	// Change column type
	r.WriteLinef("ALTER TABLE %s ALTER COLUMN %s TYPE %s;", tableName, column.Name, column.Type)

	// Change nullability
	if column.Nullable {
		r.WriteLinef("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL;", tableName, column.Name)
	} else {
		r.WriteLinef("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;", tableName, column.Name)
	}

	// Change default value
	if column.Default != nil {
		if column.Default.Function != "" {
			r.WriteLinef("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s;", tableName, column.Name, column.Default.Function)
		} else if column.Default.Value != "" {
			r.WriteLinef("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT '%s';", tableName, column.Name, column.Default.Value)
		}
	} else {
		r.WriteLinef("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT;", tableName, column.Name)
	}
}

// renderTableOptions renders PostgreSQL table options (PostgreSQL doesn't support ENGINE)
func (r *PostgreSQLRenderer) renderTableOptions(options map[string]string) string {
	// PostgreSQL doesn't support table options like MySQL's ENGINE
	// We could support other PostgreSQL-specific options here if needed
	var parts []string

	for key, value := range options {
		// Skip MySQL-specific options
		if key == "ENGINE" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}

	return strings.Join(parts, " ")
}

// processFieldType processes field type for PostgreSQL, handling enums appropriately
func (r *PostgreSQLRenderer) processFieldType(fieldType string, enums []string) string {
	// For PostgreSQL, enum types are used directly (they're defined separately)
	// Check if this type is an enum
	for _, enumName := range enums {
		if fieldType == enumName {
			return fieldType // Use enum type directly
		}
	}

	// Handle other PostgreSQL-specific type mappings if needed
	switch fieldType {
	case "AUTO_INCREMENT":
		return "SERIAL"
	case "BIGINT AUTO_INCREMENT":
		return "BIGSERIAL"
	default:
		return fieldType
	}
}

// VisitCreateTable renders CREATE TABLE with PostgreSQL-specific handling
func (r *PostgreSQLRenderer) VisitCreateTable(node *ast.CreateTableNode) error {
	// Table comment
	if node.Comment != "" {
		r.WriteLinef("-- %s TABLE: %s (%s) --", strings.ToUpper(r.dialect), node.Name, node.Comment)
	} else {
		r.WriteLinef("-- %s TABLE: %s --", strings.ToUpper(r.dialect), node.Name)
	}

	// CREATE TABLE statement
	r.WriteLinef("CREATE TABLE %s (", node.Name)

	var lines []string

	// Render columns using PostgreSQL-specific column rendering
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

	// Convert column-level foreign keys to table-level constraints
	for _, column := range node.Columns {
		if column.ForeignKey != nil {
			fk := column.ForeignKey
			constraint := &ast.ConstraintNode{
				Type:    ast.ForeignKeyConstraint,
				Name:    fk.Name,
				Columns: []string{column.Name},
				Reference: &ast.ForeignKeyRef{
					Table:    fk.Table,
					Column:   fk.Column,
					OnDelete: fk.OnDelete,
					OnUpdate: fk.OnUpdate,
					Name:     fk.Name,
				},
			}
			line, err := r.renderConstraint(constraint)
			if err != nil {
				return fmt.Errorf("error rendering foreign key constraint: %w", err)
			}
			if line != "" {
				lines = append(lines, line)
			}
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

	r.WriteLine(");")
	r.WriteLine("")
	return nil
}

// VisitColumn delegates to base implementation
func (r *PostgreSQLRenderer) VisitColumn(node *ast.ColumnNode) error {
	return r.BaseRenderer.VisitColumn(node)
}

// VisitConstraint delegates to base implementation
func (r *PostgreSQLRenderer) VisitConstraint(node *ast.ConstraintNode) error {
	return r.BaseRenderer.VisitConstraint(node)
}

// VisitIndex delegates to base implementation
func (r *PostgreSQLRenderer) VisitIndex(node *ast.IndexNode) error {
	return r.BaseRenderer.VisitIndex(node)
}

// VisitComment delegates to base implementation
func (r *PostgreSQLRenderer) VisitComment(node *ast.CommentNode) error {
	return r.BaseRenderer.VisitComment(node)
}

// Render overrides the base Render method to ensure proper method resolution
func (r *PostgreSQLRenderer) Render(node ast.Node) (string, error) {
	r.Reset()
	if err := node.Accept(r); err != nil {
		return "", err
	}
	return r.GetOutput(), nil
}

// RenderSchema renders a complete schema with proper ordering for PostgreSQL
func (r *PostgreSQLRenderer) RenderSchema(statements *ast.StatementList) (string, error) {
	r.Reset()

	// First pass: render all enums
	for _, stmt := range statements.Statements {
		if enum, ok := stmt.(*ast.EnumNode); ok {
			if err := r.VisitEnum(enum); err != nil {
				return "", fmt.Errorf("error rendering enum %s: %w", enum.Name, err)
			}
		}
	}

	// Add separator if we rendered any enums
	hasEnums := false
	for _, stmt := range statements.Statements {
		if _, ok := stmt.(*ast.EnumNode); ok {
			hasEnums = true
			break
		}
	}
	if hasEnums {
		r.WriteLine("")
	}

	// Second pass: render everything else
	for _, stmt := range statements.Statements {
		// Skip enums as they were already rendered
		if _, ok := stmt.(*ast.EnumNode); ok {
			continue
		}

		if err := stmt.Accept(r); err != nil {
			return "", fmt.Errorf("error rendering statement: %w", err)
		}
	}

	return r.GetOutput(), nil
}

// Helper method to check if a type is an enum
func (r *PostgreSQLRenderer) isEnumType(fieldType string, enums []string) bool {
	for _, enumName := range enums {
		if fieldType == enumName {
			return true
		}
	}
	return false
}

// renderColumn overrides base column rendering with PostgreSQL-specific handling
func (r *PostgreSQLRenderer) renderColumn(column *ast.ColumnNode) (string, error) {
	var parts []string

	// Handle PostgreSQL-specific type conversions
	columnType := r.processFieldType(column.Type, nil) // TODO: Pass actual enums if needed

	// Column name and type
	parts = append(parts, fmt.Sprintf("  %s %s", column.Name, columnType))

	// Column constraints - PostgreSQL order: PRIMARY KEY, then NOT NULL, then UNIQUE
	if column.Primary {
		parts = append(parts, "PRIMARY KEY")
		// Primary keys are always NOT NULL in PostgreSQL, show it explicitly for schema comparison
		parts = append(parts, "NOT NULL")
	} else {
		if column.Unique {
			parts = append(parts, "UNIQUE")
		}
		if !column.Nullable {
			parts = append(parts, "NOT NULL")
		}
	}

	// PostgreSQL doesn't use AUTO_INCREMENT keyword - it's handled by SERIAL types
	// So we skip the auto increment rendering

	// Default value
	if column.Default != nil {
		if column.Default.Function != "" {
			parts = append(parts, fmt.Sprintf("DEFAULT %s", column.Default.Function))
		} else if column.Default.Value != "" {
			parts = append(parts, fmt.Sprintf("DEFAULT '%s'", column.Default.Value))
		}
	}

	// Check constraint
	if column.Check != "" {
		parts = append(parts, fmt.Sprintf("CHECK (%s)", column.Check))
	}

	return strings.Join(parts, " "), nil
}
