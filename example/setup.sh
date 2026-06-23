#!/usr/bin/env bash
#
# setup.sh — one-command deployment preparer for the Inventario example stack.
#
# Generates secrets, writes .env and docker-compose.override.yaml for the
# chosen run mode (prebuilt ghcr image, or build from local source), validates
# the resulting Compose configuration, and optionally brings the stack up.
#
# Run it from anywhere; it resolves and operates in its own directory.
#
#   ./setup.sh --help
#   ./setup.sh                       # interactive
#   ./setup.sh --mode image -y       # non-interactive, prebuilt image, no start
#   ./setup.sh --mode source --start # build from source and start
#
set -euo pipefail

# ---------------------------------------------------------------------------
# Resolve script directory and operate there.
# ---------------------------------------------------------------------------
SCRIPT_SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SCRIPT_SOURCE" ]; do
  dir="$(cd -P "$(dirname "$SCRIPT_SOURCE")" >/dev/null 2>&1 && pwd)"
  SCRIPT_SOURCE="$(readlink "$SCRIPT_SOURCE")"
  [[ "$SCRIPT_SOURCE" != /* ]] && SCRIPT_SOURCE="$dir/$SCRIPT_SOURCE"
done
SCRIPT_DIR="$(cd -P "$(dirname "$SCRIPT_SOURCE")" >/dev/null 2>&1 && pwd)"
cd "$SCRIPT_DIR"

ENV_FILE="$SCRIPT_DIR/.env"
OVERRIDE_FILE="$SCRIPT_DIR/docker-compose.override.yaml"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yaml"

IMAGE_REPO="ghcr.io/denisvmedia/inventario"

# Track temp files for guaranteed cleanup on any exit (set -u safe on bash 3.2).
TMP_FILES=()
cleanup() {
  if [ "${#TMP_FILES[@]}" -gt 0 ]; then
    rm -f "${TMP_FILES[@]+"${TMP_FILES[@]}"}"
  fi
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Colored logging (to stderr so stdout stays clean for any captured values).
# ---------------------------------------------------------------------------
if [ -t 2 ]; then
  C_RESET=$'\033[0m'; C_BOLD=$'\033[1m'
  C_BLUE=$'\033[34m'; C_GREEN=$'\033[32m'; C_YELLOW=$'\033[33m'; C_RED=$'\033[31m'; C_CYAN=$'\033[36m'
else
  C_RESET=''; C_BOLD=''; C_BLUE=''; C_GREEN=''; C_YELLOW=''; C_RED=''; C_CYAN=''
fi

log()      { printf '%s\n' "${C_BLUE}==>${C_RESET} $*" >&2; }
section()  { printf '\n%s\n' "${C_BOLD}${C_CYAN}== $* ==${C_RESET}" >&2; }
info()     { printf '%s\n' "    $*" >&2; }
success()  { printf '%s\n' "${C_GREEN}[ok]${C_RESET} $*" >&2; }
warn()     { printf '%s\n' "${C_YELLOW}[warn]${C_RESET} $*" >&2; }
err()      { printf '%s\n' "${C_RED}[error]${C_RESET} $*" >&2; }
die()      { err "$*"; exit 1; }

# ---------------------------------------------------------------------------
# Defaults (overridable via flags).
# ---------------------------------------------------------------------------
MODE=""                       # image | source ; empty => prompt/derive
IMAGE_TAG="edge"
PORT="3333"
ADMIN_EMAIL="admin@example.com"
ADMIN_PASSWORD=""             # empty => generate
ADMIN_PASSWORD_GENERATED=0
TENANT_NAME="Default Organization"
TENANT_SLUG="default"
EPHEMERAL_SECRETS=0
START=""                      # "" => prompt ; 1 => start ; 0 => no-start
SEED=""                       # "" => prompt ; 1 => seed ; 0 => no-seed
SEEDED=0                      # 1 once demo data has been seeded this run
ASSUME_YES=0
FORCE=0
ENV_FLAGS_SUPPLIED=0          # 1 if any .env-only value flag was passed

# Back-office (platform operator) — auto-provisioned by init-data on first run.
BACKOFFICE_EMAIL="backoffice@example.com"
BACKOFFICE_NAME="Platform Admin"
BACKOFFICE_PASSWORD=""        # empty => generate
BACKOFFICE_PASSWORD_GENERATED=0
BACKOFFICE_MFA_ENFORCED="false"   # demo default: password-only login

usage() {
  cat >&2 <<EOF
${C_BOLD}Inventario deployment setup${C_RESET}

Prepares this directory for a Docker Compose deployment: generates secrets,
writes .env and docker-compose.override.yaml for the chosen run mode, validates
the configuration, and optionally starts the stack.

${C_BOLD}Usage:${C_RESET}
  ./setup.sh [options]

${C_BOLD}Options:${C_RESET}
  --mode <image|source>     Run mode. 'image' runs the prebuilt ghcr image with
                            no local compilation; 'source' builds from the local
                            repository. Default: prompt ('image' preselected);
                            with -y defaults to 'image'.
  --image-tag <tag>         Image tag for image mode (edge|latest|vX.Y.Z|sha-...).
                            Default: edge. Only meaningful in image mode.
  --port <port>             Host port to publish. Default: 3333.
  --admin-email <email>     Initial admin email. Default: admin@example.com.
  --admin-password <pw>     Initial admin password. If omitted, a strong random
                            password is generated and printed once at the end.
  --tenant-name <name>      Default organization name. Default: "Default Organization".
  --tenant-slug <slug>      Default tenant slug. Default: default.
  --backoffice-email <e>    Back-office (platform operator) email. Default: backoffice@example.com.
  --backoffice-password <p> Back-office password. If omitted, a strong random one
                            is generated and printed once at the end.
  --ephemeral-secrets       Do NOT persist JWT/file-signing secrets; leave them
                            empty so the app auto-generates throwaway keys per
                            restart (fine for local testing, not production).
  --start                   Bring the stack up after preparing.
  --no-start                Do not start the stack (just prepare files).
                            Default: prompt; with -y defaults to --no-start.
  --seed                    After start, seed the database with demo data so the
                            app is not empty. Enables the seed endpoint only for
                            the seeding step, then disables it again. Implies --start.
  --no-seed                 Do not seed. Default: prompt when starting; no with -y.
  -y, --yes                 Non-interactive; accept all defaults.
  --force                   Overwrite existing .env/override, backing up the old
                            file to <file>.bak.<n> first.
  -h, --help                Show this help and exit.

${C_BOLD}Examples:${C_RESET}
  ./setup.sh
  ./setup.sh --mode image -y
  ./setup.sh --mode image --image-tag latest --port 8080 --start
  ./setup.sh --mode source --admin-email me@example.com --start
EOF
}

# ---------------------------------------------------------------------------
# Argument parsing — supports both "--flag value" and "--flag=value".
# ---------------------------------------------------------------------------
parse_args() {
  while [ "$#" -gt 0 ]; do
    local arg="$1"
    local key="$arg" val="" has_val=0
    if [[ "$arg" == --*=* ]]; then
      key="${arg%%=*}"
      val="${arg#*=}"
      has_val=1
    fi
    case "$key" in
      --mode)
        if [ "$has_val" -eq 0 ]; then [ "$#" -ge 2 ] || die "Option '$key' requires a value."; val="$2"; shift; fi
        [ -n "$val" ] || die "--mode must not be empty (expected 'image' or 'source')."
        MODE="$val" ;;
      --image-tag)
        if [ "$has_val" -eq 0 ]; then [ "$#" -ge 2 ] || die "Option '$key' requires a value."; val="$2"; shift; fi
        IMAGE_TAG="$val" ;;
      --port)
        if [ "$has_val" -eq 0 ]; then [ "$#" -ge 2 ] || die "Option '$key' requires a value."; val="$2"; shift; fi
        PORT="$val"; ENV_FLAGS_SUPPLIED=1 ;;
      --admin-email)
        if [ "$has_val" -eq 0 ]; then [ "$#" -ge 2 ] || die "Option '$key' requires a value."; val="$2"; shift; fi
        ADMIN_EMAIL="$val"; ENV_FLAGS_SUPPLIED=1 ;;
      --admin-password)
        if [ "$has_val" -eq 0 ]; then [ "$#" -ge 2 ] || die "Option '$key' requires a value."; val="$2"; shift; fi
        ADMIN_PASSWORD="$val"; ENV_FLAGS_SUPPLIED=1 ;;
      --tenant-name)
        if [ "$has_val" -eq 0 ]; then [ "$#" -ge 2 ] || die "Option '$key' requires a value."; val="$2"; shift; fi
        TENANT_NAME="$val"; ENV_FLAGS_SUPPLIED=1 ;;
      --tenant-slug)
        if [ "$has_val" -eq 0 ]; then [ "$#" -ge 2 ] || die "Option '$key' requires a value."; val="$2"; shift; fi
        TENANT_SLUG="$val"; ENV_FLAGS_SUPPLIED=1 ;;
      --backoffice-email)
        if [ "$has_val" -eq 0 ]; then [ "$#" -ge 2 ] || die "Option '$key' requires a value."; val="$2"; shift; fi
        BACKOFFICE_EMAIL="$val"; ENV_FLAGS_SUPPLIED=1 ;;
      --backoffice-password)
        if [ "$has_val" -eq 0 ]; then [ "$#" -ge 2 ] || die "Option '$key' requires a value."; val="$2"; shift; fi
        BACKOFFICE_PASSWORD="$val"; ENV_FLAGS_SUPPLIED=1 ;;
      --ephemeral-secrets)
        EPHEMERAL_SECRETS=1 ;;
      --start)
        START=1 ;;
      --no-start)
        START=0 ;;
      --seed)
        SEED=1 ;;
      --no-seed)
        SEED=0 ;;
      -y|--yes)
        ASSUME_YES=1 ;;
      --force)
        FORCE=1 ;;
      -h|--help)
        usage; exit 0 ;;
      *)
        die "Unknown option: $arg (try --help)" ;;
    esac
    shift
  done
}

# ---------------------------------------------------------------------------
# Interactive prompts (skipped entirely with -y).
# ---------------------------------------------------------------------------
prompt_default() {
  # $1 = prompt text, $2 = default ; echoes the answer
  local text="$1" def="$2" ans=""
  if [ "$ASSUME_YES" -eq 1 ] || [ ! -t 0 ]; then
    printf '%s' "$def"
    return 0
  fi
  read -r -p "$text [$def]: " ans </dev/tty || ans=""
  printf '%s' "${ans:-$def}"
}

prompt_yes_no() {
  # $1 = prompt text, $2 = default (y|n) ; returns 0 for yes, 1 for no
  local text="$1" def="$2" ans=""
  if [ "$ASSUME_YES" -eq 1 ] || [ ! -t 0 ]; then
    [ "$def" = "y" ] && return 0 || return 1
  fi
  local hint="y/N"; [ "$def" = "y" ] && hint="Y/n"
  read -r -p "$text [$hint]: " ans </dev/tty || ans=""
  ans="${ans:-$def}"
  case "$ans" in
    y|Y|yes|YES|Yes) return 0 ;;
    *) return 1 ;;
  esac
}

# ---------------------------------------------------------------------------
# Preflight: docker, docker compose v2, reachable daemon.
# ---------------------------------------------------------------------------
COMPOSE=()   # filled with the working compose invocation

preflight() {
  section "Preflight"

  command -v docker >/dev/null 2>&1 || die "'docker' is not installed or not on PATH."
  success "docker found: $(docker --version 2>/dev/null || echo unknown)"

  # Prefer Compose v2 plugin ('docker compose'); fall back to legacy 'docker-compose'.
  if docker compose version >/dev/null 2>&1; then
    COMPOSE=(docker compose)
  elif command -v docker-compose >/dev/null 2>&1 && docker-compose version >/dev/null 2>&1; then
    COMPOSE=(docker-compose)
    warn "Using legacy 'docker-compose' (v1). Consider upgrading to the Compose v2 plugin."
  else
    die "Docker Compose v2 is required ('docker compose'). Neither it nor legacy 'docker-compose' was found."
  fi
  success "compose found: $("${COMPOSE[@]}" version 2>/dev/null | head -n1 || echo unknown)"

  # Daemon must be reachable.
  if ! docker info >/dev/null 2>&1; then
    die "The Docker daemon is not reachable. Start Docker and try again."
  fi
  success "Docker daemon is reachable."
}

# ---------------------------------------------------------------------------
# Secret generation: 64-hex chars. Prefer openssl; fall back to /dev/urandom.
# ---------------------------------------------------------------------------
gen_secret() {
  local s=""
  if command -v openssl >/dev/null 2>&1; then
    s="$(openssl rand -hex 32 2>/dev/null || true)"
  fi
  if [ "${#s}" -ne 64 ]; then
    s="$(LC_ALL=C tr -dc 'a-f0-9' </dev/urandom 2>/dev/null | head -c 64 || true)"
  fi
  [ "${#s}" -eq 64 ] || die "Failed to generate a 64-hex secret (no openssl and /dev/urandom unavailable)."
  printf '%s' "$s"
}

# Returns 0 only if the password has an upper, a lower, and a digit (the
# backend's ValidatePassword policy). Used in an `if` so `set -e` is exempt.
password_meets_policy() {
  case "$1" in *[A-Z]*) ;; *) return 1 ;; esac
  case "$1" in *[a-z]*) ;; *) return 1 ;; esac
  case "$1" in *[0-9]*) ;; *) return 1 ;; esac
  return 0
}

gen_password() {
  # Strong, shell/URL-safe alphanumeric password (no special chars that need
  # quoting). A uniform [A-Za-z0-9] draw can lack a required character class
  # (~1.5% of the time), which the backend rejects and which would abort
  # init-data — so retry until it satisfies the upper/lower/digit policy.
  local p="" tries=0
  while :; do
    tries=$((tries + 1))
    p=""
    if command -v openssl >/dev/null 2>&1; then
      p="$(openssl rand -base64 24 2>/dev/null | LC_ALL=C tr -dc 'A-Za-z0-9' | head -c 24 || true)"
    fi
    if [ "${#p}" -lt 20 ]; then
      p="$(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom 2>/dev/null | head -c 24 || true)"
    fi
    [ "${#p}" -ge 20 ] || die "Failed to generate an admin password."
    if password_meets_policy "$p" || [ "$tries" -ge 100 ]; then
      break
    fi
  done
  printf '%s' "$p"
}

# ---------------------------------------------------------------------------
# Escape '$' -> '$$' for values written into .env. Docker Compose runs its own
# ${VAR} interpolation over .env values, so an unescaped '$' is treated as a
# variable reference and would silently corrupt the value (e.g. an admin
# password). Compose un-escapes '$$' back to a literal '$' on substitution.
# ---------------------------------------------------------------------------
esc_env() {
  local s="$1"
  printf '%s' "${s//\$/\$\$}"
}

unesc_env() {
  local s="$1"
  printf '%s' "${s//\$\$/\$}"
}

# Reject newline/CR in operator-supplied values: a newline would write an extra
# KEY=value line into .env and (Compose .env is last-wins) could override a
# generated secret. esc_env neutralizes '$' but not newlines.
reject_ctrl() {
  case "$1" in
    *$'\n'*|*$'\r'*) die "$2 must not contain newline or carriage-return characters." ;;
  esac
}

# Read the last KEY=... value from the existing .env (empty if absent).
read_env_value() {
  local key="$1" line=""
  [ -f "$ENV_FILE" ] || return 0
  line="$(grep -E "^${key}=" "$ENV_FILE" 2>/dev/null | tail -n1 || true)"
  line="${line#*=}"
  line="${line%$'\r'}"                        # strip trailing CR (CRLF .env)
  line="${line#"${line%%[![:space:]]*}"}"     # strip leading whitespace
  line="${line%"${line##*[![:space:]]}"}"     # strip trailing whitespace
  printf '%s' "$line"
}

# ---------------------------------------------------------------------------
# Back up an existing file to <file>.bak.<n> (lowest free n).
# ---------------------------------------------------------------------------
backup_file() {
  local f="$1"
  [ -e "$f" ] || return 0
  local n=1
  while [ -e "$f.bak.$n" ]; do
    n=$((n + 1))
  done
  cp -p "$f" "$f.bak.$n"
  chmod 600 "$f.bak.$n" 2>/dev/null || true
  warn "Backed up existing $(basename "$f") -> $(basename "$f").bak.$n"
}

# ---------------------------------------------------------------------------
# .env generation. Reuse an existing .env unless --force.
# ---------------------------------------------------------------------------
ENV_REUSED=0

write_env() {
  section "Environment file (.env)"

  if [ -f "$ENV_FILE" ] && [ "$FORCE" -eq 0 ]; then
    ENV_REUSED=1
    success "Existing .env found — reusing it untouched (use --force to regenerate)."
    info "Secrets, ports and credentials already in .env are preserved."
    return 0
  fi

  if [ -f "$ENV_FILE" ]; then
    if [ -e "$SCRIPT_DIR/data/init-state/data-initialized" ]; then
      warn "Stack already initialized: a regenerated ADMIN_PASSWORD will NOT replace the seeded admin,"
      warn "and rotated JWT/FILE_SIGNING keys will invalidate existing sessions and signed file URLs."
    fi
    backup_file "$ENV_FILE"
  fi

  local jwt_secret file_signing_key
  if [ "$EPHEMERAL_SECRETS" -eq 1 ]; then
    jwt_secret=""
    file_signing_key=""
    warn "Ephemeral secrets: JWT_SECRET and FILE_SIGNING_KEY left empty (auto-generated per restart)."
  else
    jwt_secret="$(gen_secret)"
    file_signing_key="$(gen_secret)"
    success "Generated JWT_SECRET and FILE_SIGNING_KEY (64-hex each)."
  fi

  # User-supplied values may contain '$'; escape them so Compose does not
  # interpolate them away. Generated secrets/passwords are alnum/hex (no-op).
  local env_tenant_name env_tenant_slug env_admin_email env_admin_password
  local env_bo_email env_bo_name env_bo_password
  env_tenant_name="$(esc_env "$TENANT_NAME")"
  env_tenant_slug="$(esc_env "$TENANT_SLUG")"
  env_admin_email="$(esc_env "$ADMIN_EMAIL")"
  env_admin_password="$(esc_env "$ADMIN_PASSWORD")"
  env_bo_email="$(esc_env "$BACKOFFICE_EMAIL")"
  env_bo_name="$(esc_env "$BACKOFFICE_NAME")"
  env_bo_password="$(esc_env "$BACKOFFICE_PASSWORD")"

  # Default DB credentials match .env.example (fine for a single-host example).
  # Subshell with a tight umask so the secrets file is 0600 from the first byte
  # (no world-readable window before the chmod below).
  ( umask 077
  cat > "$ENV_FILE" <<EOF
# Inventario deployment environment — generated by setup.sh on $(date -u '+%Y-%m-%dT%H:%M:%SZ')
# This file is gitignored and written mode 0600. It holds CLEARTEXT secrets
# (JWT/file-signing keys, DB and admin passwords) — keep it private.
#
# Run mode and image tag are wired through docker-compose.override.yaml.

# Application Configuration
INVENTARIO_HOST_PORT=${PORT}

# Secrets. Generate each with: openssl rand -hex 32
# Empty values cause the app to auto-generate throwaway keys per restart, which
# invalidates existing sessions and previously-signed file URLs on every restart.
JWT_SECRET=${jwt_secret}
FILE_SIGNING_KEY=${file_signing_key}
FILE_URL_EXPIRATION=15m

# Worker limits
MAX_CONCURRENT_EXPORTS=3
MAX_CONCURRENT_IMPORTS=1

# Thumbnail Generation Configuration
THUMBNAIL_MAX_CONCURRENT_PER_USER=5
THUMBNAIL_RATE_LIMIT_PER_MINUTE=50
THUMBNAIL_SLOT_DURATION=30m

# Token blacklist (Redis)
TOKEN_BLACKLIST_REDIS_URL=redis://redis:6379

# System
TZ=UTC

# Database Configuration (local defaults — fine for a single-host example)
POSTGRES_DB=inventario
POSTGRES_USER=inventario
POSTGRES_PASSWORD=inventario_password
POSTGRES_MIGRATOR_USER=inventario_migrator
POSTGRES_MIGRATOR_PASSWORD=inventario_migrator_password

# Initial Data Setup (runs only once)
DEFAULT_TENANT_NAME=${env_tenant_name}
DEFAULT_TENANT_SLUG=${env_tenant_slug}
ADMIN_EMAIL=${env_admin_email}
ADMIN_PASSWORD=${env_admin_password}
ADMIN_NAME=System Administrator

# Back-office (platform operator) — auto-provisioned on first run (/backoffice/login).
BACKOFFICE_EMAIL=${env_bo_email}
BACKOFFICE_NAME=${env_bo_name}
BACKOFFICE_PASSWORD=${env_bo_password}
BACKOFFICE_MFA_ENFORCED=${BACKOFFICE_MFA_ENFORCED}
EOF
  )

  chmod 600 "$ENV_FILE"
  success "Wrote .env (mode 0600)"
}

# ---------------------------------------------------------------------------
# Override generation. Always (re)generate for the chosen mode.
# ---------------------------------------------------------------------------
write_override() {
  section "Compose override (docker-compose.override.yaml)"

  # Render to a temp file first; only back up + replace if the content actually
  # changed, so repeated runs with identical inputs don't litter .bak.<n> files.
  # No timestamp in the body, otherwise every run would differ.
  local tmp
  tmp="$(mktemp "${TMPDIR:-/tmp}/inventario-override.XXXXXX")"
  TMP_FILES+=("$tmp")

  if [ "$MODE" = "image" ]; then
    local image_ref="${IMAGE_REPO}:${IMAGE_TAG}"
    # Image mode: for ALL FOUR inventario services, remove the build block with
    # `build: !reset null` and pin `image:`. This guarantees a plain
    # `docker compose up` can never trigger a local Node+Go compile (the build
    # key is fully removed from the resolved config, not merely emptied).
    # The main service also publishes the host port and wires the file signing key.
    cat > "$tmp" <<EOF
# Inventario Compose override — IMAGE mode — generated by setup.sh.
#
# Runs the prebuilt multi-arch image ${image_ref} with NO local compilation.
# 'build: !reset null' fully removes the build block from the resolved config so
# 'docker compose up' can never rebuild from source. Requires Compose >= v2.24.0.
#
# Re-generated by setup.sh on each run; edit setup.sh flags rather than this file.

services:
  inventario-bootstrap:
    build: !reset null
    image: ${image_ref}

  inventario-migrate:
    build: !reset null
    image: ${image_ref}

  inventario-init-data:
    build: !reset null
    image: ${image_ref}

  inventario:
    build: !reset null
    image: ${image_ref}
    # Base compose publishes NO host port; the override must add it for reachability.
    ports:
      - "\${INVENTARIO_HOST_PORT:-3333}:3333"
    environment:
      # Base compose does NOT wire the file signing key into the main service;
      # supply it from .env so signed file URLs survive restarts.
      INVENTARIO_RUN_FILE_SIGNING_KEY: \${FILE_SIGNING_KEY:-}
      INVENTARIO_RUN_FILE_URL_EXPIRATION: \${FILE_URL_EXPIRATION:-15m}
EOF
  else
    # Source mode: services keep building from the local repository. The override
    # only adds the host port mapping and the file signing key wiring.
    cat > "$tmp" <<EOF
# Inventario Compose override — SOURCE mode — generated by setup.sh.
#
# Services keep building from the local repository (base compose 'build:' blocks).
# This override only publishes the host port and wires the file signing key.
#
# Re-generated by setup.sh on each run; edit setup.sh flags rather than this file.

services:
  inventario:
    # Base compose publishes NO host port; the override must add it for reachability.
    ports:
      - "\${INVENTARIO_HOST_PORT:-3333}:3333"
    environment:
      # Base compose does NOT wire the file signing key into the main service;
      # supply it from .env so signed file URLs survive restarts.
      INVENTARIO_RUN_FILE_SIGNING_KEY: \${FILE_SIGNING_KEY:-}
      INVENTARIO_RUN_FILE_URL_EXPIRATION: \${FILE_URL_EXPIRATION:-15m}
EOF
  fi

  # Opt-in DB seeding (setup.sh --seed): enable the public, unauthenticated,
  # RLS-bypassing seed endpoint + bundled blob fixtures on the main service.
  # These lines append under the inventario service's environment: block (the
  # last block in both modes). setup.sh enables them ONLY to seed, then
  # regenerates this override WITHOUT them and recreates the container, so the
  # endpoint is OFF in the steady state.
  if [ "${SEED:-0}" = "1" ]; then
    {
      printf '      %s\n' 'INVENTARIO_RUN_ENABLE_SEED_ENDPOINT: "true"'
      printf '      %s\n' 'INVENTARIO_SEED_ALLOW_BLOB_UPLOADS: "true"'
    } >> "$tmp"
  fi

  if [ -f "$OVERRIDE_FILE" ] && cmp -s "$tmp" "$OVERRIDE_FILE"; then
    rm -f "$tmp"
    success "docker-compose.override.yaml already current (${MODE} mode) — left unchanged."
    return 0
  fi

  backup_file "$OVERRIDE_FILE"
  mv "$tmp" "$OVERRIDE_FILE"
  chmod 600 "$OVERRIDE_FILE" 2>/dev/null || true
  if [ "$MODE" = "image" ]; then
    success "Wrote docker-compose.override.yaml (image mode -> ${IMAGE_REPO}:${IMAGE_TAG})"
  else
    success "Wrote docker-compose.override.yaml (source mode)"
  fi
}

# ---------------------------------------------------------------------------
# Validate the merged Compose configuration.
# ---------------------------------------------------------------------------
validate_config() {
  section "Validate"
  local errf
  errf="$(mktemp "${TMPDIR:-/tmp}/inventario-compose.XXXXXX")"
  TMP_FILES+=("$errf")
  if "${COMPOSE[@]}" config -q 2>"$errf"; then
    success "Compose configuration is valid ('${COMPOSE[*]} config -q' exited 0)."
    rm -f "$errf"
  else
    err "Compose configuration is INVALID:"
    cat "$errf" >&2 || true
    rm -f "$errf"
    die "Aborting — fix the configuration above and re-run."
  fi
}

# ---------------------------------------------------------------------------
# Determine whether the compose 'up' supports --wait.
# ---------------------------------------------------------------------------
compose_supports_wait() {
  "${COMPOSE[@]}" up --help 2>/dev/null | grep -q -- '--wait'
}

# ---------------------------------------------------------------------------
# Pre-create host bind-mount dirs. On native Linux, Docker auto-creates missing
# bind-mount dirs as root:root, which the non-root (uid 1001) app/init container
# cannot write — init-data's `touch /app/state/data-initialized` then fails and
# aborts the whole `up`. Pre-own the inventario-owned mounts to the image user.
# ---------------------------------------------------------------------------
prepare_data_dirs() {
  mkdir -p data/postgres data/redis data/init-state data/uploads
  if [ "$(uname -s)" = "Linux" ]; then
    if ! chown -R 1001:1001 data/init-state data/uploads 2>/dev/null; then
      warn "Could not chown data/init-state, data/uploads to uid 1001. If 'up' fails with"
      warn "permission errors on native Linux, run: sudo chown -R 1001:1001 data/init-state data/uploads"
    fi
  fi
}

# ---------------------------------------------------------------------------
# Seed demo data via the (transiently enabled) seed endpoint. The caller is
# responsible for disabling the endpoint again afterwards.
# ---------------------------------------------------------------------------
seed_database() {
  section "Seed demo data"
  local port email slug body
  port="$(unesc_env "$(read_env_value INVENTARIO_HOST_PORT)")"; port="${port:-$PORT}"
  email="$(unesc_env "$(read_env_value ADMIN_EMAIL)")"; email="${email:-$ADMIN_EMAIL}"
  slug="$(unesc_env "$(read_env_value DEFAULT_TENANT_SLUG)")"; slug="${slug:-$TENANT_SLUG}"

  if ! command -v curl >/dev/null 2>&1; then
    warn "curl not found on host; skipping seeding. Seed manually: POST http://localhost:${port}/api/v1/seed"
    return 0
  fi

  body="$(printf '{"user_email":"%s","tenant_slug":"%s"}' "$email" "$slug")"
  log "Seeding demo data into tenant '${slug}' (user ${email}) ..."
  if curl -fsS --max-time 120 -X POST "http://localhost:${port}/api/v1/seed" \
       -H 'Content-Type: application/json' -d "$body" >/dev/null 2>&1; then
    SEEDED=1
    success "Database seeded with demo data."
  else
    warn "Seed request failed. The stack is up; retry with:"
    warn "  curl -X POST http://localhost:${port}/api/v1/seed -H 'Content-Type: application/json' -d '${body}'"
  fi
}

# ---------------------------------------------------------------------------
# Start the stack.
# ---------------------------------------------------------------------------
start_stack() {
  section "Start"
  local wait_flag=()
  if compose_supports_wait; then
    wait_flag=(--wait)
  fi

  if [ "$MODE" = "image" ]; then
    log "Pulling prebuilt image ${IMAGE_REPO}:${IMAGE_TAG} ..."
    if ! "${COMPOSE[@]}" pull; then
      die "Pull failed for ${IMAGE_REPO}:${IMAGE_TAG}. Verify the tag exists at ghcr.io (edge/latest/master/sha-*/vX.Y.Z) and is reachable; for a private tag run 'docker login ghcr.io'."
    fi
    log "Starting stack (no build) ..."
    "${COMPOSE[@]}" up -d --no-build "${wait_flag[@]+"${wait_flag[@]}"}"
  else
    log "Building from source and starting stack ..."
    "${COMPOSE[@]}" up -d --build "${wait_flag[@]+"${wait_flag[@]}"}"
  fi
  success "Stack is up."

  # Seed demo data, then disable the seed endpoint again for a secure steady state.
  if [ "${SEED:-0}" = "1" ]; then
    seed_database
    log "Disabling the seed endpoint ..."
    SEED=0
    write_override
    "${COMPOSE[@]}" up -d --no-build "${wait_flag[@]+"${wait_flag[@]}"}"
    success "Seed endpoint disabled."
  fi
}

