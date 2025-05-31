# Ptah Executor - Database Schema Management

This package provides functionality to read, write, compare, and manage database schemas. It's part of the **Ptah** schema management tool that generates database schemas from annotated Go entities.

## Features

- **PostgreSQL Support**: Full schema reading including tables, columns, constraints, indexes, and enums
- **MySQL/MariaDB Support**: Complete schema reading and writing capabilities
- **Connection Management**: Secure database connections with URL parsing and connection pooling
- **Schema Formatting**: Human-readable schema output with detailed information
- **Schema Writing**: Create complete database schemas from Go entities
- **Schema Dropping**: Drop all tables and enums (with safety confirmations)
- **Schema Comparison**: Compare generated schemas with existing database schemas
- **Error Handling**: Comprehensive error messages and connection diagnostics

## Usage

### Reading Database Schema

```bash
# Read PostgreSQL schema
go run ./ptah/cmd read-db --db-url postgres://username:password@localhost:5432/database_name

# Read MySQL schema
go run ./ptah/cmd read-db --db-url mysql://username:password@tcp(localhost:3306)/database_name

# Read with specific root directory for Go entities
go run ./ptah/cmd read-db --root-dir ./models --db-url postgres://username:password@localhost:5432/database_name
```

### Writing Schema to Database

```bash
# Write schema from Go entities to PostgreSQL
go run ./ptah/cmd write-db --root-dir ./models --db-url postgres://username:password@localhost:5432/database_name

# Write schema to MySQL
go run ./ptah/cmd write-db --root-dir ./models --db-url mysql://username:password@tcp(localhost:3306)/database_name
```

### Comparing Schemas

```bash
# Compare generated schema with database
go run ./ptah/cmd compare --root-dir ./models --db-url postgres://username:password@localhost:5432/database_name
```

### Dropping Database Schema

```bash
# Drop PostgreSQL schema (DANGEROUS!)
go run ./ptah/cmd drop-schema --root-dir ./models --db-url postgres://username:password@localhost:5432/database_name

# Drop MySQL schema (DANGEROUS!)
go run ./ptah/cmd drop-schema --root-dir ./models --db-url mysql://username:password@tcp(localhost:3306)/database_name

# Drop all tables (even more DANGEROUS!)
go run ./ptah/cmd drop-all --db-url postgres://username:password@localhost:5432/database_name

# All drop commands require confirmation - user must type 'YES' to confirm
```

### Database URL Format

**PostgreSQL:**
```
postgres://username:password@host:port/database_name
postgresql://username:password@host:port/database_name
```

**MySQL/MariaDB:**
```
mysql://username:password@tcp(host:port)/database_name
```

**Security Note**: Passwords in URLs are automatically masked in output for security.

## Example Output

```
Reading schema from database: postgres://user:***@localhost:5432/mydb
=== DATABASE SCHEMA ===

Connected to postgres database successfully!

=== DATABASE SCHEMA (POSTGRES) ===
Version: PostgreSQL 15.4 on x86_64-pc-linux-gnu
Schema: public

SUMMARY:
- Tables: 3
- Enums: 2
- Indexes: 5
- Constraints: 8

=== ENUMS ===
- enum_user_role: [admin, user, guest]
- enum_product_status: [active, inactive, discontinued]

=== TABLES ===
1. users (TABLE)
   Columns:
     - id SERIAL PRIMARY KEY AUTO_INCREMENT DEFAULT nextval('users_id_seq'::regclass)
     - email VARCHAR(255) UNIQUE NOT NULL
     - role enum_user_role DEFAULT 'user'
     - created_at TIMESTAMP NOT NULL DEFAULT now()
   Constraints:
     - PRIMARY KEY (id)
     - UNIQUE (email)
   Indexes:
     - PRIMARY KEY users_pkey (id)
     - UNIQUE INDEX users_email_key (email)

2. products (TABLE)
   Columns:
     - id SERIAL PRIMARY KEY AUTO_INCREMENT DEFAULT nextval('products_id_seq'::regclass)
     - name VARCHAR(255) NOT NULL
     - status enum_product_status NOT NULL DEFAULT 'active'
     - price DECIMAL(10,2) NOT NULL
   Constraints:
     - PRIMARY KEY (id)
     - CHECK price CHECK ((price > (0)::numeric))
```

