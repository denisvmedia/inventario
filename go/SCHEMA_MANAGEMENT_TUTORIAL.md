# Database Schema Management Tutorial

This comprehensive tutorial covers all database schema operations using the `package-migrator` tool. The tool supports **PostgreSQL**, **MySQL**, and **MariaDB** with identical functionality across all platforms.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Database Connection URLs](#database-connection-urls)
3. [Schema Operations](#schema-operations)
   - [Generate Schema (SQL Output)](#1-generate-schema-sql-output)
   - [Create Schema (Write to Database)](#2-create-schema-write-to-database)
   - [Read Schema (Inspect Database)](#3-read-schema-inspect-database)
   - [Compare Schemas (Diff Operation)](#4-compare-schemas-diff-operation)
   - [Update Schema (Migration)](#5-update-schema-migration)
   - [Drop Schema (Cleanup)](#6-drop-schema-cleanup)
4. [Complete Workflow Examples](#complete-workflow-examples)
5. [Advanced Features](#advanced-features)
6. [Troubleshooting](#troubleshooting)

## Prerequisites

### 1. Go Entity Definitions

Create Go structs with migrator annotations in your project:

```go
package models

import "time"

//migrator:schema:table name="users" comment="User accounts"
type User struct {
    //migrator:schema:field name="id" type="SERIAL" primary="true"
    ID int `json:"id"`

    //migrator:schema:field name="email" type="VARCHAR(255)" unique="true" not_null="true"
    Email string `json:"email"`

    //migrator:schema:field name="username" type="VARCHAR(100)" unique="true" not_null="true"
    Username string `json:"username"`

    //migrator:schema:field name="role" type="ENUM" enum="admin,user,guest" default="user"
    Role string `json:"role"`

    //migrator:schema:field name="is_active" type="BOOLEAN" not_null="true" default="true"
    IsActive bool `json:"is_active"`

    //migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="NOW()"
    CreatedAt time.Time `json:"created_at"`

    //migrator:schema:field name="updated_at" type="TIMESTAMP"
    UpdatedAt *time.Time `json:"updated_at"`
}

//migrator:schema:table name="products" comment="Product catalog"
type Product struct {
    //migrator:schema:field name="id" type="SERIAL" primary="true"
    ID int `json:"id"`

    //migrator:schema:field name="name" type="VARCHAR(255)" not_null="true"
    Name string `json:"name"`

    //migrator:schema:field name="price" type="DECIMAL(10,2)" not_null="true" check="price > 0"
    Price float64 `json:"price"`

    //migrator:schema:field name="status" type="ENUM" enum="active,inactive,discontinued" default="active"
    Status string `json:"status"`

    //migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="NOW()"
    CreatedAt time.Time `json:"created_at"`
}

//migrator:schema:index name="idx_products_status" fields="status"
```

### 2. Database Setup

Ensure you have access to one or more of the supported databases:

- **PostgreSQL** (version 12+)
- **MySQL** (version 8.0+)
- **MariaDB** (version 10.3+)

## Database Connection URLs

The tool supports standard database connection URLs for all supported databases:

### PostgreSQL
```bash
postgres://username:password@hostname:port/database_name
postgres://user:pass@localhost:5432/mydb
postgres://user:pass@localhost/mydb  # Default port 5432

# Example with test database:
postgres://inventario:inventario_password@localhost:5432/inventario?sslmode=disable
```

### MySQL
```bash
mysql://username:password@tcp(hostname:port)/database_name?charset=utf8mb4&parseTime=True&loc=Local
mysql://user:pass@tcp(localhost:3306)/mydb?charset=utf8mb4&parseTime=True&loc=Local
mysql://user:pass@tcp(localhost)/mydb?charset=utf8mb4&parseTime=True&loc=Local  # Default port 3306

# Example with test database:
mysql://inventario:inventario_password@tcp(localhost:3306)/inventario?charset=utf8mb4&parseTime=True&loc=Local
```

### MariaDB
```bash
mariadb://username:password@tcp(hostname:port)/database_name?charset=utf8mb4&parseTime=True&loc=Local
mariadb://user:pass@tcp(localhost:3307)/mydb?charset=utf8mb4&parseTime=True&loc=Local

# Example with test database:
mariadb://inventario:inventario_password@tcp(localhost:3307)/inventario?charset=utf8mb4&parseTime=True&loc=Local
```

> **Security Note**: Passwords are automatically masked in all output as `***` for security.

## Schema Operations

### 1. Generate Schema (SQL Output)

Generate SQL schema from your Go entities without touching any database. This is useful for:
- Reviewing generated SQL before applying
- Creating migration files
- Documentation purposes
- CI/CD pipeline validation

#### Command Syntax
```bash
go run ./cmd/package-migrator generate [flags]
```

#### Available Flags
- `--root-dir string`: Root directory to scan for Go entities (default "./")
- `--dialect string`: Database dialect (postgres, mysql, mariadb). If empty, generates for all dialects

#### Examples

**Generate for all supported databases:**
```bash
go run ./cmd/package-migrator generate --root-dir ./models
```

**Generate for specific database:**
```bash
# PostgreSQL
go run ./cmd/package-migrator generate --root-dir ./models --dialect postgres

# MySQL
go run ./cmd/package-migrator generate --root-dir ./models --dialect mysql

# MariaDB
go run ./cmd/package-migrator generate --root-dir ./models --dialect mariadb
```

#### Sample Output

**PostgreSQL:**
```sql
=== POSTGRES SCHEMA ===

-- ENUMS --
CREATE TYPE enum_user_role AS ENUM ('admin', 'user', 'guest');
CREATE TYPE enum_product_status AS ENUM ('active', 'inactive', 'discontinued');

-- POSTGRES TABLE: users (User accounts) --
CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  email VARCHAR(255) UNIQUE NOT NULL,
  username VARCHAR(100) UNIQUE NOT NULL,
  role enum_user_role DEFAULT 'user',
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP
);

-- POSTGRES TABLE: products (Product catalog) --
CREATE TABLE products (
  id SERIAL PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  price DECIMAL(10,2) NOT NULL CHECK (price > 0),
  status enum_product_status DEFAULT 'active',
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_products_status ON products (status);
```

**MySQL:**
```sql
=== MYSQL SCHEMA ===

-- MYSQL TABLE: users (User accounts) --
CREATE TABLE users (
  id INT PRIMARY KEY AUTO_INCREMENT,
  email VARCHAR(255) UNIQUE NOT NULL,
  username VARCHAR(100) UNIQUE NOT NULL,
  role ENUM('admin', 'user', 'guest') DEFAULT 'user',
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP
);

-- MYSQL TABLE: products (Product catalog) --
CREATE TABLE products (
  id INT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(255) NOT NULL,
  price DECIMAL(10,2) NOT NULL CHECK (price > 0),
  status ENUM('active', 'inactive', 'discontinued') DEFAULT 'active',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_products_status ON products (status);
```

### 2. Create Schema (Write to Database)

Write the generated schema directly to a database. This operation:
- Creates all tables, indexes, constraints, and enums
- Uses database transactions (all-or-nothing)
- Skips existing tables safely
- Creates objects in dependency order

#### Command Syntax
```bash
go run ./cmd/package-migrator write-db [flags]
```

#### Available Flags
- `--root-dir string`: Root directory to scan for Go entities (default "./")
- `--db-url string`: Database URL (required). Example: postgres://user:pass@localhost/db

#### Examples

**PostgreSQL:**
```bash
go run ./cmd/package-migrator write-db --root-dir ./models --db-url postgres://inventario:inventario_password@localhost:5432/inventario?sslmode=disable
```

**MySQL:**
```bash
go run ./cmd/package-migrator write-db --root-dir ./models --db-url mysql://inventario:inventario_password@tcp(localhost:3306)/inventario?charset=utf8mb4&parseTime=True&loc=Local
```

**MariaDB:**
```bash
go run ./cmd/package-migrator write-db --root-dir ./models --db-url mariadb://inventario:inventario_password@tcp(localhost:3307)/inventario?charset=utf8mb4&parseTime=True&loc=Local
```

#### Sample Output
```
Writing schema from ./models to database postgres://inventario:***@localhost:5432/inventario?sslmode=disable
=== WRITE SCHEMA TO DATABASE ===

Parsed 2 tables, 2 enums from Go entities
Connected to postgres database successfully!
Writing schema to database...
Creating enum 1/2...
Creating enum 2/2...
Creating table 1/2...
Creating table 2/2...
Successfully created 2 tables, 2 enums
✅ Schema written successfully!
```

#### Handling Existing Tables
If tables already exist, the tool will skip them safely:
```
⚠️  WARNING: The following tables already exist: [users]
This operation will skip existing tables.
Use 'compare' command to see differences, or 'migrate' to generate update SQL.
```

### 3. Read Schema (Inspect Database)

Read and display the complete schema from an existing database. This shows:
- All tables with columns, types, and constraints
- Indexes and their definitions
- Enums and their values
- Foreign key relationships
- Database version and metadata

#### Command Syntax
```bash
go run ./cmd/package-migrator read-db [flags]
```

#### Available Flags
- `--db-url string`: Database URL (required). Example: postgres://user:pass@localhost/db

#### Examples

**PostgreSQL:**
```bash
go run ./cmd/package-migrator read-db --db-url postgres://inventario:inventario_password@localhost:5432/inventario?sslmode=disable
```

**MySQL:**
```bash
go run ./cmd/package-migrator read-db --db-url mysql://inventario:inventario_password@tcp(localhost:3306)/inventario?charset=utf8mb4&parseTime=True&loc=Local
```

**MariaDB:**
```bash
go run ./cmd/package-migrator read-db --db-url mariadb://inventario:inventario_password@tcp(localhost:3307)/inventario?charset=utf8mb4&parseTime=True&loc=Local
```

#### Sample Output

**PostgreSQL:**
```
Reading schema from database: postgres://inventario:***@localhost:5432/inventario?sslmode=disable
=== DATABASE SCHEMA ===

Connected to postgres database successfully!

=== DATABASE SCHEMA (POSTGRES) ===
Version: PostgreSQL 15.4 on x86_64-pc-linux-gnu
Schema: public

SUMMARY:
- Tables: 2
- Enums: 2
- Indexes: 4
- Constraints: 6

=== ENUMS ===
- enum_user_role: [admin, user, guest]
- enum_product_status: [active, inactive, discontinued]

=== TABLES ===
1. users (BASE TABLE) - User accounts
   Columns:
     - id integer PRIMARY KEY NOT NULL AUTO_INCREMENT
     - email character varying(255) UNIQUE NOT NULL
     - username character varying(100) UNIQUE NOT NULL
     - role enum_user_role DEFAULT 'user'
     - is_active boolean NOT NULL DEFAULT true
     - created_at timestamp without time zone NOT NULL DEFAULT now()
     - updated_at timestamp without time zone
   Constraints:
     - PRIMARY KEY (id)
     - UNIQUE (email)
     - UNIQUE (username)
   Indexes:
     - PRIMARY KEY PRIMARY (id)
     - UNIQUE INDEX users_email_key (email)
     - UNIQUE INDEX users_username_key (username)

2. products (BASE TABLE) - Product catalog
   Columns:
     - id integer PRIMARY KEY NOT NULL AUTO_INCREMENT
     - name character varying(255) NOT NULL
     - price numeric(10,2) NOT NULL
     - status enum_product_status DEFAULT 'active'
     - created_at timestamp without time zone NOT NULL DEFAULT now()
   Constraints:
     - PRIMARY KEY (id)
     - CHECK (price > 0)
   Indexes:
     - PRIMARY KEY PRIMARY (id)
     - INDEX idx_products_status (status)
```

**MySQL:**
```
=== DATABASE SCHEMA (MYSQL) ===
Version: 8.0.42
Schema: inventario

SUMMARY:
- Tables: 2
- Enums: 2
- Indexes: 4
- Constraints: 6

=== ENUMS ===
- enum_admin_user_guest: [admin, user, guest]
- enum_active_inactive_discontinued: [active, inactive, discontinued]

=== TABLES ===
1. users (BASE TABLE)
   Columns:
     - id int(10) PRIMARY KEY NOT NULL AUTO_INCREMENT
     - email varchar(255)(255) UNIQUE NOT NULL
     - username varchar(100)(100) UNIQUE NOT NULL
     - role enum('admin','user','guest')(9) DEFAULT user
     - is_active tinyint(1)(3) NOT NULL DEFAULT 1
     - created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
     - updated_at timestamp
   Constraints:
     - PRIMARY KEY (id)
     - UNIQUE (email)
     - UNIQUE (username)
   Indexes:
     - PRIMARY KEY PRIMARY (id)
     - UNIQUE INDEX email (email)
     - UNIQUE INDEX username (username)

2. products (BASE TABLE)
   Columns:
     - id int(10) PRIMARY KEY NOT NULL AUTO_INCREMENT
     - name varchar(255)(255) NOT NULL
     - price decimal(10,2)(10,2) NOT NULL
     - status enum('active','inactive','discontinued')(12) DEFAULT active
     - created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
   Constraints:
     - PRIMARY KEY (id)
     - CHECK (price > 0)
   Indexes:
     - PRIMARY KEY PRIMARY (id)
     - INDEX idx_products_status (status)
```

### 4. Compare Schemas (Diff Operation)

Compare your Go entity definitions with the current database schema to identify differences. This operation:
- Detects new tables, columns, indexes, and enums to add
- Identifies removed tables, columns, indexes, and enums
- Shows modified columns and constraints
- Provides a clear summary of required changes

#### Command Syntax
```bash
go run ./cmd/package-migrator compare [flags]
```

#### Available Flags
- `--root-dir string`: Root directory to scan for Go entities (default "./")
- `--db-url string`: Database URL (required). Example: postgres://user:pass@localhost/db

#### Examples

**PostgreSQL:**
```bash
go run ./cmd/package-migrator compare --root-dir ./models --db-url postgres://inventario:inventario_password@localhost:5432/inventario?sslmode=disable
```

**MySQL:**
```bash
go run ./cmd/package-migrator compare --root-dir ./models --db-url mysql://inventario:inventario_password@tcp(localhost:3306)/inventario?charset=utf8mb4&parseTime=True&loc=Local
```

**MariaDB:**
```bash
go run ./cmd/package-migrator compare --root-dir ./models --db-url mariadb://inventario:inventario_password@tcp(localhost:3307)/inventario?charset=utf8mb4&parseTime=True&loc=Local
```

#### Sample Output

**No Changes Detected:**
```
Comparing schema from ./models with database postgres://inventario:***@localhost:5432/inventario?sslmode=disable
=== SCHEMA COMPARISON ===

=== NO SCHEMA CHANGES DETECTED ===
The database schema matches your entity definitions.
```

**Changes Detected:**
```
Comparing schema from ./models with database postgres://inventario:***@localhost:5432/inventario?sslmode=disable
=== SCHEMA COMPARISON ===

=== SCHEMA DIFFERENCES DETECTED ===

SUMMARY: 5 changes detected
- Tables: +1 -0 ~1
- Enums: +0 -0 ~1
- Indexes: +2 -1

📋 TABLES TO ADD:
  + orders

🔧 TABLES TO MODIFY:
  ~ users
    + Column: last_login (TIMESTAMP)
    + Column: phone (VARCHAR(20))
    ~ Column: email (VARCHAR(255) → VARCHAR(320))

🏷️  ENUMS TO MODIFY:
  ~ enum_user_role
    + Value: moderator
    + Value: admin

📊 INDEXES TO ADD:
  + idx_users_phone
  + idx_orders_status

📊 INDEXES TO REMOVE:
  - old_unused_index
```

### 5. Update Schema (Migration)

Generate migration SQL to update the database schema to match your Go entity definitions. This operation:
- Analyzes differences between entities and database
- Generates safe SQL migration statements
- Includes warnings for potentially dangerous operations
- Provides manual review recommendations

#### Command Syntax
```bash
go run ./cmd/package-migrator migrate [flags]
```

#### Available Flags
- `--root-dir string`: Root directory to scan for Go entities (default "./")
- `--db-url string`: Database URL (required). Example: postgres://user:pass@localhost/db

#### Examples

**PostgreSQL:**
```bash
go run ./cmd/package-migrator migrate --root-dir ./models --db-url postgres://inventario:inventario_password@localhost:5432/inventario?sslmode=disable
```

**MySQL:**
```bash
go run ./cmd/package-migrator migrate --root-dir ./models --db-url mysql://inventario:inventario_password@tcp(localhost:3306)/inventario?charset=utf8mb4&parseTime=True&loc=Local
```

**MariaDB:**
```bash
go run ./cmd/package-migrator migrate --root-dir ./models --db-url mariadb://inventario:inventario_password@tcp(localhost:3307)/inventario?charset=utf8mb4&parseTime=True&loc=Local
```

#### Sample Output

**PostgreSQL Migration:**
```
Generating migration from ./models to database postgres://inventario:***@localhost:5432/inventario?sslmode=disable
=== GENERATE MIGRATION SQL ===

-- Migration generated from schema differences
-- Generated on: 2024-01-15 14:30:00
-- Source: ./models
-- Target: postgres://inventario:***@localhost:5432/inventario?sslmode=disable

-- Add new enum values
ALTER TYPE enum_user_role ADD VALUE 'moderator';
ALTER TYPE enum_user_role ADD VALUE 'admin';

-- Add new tables
CREATE TABLE orders (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL,
  total_amount DECIMAL(10,2) NOT NULL CHECK (total_amount >= 0),
  status enum_order_status DEFAULT 'pending',
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  CONSTRAINT fk_order_user FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Modify existing tables
ALTER TABLE users ADD COLUMN last_login TIMESTAMP;
ALTER TABLE users ADD COLUMN phone VARCHAR(20);
ALTER TABLE users ALTER COLUMN email TYPE VARCHAR(320);

-- Add new indexes
CREATE INDEX idx_users_phone ON users (phone);
CREATE INDEX idx_orders_status ON orders (status);

-- Remove old indexes
DROP INDEX IF EXISTS old_unused_index;

Generated 8 migration statements.
⚠️  Review the SQL carefully before executing!
⚠️  Test on a backup database first!
```

**MySQL Migration:**
```
-- Migration generated from schema differences
-- Generated on: 2024-01-15 14:30:00
-- Source: ./models
-- Target: mysql://inventario:***@tcp(localhost:3306)/inventario

-- Add new tables
CREATE TABLE orders (
  id INT PRIMARY KEY AUTO_INCREMENT,
  user_id INTEGER NOT NULL,
  total_amount DECIMAL(10,2) NOT NULL CHECK (total_amount >= 0),
  status ENUM('pending', 'processing', 'shipped', 'delivered') DEFAULT 'pending',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_order_user FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Modify existing tables
ALTER TABLE users ADD COLUMN last_login TIMESTAMP;
ALTER TABLE users ADD COLUMN phone VARCHAR(20);
ALTER TABLE users MODIFY COLUMN email VARCHAR(320) NOT NULL;

-- Add new indexes
CREATE INDEX idx_users_phone ON users (phone);
CREATE INDEX idx_orders_status ON orders (status);

-- Remove old indexes
DROP INDEX old_unused_index ON users;

Generated 6 migration statements.
⚠️  Review the SQL carefully before executing!
⚠️  Test on a backup database first!
```

#### Applying Migrations

The tool generates SQL but doesn't automatically apply it. You should:

1. **Review the generated SQL carefully**
2. **Test on a backup/staging database first**
3. **Apply manually using your preferred database client**

**PostgreSQL:**
```bash
# Save migration to file
go run ./cmd/package-migrator migrate --root-dir ./models --db-url postgres://inventario:inventario_password@localhost:5432/inventario?sslmode=disable > migration.sql

# Apply using psql
psql -h localhost -U inventario -d inventario -f migration.sql
```

**MySQL:**
```bash
# Save migration to file
go run ./cmd/package-migrator migrate --root-dir ./models --db-url mysql://inventario:inventario_password@tcp(localhost:3306)/inventario > migration.sql

# Apply using mysql client
mysql -h localhost -u inventario -p inventario < migration.sql
```

### 6. Drop Schema (Cleanup)

> **⚠️ DANGER: This operation will permanently delete data!**

Drop tables and enums defined in your Go entities. This operation is useful for:
- Cleaning up development/test databases
- Removing specific schema objects defined in your code
- Selective cleanup before recreating schema

**⚠️ WARNING**: This operation permanently deletes data and cannot be undone!

### 7. Drop All Tables (Complete Cleanup)

> **🚨 EXTREME DANGER: This operation will delete EVERYTHING in the database!**

Drop ALL tables and enums in the entire database, regardless of whether they're defined in your Go code. This operation is useful for:
- Complete database reset during development
- Cleaning up databases with mixed schema sources
- Starting completely fresh

**🚨 EXTREME WARNING**: This operation completely empties the database and cannot be undone!

#### Drop Schema Command Syntax
```bash
go run ./cmd/package-migrator drop-schema [flags]
```

#### Available Flags for drop-schema
- `--root-dir string`: Root directory to scan for Go entities (default "./")
- `--db-url string`: Database URL (required). Example: postgres://user:pass@localhost/db

#### Drop All Tables Command Syntax
```bash
go run ./cmd/package-migrator drop-all [flags]
```

#### Available Flags for drop-all
- `--db-url string`: Database URL (required). Example: postgres://user:pass@localhost/db

#### Examples

**Drop Schema (Go entities only):**

PostgreSQL:
```bash
go run ./cmd/package-migrator drop-schema --root-dir ./models --db-url postgres://inventario:inventario_password@localhost:5432/inventario?sslmode=disable
```

MySQL:
```bash
go run ./cmd/package-migrator drop-schema --root-dir ./models --db-url mysql://inventario:inventario_password@tcp(localhost:3306)/inventario?charset=utf8mb4&parseTime=True&loc=Local
```

MariaDB:
```bash
go run ./cmd/package-migrator drop-schema --root-dir ./models --db-url mariadb://inventario:inventario_password@tcp(localhost:3307)/inventario?charset=utf8mb4&parseTime=True&loc=Local
```

**Drop All Tables (Complete cleanup):**

PostgreSQL:
```bash
go run ./cmd/package-migrator drop-all --db-url postgres://inventario:inventario_password@localhost:5432/inventario?sslmode=disable
```

MySQL:
```bash
go run ./cmd/package-migrator drop-all --db-url mysql://inventario:inventario_password@tcp(localhost:3306)/inventario?charset=utf8mb4&parseTime=True&loc=Local
```

MariaDB:
```bash
go run ./cmd/package-migrator drop-all --db-url mariadb://inventario:inventario_password@tcp(localhost:3307)/inventario?charset=utf8mb4&parseTime=True&loc=Local
```

#### Sample Output

**Drop Schema (Go entities only):**
```
Dropping schema from postgres://inventario:***@localhost:5432/inventario?sslmode=disable based on entities in ./models
=== DROP SCHEMA FROM DATABASE ===

Found 2 tables, 2 enums to drop
Connected to postgres database successfully!

⚠️  WARNING: This operation will permanently delete all tables and enums!
⚠️  This action cannot be undone!
⚠️  Tables to be dropped: [users products]
⚠️  Enums to be dropped: [enum_user_role enum_product_status]

Type 'YES' to confirm: YES
Dropping schema from database...
WARNING: This will drop all tables and enums!
Dropping table: products
Dropping table: users
Dropping enum: enum_product_status
Dropping enum: enum_user_role
Successfully dropped 2 tables, 2 enums
✅ Schema dropped successfully!
```

**Drop All Tables (Complete cleanup):**
```
Dropping ALL tables and enums from database postgres://inventario:***@localhost:5432/inventario?sslmode=disable
=== DROP ALL TABLES FROM DATABASE ===

Connected to postgres database successfully!

🚨 EXTREME WARNING: This operation will permanently delete ALL tables and enums!
🚨 This will delete EVERYTHING in the database, not just your Go entities!
🚨 This action cannot be undone!
🚨 ALL DATA WILL BE LOST!

Type 'DELETE EVERYTHING' to confirm this destructive operation: DELETE EVERYTHING

⚠️  Last chance! Type 'YES I AM SURE' to proceed: YES I AM SURE
Dropping all tables and enums from database...
WARNING: This will drop ALL tables and enums in the database!
Dropping table: users
Dropping table: products
Dropping table: legacy_data
Dropping table: temp_imports
Dropping enum: enum_user_role
Dropping enum: enum_product_status
Dropping enum: enum_legacy_status
Dropping sequence: users_id_seq
Dropping sequence: products_id_seq
Dropping sequence: legacy_data_id_seq
Successfully dropped 4 tables, 3 enums, 3 sequences
✅ All tables and enums dropped successfully!
🔥 Database is now completely empty!
```

#### Safety Features

**Drop Schema:**
- **Explicit Confirmation**: Requires typing 'YES' to proceed
- **Selective Cleanup**: Only drops tables/enums defined in your Go entities
- **Transaction-based**: All operations are wrapped in a transaction
- **Dependency Order**: Drops tables in reverse dependency order
- **Foreign Key Handling**: Disables foreign key checks during operation (MySQL/MariaDB)
- **Detailed Output**: Shows exactly what will be dropped before confirmation

**Drop All Tables:**
- **Double Confirmation**: Requires typing 'DELETE EVERYTHING' then 'YES I AM SURE'
- **Complete Cleanup**: Drops ALL tables, enums, and sequences in the database
- **Database Query**: Queries database for complete list of objects to drop
- **Transaction-based**: All operations are wrapped in a transaction
- **Foreign Key Handling**: Disables foreign key checks during operation (MySQL/MariaDB)
- **Sequence Cleanup**: Drops all sequences to prevent orphaned sequences (PostgreSQL)
- **Extreme Warnings**: Multiple warnings about complete data loss

#### Programmatic Usage

```go
package main

import (
    "github.com/denisvmedia/inventario/cmd/migrator/migratorlib"
    "github.com/denisvmedia/inventario/cmd/migrator/migratorlib/dbschema"
)

func dropSchema() {
    // Connect to database
    conn, err := dbschema.ConnectToDatabase("postgres://user:pass@localhost/testdb")
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    // Parse entities to know what to drop
    result, err := migratorlib.ParsePackageRecursively("./models")
    if err != nil {
        panic(err)
    }

    // Drop schema (PostgreSQL only for now)
    if pgWriter, ok := conn.Writer.(*dbschema.PostgreSQLWriter); ok {
        err = pgWriter.DropSchema(result)
        if err != nil {
            panic(err)
        }
    }
}
```

#### What Gets Dropped

- **All tables** in reverse dependency order (to handle foreign keys)
- **All enums** (PostgreSQL only)
- **All indexes** (automatically dropped with tables)
- **All constraints** (automatically dropped with tables)

#### Safety Features

- Uses `DROP TABLE IF EXISTS` and `DROP TYPE IF EXISTS`
- Uses `CASCADE` to handle dependencies
- Wrapped in database transaction
- Requires explicit confirmation in code

## Complete Workflow Examples

### Example 1: New Project Setup

**Step 1: Create your Go entities**
```go
// models/user.go
package models

//migrator:schema:table name="users"
type User struct {
    //migrator:schema:field name="id" type="SERIAL" primary="true"
    ID int `json:"id"`

    //migrator:schema:field name="email" type="VARCHAR(255)" unique="true" not_null="true"
    Email string `json:"email"`

    //migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="NOW()"
    CreatedAt time.Time `json:"created_at"`
}
```

**Step 2: Generate and review schema**
```bash
# Review what will be created
go run ./cmd/package-migrator generate --root-dir ./models --dialect postgres
```

**Step 3: Create database schema**
```bash
# Create the schema in your database
go run ./cmd/package-migrator write-db --root-dir ./models --db-url postgres://inventario:inventario_password@localhost:5432/inventario?sslmode=disable
```

**Step 4: Verify creation**
```bash
# Confirm everything was created correctly
go run ./cmd/package-migrator read-db --db-url postgres://inventario:inventario_password@localhost:5432/inventario?sslmode=disable
```

### Example 2: Adding New Features

**Step 1: Update your entities**
```go
// Add new fields to existing entity
//migrator:schema:field name="phone" type="VARCHAR(20)"
Phone string `json:"phone"`

//migrator:schema:field name="role" type="ENUM" enum="admin,user,guest" default="user"
Role string `json:"role"`

// Add new entity
//migrator:schema:table name="orders"
type Order struct {
    //migrator:schema:field name="id" type="SERIAL" primary="true"
    ID int `json:"id"`

    //migrator:schema:field name="user_id" type="INTEGER" not_null="true" foreign="users(id)"
    UserID int `json:"user_id"`

    //migrator:schema:field name="total" type="DECIMAL(10,2)" not_null="true"
    Total float64 `json:"total"`
}
```

**Step 2: Check what changed**
```bash
# See what needs to be updated
go run ./cmd/package-migrator compare --root-dir ./models --db-url postgres://inventario:inventario_password@localhost:5432/inventario?sslmode=disable
```

**Step 3: Generate migration**
```bash
# Generate SQL to apply changes
go run ./cmd/package-migrator migrate --root-dir ./models --db-url postgres://inventario:inventario_password@localhost:5432/inventario?sslmode=disable > migration.sql
```

**Step 4: Review and apply migration**
```bash
# Review the generated SQL
cat migration.sql

# Apply to database
psql -h localhost -U user -d myapp -f migration.sql
```

**Step 5: Verify changes**
```bash
# Confirm changes were applied
go run ./cmd/package-migrator compare --root-dir ./models --db-url postgres://inventario:inventario_password@localhost:5432/inventario?sslmode=disable
# Should show "NO SCHEMA CHANGES DETECTED"
```

### Example 3: Multi-Database Support

**Generate for all databases:**
```bash
# PostgreSQL
go run ./cmd/package-migrator write-db --root-dir ./models --db-url postgres://inventario:inventario_password@localhost:5432/inventario?sslmode=disable

# MySQL
go run ./cmd/package-migrator write-db --root-dir ./models --db-url mysql://inventario:inventario_password@tcp(localhost:3306)/inventario?charset=utf8mb4&parseTime=True&loc=Local

# MariaDB
go run ./cmd/package-migrator write-db --root-dir ./models --db-url mariadb://inventario:inventario_password@tcp(localhost:3307)/inventario?charset=utf8mb4&parseTime=True&loc=Local
```

**Compare across databases:**
```bash
# Check PostgreSQL
go run ./cmd/package-migrator compare --root-dir ./models --db-url postgres://inventario:inventario_password@localhost:5432/inventario?sslmode=disable

# Check MySQL
go run ./cmd/package-migrator compare --root-dir ./models --db-url mysql://inventario:inventario_password@tcp(localhost:3306)/inventario?charset=utf8mb4&parseTime=True&loc=Local

# Check MariaDB
go run ./cmd/package-migrator compare --root-dir ./models --db-url mariadb://inventario:inventario_password@tcp(localhost:3307)/inventario?charset=utf8mb4&parseTime=True&loc=Local
```

## Advanced Features

### 1. Platform-Specific Overrides

You can specify different configurations for different databases:

```go
//migrator:schema:field name="data" type="TEXT" platform.mysql.type="JSON" platform.postgres.type="JSONB"
Data string `json:"data"`

//migrator:schema:table name="users" platform.mysql.engine="InnoDB" platform.mysql.charset="utf8mb4"
type User struct {
    // fields...
}
```

### 2. Foreign Key Relationships

```go
//migrator:schema:field name="user_id" type="INTEGER" not_null="true" foreign="users(id)" foreign_key_name="fk_order_user"
UserID int `json:"user_id"`
```

### 3. Check Constraints

```go
//migrator:schema:field name="price" type="DECIMAL(10,2)" not_null="true" check="price > 0"
Price float64 `json:"price"`

//migrator:schema:field name="age" type="INTEGER" check="age >= 0 AND age <= 150"
Age int `json:"age"`
```

### 4. Custom Indexes

```go
//migrator:schema:index name="idx_users_email_active" fields="email,is_active" unique="true"
//migrator:schema:index name="idx_orders_created_at" fields="created_at"
```

### 5. Embedded Fields Support

```go
//migrator:embedded mode="inline" prefix="billing_"
BillingAddress Address `json:"billing_address"`

//migrator:embedded mode="json" name="metadata" type="JSONB"
Metadata map[string]interface{} `json:"metadata"`

//migrator:embedded mode="relation" field="profile_id" ref="profiles(id)"
Profile UserProfile `json:"profile"`
```

## Troubleshooting

### Common Issues

#### 1. Connection Errors

**Problem**: `Error connecting to database: dial tcp: connect: connection refused`

**Solutions**:
- Verify database server is running
- Check hostname and port in connection URL
- Verify firewall settings
- Test connection with database client first

#### 2. Permission Errors

**Problem**: `Error: permission denied for schema public`

**Solutions**:
- Ensure user has CREATE privileges
- Grant necessary permissions: `GRANT CREATE ON SCHEMA public TO username;`
- Use a superuser account for initial setup

#### 3. Existing Tables

**Problem**: `WARNING: The following tables already exist`

**Solutions**:
- Use `compare` command to see differences
- Use `migrate` command to generate update SQL
- Drop existing tables if starting fresh (⚠️ data loss!)

#### 4. Type Conversion Issues

**Problem**: `Error: column "price" cannot be cast automatically to type numeric`

**Solutions**:
- Review generated migration SQL carefully
- Add explicit type conversion in migration
- Consider data migration strategy

#### 5. Foreign Key Violations

**Problem**: `Error: insert or update on table violates foreign key constraint`

**Solutions**:
- Ensure referenced tables exist first
- Check dependency order in generated SQL
- Verify foreign key references are correct

### Database-Specific Notes

#### PostgreSQL
- Requires enum types to be created before tables
- Supports advanced features like JSONB, arrays
- Case-sensitive identifiers when quoted
- Excellent foreign key constraint support

#### MySQL
- Enums are defined inline in column definitions
- Uses `AUTO_INCREMENT` instead of `SERIAL`
- Uses `CURRENT_TIMESTAMP` instead of `NOW()`
- Table engine can be specified (InnoDB recommended)

#### MariaDB
- Compatible with MySQL syntax
- Uses MySQL driver for connections
- Supports most MySQL features
- Some differences in function names and defaults

### Performance Tips

1. **Use indexes wisely**: Add indexes for frequently queried columns
2. **Foreign key order**: Create referenced tables before referencing tables
3. **Batch operations**: Use transactions for multiple changes
4. **Test migrations**: Always test on staging/backup databases first
5. **Monitor size**: Large schema changes may take time on big databases

### Best Practices

1. **Version control**: Keep entity definitions in version control
2. **Migration files**: Save generated migrations for deployment
3. **Backup first**: Always backup before applying migrations
4. **Test thoroughly**: Test schema changes in staging environment
5. **Document changes**: Add comments to explain complex migrations
6. **Rollback plan**: Have a rollback strategy for failed migrations

## Conclusion

The `package-migrator` tool provides a comprehensive solution for database schema management across PostgreSQL, MySQL, and MariaDB. With its support for:

- ✅ **Multi-database compatibility**
- ✅ **Safe transaction-based operations**
- ✅ **Comprehensive schema comparison**
- ✅ **Automatic migration generation**
- ✅ **Advanced database features**

You can maintain consistent database schemas across different environments and database platforms while keeping your Go entity definitions as the single source of truth.

For more information and examples, see the [WORKFLOW.md](../ptah/executor/WORKFLOW.md) file in the project repository.
