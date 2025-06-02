# Ptah

**Ptah** is a schema management tool for relational databases, inspired by the ancient Egyptian god of creation. In
mythology, Ptah brought the world into existence through thought and speech—shaping order from chaos. This tool follows
a similar philosophy: it turns structured Go code into coherent, executable database schemas, ensuring consistency
between code and data.

The name **Ptah** is also an acronym:

> **P.T.A.H.** — *Parse, Transform, Apply, Harmonize*

- **Parse** – extract schema definitions from annotated Go structs
- **Transform** – generate SQL DDL and schema diffs
- **Apply** – execute up/down migrations with version tracking
- **Harmonize** – synchronize code-defined schema with actual database state

---

## Key Features

`ptah` provides a unified workflow to define, evolve, and apply database schemas based on Go code annotations. Its main
capabilities include:

- 📘 **Go Struct Parsing**
  Extracts tables, columns, indexes, foreign keys, and constraints from structured comments in Go code.

- 🧱 **Schema Generation (DDL)**
  Builds platform-specific `CREATE TABLE`, `CREATE INDEX`, and other DDL statements.

- 🔍 **Database Introspection**
  Reads the current schema directly from Postgres or MySQL for comparison and analysis.

- 🧮 **Schema Diffing**
  Compares code-based schema with the live database schema using AST representations.

- 🪄 **Migration Generation**
  Automatically generates `up` and `down` SQL migrations to bring the database in sync.

- 🚀 **Migration Execution**
  Applies versioned migrations in both directions, tracking state via a migrations table.

- 💥 **Database Cleaning**
  Drops all user-defined schema objects—useful for testing or re-initialization.

---

## Package Structure

Ptah is organized into several key packages that work together to provide comprehensive database schema management:

### Core Packages

#### `core/` - Core Schema Processing Components
The core package contains all fundamental components for parsing, transforming, and representing database schemas:

- **`ast/`** - Abstract Syntax Tree representation for SQL DDL statements
  - Provides database-agnostic AST nodes for CREATE TABLE, ALTER TABLE, CREATE INDEX, etc.
  - Implements visitor pattern for dialect-specific SQL generation
  - Core node types: `CreateTableNode`, `AlterTableNode`, `ColumnNode`, `ConstraintNode`, `IndexNode`, `EnumNode`

- **`astbuilder/`** - Fluent API for building SQL DDL AST nodes
  - Provides convenient builder pattern for constructing complex schemas
  - SchemaBuilder, TableBuilder, ColumnBuilder, IndexBuilder interfaces
  - Integrates seamlessly with the core AST package

- **`goschema/`** - Go package parsing and entity extraction
  - Recursively parses Go source files to discover entity definitions
  - Extracts table directives, field mappings, indexes, enums, and embedded fields
  - Handles dependency analysis and topological sorting for proper table creation order

- **`parser/`** - SQL DDL token-to-AST parser
  - Converts SQL DDL tokens into Abstract Syntax Tree nodes
  - Supports CREATE TABLE, ALTER TABLE, CREATE INDEX, CREATE TYPE statements
  - Provides comprehensive error handling and position information

- **`lexer/`** - SQL tokenization and lexical analysis
  - Tokenizes SQL input for parser consumption
  - Handles strings, comments, identifiers, operators, and whitespace
  - Provides position tracking for error reporting

- **`renderer/`** - Dialect-specific SQL generation from AST
  - Converts AST nodes to database-specific SQL statements
  - Supports PostgreSQL, MySQL, MariaDB dialects
  - Implements visitor pattern for extensible rendering

- **`platform/`** - Database platform constants and identifiers
  - Defines platform-specific constants used throughout the system
  - Provides standardized platform identification

- **`sqlutil/`** - SQL utility functions
  - SQL statement splitting and comment removal
  - AST-based parsing for proper handling of strings and comments

- **`convert/`** - Schema conversion utilities
  - Converts between different schema representations
  - Handles transformations between goschema and database schema formats

#### `migration/` - Migration Management System
Provides comprehensive database migration functionality:

- **`generator/`** - Dynamic migration file generation
  - Generates up/down migration files from schema differences
  - Compares Go entities with current database state
  - Creates timestamped migration files with proper SQL

- **`migrator/`** - Migration execution engine
  - Applies and rolls back database migrations
  - Tracks migration history and versions
  - Provides dry-run capabilities and transaction safety

- **`planner/`** - Migration planning and SQL generation
  - Converts schema differences into executable SQL statements
  - Dialect-specific planners for PostgreSQL, MySQL, MariaDB
  - Handles dependency ordering and safety checks

