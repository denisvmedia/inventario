# Testing Ptah Database Schema Functionality

This guide shows how to test the Ptah database schema reading, writing, and comparison functionality with real databases.

## Quick Test with Docker

### 1. Start PostgreSQL Container

```bash
docker run --name test-postgres \
  -e POSTGRES_PASSWORD=testpass \
  -e POSTGRES_DB=testdb \
  -p 5432:5432 \
  -d postgres:15
```

### 2. Create Test Schema

Connect to the database and create some test data:

```bash
# Connect to PostgreSQL
docker exec -it test-postgres psql -U postgres -d testdb
```

```sql
-- Create enums
CREATE TYPE user_role AS ENUM ('admin', 'user', 'guest');
CREATE TYPE product_status AS ENUM ('active', 'inactive', 'discontinued', 'out_of_stock');

-- Create tables
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(100) UNIQUE NOT NULL,
    role user_role DEFAULT 'user',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP
);

CREATE TABLE categories (
    id VARCHAR(36) PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT
);

CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    sku VARCHAR(50) UNIQUE NOT NULL,
    price DECIMAL(10,2) NOT NULL CHECK (price > 0),
    status product_status NOT NULL DEFAULT 'active',
    in_stock BOOLEAN NOT NULL DEFAULT true,
    category_id VARCHAR(36) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP,
    CONSTRAINT fk_product_category FOREIGN KEY (category_id) REFERENCES categories(id)
);

-- Create indexes
CREATE INDEX idx_users_role ON users(role);
CREATE INDEX idx_users_created_at ON users(created_at);
CREATE INDEX idx_products_category ON products(category_id);
CREATE INDEX idx_products_status ON products(status);

-- Add some sample data
INSERT INTO categories (id, name, description) VALUES 
    ('cat-1', 'Electronics', 'Electronic devices and gadgets'),
    ('cat-2', 'Books', 'Physical and digital books');

INSERT INTO users (email, username, role) VALUES 
    ('admin@example.com', 'admin', 'admin'),
    ('user@example.com', 'user', 'user');

INSERT INTO products (name, sku, price, category_id) VALUES 
    ('Laptop', 'LAP-001', 999.99, 'cat-1'),
    ('Programming Book', 'BOOK-001', 49.99, 'cat-2');
```

### 3. Test Schema Reading

```bash
# From the project root directory
go run ./ptah/cmd read-db --db-url postgres://postgres:testpass@localhost:5432/testdb
```

### 4. Expected Output

You should see output like this:

```
Reading schema from database: postgres://postgres:***@localhost:5432/testdb
=== DATABASE SCHEMA ===

Connected to postgres database successfully!

=== DATABASE SCHEMA (POSTGRES) ===
Version: PostgreSQL 15.4 (Debian 15.4-2.pgdg120+1) on x86_64-pc-linux-gnu, compiled by gcc (Debian 12.2.0-14) 12.2.0, 64-bit
Schema: public

SUMMARY:
- Tables: 3
- Enums: 2
- Indexes: 8
- Constraints: 12

=== ENUMS ===
- user_role: [admin, user, guest]
- product_status: [active, inactive, discontinued, out_of_stock]

=== TABLES ===
1. categories (TABLE)
   Columns:
     - id VARCHAR(36) PRIMARY KEY NOT NULL
     - name TEXT NOT NULL
     - description TEXT
   Constraints:
     - PRIMARY KEY (id)
   Indexes:
     - PRIMARY KEY categories_pkey (id)

2. products (TABLE)
   Columns:
     - id SERIAL PRIMARY KEY AUTO_INCREMENT NOT NULL DEFAULT nextval('products_id_seq'::regclass)
     - name VARCHAR(255) NOT NULL
     - description TEXT
     - sku VARCHAR(50) UNIQUE NOT NULL
     - price DECIMAL(10,2) NOT NULL
     - status product_status NOT NULL DEFAULT 'active'
     - in_stock BOOLEAN NOT NULL DEFAULT true
     - category_id VARCHAR(36) NOT NULL
     - created_at TIMESTAMP NOT NULL DEFAULT now()
     - updated_at TIMESTAMP
   Constraints:
     - PRIMARY KEY (id)
     - UNIQUE (sku)
     - FOREIGN KEY category_id -> categories(id)
     - CHECK price CHECK ((price > (0)::numeric))
   Indexes:
     - PRIMARY KEY products_pkey (id)
     - UNIQUE INDEX products_sku_key (sku)
     - INDEX idx_products_category (category_id)
     - INDEX idx_products_status (status)

3. users (TABLE)
   Columns:
     - id SERIAL PRIMARY KEY AUTO_INCREMENT NOT NULL DEFAULT nextval('users_id_seq'::regclass)
     - email VARCHAR(255) UNIQUE NOT NULL
     - username VARCHAR(100) UNIQUE NOT NULL
     - role user_role DEFAULT 'user'
     - is_active BOOLEAN NOT NULL DEFAULT true
     - created_at TIMESTAMP NOT NULL DEFAULT now()
     - updated_at TIMESTAMP
   Constraints:
     - PRIMARY KEY (id)
     - UNIQUE (email)
     - UNIQUE (username)
   Indexes:
     - PRIMARY KEY users_pkey (id)
     - UNIQUE INDEX users_email_key (email)
     - UNIQUE INDEX users_username_key (username)
     - INDEX idx_users_role (role)
     - INDEX idx_users_created_at (created_at)
```

### 5. Cleanup

```bash
# Stop and remove the container
docker stop test-postgres
docker rm test-postgres
```

## Testing with Existing Database

If you have an existing PostgreSQL database, you can test with it directly:

```bash
# Replace with your actual database credentials
go run ./ptah/cmd read-db --db-url postgres://username:password@host:port/database_name
```

## Testing Additional Functionality

### Schema Writing

Test writing a schema from Go entities to the database:

```bash
# Write schema from Go entities (requires entity files)
go run ./ptah/cmd write-db --root-dir ./models --db-url postgres://postgres:testpass@localhost:5432/testdb
```

### Schema Comparison

Test comparing generated schema with database:

```bash
# Compare schemas
go run ./ptah/cmd compare --root-dir ./models --db-url postgres://postgres:testpass@localhost:5432/testdb
```

### MySQL Testing

Test with MySQL/MariaDB:

```bash
# Start MySQL container
docker run --name test-mysql -e MYSQL_ROOT_PASSWORD=testpass -e MYSQL_DATABASE=testdb -p 3306:3306 -d mysql:8.0

# Read MySQL schema
go run ./ptah/cmd read-db --db-url mysql://root:testpass@tcp(localhost:3306)/testdb

# Cleanup
docker stop test-mysql && docker rm test-mysql
```

## Troubleshooting

### Connection Issues

1. **"No connection could be made"**: PostgreSQL server is not running
2. **"password authentication failed"**: Wrong username/password
3. **"database does not exist"**: Database name is incorrect
4. **"permission denied"**: User doesn't have access to the database/schema

### Common Solutions

1. **Check PostgreSQL is running**:
   ```bash
   docker ps  # Should show the postgres container
   ```

2. **Check connection parameters**:
   ```bash
   # Test connection with psql
   psql postgres://postgres:testpass@localhost:5432/testdb
   ```

3. **Check database exists**:
   ```sql
   \l  -- List all databases
   ```

4. **Check schema access**:
   ```sql
   \dn  -- List all schemas
   SET search_path TO public;  -- Set schema
   ```
