package postgres

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

// Renderer provides PostgreSQL-specific SQL rendering
type Renderer struct {
	// currentEnums stores enum names available in the current rendering context
	currentEnums []string
	dialect      string
	dialectUpper string
	w            bufwriter.Writer
}

func (r *Renderer) VisitDropIndex(node *ast.DropIndexNode) error {
	// Build DROP INDEX statement for PostgreSQL
	var parts []string
	parts = append(parts, "DROP INDEX")

	if node.IfExists {
		parts = append(parts, "IF EXISTS")
	}

	parts = append(parts, node.Name)

	// PostgreSQL doesn't require table name in DROP INDEX
	// but we can add CASCADE if needed
	// Note: node.Table is ignored for PostgreSQL

	sql := strings.Join(parts, " ") + ";"

	// Add comment if provided
	if node.Comment != "" {
		r.w.WriteLinef("-- %s", node.Comment)
	}

	r.w.WriteLine(sql)
	return nil
}

func (r *Renderer) VisitCreateType(node *ast.CreateTypeNode) error {
	// Add comment if provided
	if node.Comment != "" {
		r.w.WriteLinef("-- %s", node.Comment)
	}

	// Handle different type definitions
	switch typeDef := node.TypeDef.(type) {
	case *ast.EnumTypeDef:
		// CREATE TYPE name AS ENUM (value1, value2, ...)
		values := make([]string, len(typeDef.Values))
		for i, value := range typeDef.Values {
			values[i] = fmt.Sprintf("'%s'", value)
		}
		r.w.WriteLinef("CREATE TYPE %s AS ENUM (%s);", node.Name, strings.Join(values, ", "))

	case *ast.CompositeTypeDef:
		// CREATE TYPE name AS (field1 type1, field2 type2, ...)
		fields := make([]string, len(typeDef.Fields))
		for i, field := range typeDef.Fields {
			fields[i] = fmt.Sprintf("%s %s", field.Name, field.Type)
		}
		r.w.WriteLinef("CREATE TYPE %s AS (%s);", node.Name, strings.Join(fields, ", "))

	case *ast.DomainTypeDef:
		// CREATE DOMAIN name AS base_type [NOT NULL] [DEFAULT value] [CHECK (constraint)]
		sql := fmt.Sprintf("CREATE DOMAIN %s AS %s", node.Name, typeDef.BaseType)

		// Add NOT NULL if specified
		if !typeDef.Nullable {
			sql += " NOT NULL"
		}

		// Add DEFAULT if specified
		if typeDef.Default != nil {
			if typeDef.Default.Value != "" {
				sql += fmt.Sprintf(" DEFAULT '%s'", typeDef.Default.Value)
			} else if typeDef.Default.Expression != "" {
				sql += fmt.Sprintf(" DEFAULT %s", typeDef.Default.Expression)
			}
		}

		// Add CHECK constraint if specified
		if typeDef.Check != "" {
			sql += fmt.Sprintf(" CHECK (%s)", typeDef.Check)
		}

		r.w.WriteLinef("%s;", sql)

	default:
		return fmt.Errorf("unsupported type definition: %T", typeDef)
	}

	return nil
}

func (r *Renderer) VisitAlterType(node *ast.AlterTypeNode) error {
	// Process each operation
	for _, operation := range node.Operations {
		switch op := operation.(type) {
		case *ast.AddEnumValueOperation:
			// ALTER TYPE name ADD VALUE 'new_value' [BEFORE 'existing_value' | AFTER 'existing_value']
			sql := fmt.Sprintf("ALTER TYPE %s ADD VALUE '%s'", node.Name, op.Value)

			if op.Before != "" {
				sql += fmt.Sprintf(" BEFORE '%s'", op.Before)
			} else if op.After != "" {
				sql += fmt.Sprintf(" AFTER '%s'", op.After)
			}

			r.w.WriteLinef("%s;", sql)

		case *ast.RenameEnumValueOperation:
			// ALTER TYPE name RENAME VALUE 'old_value' TO 'new_value'
			r.w.WriteLinef("ALTER TYPE %s RENAME VALUE '%s' TO '%s';",
				node.Name, op.OldValue, op.NewValue)

		case *ast.RenameTypeOperation:
			// ALTER TYPE name RENAME TO new_name
			r.w.WriteLinef("ALTER TYPE %s RENAME TO %s;", node.Name, op.NewName)

		default:
			return fmt.Errorf("unsupported alter type operation: %T", operation)
		}
	}

	return nil
}

// New creates a new PostgreSQL renderer
func New() *Renderer {
	return &Renderer{
		currentEnums: nil,
		dialect:      "postgres",
		dialectUpper: "POSTGRES",
	}
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

// VisitCreateTable renders CREATE TABLE with PostgreSQL-specific handling
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
			r.w.WriteLine(line) // Last line without comma
		} else {
			r.w.WriteLinef("%s,", line)
		}
	}

	r.w.Write(");")

	// Table options (PostgreSQL-specific filtering applied)
	if len(node.Options) > 0 {
		r.w.Write(" ")
		r.w.Write(r.renderTableOptions(node.Options))
	}

	r.w.WriteLine("")
	r.w.WriteLine("")

	return nil
}