- **`schemadiff/`** - Schema comparison and difference analysis
  - Compares generated schemas with live database schemas
  - Identifies tables, columns, indexes, and enum differences
  - Provides detailed change analysis for migration planning

#### `dbschema/` - Database Schema Operations
Handles all database interactions and schema operations:

- **Connection management** for PostgreSQL, MySQL, MariaDB
- **Schema reading and introspection** from live databases
- **Schema writing and migration execution** with transaction support
- **Database cleaning and schema dropping** capabilities
- **Type definitions** for database schema representation

### Command Line Interface

#### `cmd/` - CLI Commands
Provides command-line interface for all Ptah operations:

- **`generate`** - Generate SQL schema from Go entities without touching database
- **`readdb`** - Read and display current database schema
- **`compare`** - Compare Go entities with current database schema
- **`migrate`** - Generate migration SQL for schema differences
- **`migrateup`** - Apply migrations to bring database up to latest version
- **`migratedown`** - Roll back migrations to previous versions
- **`migratestatus`** - Show current migration status and history
- **`dropall`** - Drop ALL tables and enums in database (VERY DANGEROUS!) (supports `--dry-run`)
- **`integration-test`** - Run comprehensive integration tests across database platforms

### Supporting Components

#### `examples/` - Usage Examples and Demos
- **`ast_demo/`** - Demonstrates AST-based SQL generation
- **`migrator_parser/`** - Shows parsing and generation workflow

#### `stubs/` - Example Entity Definitions
Contains sample Go structs with schema annotations for testing and demonstration:
- `product.go`, `category.go` - Real-world entity examples
- Various test files showing different annotation patterns and features

---

## Go Struct Annotations

Ptah uses structured comments to define database schema information directly in Go structs. Here's the annotation format:

### Table Definition
```go
//migrator:schema:table name="products" platform.mysql.engine="InnoDB" platform.mysql.comment="Product catalog"
type Product struct {
    // fields...
}
```

### Field Definition
```go
//migrator:schema:field name="id" type="SERIAL" primary="true" platform.mysql.type="INT AUTO_INCREMENT"
ID int64

//migrator:schema:field name="name" type="VARCHAR(255)" not_null="true"
Name string

//migrator:schema:field name="price" type="DECIMAL(10,2)" not_null="true" check="price > 0"
Price float64

//migrator:schema:field name="status" type="ENUM" enum="active,inactive,discontinued" not_null="true" default="active"
Status string

//migrator:schema:field name="category_id" type="INT" not_null="true" foreign="categories(id)" foreign_key_name="fk_product_category"
CategoryID int64
```

### Index Definition
```go
//migrator:schema:index name="idx_products_category" fields="category_id"
_ int
```

### Supported Attributes
- `name` - Database column/table name
- `type` - SQL data type
- `primary` - Primary key constraint
- `not_null` - NOT NULL constraint
- `unique` - UNIQUE constraint
- `default` - Default value
- `default_fn` - Default function (e.g., "NOW()")
- `check` - CHECK constraint
- `foreign` - Foreign key reference (table(column))
- `foreign_key_name` - Custom foreign key constraint name
- `enum` - Enum values (comma-separated)
- `platform.{dialect}.{attribute}` - Platform-specific overrides

---

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/denisvmedia/inventario.git
cd inventario/ptah

# Build the CLI tool
go build -o ptah ./cmd
```

### Basic Workflow

1. **Define your entities** with schema annotations:

```go
//migrator:schema:table name="users"
type User struct {
    //migrator:schema:field name="id" type="SERIAL" primary="true"
    ID int64

    //migrator:schema:field name="email" type="VARCHAR(255)" not_null="true" unique="true"
    Email string

    //migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="NOW()"
    CreatedAt time.Time
}
```

2. **Generate SQL schema**:

```bash
# Generate for PostgreSQL
go run ./cmd generate --root-dir ./models --dialect postgres

# Generate for MySQL
go run ./cmd generate --root-dir ./models --dialect mysql
```

3. **Write schema to database**:

```bash
# Write to PostgreSQL
go run ./cmd write-db --root-dir ./models --db-url postgres://user:pass@localhost/db

# Write to MySQL
go run ./cmd write-db --root-dir ./models --db-url mysql://user:pass@tcp(localhost:3306)/db
```

4. **Compare and migrate**:

```bash
# Compare current database with Go entities
go run ./cmd compare --root-dir ./models --db-url postgres://user:pass@localhost/db

