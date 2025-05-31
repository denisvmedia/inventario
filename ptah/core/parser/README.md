# SQL Parser

This package provides a comprehensive SQL DDL (Data Definition Language) parser that converts SQL tokens into Abstract Syntax Tree (AST) nodes. The parser is designed to work with the Ptah schema management system and supports multiple SQL dialects.

## Features

The parser supports the following SQL DDL statements:

### CREATE TABLE
- Column definitions with data types
- Column constraints (PRIMARY KEY, UNIQUE, NOT NULL, AUTO_INCREMENT)
- Default values (literals and function calls)
- Check constraints
- Foreign key references with ON DELETE/UPDATE actions
- Table-level constraints (PRIMARY KEY, UNIQUE, FOREIGN KEY, CHECK)
- Table options (ENGINE, CHARSET, COLLATE, COMMENT)

### ALTER TABLE
- ADD COLUMN operations
- DROP COLUMN operations  
- MODIFY/ALTER COLUMN operations
- Multiple operations in a single statement

### CREATE INDEX
- Regular indexes
- Unique indexes
- Multi-column indexes

### CREATE TYPE (ENUM)
- PostgreSQL-style enum type definitions

## Usage

### Basic Usage

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/denisvmedia/inventario/ptah/core/parser"
)

func main() {
    sql := `CREATE TABLE users (
        id INTEGER PRIMARY KEY,
        email VARCHAR(255) NOT NULL UNIQUE,
        created_at TIMESTAMP DEFAULT NOW()
    );`
    
    parser := parser.NewParser(sql)
    statements, err := parser.Parse()
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Parsed %d statements\n", len(statements.Statements))
}
```

### Parsing Multiple Statements

```go
sql := `
    CREATE TABLE users (id INTEGER PRIMARY KEY);
    CREATE INDEX idx_users_id ON users (id);
    ALTER TABLE users ADD COLUMN name VARCHAR(255);
`

parser := parser.NewParser(sql)
statements, err := parser.Parse()
// statements.Statements will contain 3 AST nodes
```

### Working with AST Nodes

```go
for _, stmt := range statements.Statements {
    switch s := stmt.(type) {
    case *ast.CreateTableNode:
        fmt.Printf("Table: %s\n", s.Name)
        fmt.Printf("Columns: %d\n", len(s.Columns))
        
    case *ast.IndexNode:
        fmt.Printf("Index: %s on %s\n", s.Name, s.Table)
        
    case *ast.AlterTableNode:
        fmt.Printf("Altering table: %s\n", s.Name)
    }
}
```

## Supported SQL Syntax

### Column Types
- Basic types: `INTEGER`, `VARCHAR(255)`, `TEXT`, `TIMESTAMP`
- Parameterized types: `DECIMAL(10,2)`, `CHAR(50)`
- Complex types: `ENUM('value1', 'value2')`

### Column Constraints
- `PRIMARY KEY`
- `NOT NULL` / `NULL`
- `UNIQUE`
- `AUTO_INCREMENT` / `AUTOINCREMENT`
- `DEFAULT value` or `DEFAULT function()`
- `CHECK (expression)`
- `REFERENCES table(column) [ON DELETE action] [ON UPDATE action]`

### Table Constraints
- `PRIMARY KEY (column1, column2)`
- `UNIQUE (column1, column2)`
- `FOREIGN KEY (column) REFERENCES table(column)`
- `CHECK (expression)`
- `CONSTRAINT name PRIMARY KEY (columns)`

### Table Options
- `ENGINE=InnoDB`
- `CHARSET=utf8mb4` / `CHARACTER SET=utf8mb4`
- `COLLATE=utf8mb4_unicode_ci`
- `COMMENT='table description'`

## Architecture

The parser follows a recursive descent parsing approach:

1. **Lexer Integration**: Uses the `ptah/core/lexer` package for tokenization
2. **AST Generation**: Converts tokens into `ptah/core/ast` nodes
3. **Error Handling**: Provides detailed error messages with position information
4. **Whitespace Handling**: Automatically skips whitespace and comments

### Key Components

- `Parser`: Main parser struct that manages token stream
- `Parse()`: Entry point that returns a `StatementList`
- Statement parsers: `parseCreateTable()`, `parseAlterTable()`, etc.
- Helper parsers: `parseColumnDefinition()`, `parseConstraint()`, etc.

## Error Handling

The parser provides detailed error messages including:
- Expected vs actual token types
- Position information in the input
- Context about what was being parsed

```go
parser := parser.NewParser("CREATE TABLE (id INTEGER);")
_, err := parser.Parse()
// Error: "expected table name: expected identifier, got Operator at position 13"
```

## Testing

The parser includes comprehensive tests covering:
- Basic CREATE TABLE statements
- Complex tables with constraints and options
- ALTER TABLE operations
- CREATE INDEX statements
- CREATE TYPE (ENUM) statements
- Multiple statement parsing
- Error conditions

Run tests with:
```bash
go test -v ./ptah/core/parser
```

## Integration

The parser integrates with other Ptah components:
- **AST Package**: Generates standardized AST nodes
- **Lexer Package**: Consumes SQL tokens
- **Renderer Package**: AST nodes can be rendered back to SQL
- **Migration System**: Parses existing schemas for comparison

## Limitations

Current limitations include:
- Limited to DDL statements (no DML like INSERT, UPDATE, DELETE)
- Basic expression parsing in CHECK constraints
- Simplified handling of complex data types
- No support for stored procedures or functions

## Future Enhancements

Planned improvements:
- Enhanced expression parsing
- Support for more SQL dialects
- Better error recovery
- Performance optimizations
- Extended DDL statement support (DROP TABLE, etc.)
