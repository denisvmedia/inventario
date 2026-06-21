# Inventario Bare-Metal / systemd Deployment Guide

This guide covers how to deploy and run the Inventario application directly on a
single host (bare metal or VM) under systemd.

> **Kubernetes is the canonical production target.** For a Helm/Kubernetes deploy
> — including the full configuration surface, secret handling, Redis, object
> storage, email, AI vision, ingress/TLS, monitoring, and backups — use the
> [`PRODUCTION.md`](PRODUCTION.md) runbook. The authoritative source of truth for
> every config key and env var is [`helm/inventario/values.yaml`](helm/inventario/values.yaml)
> and the `run` config in `go/cmd/inventario/run/bootstrap/config.go`. This bare-metal
> guide mirrors the same env vars; where it is terser, defer to `PRODUCTION.md`.

## Prerequisites

### System Requirements

- **Operating System**: Linux, macOS, or Windows
- **PostgreSQL**: Version 12 or higher (`memory://` is dev-only — data is lost on restart)
- **Redis**: Recommended. Backs the token blacklist, auth + global rate limiting, CSRF,
  and the email queue. Without it the app uses an in-memory fallback that logs a
  "not suitable for multi-instance" warning and loses that state on restart.
- **Object storage**: `file://` (single host) or S3/R2/GCS/Azure for the upload location
- **SMTP / email provider**: Required for real email (registration, password reset,
  invites, magic-link). The default `stub` provider only logs.
- **Disk Space**: Minimum 1GB for application and initial data storage
- **Memory**: Minimum 512MB RAM (1GB+ recommended)
- **Network**: Access to PostgreSQL database, Redis, and the file storage location

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

### 5. Create the Back-Office (Platform-Operator) Account

`db migrate data` creates the **tenant** and the tenant **admin** user. The
**back-office / platform-operator** account lives in a separate auth plane and
is created with the `backoffice` CLI (run it on the application host with the
DB DSN available in the environment, e.g. `INVENTARIO_DB_DSN`):

```bash
# Create the operator account (a strong random password is generated if --password is omitted)
./inventario backoffice bootstrap \
  --email="ops@yourcompany.com" \
  --name="Operations"

# Enroll TOTP for the operator (required by default — bootstrap sets --mfa-enforced=true)
./inventario backoffice mfa setup --email="ops@yourcompany.com"
```

## Application Configuration

### Environment Variables

Create a production configuration using environment variables:

```bash
# Database configuration (top-level key — NOT under a run/ prefix)
export INVENTARIO_DB_DSN="postgres://inventario:your_secure_password@localhost:5432/inventario?sslmode=disable"

# Server configuration
# The default port is :3333. The run server reads its settings under the
# INVENTARIO_RUN_ prefix; INVENTARIO_ADDR / INVENTARIO_UPLOAD_LOCATION (no prefix)
# are NOT consumed.
export INVENTARIO_RUN_ADDR=":3333"
export INVENTARIO_RUN_UPLOAD_LOCATION="file:///var/lib/inventario/uploads?create_dir=1"
export INVENTARIO_RUN_PUBLIC_URL="https://inventario.example.com"   # used in email links

# Security configuration (BOTH REQUIRED for production)
# If either is left empty the app auto-generates an ephemeral key at boot and logs
# it — that breaks user sessions / MFA (JWT) and every signed file URL (file signing
# key) across restarts, so set stable values and keep them constant forever.
export INVENTARIO_RUN_JWT_SECRET="$(openssl rand -hex 32)"
export INVENTARIO_RUN_FILE_SIGNING_KEY="$(openssl rand -hex 32)"

# Optional: backup signing key for signed .inb backups (#534).
# Auto-generated if empty; set it stably to keep .inb archives verifiable.
export INVENTARIO_RUN_BACKUP_SIGNING_KEY="$(openssl rand -hex 32)"

# CORS (the bundled SPA is same-origin; leave empty to deny cross-origin)
export INVENTARIO_RUN_ALLOWED_ORIGINS=""

# Redis (recommended). Wire each Redis-backed feature to your Redis instance;
# without these the app uses single-instance in-memory fallbacks.
export INVENTARIO_RUN_TOKEN_BLACKLIST_REDIS_URL="redis://localhost:6379/0"
export INVENTARIO_RUN_AUTH_RATE_LIMIT_REDIS_URL="redis://localhost:6379/0"
export INVENTARIO_RUN_GLOBAL_RATE_LIMIT_REDIS_URL="redis://localhost:6379/0"
export INVENTARIO_RUN_CSRF_REDIS_URL="redis://localhost:6379/0"
export INVENTARIO_RUN_EMAIL_QUEUE_REDIS_URL="redis://localhost:6379/0"

# Email provider (default "stub" only logs; use "smtp" for real delivery)
export INVENTARIO_RUN_EMAIL_PROVIDER="smtp"
export INVENTARIO_RUN_EMAIL_FROM="no-reply@example.com"
export INVENTARIO_RUN_SMTP_HOST="smtp.example.com"
export INVENTARIO_RUN_SMTP_PORT="587"
export INVENTARIO_RUN_SMTP_USERNAME="smtp-user"
export INVENTARIO_RUN_SMTP_PASSWORD="smtp-password"
export INVENTARIO_RUN_SMTP_USE_TLS="true"

# Worker configuration (optional)
export INVENTARIO_RUN_MAX_CONCURRENT_EXPORTS="3"
export INVENTARIO_RUN_MAX_CONCURRENT_IMPORTS="1"

# Thumbnail generation configuration (optional)
export INVENTARIO_RUN_THUMBNAIL_MAX_CONCURRENT_PER_USER="5"
export INVENTARIO_RUN_THUMBNAIL_RATE_LIMIT_PER_MINUTE="50"
export INVENTARIO_RUN_THUMBNAIL_SLOT_DURATION="30m"

# Timezone (optional)
export TZ="UTC"
```