# ---------------------------------------------------------------------------
# Final summary / next steps.
# ---------------------------------------------------------------------------
print_access_info() {
  local started="$1"   # 1 if started, 0 if not
  local compose_str="${COMPOSE[*]}"

  section "Summary"
  info "Mode:        ${MODE}"
  if [ "$MODE" = "image" ]; then
    info "Image:       ${IMAGE_REPO}:${IMAGE_TAG}"
  fi
  info "Host port:   ${PORT}"
  info "Admin email: ${ADMIN_EMAIL}"
  if [ "$ADMIN_PASSWORD_GENERATED" -eq 1 ]; then
    printf '%s\n' "    Admin password (generated, shown once): ${C_BOLD}${ADMIN_PASSWORD}${C_RESET}" >&2
  elif [ "$ENV_REUSED" -eq 1 ]; then
    info "Admin password: (unchanged — taken from existing .env)"
    info "Recover it with: grep '^ADMIN_PASSWORD=' \"$ENV_FILE\""
  else
    info "Admin password: (as supplied via --admin-password)"
  fi
  info "Backoffice:  ${BACKOFFICE_EMAIL}  (sign in at /backoffice/login)"
  if [ "$BACKOFFICE_PASSWORD_GENERATED" -eq 1 ]; then
    printf '%s\n' "    Backoffice password (generated, shown once): ${C_BOLD}${BACKOFFICE_PASSWORD}${C_RESET}" >&2
  elif [ "$ENV_REUSED" -eq 1 ]; then
    info "Backoffice password: (unchanged — grep '^BACKOFFICE_PASSWORD=' \"$ENV_FILE\")"
  else
    info "Backoffice password: (as supplied via --backoffice-password)"
  fi
  info ".env:        $ENV_FILE"
  info "override:    $OVERRIDE_FILE"

  if [ "$started" -eq 1 ]; then
    printf '\n%s\n' "${C_GREEN}${C_BOLD}Inventario is starting.${C_RESET}" >&2
    info "Web UI:    http://localhost:${PORT}"
    info "Backoffice: http://localhost:${PORT}/backoffice/login"
    info "Swagger:   http://localhost:${PORT}/swagger  (if enabled by the app build)"
    info "Readiness: http://localhost:${PORT}/readyz"
    if [ "$SEEDED" -eq 1 ]; then
      info "Demo data: seeded (the app is pre-populated)."
    fi
    printf '\n%s\n' "    Follow logs:  ${compose_str} logs -f inventario" >&2
    info "Stop:         ${compose_str} down"
    info "Stop + wipe:  ${compose_str} down && rm -rf ./data"
  else
    printf '\n%s\n' "${C_GREEN}${C_BOLD}Preparation complete.${C_RESET} To start the stack, run:" >&2
    if [ "$MODE" = "image" ]; then
      printf '%s\n' "    ${C_BOLD}${compose_str} pull && ${compose_str} up -d --no-build${C_RESET}" >&2
    else
      printf '%s\n' "    ${C_BOLD}${compose_str} up -d --build${C_RESET}" >&2
    fi
    info "Then open: http://localhost:${PORT}"
    info "Validate config any time: ${compose_str} config -q"
    if [ "$ADMIN_PASSWORD_GENERATED" -eq 1 ]; then
      printf '%s\n' "${C_YELLOW}Note:${C_RESET} the generated admin password above is also stored in .env (ADMIN_PASSWORD)." >&2
    fi
  fi
}