// VisitAlterTable renders PostgreSQL-specific ALTER TABLE statements
func (r *Renderer) VisitAlterTable(node *ast.AlterTableNode) error {
	r.w.WriteLine("-- ALTER statements: --")

	for _, operation := range node.Operations {
		switch op := operation.(type) {
		case *ast.AddColumnOperation:
			line, err := r.renderColumn(op.Column)
			if err != nil {
				return fmt.Errorf("error rendering add column: %w", err)
			}
			// Remove the leading spaces from column rendering for ALTER
			line = strings.TrimPrefix(line, "  ")
			r.w.WriteLinef("ALTER TABLE %s ADD COLUMN %s;", node.Name, line)
		case *ast.DropColumnOperation:
			r.w.WriteLinef("ALTER TABLE %s DROP COLUMN %s;", node.Name, op.ColumnName)
		case *ast.ModifyColumnOperation:
			// PostgreSQL uses different syntax for modifying columns
			r.renderPostgreSQLModifyColumn(node.Name, op.Column)
		default:
			return fmt.Errorf("unknown alter operation type: %T", operation)
		}
	}

	r.w.WriteLine("")

	return nil
}

func (r *Renderer) VisitColumn(node *ast.ColumnNode) error {
	// This is typically called from within other visitors
	// The actual rendering is done by RenderColumn
	return nil
}

func (r *Renderer) VisitConstraint(node *ast.ConstraintNode) error {
	// This is typically called from within other visitors
	// The actual rendering is done by RenderConstraint
	return nil
}

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

// VisitEnum renders CREATE TYPE ... AS ENUM for PostgreSQL
func (r *Renderer) VisitEnum(node *ast.EnumNode) error {
	values := make([]string, len(node.Values))
	for i, value := range node.Values {
		values[i] = fmt.Sprintf("'%s'", value)
	}

	r.w.WriteLinef("CREATE TYPE %s AS ENUM (%s);", node.Name, strings.Join(values, ", "))
	return nil
}

// VisitComment renders a comment
func (r *Renderer) VisitComment(node *ast.CommentNode) error {
	r.w.WriteLinef("-- %s --", node.Text)
	return nil
}

func (r *Renderer) VisitDropTable(node *ast.DropTableNode) error {
	// Build DROP TABLE statement with PostgreSQL-specific features
	var parts []string
	parts = append(parts, "DROP TABLE")

	if node.IfExists {
		parts = append(parts, "IF EXISTS")
	}

	parts = append(parts, node.Name)

	if node.Cascade {
		parts = append(parts, "CASCADE")
	}

	sql := strings.Join(parts, " ") + ";"

	// Add comment if provided
	if node.Comment != "" {
		r.w.WriteLinef("-- %s", node.Comment)
	}

	r.w.WriteLine(sql)
	return nil
}

// VisitDropType renders PostgreSQL-specific DROP TYPE statements
func (r *Renderer) VisitDropType(node *ast.DropTypeNode) error {
	// Build DROP TYPE statement (PostgreSQL-specific)
	var parts []string
	parts = append(parts, "DROP TYPE")

	if node.IfExists {
		parts = append(parts, "IF EXISTS")
	}

	parts = append(parts, node.Name)

	if node.Cascade {
		parts = append(parts, "CASCADE")
	}

	sql := strings.Join(parts, " ") + ";"

	// Add comment if provided
	if node.Comment != "" {
		r.w.WriteLinef("-- %s", node.Comment)
	}

	r.w.WriteLine(sql)
	return nil
}

// renderColumn overrides base column rendering with PostgreSQL-specific handling
func (r *Renderer) renderColumn(column *ast.ColumnNode) (string, error) {
	var parts []string

	// Handle PostgreSQL-specific type conversions using current enum context
	columnType := r.processFieldType(column.Type, r.currentEnums)

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

// processFieldType processes field type for PostgreSQL, handling enums appropriately
func (r *Renderer) processFieldType(fieldType string, enums []string) string {
	// For PostgreSQL, enum types are used directly (they're defined separately)
	// Check if this type is an enum using the helper method
	if r.isEnumType(fieldType, enums) {
		return fieldType // Use enum type directly
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

// Helper method to check if a type is an enum
func (r *Renderer) isEnumType(fieldType string, enums []string) bool {
	for _, enumName := range enums {
		if fieldType == enumName {
			return true
		}
	}
	return false
}

// RenderConstraint renders a table-level constraint
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

// renderTableOptions renders PostgreSQL table options (PostgreSQL doesn't support ENGINE)
func (r *Renderer) renderTableOptions(options map[string]string) string {
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

// renderPostgreSQLModifyColumn renders PostgreSQL-specific column modifications
func (r *Renderer) renderPostgreSQLModifyColumn(tableName string, column *ast.ColumnNode) {
	// PostgreSQL requires separate ALTER statements for different column properties

	// Process the column type with enum support
	columnType := r.processFieldType(column.Type, r.currentEnums)

	// Change column type (with USING clause for complex conversions if needed)
	if columnType != column.Type {
		// Type was transformed (e.g., enum handling), use the processed type
		r.w.WriteLinef("ALTER TABLE %s ALTER COLUMN %s TYPE %s;", tableName, column.Name, columnType)
	} else {
		r.w.WriteLinef("ALTER TABLE %s ALTER COLUMN %s TYPE %s;", tableName, column.Name, column.Type)
	}

	// Change nullability
	if column.Nullable {
		r.w.WriteLinef("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL;", tableName, column.Name)
	} else {
		r.w.WriteLinef("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;", tableName, column.Name)
	}

	// Change default value
	switch {
	case column.Default == nil:
		r.w.WriteLinef("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT;", tableName, column.Name)
	case column.Default.Value != "":
		r.w.WriteLinef("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT '%s';", tableName, column.Name, column.Default.Value) // TODO: escape!
	case column.Default.Expression != "":
		r.w.WriteLinef("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s;", tableName, column.Name, column.Default.Expression)
	}
}
