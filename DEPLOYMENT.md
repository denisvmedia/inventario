# Inventario Production Deployment Guide

This guide covers how to deploy and run the Inventario application in a production environment.

## Prerequisites

### System Requirements

- **Operating System**: Linux, macOS, or Windows
- **PostgreSQL**: Version 12 or higher
- **Disk Space**: Minimum 1GB for application and initial data storage
- **Memory**: Minimum 512MB RAM (1GB+ recommended)
- **Network**: Access to PostgreSQL database and file storage location

### Required Tools

- PostgreSQL client tools (`psql`, `createdb`)
- OpenSSL (for generating JWT secrets)
- Compiled Inventario binary

## Database Setup

### 1. Create Database and Users

First, create the PostgreSQL database and required users. Connect to PostgreSQL as a superuser:

```bash
# Connect as postgres superuser
psql -U postgres

# Create the database
CREATE DATABASE inventario;

# Create operational user (for running the application)
CREATE USER inventario WITH PASSWORD 'your_secure_password_here';

# Create migration user (for running database migrations)
CREATE USER inventario_migrator WITH PASSWORD 'your_migration_password_here';

# Grant database access
GRANT CONNECT ON DATABASE inventario TO inventario;
GRANT CONNECT ON DATABASE inventario TO inventario_migrator;

# Exit psql
\q
```

### 2. Run Bootstrap Migrations

Bootstrap migrations set up database extensions and roles. Run these with a privileged database user:

```bash
# Apply bootstrap migrations (requires superuser privileges)
./inventario db bootstrap apply \
  --db-dsn="postgres://postgres:password@localhost:5432/inventario?sslmode=disable" \
  --username=inventario \
  --username-for-migrations=inventario_migrator
```

**Note**: Bootstrap migrations are idempotent and can be run multiple times safely.

### 3. Run Schema Migrations

Apply the database schema migrations:

```bash
# Run schema migrations
./inventario db migrate up \
  --db-dsn="postgres://inventario_migrator:migration_password@localhost:5432/inventario?sslmode=disable"
```

### 4. Setup Initial Dataset

Create the initial tenant and admin user:

```bash
# Setup initial dataset with default values
./inventario db migrate data \
  --db-dsn="postgres://inventario_migrator:migration_password@localhost:5432/inventario?sslmode=disable"

# Or with custom values
./inventario db migrate data \
  --db-dsn="postgres://inventario_migrator:migration_password@localhost:5432/inventario?sslmode=disable" \
  --admin-email="admin@yourcompany.com" \
  --admin-password="secure_admin_password" \
  --admin-name="System Administrator" \
  --default-tenant-name="Your Organization"
```

## Application Configuration

### Environment Variables

Create a production configuration using environment variables:

```bash
# Database configuration
export INVENTARIO_DB_DSN="postgres://inventario:your_secure_password@localhost:5432/inventario?sslmode=disable"

# Server configuration
export INVENTARIO_ADDR=":8080"
export INVENTARIO_UPLOAD_LOCATION="file:///var/lib/inventario/uploads?create_dir=1"

# Security configuration (REQUIRED for production)
export INVENTARIO_RUN_JWT_SECRET="$(openssl rand -hex 32)"

# Worker configuration (optional)
export INVENTARIO_RUN_MAX_CONCURRENT_EXPORTS="3"
export INVENTARIO_RUN_MAX_CONCURRENT_IMPORTS="1"

# Timezone (optional)
export TZ="UTC"
```

### Configuration File (Alternative)

Instead of environment variables, you can create a `config.yaml` file:

```yaml
database:
  db-dsn: "postgres://inventario:your_secure_password@localhost:5432/inventario?sslmode=disable"

run:
  addr: ":8080"
  upload-location: "file:///var/lib/inventario/uploads?create_dir=1"
  jwt-secret: "your-secure-32-byte-secret-here"
  max-concurrent-exports: 3
  max-concurrent-imports: 1
```