# Generate migration SQL
go run ./cmd migrate --root-dir ./models --db-url postgres://user:pass@localhost/db
```

---

## Command Reference

### Generate Schema
Generate SQL DDL statements from Go entities without touching the database:

```bash
# Generate for all supported dialects
go run ./cmd generate --root-dir ./models

# Generate for specific dialect
go run ./cmd generate --root-dir ./models --dialect postgres
go run ./cmd generate --root-dir ./models --dialect mysql
go run ./cmd generate --root-dir ./models --dialect mariadb
```

### Database Operations

#### Write Schema
Write the generated schema directly to a database:

```bash
# PostgreSQL
go run ./cmd write-db --root-dir ./models --db-url postgres://user:pass@localhost:5432/database

# MySQL
go run ./cmd write-db --root-dir ./models --db-url mysql://user:pass@tcp(localhost:3306)/database
```

**Features:**
- ✅ Creates enums first (PostgreSQL requirement)
- ✅ Creates tables in dependency order
- ✅ Skips existing tables (safe)
- ✅ Transaction-based (all-or-nothing)
- ✅ Detailed progress output

#### Read Schema
Read and display the current database schema:

```bash
go run ./cmd read-db --db-url postgres://user:pass@localhost:5432/database
```

**Output:** Complete schema information including tables, columns, constraints, indexes, and enums

#### Compare Schemas
Compare your Go entities with the current database schema:

```bash
go run ./cmd compare --root-dir ./models --db-url postgres://user:pass@localhost:5432/database
```

**Output:** Detailed differences showing what needs to be added, removed, or modified

#### Generate Migrations
Generate SQL migration statements to synchronize schemas:

```bash
go run ./cmd migrate --root-dir ./models --db-url postgres://user:pass@localhost:5432/database
```

**Output:** SQL statements to bring the database in sync with Go entities

### Dangerous Operations

#### Drop Schema
Drop tables and enums defined in Go entities:

```bash
go run ./cmd drop-schema --root-dir ./models --db-url postgres://user:pass@localhost:5432/database
```

#### Drop All
Drop ALL tables and enums in the database:

```bash
go run ./cmd drop-all --db-url postgres://user:pass@localhost:5432/database
```

**⚠️ Warning:** Both drop commands require confirmation - user must type 'YES' to confirm

---

## Programming Examples

### Using the AST API

```go
package main

import (
    "fmt"
    "github.com/denisvmedia/inventario/ptah/core/ast"
    "github.com/denisvmedia/inventario/ptah/core/renderer"
)

func main() {
    // Create a table using the AST API
    table := ast.NewCreateTable("users").
        AddColumn(
            ast.NewColumn("id", "SERIAL").
                SetPrimary().
                SetAutoIncrement(),
        ).
        AddColumn(
            ast.NewColumn("email", "VARCHAR(255)").
                SetNotNull().
                SetUnique(),
        ).
        AddColumn(
            ast.NewColumn("created_at", "TIMESTAMP").
                SetDefaultFunction("CURRENT_TIMESTAMP"),
        ).
        AddConstraint(ast.NewUniqueConstraint("uk_users_email", "email"))

    // Render for PostgreSQL
    pgRenderer := renderer.NewRenderer("postgresql")
    pgSQL, err := renderer.RenderSQL("postgresql", table)
    if err != nil {
        panic(err)
    }
    fmt.Println("PostgreSQL:")
    fmt.Println(pgSQL)

    // Render for MySQL
    mysqlSQL, err := renderer.RenderSQL("mysql", table)
    if err != nil {
        panic(err)
    }
    fmt.Println("MySQL:")
    fmt.Println(mysqlSQL)
}
```

### Parsing Go Packages

```go
package main

import (
    "fmt"
    "github.com/denisvmedia/inventario/ptah/core/goschema"
    "github.com/denisvmedia/inventario/ptah/core/renderer"
)

func main() {
    // Parse Go entities from a directory
    result, err := goschema.ParseDir("./models")
    if err != nil {
        panic(err)
    }

    // Generate ordered CREATE TABLE statements
    statements, err := renderer.RenderSchemaSQL("postgresql", result)
    if err != nil {
        panic(err)
    }

    for _, stmt := range statements {
        fmt.Println(stmt)
    }
}
```

### Schema Comparison

```go
package main

import (
    "fmt"
    "github.com/denisvmedia/inventario/ptah/core/goschema"
    "github.com/denisvmedia/inventario/ptah/migration/schemadiff"
    "github.com/denisvmedia/inventario/ptah/migration/planner"
    "github.com/denisvmedia/inventario/ptah/dbschema"
)

