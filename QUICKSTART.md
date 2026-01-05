# Inventario Quick Start Guide

Get Inventario up and running in under 5 minutes using Docker Compose.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [What Just Happened?](#what-just-happened)
- [Configuration Reference](#configuration-reference)
- [Data Persistence](#data-persistence)
- [Troubleshooting](#troubleshooting)
- [Advanced Usage](#advanced-usage)
- [Upgrading and Migrations](#upgrading-and-migrations)
- [Security Notes](#security-notes)
- [Getting Help](#getting-help)
- [What's Next?](#whats-next)

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/) installed
- Git (to clone the repository)

## Quick Start

### 1. Clone the Repository

```bash
git clone https://github.com/denisvmedia/inventario.git
cd inventario
```

### 2. Start the Application

```bash
docker-compose up -d
```

That's it! The first startup will take a few minutes to:
- Build the application image
- Set up PostgreSQL database
- Run database migrations
- Create a default tenant and admin user
- Seed the database with example data (optional, enabled by default)

### 3. Access the Application

Open your browser and navigate to:

```
http://localhost:3333
```

**Login Credentials** (from `.env.example` configuration):
- **Email:** `admin@example.com`
- **Password:** `admin123`

> **Note:** If you created a custom `.env` file with different `ADMIN_EMAIL` and `ADMIN_PASSWORD` values, use those credentials instead.
>
> **Important:** Change the admin password after your first login!

## What Just Happened?

The docker-compose setup automatically:

1. **Started PostgreSQL** database server
2. **Bootstrapped** database extensions and roles
3. **Ran migrations** to create all database tables
4. **Created initial data**:
   - Default tenant: "Test Organization"
   - Admin user: admin@example.com
5. **Seeded example data** (enabled by default):
   - System settings (Main Currency: CZK)
   - Sample locations (Home, Office, Storage Unit)
   - Sample areas and commodities
   - **Note:** Set `SEED_DATABASE=false` in `.env` for a clean start
6. **Started the application** on port 3333

## Next Steps

### Change Default Credentials (Recommended)

For production use, customize the setup:

```bash
# 1. Stop the containers
docker-compose down -v

# 2. Create environment file
cp .env.example .env

# 3. Generate secure secrets
openssl rand -hex 32  # Copy this for JWT_SECRET
openssl rand -hex 32  # Copy this for FILE_SIGNING_KEY

# 4. Edit .env file with your values
nano .env  # or use your preferred editor
```

Update these critical values in `.env`:
- `JWT_SECRET` - Secure random value for authentication
- `FILE_SIGNING_KEY` - Secure random value for file URLs
- `ADMIN_EMAIL` - Your admin email
- `ADMIN_PASSWORD` - Strong password
- `POSTGRES_PASSWORD` - Database password

```bash
# 5. Restart with new configuration
docker-compose up -d
```

### View Logs

```bash
# All services
docker-compose logs -f

# Just the application
docker-compose logs -f inventario

# Just the database
docker-compose logs -f postgres
```

### Stop the Application

```bash
# Stop containers (data persists)
docker-compose down

# Stop and remove all data (fresh start)
docker-compose down -v
```

## Configuration Reference

### Environment Variables (.env file)

| Variable | Default | Description |
|----------|---------|-------------|
| `INVENTARIO_PORT` | `3333` | Port to access the application |
| `POSTGRES_PASSWORD` | `inventario_password` | PostgreSQL password |
| `JWT_SECRET` | (see .env.example) | JWT signing secret (32+ chars) |
| `FILE_SIGNING_KEY` | (see .env.example) | File URL signing key (32+ chars) |
| `ADMIN_EMAIL` | `admin@example.com` | Initial admin email |
| `ADMIN_PASSWORD` | `admin123` | Initial admin password |
| `DEFAULT_TENANT_NAME` | `Test Organization` | Organization name |
| `SEED_DATABASE` | `true` | Seed with example data and settings (set to `false` for clean start) |

See `.env.example` for all available options.

### Port Configuration

If port 3333 is already in use, change it in `.env`:

```bash
INVENTARIO_PORT=8080
```

Then access the app at `http://localhost:8080`

## Data Persistence

All data is stored in `.docker/` directory:

```
.docker/
├── postgresql/       # Database files
├── inventario/
│   └── uploads/      # Uploaded files (images, documents)
└── init-state/       # Initialization tracking
```

**Backup your data:**

```bash
# Create a backup
tar -czf inventario-backup-$(date +%Y%m%d).tar.gz .docker/

# Restore from backup
tar -xzf inventario-backup-YYYYMMDD.tar.gz
```

## Troubleshooting

### Port Already in Use

**Error:** `Bind for 0.0.0.0:3333 failed: port is already allocated`

**Solution:** Change `INVENTARIO_PORT` in `.env` file to a different port (e.g., 8080)

### Cannot Login

**Problem:** Login fails with default credentials

**Solutions:**
1. Ensure containers are fully started: `docker-compose ps` (all should be "healthy" or "running")
2. Check logs: `docker-compose logs inventario-init-data`
3. Reset data: `docker-compose down -v && docker-compose up -d`

### Database Connection Errors

**Problem:** Application can't connect to database

**Solutions:**
1. Check PostgreSQL is healthy: `docker-compose ps postgres`
2. View database logs: `docker-compose logs postgres`
3. Restart services: `docker-compose restart`

### Permission Denied Errors

**Problem:** Docker volume mount errors

**Solution:** Ensure Docker has permission to create directories in the project folder

## Advanced Usage

### Running Tests

```bash
# Run all tests (Go + Frontend)
make test

# Run with PostgreSQL test database
docker-compose --profile test up -d postgres-test
docker-compose --profile test run inventario-test
```

### Development Mode

For active development, use the development setup:

```bash
# Backend (runs on :3333)
make run-backend

# Frontend (runs on :5173 with hot reload)
make run-frontend

# Both concurrently
make run-dev
```

### Database Operations

```bash
# Access PostgreSQL directly
docker-compose exec postgres psql -U inventario inventario

# Create database backup
docker-compose exec postgres pg_dump -U inventario inventario > backup.sql

# Restore from backup
docker-compose exec -T postgres psql -U inventario inventario < backup.sql
```

## Upgrading and Migrations

### Upgrading to a New Version

When upgrading Inventario, migrations run automatically:

```bash
# 1. Create a backup first (IMPORTANT!)
docker-compose exec postgres pg_dump -U inventario inventario > backup-$(date +%Y%m%d).sql
tar -czf uploads-backup-$(date +%Y%m%d).tar.gz .docker/inventario/uploads/

# 2. Pull latest code
git pull

# 3. Stop services
docker-compose down

# 4. Rebuild with latest code
docker-compose build --pull

# 5. Start services (migrations run automatically)
docker-compose up -d

# 6. Check migration logs
docker-compose logs inventario-bootstrap
docker-compose logs inventario-migrate

# 7. Verify application is healthy
docker-compose ps
curl http://localhost:3333/api/health
```

**What happens during startup:**
1. **Bootstrap service** - Updates database extensions and roles (idempotent)
2. **Migration service** - Applies new schema changes
3. **Init-data service** - Skipped (only runs on first installation)
4. **Application** - Starts after migrations complete

### Manual Migration Operations

For advanced use cases, you can run migrations manually:

#### Check Migration Status

```bash
# See which migrations have been applied
docker-compose exec inventario ./inventario db migrate status

# Expected output:
# Migration: 001_initial_schema.up.sql - Applied
# Migration: 002_add_user_roles.up.sql - Applied
# Migration: 003_add_file_signing.up.sql - Pending
```

#### Run Migrations Manually

```bash
# Run all pending migrations
docker-compose exec inventario ./inventario db migrate up

# Run specific number of migrations
docker-compose exec inventario ./inventario db migrate up --steps 1

# Preview migrations without executing (dry-run)
docker-compose exec inventario ./inventario db migrate up --dry-run
```

#### Rollback Migrations

```bash
# Rollback last migration
docker-compose exec inventario ./inventario db migrate down --steps 1

# Preview rollback without executing
docker-compose exec inventario ./inventario db migrate down --steps 1 --dry-run

# Check status after rollback
docker-compose exec inventario ./inventario db migrate status
```

#### Force Migration Version

```bash
# Mark a migration as applied without running it
docker-compose exec inventario ./inventario db migrate force --version 3

# Useful for recovery scenarios
```

### Migration Troubleshooting

#### Migration Failed During Upgrade

**Problem:** Migration service exits with error

**Solution:**

```bash
# 1. Check migration logs
docker-compose logs inventario-migrate

# 2. Check bootstrap logs
docker-compose logs inventario-bootstrap

# 3. Access database to inspect issue
docker-compose exec postgres psql -U inventario inventario

# 4. If migration is stuck, check schema_migrations table
docker-compose exec postgres psql -U inventario inventario -c "SELECT * FROM schema_migrations ORDER BY version DESC LIMIT 5;"

# 5. Restore from backup if needed
docker-compose down
docker-compose exec -T postgres psql -U inventario inventario < backup-YYYYMMDD.sql
```

#### Application Won't Start After Upgrade

**Problem:** Application container exits immediately

**Solutions:**

```bash
# 1. Check application logs
docker-compose logs inventario

# 2. Verify all migrations completed successfully
docker-compose logs inventario-migrate | grep -i "error\|failed"

# 3. Check database connection
docker-compose exec postgres pg_isready -U inventario

# 4. Verify database schema
docker-compose exec inventario ./inventario db migrate status

# 5. If database is corrupted, restore from backup
docker-compose down
# Restore PostgreSQL data from backup
rm -rf .docker/postgresql
docker-compose up -d postgres
# Wait for PostgreSQL to initialize
docker-compose exec -T postgres psql -U inventario inventario < backup-YYYYMMDD.sql
docker-compose up -d
```

#### Migrations Running But App Shows Old Version

**Problem:** Schema changes not reflected in application

**Solutions:**

```bash
# 1. Force rebuild without cache
docker-compose build --no-cache

# 2. Ensure old containers are removed
docker-compose down
docker system prune -f

# 3. Start fresh
docker-compose up -d --force-recreate
```

### Zero-Downtime Upgrades (Advanced)

For production environments:

```bash
# 1. Backup everything
docker-compose exec postgres pg_dump -U inventario inventario > backup.sql

# 2. Test upgrade in separate environment first
# (use different port and database)

# 3. Run migrations in separate container
docker-compose run --rm inventario-migrate

# 4. If successful, rebuild and restart main app
docker-compose up -d --no-deps inventario

# 5. Monitor logs for issues
docker-compose logs -f inventario
```

### Database Schema Information

```bash
# List all tables
docker-compose exec postgres psql -U inventario inventario -c "\dt"

# View table structure
docker-compose exec postgres psql -U inventario inventario -c "\d+ commodities"

# Check database size
docker-compose exec postgres psql -U inventario inventario -c "SELECT pg_size_pretty(pg_database_size('inventario'));"

# View migration history
docker-compose exec postgres psql -U inventario inventario -c "SELECT * FROM schema_migrations ORDER BY version;"
```

### Backup and Restore

#### Complete Backup

```bash
# Create complete backup (database + files)
./scripts/backup.sh  # If you have a backup script

# Or manually:
docker-compose exec postgres pg_dump -U inventario inventario > backup-$(date +%Y%m%d-%H%M%S).sql
tar -czf inventario-full-backup-$(date +%Y%m%d-%H%M%S).tar.gz \
  backup-*.sql \
  .docker/inventario/uploads/ \
  .env
```

#### Restore from Backup

```bash
# 1. Stop application
docker-compose down

# 2. Remove old data
rm -rf .docker/postgresql .docker/inventario/uploads

# 3. Start database only
docker-compose up -d postgres

# 4. Wait for database to be ready
docker-compose exec postgres pg_isready -U inventario

# 5. Restore database
docker-compose exec -T postgres psql -U inventario inventario < backup-YYYYMMDD.sql

# 6. Restore files
tar -xzf inventario-full-backup-YYYYMMDD.tar.gz

# 7. Start all services
docker-compose up -d

# 8. Verify
curl http://localhost:3333/api/health
```

## Security Notes

**Default configuration is for development only!**

For production deployment:

- ✅ Generate secure random values for `JWT_SECRET` and `FILE_SIGNING_KEY`
- ✅ Use strong passwords for database users
- ✅ Change default admin credentials
- ✅ Use HTTPS with a reverse proxy (nginx, Traefik, Caddy)
- ✅ Restrict PostgreSQL port (remove `ports:` from postgres service)
- ✅ Enable firewall rules
- ✅ Regular backups of `.docker/postgresql/` and `.docker/inventario/uploads/`

## Getting Help

- **Documentation:** See [README.md](README.md) for detailed documentation
- **Issues:** Report bugs at [GitHub Issues](https://github.com/denisvmedia/inventario/issues)
- **Example Setup:** Check `example/` directory for production-ready configuration

## What's Next?

Now that you have Inventario running:

1. **Add your first items** - Start organizing your inventory
2. **Create locations** - Set up rooms, storage areas, etc.
3. **Upload images** - Document your items visually
4. **Explore features** - Check out categories, tags, and search
5. **Read the docs** - Learn about advanced features in README.md

Enjoy managing your inventory!
