# Inventario Production Docker Deployment

A production-ready Docker Compose stack for Inventario with PostgreSQL, Redis, and automatic database initialization — runnable from a prebuilt image (no local compilation) or built from source.

## Quick Start

The fastest way to get going is the bundled setup script. It asks whether you want to run the **prebuilt image** or **build from source**, generates secrets, writes your `.env` and `docker-compose.override.yaml`, validates the configuration, and (optionally) starts the stack. You do not need to read through all the settings below to get a working deployment.

```bash
./setup.sh
```

Or skip the prompts with one of the two fastest paths:

```bash
# Prebuilt image, no compilation (RECOMMENDED for trying it out).
# Pulls ghcr.io/denisvmedia/inventario:edge and starts the stack.
./setup.sh --mode image -y --start

# Build from local source (for development / customization).
./setup.sh --mode source -y --start
```

When the stack is up, the script prints the access URL, the Swagger location, and the credentials for **two** logins (see [Logins](#logins) below). Both passwords are generated, printed once, and saved to `.env`. The script also asks whether to **seed demo data** so you don't log into an empty app (or pass `--seed`). By default the app is published on `http://localhost:3333` (change with `--port`).

> The script is safe to re-run. It never clobbers an existing `.env` (re-run with `--force` to regenerate). It always regenerates the override for the mode you pick, backing up any previous one.

## Logins

The first run auto-provisions two independent accounts (created by the `inventario-init-data` service, or by `setup.sh` which generates their passwords into `.env`):

| Login | URL | Default email | Password (in `.env`) |
|-------|-----|---------------|----------------------|
| **Tenant admin** (the app) | `http://localhost:3333/` | `admin@example.com` | `ADMIN_PASSWORD` |
| **Back-office operator** (platform admin) | `http://localhost:3333/backoffice/login` | `backoffice@example.com` | `BACKOFFICE_PASSWORD` |

Back-office users live **outside** the tenant model (no tenant, no group scoping) and manage the platform. The example provisions the operator with **MFA disabled** (`BACKOFFICE_MFA_ENFORCED=false`) so you can sign in with just a password. For production, set `BACKOFFICE_MFA_ENFORCED=true` and enrol TOTP via `inventario backoffice mfa setup` before the first sign-in.

### Seeding demo data

`setup.sh` asks whether to seed the database (or pass `--seed`) so you don't land in an empty app. Seeding briefly enables the unauthenticated, RLS-bypassing seed endpoint (`POST /api/v1/seed`), loads bundled example data into the default tenant, then **disables the endpoint again** and recreates the app container — so it is off in the steady state. `--seed` implies `--start`. Never leave the seed endpoint enabled in production.

## Deployment modes

| Mode | What it does | Use it for |
|------|--------------|------------|
| **image** (default) | Runs the public prebuilt image `ghcr.io/denisvmedia/inventario`. No toolchain, no compile — just pulls the image. Multi-arch (`linux/amd64` + `linux/arm64`). | Trying Inventario, fast deploys, servers without a Go/Node toolchain. |
| **source** | Builds the image locally from the repository `Dockerfile` (`production` target — includes the frontend build and version injection). | Development, customization, running un-released changes. |

In **image** mode the override sets `image:` and removes the `build:` section from all four `inventario*` services (using `build: !reset null`), so a plain `docker compose up` can never trigger a local build. Available image tags: `edge` (latest `master`), `latest` (newest release), and pinned versions like `vX.Y.Z`. The prebuilt image is identical to the `Dockerfile` `production` target: it ships `curl`, `tzdata`, and `ca-certificates`, runs as non-root uid `1001`, and has the `inventario` binary on `PATH`, so the bootstrap/migrate/init-data scripts run unmodified.

## setup.sh options

| Flag | Default | Description |
|------|---------|-------------|
| `--mode <image\|source>` | interactive prompt (`image` preselected); `image` with `-y` | Choose the prebuilt image or a local source build. |
| `--image-tag <tag>` | `edge` | Image tag for image mode (`edge`, `latest`, `vX.Y.Z`, `sha-…`). Ignored in source mode. |
| `--port <port>` | `3333` | Host port to publish the app on. |
| `--admin-email <email>` | `admin@example.com` | Initial admin email. |
| `--admin-password <pw>` | generated | Initial admin password. If omitted, a strong random password is generated and printed once at the end. |
| `--tenant-name <name>` | `Default Organization` | Initial organization (tenant) name. |
| `--tenant-slug <slug>` | `default` | Initial tenant slug. |
| `--backoffice-email <email>` | `backoffice@example.com` | Back-office (platform operator) email. |
| `--backoffice-password <pw>` | generated | Back-office password. If omitted, a strong random one is generated and printed once. |
| `--ephemeral-secrets` | off | Do **not** persist `JWT_SECRET` / `FILE_SIGNING_KEY`. They are left empty, so the app auto-generates throwaway keys per restart (fine for local testing, not for production). |
| `--start` / `--no-start` | interactive prompt; `--no-start` with `-y` | Bring the stack up after preparing the files. |
| `--seed` / `--no-seed` | interactive prompt when starting; no with `-y` | After start, seed demo data so the app isn't empty. Enables the seed endpoint only for the seeding step, then disables it. `--seed` implies `--start`. |
| `-y`, `--yes` | off | Non-interactive; accept all defaults (mode `image`, no start). |
| `--force` | off | Overwrite an existing `.env`/override. The old file is backed up to `<file>.bak.<n>` first. |
| `-h`, `--help` | — | Show usage. |

By default the script generates 64-hex secrets (`openssl rand -hex 32`, with a `/dev/urandom` fallback) for `JWT_SECRET` and `FILE_SIGNING_KEY` and writes them to `.env`. With `--ephemeral-secrets` those keys are left empty.

## Manual setup (advanced)

If you want full control instead of the script, configure the two files by hand.

1. **Create your environment file.** `.env` drives the variable substitution in the base compose file (`${JWT_SECRET}`, `${POSTGRES_*}`, etc.).

   ```bash
   cp .env.example .env
   chmod 600 .env          # it will hold cleartext secrets
   ```

2. **Generate both secrets** and put them in `.env`. The placeholder values in `.env.example` are rejected at startup (the app refuses to boot on a public example secret), so you must replace them. Leaving a secret unset/commented makes the app auto-generate an ephemeral key per restart — fine for local testing, not for production.

   ```bash
   openssl rand -hex 32   # -> JWT_SECRET
   openssl rand -hex 32   # -> FILE_SIGNING_KEY
   ```

   Also set a strong **`ADMIN_PASSWORD`** in `.env` (≥8 chars with an upper-case letter, a lower-case letter, and a digit). `.env.example` ships it empty and the base compose **fails closed** — `docker compose config`/`up` errors until you set it. Change the default `POSTGRES_PASSWORD` / `POSTGRES_MIGRATOR_PASSWORD` too before production.

3. **Create the override file.** This is where you make structural changes (host port, image vs. source, resource limits). Two things in the override are **mandatory** for a working deployment, because the base `docker-compose.yaml` provides neither:

   - **A host port mapping.** The base compose publishes **no** host port for the main `inventario` service, so the app is unreachable from the host with the base file alone. The override must add:

     ```yaml
     services:
       inventario:
         ports:
           - "${INVENTARIO_HOST_PORT:-3333}:3333"
     ```

   - **The file signing key.** The base compose does **not** wire `INVENTARIO_RUN_FILE_SIGNING_KEY` into the main service (only `INVENTARIO_RUN_JWT_SECRET`). Without it, signed file URLs are unavailable/insecure. The override must add:

     ```yaml
     services:
       inventario:
         environment:
           INVENTARIO_RUN_FILE_SIGNING_KEY: ${FILE_SIGNING_KEY:-}
           INVENTARIO_RUN_FILE_URL_EXPIRATION: ${FILE_URL_EXPIRATION:-15m}
     ```

   > The bundled `docker-compose.override.yaml.example` is an **illustrative reference** (resource limits, reverse-proxy labels, a fully custom production database) — **not** a drop-in for the bundled `.env`. It renames the database/user and ships placeholder secrets the app rejects at boot, so copying it verbatim yields a non-booting or mis-wired stack. Prefer `./setup.sh`, or hand-write the minimal blocks shown above and consult the `.example` only for the optional `deploy.resources` / labels snippets.

4. **For image mode (no local build)**, point all four `inventario*` services at the prebuilt image and drop their `build:` section so `up` never compiles. Repeat this block for `inventario`, `inventario-bootstrap`, `inventario-migrate`, and `inventario-init-data` (swap `:edge` for any published tag — `latest`, `vX.Y.Z`, or a `sha-…` pin; this mirrors what `./setup.sh --mode image --image-tag <tag>` writes):

   ```yaml
   services:
     inventario:
       build: !reset null
       image: ghcr.io/denisvmedia/inventario:edge
       ports:
         - "${INVENTARIO_HOST_PORT:-3333}:3333"
       environment:
         INVENTARIO_RUN_FILE_SIGNING_KEY: ${FILE_SIGNING_KEY:-}
         INVENTARIO_RUN_FILE_URL_EXPIRATION: ${FILE_URL_EXPIRATION:-15m}
     inventario-bootstrap:
       build: !reset null
       image: ghcr.io/denisvmedia/inventario:edge
     inventario-migrate:
       build: !reset null
       image: ghcr.io/denisvmedia/inventario:edge
     inventario-init-data:
       build: !reset null
       image: ghcr.io/denisvmedia/inventario:edge
   ```

   > `build: !reset null` requires Docker Compose v2.24.4 or newer. It removes the build context entirely from the resolved config (verify with `docker compose config --images`), so a bare `docker compose up` can never rebuild. `setup.sh` checks this and refuses image mode on an older/legacy Compose with a clear message.

5. **Validate and start.**

   ```bash
   docker compose config -q              # exit 0 == valid

   # Image mode (no build): --no-build is optional — `build: !reset null` already
   # removes the build block, so a plain `up -d` can never compile.
   docker compose pull
   docker compose up -d                  # equivalent to: docker compose up -d --no-build

   # Source mode (build locally):
   docker compose up -d --build
   ```

## Architecture Overview

### Services

- **postgres**: PostgreSQL 18 database (internal only, no host port exposure).
- **redis**: Redis 8 cache backing the token blacklist (internal only, no host port exposure).
- **inventario-bootstrap**: Runs database bootstrap (every startup — idempotent).
- **inventario-migrate**: Runs schema migrations (every startup).
- **inventario-init-data**: Sets up initial data (first run only).
- **inventario**: Main application server.

### Initialization Flow

1. **PostgreSQL starts** and creates the migration user via the init script.
2. **Bootstrap service** runs `db bootstrap apply` to set up extensions and roles (idempotent — safe to run repeatedly).
3. **Migration service** runs `db migrate up` to apply schema changes.
4. **Init data service** (first deployment only, gated by `./data/init-state/data-initialized`) seeds the default tenant + tenant admin (`db migrate data`) and provisions the back-office operator (`backoffice bootstrap --ensure`).
5. **Main application** starts and serves requests.

### Data Persistence

All data is stored in host-mounted directories for easy access:

- **./data/postgres**: PostgreSQL database files (in the `pgdata/` subdirectory — Postgres 18 requires `PGDATA` to be a subdir of the bind mount).
- **./data/redis**: Redis persistence (token-blacklist data).
- **./data/uploads**: Application file uploads.
- **./data/init-state**: Tracks initialization state to prevent data re-setup.

**Directory Structure:**

```
example/
├── data/
│   ├── postgres/          # PostgreSQL data files (under pgdata/ — PGDATA subdir)
│   ├── redis/             # Redis token-blacklist persistence
│   ├── uploads/           # Application file uploads
│   └── init-state/        # Initialization tracking
├── docker-compose.yaml
├── docker-compose.override.yaml   # generated by setup.sh (the .example is a reference, not a drop-in)
├── .env                           # generated by setup.sh (or copied from .env.example)
└── ...
```

## Configuration

### Where settings come from

- **`.env`** supplies the shell variables the base `docker-compose.yaml` substitutes (e.g. `${JWT_SECRET}` → `INVENTARIO_RUN_JWT_SECRET`, `${POSTGRES_PASSWORD}`, `${ADMIN_EMAIL}` → `INVENTARIO_MIGRATE_DATA_ADMIN_EMAIL`). This is the primary configuration entrypoint.
- **`docker-compose.override.yaml`** is for structural changes: host port mapping, image-vs-source, resource limits, and wiring the file signing key.

> Note: bare keys placed directly under a service's `environment:` (for example `JWT_SECRET:` under `inventario`) are inert — the container reads `INVENTARIO_RUN_JWT_SECRET`. Set values via `.env` (recommended) or, in the override, use the real container variable names (`INVENTARIO_RUN_*` on `inventario`, `POSTGRES_*` on `postgres`, `INVENTARIO_MIGRATE_DATA_*` on `inventario-init-data`).

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `INVENTARIO_HOST_PORT` | `3333` | Host port the override publishes the app on. Not consumed by the base compose — only by the `ports:` mapping the override adds. |
| `JWT_SECRET` | (required) | JWT signing secret; generate with `openssl rand -hex 32`. The example placeholder is rejected at startup; leaving it unset auto-generates an ephemeral key per restart. |
| `FILE_SIGNING_KEY` | (required) | Signing key for signed file URLs; generate with `openssl rand -hex 32`. Same placeholder-rejection / ephemeral behavior as `JWT_SECRET`. Wired into the app only via the override. |
| `FILE_URL_EXPIRATION` | `15m` | Lifetime of signed file URLs. |
| `MAX_CONCURRENT_EXPORTS` | `3` | Maximum concurrent export jobs. |
| `MAX_CONCURRENT_IMPORTS` | `1` | Maximum concurrent import jobs. |
| `THUMBNAIL_MAX_CONCURRENT_PER_USER` | `5` | Max concurrent thumbnail generations per user. |
| `THUMBNAIL_RATE_LIMIT_PER_MINUTE` | `50` | Thumbnail generation requests allowed per minute. |
| `THUMBNAIL_SLOT_DURATION` | `30m` | Thumbnail generation slot duration. |
| `TOKEN_BLACKLIST_REDIS_URL` | `redis://redis:6379` | Redis URL backing the JWT token blacklist. |
| `TZ` | `UTC` | Container timezone. |
| `POSTGRES_DB` | `inventario` | PostgreSQL database name. |
| `POSTGRES_USER` | `inventario` | PostgreSQL application user. |
| `POSTGRES_PASSWORD` | `inventario_password` | PostgreSQL application password. |
| `POSTGRES_MIGRATOR_USER` | `inventario_migrator` | PostgreSQL migration user. |
| `POSTGRES_MIGRATOR_PASSWORD` | `inventario_migrator_password` | PostgreSQL migration password. |
| `DEFAULT_TENANT_NAME` | `Default Organization` | Initial organization name (first run only). |
| `DEFAULT_TENANT_SLUG` | `default` | Initial tenant slug (first run only). |
| `ADMIN_EMAIL` | `admin@example.com` | Initial admin user email (first run only). |
| `ADMIN_PASSWORD` | generated / **required** | Initial admin user password (first run only). `setup.sh` generates a strong random one unless you pass `--admin-password`. In the manual path `.env.example` ships it empty, so set a value before first run; the base compose fails closed (errors) while it is unset/empty. Must satisfy: ≥8 chars with upper, lower, and a digit. |
| `ADMIN_NAME` | `System Administrator` | Initial admin user display name (first run only). |
| `BACKOFFICE_EMAIL` | `backoffice@example.com` | Back-office operator email (first run only). |
| `BACKOFFICE_NAME` | `Platform Admin` | Back-office operator display name (first run only). |
| `BACKOFFICE_PASSWORD` | generated / **required** | Back-office operator password (first run only). `setup.sh` generates one; the manual path must set it (fails closed if empty). Same complexity policy as `ADMIN_PASSWORD`. |
| `BACKOFFICE_MFA_ENFORCED` | `false` | Demo default: password-only back-office login. Set `true` for production, then enrol TOTP via `inventario backoffice mfa setup`. |

> Build-time only (source mode): `VERSION`, `COMMIT`, and `BUILD_DATE` are optional image-label build args (defaults `dev` / `unknown` / `unknown`, auto-detected if unset) — see the commented block in `.env.example`. They are ignored in image mode.

> The `DEFAULT_TENANT_*` and `ADMIN_*` values seed initial data **once** (gated by `./data/init-state/data-initialized`). Changing them after the first boot has no effect — change the admin password in-app, or wipe `./data/init-state` to re-seed.

## Security Considerations

1. **Change default passwords** (database users and the admin account) before production.
2. **Use a strong `JWT_SECRET`** (`openssl rand -hex 32`). The example placeholder is rejected at boot; an unset value auto-generates an ephemeral throwaway key per restart.
3. **Set a strong `FILE_SIGNING_KEY`** (`openssl rand -hex 32`) and wire it via the override (`INVENTARIO_RUN_FILE_SIGNING_KEY`) — the base compose does not. Without it, signed file URLs are unavailable/insecure. `FILE_URL_EXPIRATION` (default `15m`) controls their lifetime.
4. **PostgreSQL and Redis are internal only** (no host port exposure).
5. **File uploads are persistent** but stored on the local filesystem (consider MinIO migration for scale).
6. **Use an HTTPS reverse proxy** for production (nginx, Traefik, etc.).
7. **Keep `.env` private.** `setup.sh` writes it mode `0600` (and its `.bak.*` backups too); it holds cleartext secrets (`JWT_SECRET`, `FILE_SIGNING_KEY`, DB, admin and back-office passwords). On a shared host, do not loosen its permissions. If you create `.env` by hand, `chmod 600 .env`.
8. **Enable back-office MFA** for production (`BACKOFFICE_MFA_ENFORCED=true` + `inventario backoffice mfa setup`). The example ships it disabled for convenience.
9. **The seed endpoint** (`POST /api/v1/seed`) is unauthenticated and RLS-bypassing. `setup.sh --seed` enables it only transiently and disables it again; never enable it in production.

## Maintenance

### Viewing Logs

```bash
# All services
docker compose logs -f

# Specific service
docker compose logs -f inventario
docker compose logs -f postgres
```

### Database Backup

```bash
# Create a backup
docker compose exec postgres pg_dump -U inventario inventario > backup.sql

# Or access the PostgreSQL data files directly (they are host-mounted under ./data/postgres/)

# Restore a backup
docker compose exec -T postgres psql -U inventario inventario < backup.sql

# File-level backup (stop containers first)
docker compose down
cp -r ./data/postgres ./backup-postgres-$(date +%Y%m%d)
docker compose up -d
```

### Updates

```bash
# Stop services
docker compose down
```

**Image mode** — just pull the newer image, no build:

```bash
docker compose pull
docker compose up -d
```

**Source mode** — rebuild from the latest source:

```bash
docker compose build --pull
docker compose up -d
```

Migrations run automatically on startup in either case.

### Reset Data (Development Only)

```bash
# WARNING: this deletes all data
docker compose down
rm -rf ./data/
docker compose up -d
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

# Back up the whole data directory (crash-consistent: stop services first, since
# hot-copying the live PostgreSQL/Redis data dirs can produce a non-restorable snapshot)
docker compose down
tar -czf backup-$(date +%Y%m%d).tar.gz ./data/
docker compose up -d
```

## Troubleshooting

### Common Issues

1. **Port already in use**: change `--port` (or `INVENTARIO_HOST_PORT` in `.env`) and re-run.
2. **Database connection failed**: check PostgreSQL logs and credentials.
3. **Permission denied**: ensure Docker has access to the volume mount paths.
4. **JWT token errors**: verify `JWT_SECRET` is set and consistent (an ephemeral/unset secret invalidates tokens on every restart).
5. **Signed file URLs fail**: ensure the override wires `INVENTARIO_RUN_FILE_SIGNING_KEY` from `FILE_SIGNING_KEY`.

### Health Checks

```bash
# Check service status
docker compose ps

# Test application readiness from the host (replace 3333 with your --port / INVENTARIO_HOST_PORT)
curl -f http://localhost:3333/readyz
# Note: the compose healthcheck runs the same probe INSIDE the container against
# :3333 and is independent of the published host port.

# Check database connectivity
docker compose exec postgres pg_isready -U inventario
```

## Production Deployment Notes

- Consider using an external PostgreSQL for high availability.
- Implement a proper backup strategy for the persistent volumes.
- Use a reverse proxy with SSL/TLS termination.
- Monitor resource usage and adjust limits accordingly (see the override example for `deploy.resources`).
- Consider migrating to MinIO for object storage scalability.
