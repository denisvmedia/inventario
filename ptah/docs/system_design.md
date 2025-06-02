# Ptah System Design

**Ptah** is a comprehensive database schema management tool that bridges the gap between Go application code and database schemas. Named after the ancient Egyptian god of creation, Ptah transforms structured Go code into coherent, executable database schemas while maintaining consistency between code and data.

## Overview

Ptah follows the **P.T.A.H.** philosophy:
- **Parse** – Extract schema definitions from annotated Go structs
- **Transform** – Generate SQL DDL and schema diffs
- **Apply** – Execute up/down migrations with version tracking
- **Harmonize** – Synchronize code-defined schema with actual database state

## Core Architecture

### High-Level Data Flow

For a detailed visual representation of the system architecture, see the [Top-Level Architecture Diagram](diagrams/top_level_architecture.mmd).

```
Go Code (Annotations) ──┐
                        ├─► Parsing Layer ──► Core Processing ──► Migration System ──► Database
SQL Statements ─────────┘                                                                  │
                                                                                           │
Live Database ─────────────────────────────────────────────────────────────────────────────┘
```

The system operates through four main layers:

1. **Input Sources**: Go code with annotations, SQL statements, and live database connections
2. **Parsing Layer**: Tokenization, AST construction, and entity extraction
3. **Core Processing**: Schema representation, comparison, and SQL generation
4. **Migration System**: Planning, generation, and execution of database changes

### Key Design Principles

- **Database Agnostic**: Core logic works with PostgreSQL, MySQL, and MariaDB
- **AST-Based**: Uses Abstract Syntax Trees for type-safe SQL generation
- **Visitor Pattern**: Enables dialect-specific rendering without modifying core AST
- **Dependency Aware**: Automatically handles table creation order based on foreign keys
- **Transaction Safe**: All operations are wrapped in transactions for consistency

## Core Components

### 1. Parsing Layer

#### goschema Package
- **Purpose**: Extracts database schema information from Go struct annotations
- **Key Types**: `Database`, `Table`, `Field`, `Index`, `Enum`
- **Functionality**:
  - Recursively parses Go source files
  - Discovers entity definitions and dependencies
  - Handles embedded structs and topological sorting

#### lexer Package
- **Purpose**: Tokenizes SQL statements into structured tokens
- **Functionality**: Breaks down SQL into keywords, identifiers, operators, and literals

#### parser Package
- **Purpose**: Converts SQL tokens into Abstract Syntax Tree nodes
- **Supported Statements**: CREATE TABLE, ALTER TABLE, CREATE INDEX, CREATE TYPE
- **Integration**: Works with AST package to generate standardized nodes

### 2. Core Processing

#### ast Package
- **Purpose**: Provides database-agnostic AST representation for SQL DDL
- **Key Node Types**:
  - `CreateTableNode`: Table creation with columns and constraints
  - `AlterTableNode`: Table modifications and alterations
  - `ColumnNode`: Column definitions with attributes
  - `ConstraintNode`: Primary keys, foreign keys, unique constraints
  - `IndexNode`: Index creation statements
  - `EnumNode`: Enum type definitions
- **Pattern**: Implements visitor pattern for dialect-specific rendering

#### astbuilder Package
- **Purpose**: Fluent API for building SQL DDL AST nodes
- **Key Builders**:
  - `SchemaBuilder`: Entry point for complete schemas
  - `TableBuilder`: Table construction with method chaining
  - `ColumnBuilder`: Column definitions with constraints
  - `IndexBuilder`: Index creation with options

#### renderer Package
- **Purpose**: Converts AST nodes to dialect-specific SQL statements
- **Supported Dialects**: PostgreSQL, MySQL, MariaDB
- **Functionality**: Implements visitor pattern to traverse AST and generate SQL

### 3. Database Integration

#### dbschema Package
- **Purpose**: Reads existing database schemas and manages connections
- **Key Types**:
  - `DBSchema`: Complete database schema representation
  - `DBTable`, `DBColumn`, `DBIndex`: Database object representations
  - `SchemaReader`: Interface for reading database schemas
  - `SchemaWriter`: Interface for writing to databases