## Supported Database Types

### PostgreSQL ✅
- Tables and views
- All column types including custom types
- Primary keys, foreign keys, unique constraints
- Check constraints with full clause
- Indexes (regular, unique, primary)
- Enum types with values
- Auto-increment detection (SERIAL types)
- Comments and metadata
- Full schema reading and writing
- Schema comparison and migration generation

### MySQL/MariaDB ✅
- Tables and views
- All standard column types
- Primary keys, foreign keys, unique constraints
- Check constraints (MySQL 8.0+)
- Indexes (regular, unique, primary)
- Enum types with values
- Auto-increment detection
- Full schema reading and writing
- Schema comparison and migration generation

## Error Handling

The schema reader provides detailed error messages for common issues:

- **Connection failures**: Network issues, wrong credentials, server not running
- **Permission errors**: Insufficient database privileges
- **Schema access**: Schema doesn't exist or no access
- **Query failures**: Malformed queries or database-specific issues

## Testing

### PostgreSQL Testing

To test the schema functionality with a real PostgreSQL database:

1. **Start PostgreSQL** (using Docker):
   ```bash
   docker run --name test-postgres -e POSTGRES_PASSWORD=testpass -e POSTGRES_DB=testdb -p 5432:5432 -d postgres:15
   ```

2. **Create some test data**:
   ```sql
   CREATE TYPE user_role AS ENUM ('admin', 'user', 'guest');

   CREATE TABLE users (
       id SERIAL PRIMARY KEY,
       email VARCHAR(255) UNIQUE NOT NULL,
       role user_role DEFAULT 'user',
       created_at TIMESTAMP NOT NULL DEFAULT NOW()
   );

   CREATE INDEX idx_users_role ON users(role);
   ```

3. **Read the schema**:
   ```bash
   go run ./ptah/cmd read-db --db-url postgres://postgres:testpass@localhost:5432/testdb
   ```

### MySQL Testing

To test with MySQL/MariaDB:

1. **Start MySQL** (using Docker):
   ```bash
   docker run --name test-mysql -e MYSQL_ROOT_PASSWORD=testpass -e MYSQL_DATABASE=testdb -p 3306:3306 -d mysql:8.0
   ```

2. **Read the schema**:
   ```bash
   go run ./ptah/cmd read-db --db-url mysql://root:testpass@tcp(localhost:3306)/testdb
   ```

### Integration Tests

The package includes comprehensive integration tests that can be run with:

```bash
cd ptah
go test -v ./executor/... -tags=integration
```

## Architecture

The Ptah executor package is organized into several key components:

### Core Files
- **`types.go`**: Core data structures for representing database schemas
- **`connection.go`**: Database connection management and URL parsing
- **`postgres.go`**: PostgreSQL-specific schema reading and writing implementation
- **`mysql.go`**: MySQL/MariaDB-specific schema reading and writing implementation
- **`writer.go`**: Database schema writing interfaces and implementations
- **`formatter.go`**: Human-readable schema formatting and output
- **`comparator.go`**: Schema comparison functionality for detecting differences

### Related Packages
- **`ptah/schema/builder/`**: Go package parsing and entity extraction
- **`ptah/schema/meta/`**: Metadata structures for database entities
- **`ptah/cmd/`**: Command-line interface implementations
- **`ptah/renderer/`**: SQL statement generation for different database dialects

### Integration with Ptah
This executor package is part of the larger **Ptah** schema management system:
- Parses Go entities using `ptah/schema/builder`
- Generates SQL using `ptah/renderer` dialects
- Executes operations using this `ptah/executor` package
- Provides CLI through `ptah/cmd` commands

## Future Enhancements

- [ ] Schema validation and linting
- [ ] Performance optimization for large schemas
- [ ] JSON/YAML schema export
- [ ] Advanced migration planning and rollback
- [ ] Schema versioning and history tracking
- [ ] Support for additional database types (SQLite, Oracle, etc.)
