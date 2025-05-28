# Database Schema Reader

This package provides functionality to read database schemas and compare them with generated schemas from Go entities.

## Features

- **PostgreSQL Support**: Full schema reading including tables, columns, constraints, indexes, and enums
- **Connection Management**: Secure database connections with URL parsing and connection pooling
- **Schema Formatting**: Human-readable schema output with detailed information
- **Error Handling**: Comprehensive error messages and connection diagnostics

## Usage

### Reading Database Schema

```bash
# Read PostgreSQL schema
go run ./cmd/package-migrator read-db postgres://username:password@localhost:5432/database_name

# Read with specific schema (defaults to 'public')
go run ./cmd/package-migrator read-db postgres://username:password@localhost:5432/database_name?search_path=my_schema
```

### Database URL Format

```
postgres://username:password@host:port/database_name
postgresql://username:password@host:port/database_name
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

### MySQL/MariaDB 🚧
- Coming soon! The infrastructure is ready, just need to implement the MySQL-specific queries.

## Error Handling

The schema reader provides detailed error messages for common issues:

- **Connection failures**: Network issues, wrong credentials, server not running
- **Permission errors**: Insufficient database privileges
- **Schema access**: Schema doesn't exist or no access
- **Query failures**: Malformed queries or database-specific issues

## Testing

To test the schema reader with a real PostgreSQL database:

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
   go run ./cmd/package-migrator read-db postgres://postgres:testpass@localhost:5432/testdb
   ```

## Architecture

- **`types.go`**: Core data structures for representing database schemas
- **`connection.go`**: Database connection management and URL parsing
- **`postgres.go`**: PostgreSQL-specific schema reading implementation
- **`formatter.go`**: Human-readable schema formatting
- **`README.md`**: This documentation

## Future Enhancements

- [ ] MySQL/MariaDB schema reading
- [ ] Schema comparison functionality
- [ ] Migration generation from schema differences
- [ ] JSON/YAML schema export
- [ ] Schema validation and linting
- [ ] Performance optimization for large schemas
