# Inventario Production Docker Deployment

This directory contains a production-ready Docker Compose configuration for deploying Inventario with PostgreSQL database and proper initialization handling.

## Quick Start

1. **Copy and customize the override file:**
   ```bash
   cp docker-compose.override.yaml.example docker-compose.override.yaml
   # Edit docker-compose.override.yaml with your production values
   ```

2. **Generate a secure JWT secret:**
   ```bash
   openssl rand -hex 32
   # Add this to JWT_SECRET in docker-compose.override.yaml
   ```

3. **Start the services:**
   ```bash
   docker-compose up -d
   ```

   **Note**: The Docker build process automatically includes the frontend and uses proper version injection matching the Makefile build process.

4. **Access the application:**
   - Web Interface: http://localhost:3333 (or your configured port)
   - API Documentation: http://localhost:3333/api/docs
   - Health Check: http://localhost:3333/api/health

## Architecture Overview

### Services

- **postgres**: PostgreSQL 17 database (internal only, no host port exposure)
- **inventario-bootstrap**: Runs database bootstrap migrations (every startup - idempotent)
- **inventario-migrate**: Runs schema migrations (every startup)
- **inventario-init-data**: Sets up initial data (first run only)
- **inventario**: Main application server

### Initialization Flow

1. **PostgreSQL starts** and creates necessary users via init script
2. **Bootstrap service** runs `db bootstrap apply` to set up extensions and roles (idempotent - safe to run multiple times)
3. **Migration service** runs `db migrate up` to apply schema changes
4. **Init data service** runs `db migrate data` (only on first deployment)
5. **Main application** starts and serves requests

### Data Persistence

All data is stored in host-mounted directories for easy access:

- **./data/postgres**: PostgreSQL database files
- **./data/uploads**: Application file uploads
- **./data/init-state**: Tracks initialization state to prevent data re-setup

**Directory Structure:**
```
example/
├── data/
│   ├── postgres/          # PostgreSQL data files
│   ├── uploads/           # Application file uploads
│   └── init-state/        # Initialization tracking
├── docker-compose.yaml
└── ...
```

## Configuration

### Required Configuration (docker-compose.override.yaml)

```yaml
services:
  inventario:
    environment:
      JWT_SECRET: "your-secure-32-byte-jwt-secret"
    ports:
      - "8080:3333"  # Customize host port
  
  postgres:
    environment:
      POSTGRES_PASSWORD: "secure-postgres-password"
      POSTGRES_MIGRATOR_PASSWORD: "secure-migrator-password"
  
  inventario-init-data:
    environment:
      DEFAULT_TENANT_NAME: "Your Organization"
      ADMIN_EMAIL: "admin@yourcompany.com"
      ADMIN_PASSWORD: "secure-admin-password"
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `INVENTARIO_HOST_PORT` | `3333` | Host port for application access |
| `POSTGRES_DB` | `inventario` | PostgreSQL database name |
| `POSTGRES_USER` | `inventario` | PostgreSQL application user |
| `POSTGRES_PASSWORD` | `inventario_password` | PostgreSQL application password |
| `POSTGRES_MIGRATOR_USER` | `inventario_migrator` | PostgreSQL migration user |
| `POSTGRES_MIGRATOR_PASSWORD` | `inventario_migrator_password` | PostgreSQL migration password |
| `JWT_SECRET` | (required) | JWT signing secret (32+ characters) |
| `DEFAULT_TENANT_NAME` | `Default Organization` | Initial organization name |
| `ADMIN_EMAIL` | `admin@example.com` | Initial admin user email |
| `ADMIN_PASSWORD` | `admin123` | Initial admin user password |

## Security Considerations

1. **Change default passwords** in production
2. **Use strong JWT secret** (32+ random characters)
3. **PostgreSQL is internal only** (no host port exposure)
4. **File uploads are persistent** but local (consider MinIO migration)
5. **Use HTTPS reverse proxy** for production (nginx, Traefik, etc.)

## Maintenance

### Viewing Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f inventario
docker-compose logs -f postgres
```

### Database Backup

```bash
# Create backup using docker-compose
docker-compose exec postgres pg_dump -U inventario inventario > backup.sql

# Or access PostgreSQL data files directly (since they're host-mounted)
# Data files are located in: ./data/postgres/

# Restore backup
docker-compose exec -T postgres psql -U inventario inventario < backup.sql

# File-level backup (stop containers first)
docker-compose down
cp -r ./data/postgres ./backup-postgres-$(date +%Y%m%d)
docker-compose up -d
```

### Updates

```bash
# Stop services
docker-compose down

# Pull latest images and rebuild
docker-compose build --pull
docker-compose pull

# Start services (migrations run automatically)
docker-compose up -d
```

### Reset Data (Development Only)

```bash
# WARNING: This will delete all data
docker-compose down
rm -rf ./data/
docker-compose up -d
```

### Data Access

Since all data is stored in host-mounted directories, you can easily:

```bash
# View uploaded files
ls -la ./data/uploads/

# Access PostgreSQL data files
ls -la ./data/postgres/

# Check initialization state
cat ./data/init-state/data-initialized

# Backup specific directories
tar -czf backup-$(date +%Y%m%d).tar.gz ./data/
```

## Troubleshooting

### Common Issues

1. **Port already in use**: Change `INVENTARIO_HOST_PORT` in override file
2. **Database connection failed**: Check PostgreSQL logs and credentials
3. **Permission denied**: Ensure Docker has access to volume mount paths
4. **JWT token errors**: Verify JWT_SECRET is set and consistent

### Health Checks

```bash
# Check service status
docker-compose ps

# Test application health
curl http://localhost:3333/api/health

# Check database connectivity
docker-compose exec postgres pg_isready -U inventario
```

## Production Deployment Notes

- Consider using external PostgreSQL for high availability
- Implement proper backup strategy for persistent volumes
- Use reverse proxy with SSL/TLS termination
- Monitor resource usage and adjust limits accordingly
- Consider migrating to MinIO for object storage scalability