### Security Considerations

1. **JWT Secret**: Generate a secure 32+ character secret:
   ```bash
   openssl rand -hex 32
   ```

2. **Database Passwords**: Use strong, unique passwords for database users

3. **File Permissions**: Ensure upload directory has appropriate permissions:
   ```bash
   sudo mkdir -p /var/lib/inventario/uploads
   sudo chown inventario:inventario /var/lib/inventario/uploads
   sudo chmod 755 /var/lib/inventario/uploads
   ```

4. **Network Security**: Configure firewall rules to restrict database access

5. **SSL/TLS**: Use SSL connections to PostgreSQL in production:
   ```
   postgres://user:pass@host:5432/db?sslmode=require
   ```

## Running the Application

### Direct Execution

```bash
# Run with environment variables
./inventario run

# Or with command line flags
./inventario run \
  --addr=":8080" \
  --db-dsn="postgres://inventario:password@localhost:5432/inventario?sslmode=disable" \
  --upload-location="file:///var/lib/inventario/uploads?create_dir=1"
```

### Systemd Service (Linux)

Create a systemd service file at `/etc/systemd/system/inventario.service`:

```ini
[Unit]
Description=Inventario Personal Inventory Service
After=network.target postgresql.service
Requires=postgresql.service

[Service]
Type=simple
User=inventario
Group=inventario
WorkingDirectory=/opt/inventario
ExecStart=/opt/inventario/inventario run
Restart=always
RestartSec=5

# Environment variables
Environment=INVENTARIO_DB_DSN=postgres://inventario:password@localhost:5432/inventario?sslmode=disable
Environment=INVENTARIO_ADDR=:8080
Environment=INVENTARIO_UPLOAD_LOCATION=file:///var/lib/inventario/uploads?create_dir=1
Environment=INVENTARIO_RUN_JWT_SECRET=your-secure-32-byte-secret-here

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/inventario

[Install]
WantedBy=multi-user.target
```

Enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable inventario
sudo systemctl start inventario
sudo systemctl status inventario
```

## Verification

### Health Check

Once the application is running, verify it's working:

```bash
# Check health endpoint
curl http://localhost:8080/api/health

# Check web interface
curl http://localhost:8080/
```

### Database Connection

Verify database connectivity:

```bash
# Check migration status
./inventario db status \
  --db-dsn="postgres://inventario:password@localhost:5432/inventario?sslmode=disable"
```

### Log Monitoring

Monitor application logs:

```bash
# For systemd service
sudo journalctl -u inventario -f

# For direct execution
./inventario run 2>&1 | tee /var/log/inventario.log
```

## Maintenance

### Database Backups

Regular database backups are essential:

```bash
# Create backup
pg_dump -U inventario -h localhost inventario > inventario_backup_$(date +%Y%m%d_%H%M%S).sql

# Restore from backup
psql -U inventario -h localhost inventario < inventario_backup_20240101_120000.sql
```

### Application Updates

When updating the application:

1. Stop the service
2. Backup the database
3. Replace the binary
4. Run any new migrations
5. Start the service

```bash
sudo systemctl stop inventario
# Replace binary and run migrations if needed
./inventario db migrate up --db-dsn="postgres://inventario_migrator:password@localhost:5432/inventario"
sudo systemctl start inventario
```

## Troubleshooting

### Common Issues

1. **Database Connection Failed**: Check DSN format and user permissions
2. **Permission Denied on Upload Directory**: Verify directory ownership and permissions
3. **JWT Token Issues**: Ensure JWT secret is set and consistent across restarts
4. **Migration Failures**: Check database user permissions and run bootstrap migrations first

### Logs and Debugging

Enable verbose logging by setting log level:

```bash
export LOG_LEVEL=debug
./inventario run
```

For more detailed troubleshooting, check the application logs and PostgreSQL logs.