### Configuration File (Alternative)

Instead of environment variables, you can create a `config.yaml` file:

```yaml
# db-dsn is a top-level key (not nested under a "database:" key).
db-dsn: "postgres://inventario:your_secure_password@localhost:5432/inventario?sslmode=disable"

run:
  addr: ":3333"
  upload-location: "file:///var/lib/inventario/uploads?create_dir=1"
  public-url: "https://inventario.example.com"
  jwt-secret: "your-secure-32-byte-secret-here"
  file-signing-key: "your-secure-32-byte-file-signing-key-here"
  allowed-origins: ""
  token-blacklist-redis-url: "redis://localhost:6379/0"
  csrf-redis-url: "redis://localhost:6379/0"
  email-provider: "smtp"
  email-from: "no-reply@example.com"
  smtp-host: "smtp.example.com"
  smtp-port: 587
  smtp-username: "smtp-user"
  smtp-use-tls: true
  max-concurrent-exports: 3
  max-concurrent-imports: 1
  thumbnail-max-concurrent-per-user: 5
  thumbnail-rate-limit-per-minute: 50
  thumbnail-slot-duration: "30m"
```

### Security Considerations

1. **JWT Secret & File Signing Key**: Both are required in production. Generate
   each as a secure 32+ byte value and keep them constant across restarts:
   ```bash
   openssl rand -hex 32   # INVENTARIO_RUN_JWT_SECRET
   openssl rand -hex 32   # INVENTARIO_RUN_FILE_SIGNING_KEY
   ```
   Leaving either empty makes the app auto-generate an ephemeral key at boot:
   rotating the JWT secret invalidates all sessions and MFA enrollments, and a
   changing file signing key breaks every outstanding signed download URL.

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
  --addr=":3333" \
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
Environment=INVENTARIO_RUN_ADDR=:3333
Environment=INVENTARIO_RUN_UPLOAD_LOCATION=file:///var/lib/inventario/uploads?create_dir=1
Environment=INVENTARIO_RUN_JWT_SECRET=your-secure-32-byte-secret-here
Environment=INVENTARIO_RUN_FILE_SIGNING_KEY=your-secure-32-byte-file-signing-key-here

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
# Check readiness endpoint (DB/Redis dependencies reachable)
curl http://localhost:3333/readyz

# Check web interface
curl http://localhost:3333/
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

### Observability

Prometheus metrics are **always on** at `/metrics` — on the API HTTP port (`3333`) and, in split deployments, on each worker's probe port (`3334`). No flag is required to expose them.

```bash
# Scrape metrics from a running instance
curl http://localhost:3333/metrics
```

- **Local / docker-compose:** a ready-to-run Prometheus + Grafana stack lives at [`deploy/monitoring/`](deploy/monitoring/README.md) (`docker compose --profile monitoring up -d`).
- **Kubernetes:** the Helm chart can emit `prometheus.io/*` pod annotations (operator-less discovery) or a ServiceMonitor (Prometheus Operator). Both default off — see [`helm/inventario/README.md`](helm/inventario/README.md) → "Metrics & scraping".

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

### Multi-Factor Authentication (MFA)

Inventario supports time-based one-time passwords (TOTP, RFC 6238) as a
second authentication factor. The feature is enabled out of the box —
users opt in from `Settings → Privacy & Security`.

**Operational notes:**

- **Secret storage:** TOTP secrets are stored encrypted-at-rest in the
  `user_mfa_secrets` table. Encryption keys are derived from `JWT_SECRET`
  via HKDF, so rotating `JWT_SECRET` will render every existing MFA
  enrollment unreadable. If you rotate the JWT secret, plan to reset
  every enrolled user's MFA (see "User recovery" below) and notify them
  to re-enroll. Sessions are likewise invalidated by the rotation.
- **Backup codes:** Each user receives 10 single-use backup codes at
  enrollment, stored as bcrypt hashes. Once consumed they cannot be
  recovered; users regenerate them from the same Settings page.
- **Login history:** Failed second-factor attempts surface as
  `bad_mfa` rows in `login_events`; step-1 password successes that
  required MFA surface as `mfa_required`. Operator-driven resets land
  as `mfa_admin_reset`.

**User recovery (lost authenticator):**

The supported v1 recovery flow is "contact support, the operator
clears your enrollment, you re-enroll." Run on the application host:

```bash
# Preview the reset (no changes)
./inventario users mfa-reset alex@example.com --dry-run

# Perform the reset (prompts for confirmation)
./inventario users mfa-reset alex@example.com

# Or skip the prompt (automation)
./inventario users mfa-reset alex@example.com --force
```

The user keeps their password — they just stop being challenged for a
second factor on next sign-in, and can re-enable MFA from Settings.

## Troubleshooting

### Common Issues

1. **Database Connection Failed**: Check DSN format and user permissions
2. **Permission Denied on Upload Directory**: Verify directory ownership and permissions
3. **JWT Token Issues**: Ensure JWT secret is set and consistent across restarts
4. **Migration Failures**: Check database user permissions and run bootstrap migrations first

### Logs and Debugging

There is no log-level knob (`LOG_LEVEL` / `INVENTARIO_LOG_LEVEL` are not consumed).
The only logging control is the output format — set it to `json` for structured logs:

```bash
export INVENTARIO_LOG_FORMAT=json
./inventario run
```

For more detailed troubleshooting, check the application logs and PostgreSQL logs.
