// Package ast provides an Abstract Syntax Tree (AST) representation for SQL DDL statements.
//
// This package is a core component of the Ptah schema management tool, implementing
// the visitor pattern to enable dialect-specific SQL generation from a common AST
// representation. It supports CREATE TABLE, ALTER TABLE, CREATE INDEX, and other
// DDL operations across multiple database platforms.
//
// # Architecture
//
// The package follows the visitor pattern with these key components:
//
//   - Node: Base interface for all AST nodes
//   - Visitor: Interface for traversing and rendering AST nodes
//   - Concrete node types: CreateTableNode, AlterTableNode, ColumnNode, etc.
//   - Helper types: DefaultValue, ForeignKeyRef, ConstraintType
//
// # Core Node Types
//
// The package provides several node types representing different SQL constructs:
//
//   - CreateTableNode: Represents CREATE TABLE statements with columns and constraints
//   - AlterTableNode: Represents ALTER TABLE statements with various operations
//   - ColumnNode: Represents table column definitions with all attributes
//   - ConstraintNode: Represents table-level constraints (PK, FK, UNIQUE, CHECK)
//   - IndexNode: Represents CREATE INDEX statements
//   - EnumNode: Represents CREATE TYPE ... AS ENUM statements (PostgreSQL)
//   - CommentNode: Represents SQL comments
//
// # Visitor Pattern
//
// The visitor pattern enables dialect-specific rendering without modifying the AST nodes.
// Each node implements the Accept method that calls the appropriate visitor method:
//
//	type Visitor interface {
//		VisitCreateTable(*CreateTableNode) error
//		VisitAlterTable(*AlterTableNode) error
//		VisitColumn(*ColumnNode) error
//		VisitConstraint(*ConstraintNode) error
//		VisitIndex(*IndexNode) error
//		VisitEnum(*EnumNode) error
//		VisitComment(*CommentNode) error
//	}
//
// # Usage Example
//
// Creating a table with columns and constraints:
//
//	table := ast.NewCreateTable("users").
//		AddColumn(
//			ast.NewColumn("id", "INTEGER").
//				SetPrimary().
//				SetAutoIncrement(),
//		).
//		AddColumn(
//			ast.NewColumn("email", "VARCHAR(255)").
//				SetNotNull().
//				SetUnique(),
//		).
//		AddColumn(
//			ast.NewColumn("created_at", "TIMESTAMP").
//				SetDefaultExpression("CURRENT_TIMESTAMP"),
//		).
//		AddConstraint(ast.NewUniqueConstraint("uk_users_email", "email"))
//
// Rendering with a visitor:
//
//	renderer := postgresql.NewRenderer()
//	sql, err := renderer.Render(table)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Println(sql)
//
// # Fluent API
//
// Most node types provide a fluent API for easy construction:
//
//	column := ast.NewColumn("status", "VARCHAR(20)").
//		SetNotNull().
//		SetDefault("'active'").
//		SetCheck("status IN ('active', 'inactive')")
//
// # Database Dialect Support
//
// The AST is designed to be database-agnostic, with dialect-specific rendering
// handled by visitor implementations. This allows the same AST to generate
// SQL for PostgreSQL, MySQL, MariaDB, and other databases.
//
// # Integration with Ptah
//
// This package integrates with other Ptah components:
//
//   - ptah/schema/builder: Converts parsed Go structs to AST nodes
//   - ptah/schema/renderer: Provides dialect-specific SQL rendering
//   - ptah/schema/differ: Compares AST representations for migration generation
//
// The AST serves as the central representation that bridges code parsing,
// schema comparison, and SQL generation in the Ptah ecosystem.
package ast
