# Inventario - Your Personal Inventory

Welcome to Inventario - the ultimate app for managing and organizing your personal inventory.

Note, the project is still under development.

## Future Features

- **Inventory Management**: Easily create, update, and delete items in your inventory. Add essential details such as item name, description, location, purchase date, and more.

- **Categorization and Tags**: Categorize your items into different areas, such as rooms in your house, storage units, or any custom locations you prefer. Assign tags to items for easy filtering and organization.

- **Commodity Tracking**: Track the status of your items, including whether they are in use, sold, lost, disposed of, or written off. Monitor the purchase and registration dates, as well as any comments or additional information.

- **Price and Currency Management**: Keep track of the original and current prices of your items. Inventario supports multiple currencies, allowing you to monitor the value of your inventory in your preferred currency.

- **Attachments and Documentation**: Attach images, manuals, invoices, and other important documents to your items for easy reference and documentation.

- **User-friendly Interface**: Inventario offers a clean and intuitive interface that makes managing your inventory a breeze. The app is designed with a focus on simplicity and efficiency, ensuring that you can easily navigate and access all the necessary features.

- **Locations and Areas**: Organize your items into locations and areas to create a structured inventory. Define custom locations such as rooms, storage spaces, or any other relevant categories that suit your needs.

- **Secure File Access**: Advanced signed URL system for secure file downloads without exposing authentication tokens. Files are protected with time-limited, tamper-proof URLs that automatically expire.

## Building and Running

Inventario is a Go application with a frontend built using web technologies. The following instructions will help you set up and run the application on your system.

## Database Support

Inventario supports multiple database backends:

- **Memory**: In-memory database (default, data is lost when the application is restarted)
- **PostgreSQL**: Full-featured SQL database (recommended for production use)

You can specify the database to use with the `--db-dsn` flag when running the application:

```bash
# Memory database (default)
./inventario run --db-dsn memory://

# PostgreSQL database
./inventario run --db-dsn postgres://username:password@localhost:5432/inventario
```

For PostgreSQL, you need to create the database before running the application:

```bash
# Create the database
createdb inventario

# Or using psql
psql -c "CREATE DATABASE inventario;"
```

The application will automatically create the necessary tables and indexes when it starts.

## Dry Run Mode

Inventario supports dry run mode for all database operations, allowing you to preview changes before they are executed:

```bash
# Preview database migrations without executing them
./inventario db migrate up --db-dsn postgres://user:pass@localhost/db --dry-run

# Create tenants and users for initial setup
./inventario tenants create --name="My Organization" --slug="my-org" --dry-run
./inventario users create --email="admin@example.com" --tenant="my-org" --role="admin" --dry-run
```

For schema management operations using the Ptah tool:

```bash
# Preview schema creation
go run ./ptah/cmd write-db --root-dir ./models --db-url postgres://user:pass@localhost/db --dry-run

# Preview schema deletion
go run ./ptah/cmd drop-schema --root-dir ./models --db-url postgres://user:pass@localhost/db --dry-run

# Preview complete database cleanup
go run ./ptah/cmd drop-all --db-url postgres://user:pass@localhost/db --dry-run
```

Dry run mode is especially useful for:
- Testing configurations before applying to production
- Reviewing changes in CI/CD pipelines
- Learning what operations each command performs
- Debugging schema generation issues

## CLI Commands

Inventario provides comprehensive command-line tools for system administration and initial setup.

### User and Tenant Management

Complete CRUD (Create, Read, Update, Delete) operations for users and tenants:

#### Tenant Management
```bash
# Create a tenant (organization)
./inventario tenants create --name="Acme Corporation" --slug="acme" --domain="acme.com"

# Create a tenant interactively (similar to Linux adduser)
./inventario tenants create

# List all tenants
./inventario tenants list

# List active tenants only
./inventario tenants list --status=active

# Search tenants
./inventario tenants list --search=acme

# Get detailed tenant information
./inventario tenants get acme

# Update tenant properties
./inventario tenants update acme --name="Acme Corp Ltd" --domain="newdomain.com"

# Update tenant interactively
./inventario tenants update acme --interactive

# Delete tenant with confirmation
./inventario tenants delete acme

# Preview operations without making changes
./inventario tenants create --dry-run --name="Test Org"
./inventario tenants update acme --name="New Name" --dry-run
./inventario tenants delete acme --dry-run
```

#### User Management
```bash
# Create an admin user
./inventario users create --email="admin@acme.com" --tenant="acme" --role="admin"

# Create a user interactively with secure password input
./inventario users create

# List all users
./inventario users list

# List users in specific tenant
./inventario users list --tenant=acme

# List admin users only
./inventario users list --role=admin

# Search users
./inventario users list --search=john

# Get detailed user information
./inventario users get admin@acme.com

# Update user properties
./inventario users update admin@acme.com --name="New Name" --role="user"

# Change user password
./inventario users update admin@acme.com --password

# Move user to different tenant
./inventario users update admin@acme.com --tenant="other-tenant"

# Deactivate user
./inventario users update admin@acme.com --active=false

# Update user interactively
./inventario users update admin@acme.com --interactive

# Delete user with confirmation
./inventario users delete admin@acme.com

# Preview operations without making changes
./inventario users create --dry-run --email="test@example.com" --tenant="acme"
./inventario users update admin@acme.com --role="user" --dry-run
./inventario users delete admin@acme.com --dry-run
```

**Important**: User and tenant commands only work with PostgreSQL databases. Memory databases are not supported for persistent operations.

#### Command Options