- **Functionality**: Database introspection and connection management

### 4. Migration System

For a detailed view of the migration workflow, see the [Migration Architecture Diagram](diagrams/migrations_architecture.mmd).

#### schemadiff Package
- **Purpose**: Compares Go-defined schemas with database schemas
- **Key Types**:
  - `SchemaDiff`: Comprehensive difference representation
  - `TableDiff`: Table-level changes
  - `ColumnDiff`: Column-level modifications
- **Functionality**: Identifies additions, removals, and modifications

#### planner Package
- **Purpose**: Plans migration operations based on schema differences
- **Functionality**:
  - Generates migration AST from schema diffs
  - Handles dependency ordering
  - Creates both UP and DOWN migration paths

#### generator Package
- **Purpose**: Generates migration files from planned operations
- **Features**:
  - Timestamp-based file naming
  - Bidirectional migrations (UP/DOWN)
  - Multiple database dialect support

#### migrator Package
- **Purpose**: Executes database migrations with version tracking
- **Features**:
  - Versioned migration management
  - Transaction safety
  - Dry-run capabilities
  - Migration status tracking

## Key Data Structures

### Database Schema Representation

```go
// Core schema representation from Go annotations
type Database struct {
    Tables         []Table
    Fields         []Field
    Indexes        []Index
    Enums          []Enum
    EmbeddedFields []EmbeddedField
    Dependencies   map[string][]string
}

// Database schema read from live database
type DBSchema struct {
    Tables      []DBTable
    Enums       []DBEnum
    Indexes     []DBIndex
    Constraints []DBConstraint
}
```

### AST Node Hierarchy

```go
// Base interface for all AST nodes
type Node interface {
    Accept(visitor Visitor) error
}

// Key node implementations
type CreateTableNode struct {
    Name        string
    Columns     []*ColumnNode
    Constraints []*ConstraintNode
    Options     map[string]string
}

type ColumnNode struct {
    Name         string
    DataType     string
    Constraints  []ColumnConstraint
    DefaultValue *DefaultValue
}
```

### Migration Representation

```go
// Schema differences for migration planning
type SchemaDiff struct {
    TablesAdded    []string
    TablesRemoved  []string
    TablesModified []TableDiff
    EnumsAdded     []string
    EnumsRemoved   []string
    IndexesAdded   []string
    IndexesRemoved []string
}
```

## Supported Databases

### PostgreSQL
- Full support including enums, constraints, and indexes
- Native enum types with CREATE TYPE statements
- Advanced constraint support

### MySQL/MariaDB
- Full support with platform-specific optimizations
- ENUM column types for enumerated values
- Engine-specific table options (InnoDB, MyISAM)

## Go Struct Annotations

Ptah uses structured comments to define database schema:

```go
//migrator:schema:table name="products" platform.mysql.engine="InnoDB"
type Product struct {
    //migrator:schema:field name="id" type="SERIAL" primary="true"
    ID int64

    //migrator:schema:field name="name" type="VARCHAR(255)" not_null="true" unique="true"
    Name string

    //migrator:schema:field name="category_id" type="INTEGER" foreign="categories(id)"
    CategoryID int64
}

//migrator:schema:index table="products" name="idx_products_name" columns="name" unique="true"
```

## Integration Points

### Development Workflow
1. Define entities in Go with annotations
2. Generate SQL schema for target database
3. Compare with existing database
4. Generate and review migration files
5. Apply migrations to database

### CI/CD Integration
- Docker-based integration testing
- Multiple database backend validation
- Automated migration generation and testing
- Schema drift detection

## Error Handling and Safety

### Transaction Safety
- All migration operations run in transactions
- Automatic rollback on failure
- Dry-run mode for validation

### Safety Mechanisms
- Confirmation prompts for destructive operations
- Schema validation before migration
- Dependency ordering to prevent constraint violations

### Error Recovery
- Detailed error reporting with context
- Migration status tracking for recovery
- Partial failure handling with rollback

This architecture provides a robust, extensible foundation for database schema management that bridges the gap between application code and database structure while maintaining safety and consistency across multiple database platforms.