# ---------------------------------------------------------------------------
# Main.
# ---------------------------------------------------------------------------
main() {
  parse_args "$@"

  # Operator-supplied strings must not inject extra .env lines via a newline.
  reject_ctrl "$ADMIN_PASSWORD" "--admin-password"
  reject_ctrl "$ADMIN_EMAIL" "--admin-email"
  reject_ctrl "$TENANT_NAME" "--tenant-name"
  reject_ctrl "$TENANT_SLUG" "--tenant-slug"
  reject_ctrl "$BACKOFFICE_EMAIL" "--backoffice-email"
  reject_ctrl "$BACKOFFICE_PASSWORD" "--backoffice-password"

  [ -f "$COMPOSE_FILE" ] || die "docker-compose.yaml not found in $SCRIPT_DIR — run this from the example directory."

  section "Inventario setup"
  info "Working directory: $SCRIPT_DIR"

  preflight

  # Resolve mode (prompt if not given).
  if [ -z "$MODE" ]; then
    if [ "$ASSUME_YES" -eq 1 ]; then
      MODE="image"
    else
      local choice
      choice="$(prompt_default "Run mode — 'image' (prebuilt, no compile) or 'source' (build locally)" "image")"
      MODE="$choice"
    fi
  fi
  case "$MODE" in
    image|source) ;;
    *) die "Invalid --mode '$MODE' (expected 'image' or 'source')." ;;
  esac

  # Interactive refinement of common settings (skipped with -y).
  if [ "$ASSUME_YES" -eq 0 ] && [ -t 0 ]; then
    if [ "$MODE" = "image" ]; then
      IMAGE_TAG="$(prompt_default "Image tag" "$IMAGE_TAG")"
    fi
    PORT="$(prompt_default "Host port" "$PORT")"
    ADMIN_EMAIL="$(prompt_default "Admin email" "$ADMIN_EMAIL")"
  fi

  # Basic validation of port.
  case "$PORT" in
    ''|*[!0-9]*) die "Invalid --port '$PORT' (must be numeric)." ;;
  esac
  if [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then
    die "Invalid --port '$PORT' (must be 1-65535)."
  fi

  # Validate the image tag (image mode only) for a clear message before we write.
  if [ "$MODE" = "image" ]; then
    case "$IMAGE_TAG" in
      '') die "--image-tag must not be empty." ;;
      *[!A-Za-z0-9._-]*) die "Invalid --image-tag '$IMAGE_TAG' (allowed: letters, digits, '.', '_', '-')." ;;
    esac
  fi

  # Image mode's 'build: !reset null' needs Docker Compose >= v2.24.4. Reject an
  # older / legacy v1 Compose up front, instead of failing later in validate_config
  # with an opaque YAML merge error.
  if [ "$MODE" = "image" ]; then
    local cver rest cmaj cmin cpat cnum
    cver="$("${COMPOSE[@]}" version --short 2>/dev/null | tr -dc '0-9.')"
    cmaj="${cver%%.*}"; rest="${cver#*.}"
    cmin="${rest%%.*}"; cpat="${rest#*.}"; cpat="${cpat%%.*}"
    case "${cmaj:-x}" in
      ''|*[!0-9]*)
        warn "Could not parse the Docker Compose version; image mode needs >= v2.24.4 for 'build: !reset null'." ;;
      *)
        cnum=$(( cmaj * 1000000 + ${cmin:-0} * 1000 + ${cpat:-0} ))
        if [ "$cnum" -lt 2024004 ]; then
          die "Image mode needs Docker Compose >= v2.24.4 (for 'build: !reset null'); found v${cmaj}.${cmin:-0}.${cpat:-0}. Use --mode source, or upgrade Compose."
        fi ;;
    esac
  fi

  # Resolve admin password (generate if omitted) — only matters when we (re)write .env.
  local will_write_env=0
  if [ ! -f "$ENV_FILE" ] || [ "$FORCE" -eq 1 ]; then
    will_write_env=1
  fi
  if [ "$will_write_env" -eq 1 ]; then
    # Fail fast on a weak supplied password instead of aborting later in init-data.
    if [ -z "$ADMIN_PASSWORD" ]; then
      ADMIN_PASSWORD="$(gen_password)"
      ADMIN_PASSWORD_GENERATED=1
    elif [ "${#ADMIN_PASSWORD}" -lt 8 ] || ! password_meets_policy "$ADMIN_PASSWORD"; then
      die "--admin-password must be >= 8 chars with an upper-case letter, a lower-case letter, and a digit."
    fi
    if [ -z "$BACKOFFICE_PASSWORD" ]; then
      BACKOFFICE_PASSWORD="$(gen_password)"
      BACKOFFICE_PASSWORD_GENERATED=1
    elif [ "${#BACKOFFICE_PASSWORD}" -lt 8 ] || ! password_meets_policy "$BACKOFFICE_PASSWORD"; then
      die "--backoffice-password must be >= 8 chars with an upper-case letter, a lower-case letter, and a digit."
    fi
  fi

  write_env

  # When an existing .env is reused, .env-only flags (port/admin/tenant) are NOT
  # applied. Report the values actually in effect and warn if flags were passed.
  if [ "$ENV_REUSED" -eq 1 ]; then
    local eff_port eff_email
    eff_port="$(unesc_env "$(read_env_value INVENTARIO_HOST_PORT)")"
    eff_email="$(unesc_env "$(read_env_value ADMIN_EMAIL)")"
    [ -n "$eff_port" ] && PORT="$eff_port"
    [ -n "$eff_email" ] && ADMIN_EMAIL="$eff_email"
    if [ "$ENV_FLAGS_SUPPLIED" -eq 1 ]; then
      warn "Existing .env reused — --port/--admin-email/--admin-password/--tenant-* were ignored. Re-run with --force to apply them."
    fi
  fi

  # Resolve start + seed decisions BEFORE writing the override (seeding adds env
  # to it). --seed implies starting unless --no-start was explicit.
  if [ "${SEED:-}" = "1" ] && [ -z "$START" ]; then
    START=1
  fi
  if [ -z "$START" ]; then
    if [ "$ASSUME_YES" -eq 1 ]; then
      START=0
    elif prompt_yes_no "Start the stack now?" "n"; then
      START=1
    else
      START=0
    fi
  fi
  # Seeding only makes sense once the app is running.
  if [ -z "$SEED" ]; then
    if [ "$START" -eq 1 ] && [ "$ASSUME_YES" -eq 0 ] && \
       prompt_yes_no "Seed the database with demo data (so it is not empty)?" "n"; then
      SEED=1
    else
      SEED=0
    fi
  fi
  if [ "$SEED" -eq 1 ] && [ "$START" -ne 1 ]; then
    warn "--seed requires starting the stack; skipping seeding (start it, then re-run with --seed)."
    SEED=0
  fi

  write_override
  validate_config
  prepare_data_dirs

  if [ "$START" -eq 1 ]; then
    start_stack
    print_access_info 1
  else
    print_access_info 0
  fi

  success "Done."
}

main "$@"
