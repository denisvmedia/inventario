# Docker Setup for Inventario

This document describes how to run Inventario using Docker and Docker Compose with PostgreSQL.

## Quick Start

1. **Copy the environment file**:
   ```bash
   cp .env.example .env
   ```

2. **Start the services**:
   ```bash
   docker-compose up -d
   ```

3. **Access the application**:
   Open your browser and navigate to http://localhost:3333

4. **Seed the database** (optional):
   ```bash
   curl -X POST http://localhost:3333/api/v1/seed
   ```

## Configuration

### Environment Variables

The application can be configured using environment variables in the `.env` file:

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGRES_DB` | `inventario` | PostgreSQL database name |
| `POSTGRES_USER` | `inventario` | PostgreSQL username |
| `POSTGRES_PASSWORD` | `inventario_password` | PostgreSQL password |
| `POSTGRES_PORT` | `5432` | PostgreSQL port (host mapping) |
| `INVENTARIO_PORT` | `3333` | Inventario application port (host mapping) |
| `INVENTARIO_ADDR` | `:3333` | Inventario bind address |
| `INVENTARIO_UPLOAD_LOCATION` | `file:///app/uploads?create_dir=1` | Upload storage location |
| `TZ` | `UTC` | Timezone |

### Data Persistence

Data is stored in the `.docker` directory:

- **PostgreSQL data**: `.docker/postgresql/`
- **Uploaded files**: `.docker/inventario/uploads/`
- **Application data**: `.docker/inventario/data/`

These directories are automatically created when you start the services.

## Docker Commands

### Build and Start Services
```bash
# Build and start all services
docker-compose up -d

# Build and start with logs
docker-compose up --build

# Start only specific service
docker-compose up -d postgres
```

### View Logs
```bash
# View all logs
docker-compose logs

# View logs for specific service
docker-compose logs inventario
docker-compose logs postgres

# Follow logs
docker-compose logs -f inventario
```

### Stop and Remove Services
```bash
# Stop services
docker-compose stop

# Stop and remove containers
docker-compose down

# Stop, remove containers and volumes
docker-compose down -v
```

### Database Operations
```bash
# Connect to PostgreSQL
docker-compose exec postgres psql -U inventario -d inventario

# Create database backup
docker-compose exec postgres pg_dump -U inventario inventario > backup.sql

# Restore database backup
docker-compose exec -T postgres psql -U inventario -d inventario < backup.sql
```

### Application Operations
```bash
# Execute commands in the application container
docker-compose exec inventario ./inventario --help

# Seed the database
docker-compose exec inventario curl -X POST http://localhost:3333/api/v1/seed

# Run database migrations
docker-compose exec inventario ./inventario migrate
```

## Development

### Building the Image Locally
```bash
# Build the Docker image
docker build -t inventario:latest .

# Build with specific tag
docker build -t inventario:dev .
```

### Using Custom Configuration
Create a `docker-compose.override.yml` file for development-specific settings:

```yaml
services:
  inventario:
    environment:
      INVENTARIO_LOG_LEVEL: debug
    volumes:
      - ./custom-config:/app/config
```

## Troubleshooting

### Common Issues

1. **Port already in use**:
   Change the port mapping in `.env`:
   ```
   INVENTARIO_PORT=8080
   POSTGRES_PORT=5433
   ```

2. **Permission issues with volumes**:
   ```bash
   sudo chown -R 1001:1001 .docker/
   ```

3. **Database connection issues**:
   Check if PostgreSQL is healthy:
   ```bash
   docker-compose ps
   docker-compose logs postgres
   ```

4. **Application won't start**:
   Check application logs:
   ```bash
   docker-compose logs inventario
   ```

### Health Checks

Both services include health checks:

- **PostgreSQL**: Checks if the database accepts connections
- **Inventario**: Checks if the API responds to requests

View health status:
```bash
docker-compose ps
```

## Production Deployment

For production deployment, consider:

1. **Use external PostgreSQL**: Set `INVENTARIO_DB_DSN` to point to your production database
2. **Use cloud storage**: Set `INVENTARIO_UPLOAD_LOCATION` to S3, GCS, or Azure Blob
3. **Enable HTTPS**: Use a reverse proxy like nginx or Traefik
4. **Set strong passwords**: Generate secure passwords for database access
5. **Regular backups**: Implement automated database and file backups
6. **Resource limits**: Add resource constraints to docker-compose.yaml

Example production override:
```yaml
services:
  inventario:
    environment:
      INVENTARIO_DB_DSN: "postgres://user:pass@prod-db:5432/inventario"
      INVENTARIO_UPLOAD_LOCATION: "s3://my-bucket/uploads?region=us-east-1"
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: '0.5'
```
