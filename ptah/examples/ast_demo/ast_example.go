package ast_demo

import (
	"fmt"

	builder2 "github.com/denisvmedia/inventario/ptah/core/builder"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/renderer/dialects/mysql"
	"github.com/denisvmedia/inventario/ptah/renderer/dialects/postgresql"
)

// DemonstrateASTApproach shows how to use the new AST-based SQL generation
func DemonstrateASTApproach() {
	fmt.Println("=== AST-Based SQL Generation Example ===")

	// Example 1: Building a simple table using the fluent API
	fmt.Println("1. Simple table using fluent API:")

	table := builder2.NewTable("users").
		Comment("User accounts table").
		Column("id", "SERIAL").Primary().End().
		Column("email", "VARCHAR(255)").NotNull().Unique().End().
		Column("name", "VARCHAR(100)").NotNull().End().
		Column("created_at", "TIMESTAMP").DefaultExpression("NOW()").End().
		Column("is_active", "BOOLEAN").Default("true").End().
		Build()

	// Render for PostgreSQL
	pgRenderer := postgresql.NewPostgreSQLRenderer()
	pgSQL, err := pgRenderer.Render(table)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("PostgreSQL:")
	fmt.Println(pgSQL)

	// Render for MySQL
	mysqlRenderer := mysql.NewMySQLRenderer()
	mysqlSQL, err := mysqlRenderer.Render(table)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("MySQL:")
	fmt.Println(mysqlSQL)

	// Example 2: Building a more complex schema with enums and foreign keys
	fmt.Println("\n2. Complex schema with enums and foreign keys:")

	schema := builder2.NewSchema().
		Comment("E-commerce database schema").
		Enum("order_status", "pending", "processing", "shipped", "delivered", "cancelled").
		Table("categories").
		Column("id", "SERIAL").Primary().End().
		Column("name", "VARCHAR(100)").NotNull().Unique().End().
		Column("description", "TEXT").End().
		End().
		Table("products").
		Column("id", "SERIAL").Primary().End().
		Column("name", "VARCHAR(255)").NotNull().End().
		Column("price", "DECIMAL(10,2)").NotNull().Check("price > 0").End().
		Column("category_id", "INTEGER").NotNull().
		ForeignKey("categories", "id", "fk_products_category").
		OnDelete("CASCADE").
		End().
		End().
		Table("orders").
		Column("id", "SERIAL").Primary().End().
		Column("user_id", "INTEGER").NotNull().End().
		Column("status", "order_status").NotNull().Default("pending").End().
		Column("total", "DECIMAL(10,2)").NotNull().End().
		Column("created_at", "TIMESTAMP").DefaultExpression("NOW()").End().
		End().
		Index("idx_orders_user_id", "orders", "user_id").End().
		Index("idx_orders_status", "orders", "status").End().
		Build()

	// Render complete schema for PostgreSQL
	pgSchemaSQL, err := pgRenderer.RenderSchema(schema)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("PostgreSQL Schema:")
	fmt.Println(pgSchemaSQL)

	// Example 3: Building ALTER statements
	fmt.Println("\n3. ALTER statements using AST:")

	alterTable := &ast.AlterTableNode{
		Name: "users",
		Operations: []ast.AlterOperation{
			&ast.AddColumnOperation{
				Column: ast.NewColumn("phone", "VARCHAR(20)").SetNotNull(),
			},
			&ast.ModifyColumnOperation{
				Column: ast.NewColumn("email", "VARCHAR(320)").SetNotNull().SetUnique(),
			},
			&ast.DropColumnOperation{
				ColumnName: "is_active",
			},
		},
	}

	alterSQL, err := pgRenderer.Render(alterTable)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("PostgreSQL ALTER statements:")
	fmt.Println(alterSQL)

	// Example 4: Direct AST construction (lower-level approach)
	fmt.Println("\n4. Direct AST construction:")

	directTable := ast.NewCreateTable("products").
		AddColumn(
			ast.NewColumn("id", "SERIAL").SetPrimary(),
		).
		AddColumn(
			ast.NewColumn("name", "VARCHAR(255)").SetNotNull(),
		).
		AddColumn(
			ast.NewColumn("price", "DECIMAL(10,2)").SetNotNull().SetCheck("price > 0"),
		).
		AddConstraint(
			ast.NewUniqueConstraint("uk_products_name", "name"),
		)

	directSQL, err := pgRenderer.Render(directTable)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Direct AST construction result:")
	fmt.Println(directSQL)
}