**Tenant Commands:**
- `create`: Create new tenants with validation and auto-generated slugs
- `list`: List tenants with filtering by status, search, and pagination
- `get`: Get detailed tenant information including user count
- `update`: Update tenant properties with validation
- `delete`: Delete tenants with confirmation and impact assessment

**User Commands:**
- `create`: Create new users with secure password input and validation
- `list`: List users with filtering by tenant, role, active status, and search
- `get`: Get detailed user information including tenant details
- `update`: Update user properties including password changes and tenant moves
- `delete`: Delete users with confirmation prompts

**Common Flags:**
- `--dry-run`: Preview operations without making changes
- `--interactive`: Enable guided prompts for all fields
- `--no-interactive`: Disable interactive prompts (use flags only)
- `--output`: Output format (table, json) for list and get commands
- `--force`: Skip confirmation prompts for delete operations

**Filtering and Pagination:**
- `--limit`: Maximum number of results (default: 50)
- `--offset`: Number of results to skip (default: 0)
- `--search`: Search by name, slug, or email
- `--status`: Filter by status (tenants: active, suspended, inactive)
- `--role`: Filter by role (users: admin, user)
- `--active`: Filter by active status (users: true, false)
- `--tenant`: Filter by tenant (users only)

### Database Management

```bash
# Run database migrations
./inventario db migrate up --db-dsn postgres://user:pass@localhost/db

# Initialize configuration
./inventario init-config

# Start the web server
./inventario run --db-dsn postgres://user:pass@localhost/db
```

### Prerequisites

- **Go**: Version 1.24 or higher
- **Node.js**: Version 22.15 or higher (managed via Volta)
- **Git**: For cloning the repository

### macOS

1. **Install prerequisites**:
   ```bash
   # Install Homebrew if not already installed
   /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

   # Install Go
   brew install go

   # Install Volta (Node.js version manager)
   brew install volta
   ```

2. **Clone and build the application**:
   ```bash
   git clone git@github.com:denisvmedia/inventario.git
   cd inventario
   make all
   ```

3. **Run the application**:
   ```bash
   cd bin && ./inventario run
   ```

4. **Seed the database** (optional, for development):
   ```bash
   curl -X POST http://localhost:3333/api/v1/seed
   ```

5. **Access the application**:
   Open your browser and navigate to http://localhost:3333/

### Linux

1. **Install prerequisites**:
   ```bash
   # Install Go (Ubuntu/Debian example)
   sudo apt update
   sudo apt install golang-go

   # Install Volta (Node.js version manager)
   curl https://get.volta.sh | bash
   source ~/.bashrc  # or restart your terminal
   ```

2. **Clone and build the application**:
   ```bash
   git clone git@github.com:denisvmedia/inventario.git
   cd inventario
   make all
   ```

3. **Run the application**:
   ```bash
   cd bin && ./inventario run
   ```

4. **Seed the database** (optional, for development):
   ```bash
   curl -X POST http://localhost:3333/api/v1/seed
   ```

5. **Access the application**:
   Open your browser and navigate to http://localhost:3333/

### Windows

1. **Install prerequisites**:
   - Install Go from [golang.org](https://golang.org/dl/)
   - Install Git from [git-scm.com](https://git-scm.com/download/win)
   - Install Volta using one of the following methods:
     - [Official installer](https://volta.sh/)
     - Using Scoop: `scoop install volta`
     - Using winget: `winget install volta.volta`

2. **Clone and build the application**:
   ```powershell
   git clone git@github.com:denisvmedia/inventario.git
   cd inventario
   make all
   ```
   Note: If you don't have Make installed, you can use Git Bash which includes Make.

3. **Run the application**:
   ```powershell
   cd bin
   .\inventario.exe run
   ```

4. **Seed the database** (optional, for development):
   ```powershell
   Invoke-RestMethod -Method POST -Uri "http://localhost:3333/api/v1/seed"
   ```
   or using curl if installed:
   ```
   curl -X POST http://localhost:3333/api/v1/seed
   ```

5. **Access the application**:
   Open your browser and navigate to http://localhost:3333/

## Testing

### Unit Tests

Run the unit test suite:

```bash
cd go
go test ./...
```

### Integration Tests

For integration tests with PostgreSQL:

```bash
export POSTGRES_TEST_DSN="postgres://user:password@localhost:5432/test_db?sslmode=disable"
go test -tags=integration ./...
```

### CLI Workflow Integration Test

A comprehensive integration test validates the complete workflow from fresh database setup through CLI operations to API access:

```bash
# Set PostgreSQL DSN
export POSTGRES_TEST_DSN="postgres://user:password@localhost:5432/test_db?sslmode=disable"

# Run the CLI workflow integration test
cd go
go test -tags=integration ./integration_test/cli_workflow_integration_test.go -v

# Or use the provided scripts
./scripts/run-integration-tests.sh  # Linux/macOS
.\scripts\run-integration-tests.ps1  # Windows PowerShell
```

This test validates:
- Fresh database setup with bootstrap and migrations
- CLI tenant and user creation commands
- Authentication flow (login failure/success)
- API access with JWT tokens
- Complete end-to-end workflow for CI/CD pipelines

## Documentation

- [Signed URLs for Secure File Access](SIGNED_URLS.md) - Comprehensive guide to the secure file access system

## License
This module is licensed under the MIT License. See the [LICENSE](LICENSE) file for details. You are free to use, modify, and distribute this software in accordance with the terms of the license.

## Author

[Denis Voytyuk](https://github.com/denisvmedia)