func main() {
    // Parse Go entities
    generated, err := goschema.ParseDir("./models")
    if err != nil {
        panic(err)
    }

    // Connect to database and read schema
    dbURL := "postgres://user:pass@localhost:5432/database"
    conn, err := dbschema.ConnectToDatabase(dbURL)
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    database, err := conn.Reader().ReadSchema()
    if err != nil {
        panic(err)
    }

    // Compare schemas
    diff := schemadiff.Compare(generated, database)

    // Generate migration SQL
    migrationSQL := planner.GenerateSchemaDiffSQLStatements(diff, generated, "postgres")
    for _, stmt := range migrationSQL {
        fmt.Println(stmt)
    }
}
```

---

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run integration tests (requires database)
go test -v ./executor/... -tags=integration
```

### Database Testing

For integration tests, you can use Docker to set up test databases:

#### PostgreSQL Testing
```bash
# Start PostgreSQL container
docker run --name test-postgres \
  -e POSTGRES_PASSWORD=testpass \
  -e POSTGRES_DB=testdb \
  -p 5432:5432 -d postgres:15

# Run tests
go test -v ./executor/... -tags=integration

# Test with real database
go run ./cmd read-db --db-url postgres://postgres:testpass@localhost:5432/testdb
```

#### MySQL Testing
```bash
# Start MySQL container
docker run --name test-mysql \
  -e MYSQL_ROOT_PASSWORD=testpass \
  -e MYSQL_DATABASE=testdb \
  -p 3306:3306 -d mysql:8.0

# Test with real database
go run ./cmd read-db --db-url mysql://root:testpass@tcp(localhost:3306)/testdb
```

---

## Architecture

### Data Flow

1. **Parse** - Go source files are parsed to extract schema annotations
2. **Transform** - Annotations are converted to internal schema representations
3. **Generate** - Schema representations are converted to AST nodes
4. **Render** - AST nodes are rendered to dialect-specific SQL
5. **Execute** - SQL is executed against the target database

### Key Design Principles

- **Database Agnostic**: Core logic works with any supported database
- **AST-Based**: Uses Abstract Syntax Trees for type-safe SQL generation
- **Visitor Pattern**: Enables dialect-specific rendering without modifying core AST
- **Dependency Aware**: Automatically handles table creation order based on foreign keys
- **Transaction Safe**: All operations are wrapped in transactions for consistency

### Supported Databases

- **PostgreSQL** - Full support including enums, constraints, and indexes
- **MySQL** - Full support with MySQL-specific optimizations
- **MariaDB** - Full support with MariaDB-specific features

---

## Advanced Features

### Platform-Specific Overrides

You can specify platform-specific attributes in your annotations:

```go
//migrator:schema:table name="products" platform.mysql.engine="InnoDB" platform.mysql.comment="Product catalog"
type Product struct {
    //migrator:schema:field name="id" type="SERIAL" platform.mysql.type="INT AUTO_INCREMENT" platform.mariadb.type="INT AUTO_INCREMENT"
    ID int64
}
```

### Embedded Fields

Ptah supports embedded fields with different relation modes:

```go
type Address struct {
    Street string
    City   string
}

//migrator:schema:table name="users"
type User struct {
    ID int64

    // Embedded as separate columns
    //migrator:schema:embedded mode="columns"
    Address Address

    // Embedded as JSON
    //migrator:schema:embedded mode="json" name="address_data" type="JSONB"
    Metadata Address
}
```

### Enums

Define enums for type safety:

```go
//migrator:schema:field name="status" type="ENUM" enum="active,inactive,pending" not_null="true" default="active"
Status string
```

For PostgreSQL, this creates a proper ENUM type. For MySQL/MariaDB, it uses the ENUM column type.

---

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run the test suite
6. Submit a pull request

### Development Setup

```bash
# Clone the repository
git clone https://github.com/denisvmedia/inventario.git
cd inventario/ptah

# Install dependencies
go mod download

# Run tests
go test ./...

# Build the CLI
go build -o ptah ./cmd
```

---

## License

This project is part of the Inventario system and follows the same licensing terms.

---

## Roadmap

- [ ] Support for more database dialects (SQLite, SQL Server)
- [ ] Migration versioning and rollback capabilities
- [ ] Web UI for schema visualization
- [ ] Performance optimizations for large schemas
- [ ] Schema validation and linting
- [ ] Import from existing databases
- [ ] Export to various formats (GraphQL, OpenAPI, etc.)
- [ ] Runtime performance monitoring and optimization

---