// CompareApproaches demonstrates the difference between old and new approaches
func CompareApproaches() {
	fmt.Println("=== Comparison: Old vs New Approach ===")

	fmt.Println("OLD APPROACH (string-based):")
	fmt.Printf(`
// Hard to read, error-prone, not composable
func generateOldWay() string {
    var buf strings.Builder
    fmt.Fprintf(&buf, "-- %%s TABLE: %%s --\n", "POSTGRES", "users")
    fmt.Fprintf(&buf, "CREATE TABLE %%s (\n", "users")
    fmt.Fprintf(&buf, "  %%s %%s PRIMARY KEY,\n", "id", "SERIAL")
    fmt.Fprintf(&buf, "  %%s %%s NOT NULL UNIQUE,\n", "email", "VARCHAR(255)")
    fmt.Fprintf(&buf, "  %%s %%s NOT NULL\n", "name", "VARCHAR(100)")
    fmt.Fprintf(&buf, ");\n")
    return buf.String()
}`)

	fmt.Println("\nNEW APPROACH (AST-based):")
	fmt.Println(`
// Readable, type-safe, composable, testable
func generateNewWay() string {
    table := builder.NewTable("users").
        Column("id", "SERIAL").Primary().End().
        Column("email", "VARCHAR(255)").NotNull().Unique().End().
        Column("name", "VARCHAR(100)").NotNull().End().
        Build()

    renderer := renderers.NewPostgreSQLRenderer()
    sql, _ := renderer.Render(table)
    return sql
}`)

	fmt.Println("\nBENEFITS OF AST APPROACH:")
	fmt.Println("✓ Type safety - compile-time validation")
	fmt.Println("✓ Readability - fluent API mirrors SQL structure")
	fmt.Println("✓ Composability - build complex schemas from parts")
	fmt.Println("✓ Testability - easy to unit test individual components")
	fmt.Println("✓ Maintainability - changes in one place affect all dialects")
	fmt.Println("✓ Extensibility - easy to add new SQL features")
	fmt.Println("✓ Dialect independence - same AST, different renderers")
}

// ShowAdvancedFeatures demonstrates advanced AST capabilities
func ShowAdvancedFeatures() {
	fmt.Println("=== Advanced AST Features ===")

	// Custom visitor for SQL analysis
	fmt.Println("1. Custom visitor for analyzing schema:")

	analyzer := &SchemaAnalyzer{
		TableCount:  0,
		ColumnCount: 0,
		IndexCount:  0,
	}

	schema := builder2.NewSchema().
		Table("users").
		Column("id", "SERIAL").Primary().End().
		Column("email", "VARCHAR(255)").NotNull().End().
		End().
		Table("posts").
		Column("id", "SERIAL").Primary().End().
		Column("user_id", "INTEGER").End().
		End().
		Index("idx_posts_user", "posts", "user_id").End().
		Build()

	schema.Accept(analyzer)
	fmt.Printf("Analysis result: %d tables, %d columns, %d indexes\n",
		analyzer.TableCount, analyzer.ColumnCount, analyzer.IndexCount)

	// Schema transformation
	fmt.Println("\n2. Schema transformation (add audit columns to all tables):")

	transformer := &AuditTransformer{}
	transformedSchema := transformer.Transform(schema)

	renderer := postgresql.NewPostgreSQLRenderer()
	sql, _ := renderer.RenderSchema(transformedSchema)
	fmt.Println("Transformed schema with audit columns:")
	fmt.Println(sql)
}

// SchemaAnalyzer is a custom visitor that analyzes schema structure
type SchemaAnalyzer struct {
	TableCount  int
	ColumnCount int
	IndexCount  int
}

func (a *SchemaAnalyzer) VisitCreateTable(node *ast.CreateTableNode) error {
	a.TableCount++
	a.ColumnCount += len(node.Columns)
	return nil
}

func (a *SchemaAnalyzer) VisitAlterTable(node *ast.AlterTableNode) error { return nil }
func (a *SchemaAnalyzer) VisitColumn(node *ast.ColumnNode) error         { return nil }
func (a *SchemaAnalyzer) VisitConstraint(node *ast.ConstraintNode) error { return nil }
func (a *SchemaAnalyzer) VisitIndex(node *ast.IndexNode) error {
	a.IndexCount++
	return nil
}
func (a *SchemaAnalyzer) VisitEnum(node *ast.EnumNode) error       { return nil }
func (a *SchemaAnalyzer) VisitComment(node *ast.CommentNode) error { return nil }

// AuditTransformer adds audit columns to all tables
type AuditTransformer struct{}

func (t *AuditTransformer) Transform(schema *ast.StatementList) *ast.StatementList {
	newStatements := make([]ast.Node, 0, len(schema.Statements))

	for _, stmt := range schema.Statements {
		if table, ok := stmt.(*ast.CreateTableNode); ok {
			// Add audit columns to table
			table.AddColumn(ast.NewColumn("created_at", "TIMESTAMP").SetDefaultExpression("NOW()"))
			table.AddColumn(ast.NewColumn("updated_at", "TIMESTAMP").SetDefaultExpression("NOW()"))
			table.AddColumn(ast.NewColumn("created_by", "VARCHAR(100)"))
			table.AddColumn(ast.NewColumn("updated_by", "VARCHAR(100)"))
		}
		newStatements = append(newStatements, stmt)
	}

	return &ast.StatementList{Statements: newStatements}
}
