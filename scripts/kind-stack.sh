#!/usr/bin/env bash
#
# kind-stack.sh — bring the full Inventario stack up in a local kind cluster
# for development and debugging. Mirrors the pods CI runs but is intended for
# repeated, interactive use rather than one-shot CI reproduction.
#
# Subcommands:
#   up           Build image, create cluster (if absent), load image, apply
#                manifests, wait for ready, start port-forward
#   down         Delete the cluster (and stop port-forward)
#   restart      down + up
#   status       Show cluster + pods + services in the namespace
#   logs <comp>  Tail logs for a component
#                  (inventario | postgres | redis | minio | setup | bucket)
#   reload       Rebuild the inventario image, reload into kind, restart the
#                inventario deployment in-place. Fast iteration for code
#                changes — leaves postgres/redis/minio data intact.
#   port-forward Re-create the background svc/inventario port-forward
#   smoke        Hit healthz/readyz/login/locations (same checks CI runs)
#   psql         Open `psql` inside the postgres pod
#   shell        Exec /bin/sh inside the inventario pod
#   help         Print this help
#
# Use `-h`/`--help` or `help` for usage. Most knobs are env vars (see top).

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd -P)"

KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-inventario-dev}"
K8S_NAMESPACE="${K8S_NAMESPACE:-inventario-dev}"
APP_IMAGE="${APP_IMAGE:-inventario:kind-dev}"
KIND_VERSION="${KIND_VERSION:-v0.29.0}"
KIND_CACHE_DIR="${KIND_CACHE_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/inventario-kind-stack}"
PORT_FORWARD_PORT="${PORT_FORWARD_PORT:-3333}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
# Must match k8s/dev/inventario/secret.yaml's INVENTARIO_MIGRATE_DATA_ADMIN_PASSWORD.
# Bumped from `admin123` after #1577 added password complexity rules.
ADMIN_PASSWORD="${ADMIN_PASSWORD:-Admin123}"
EXPECTED_LOCATION_NAME="${EXPECTED_LOCATION_NAME:-Home}"

STATE_DIR="${STATE_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/inventario-kind-stack/$KIND_CLUSTER_NAME}"
PORT_FORWARD_LOG="$STATE_DIR/port-forward.log"
PORT_FORWARD_PID_FILE="$STATE_DIR/port-forward.pid"

KUBECTL_CONTEXT="kind-$KIND_CLUSTER_NAME"
KIND_BIN="${KIND_BIN:-}"

mkdir -p "$STATE_DIR"

usage() {
  # Print the comment header (lines 3..first blank line), stripping the leading "# ".
  awk 'NR>=3 { if ($0 == "") exit; sub(/^# ?/, ""); print }' "${BASH_SOURCE[0]}"
  cat <<EOF

Environment overrides (current values shown):
  KIND_CLUSTER_NAME       $KIND_CLUSTER_NAME
  K8S_NAMESPACE           $K8S_NAMESPACE
  APP_IMAGE               $APP_IMAGE
  PORT_FORWARD_PORT       $PORT_FORWARD_PORT
  ADMIN_EMAIL             $ADMIN_EMAIL
  ADMIN_PASSWORD          (hidden — defaults to Admin123)
  KIND_VERSION            $KIND_VERSION (auto-downloaded if kind missing)
  STATE_DIR               $STATE_DIR

Examples:
  scripts/kind-stack.sh up
  scripts/kind-stack.sh logs inventario
  scripts/kind-stack.sh reload     # iterate on backend code
  scripts/kind-stack.sh smoke
  scripts/kind-stack.sh down
EOF
}

log() { printf '\n[%s] %s\n' "$(date '+%H:%M:%S')" "$*"; }
fail() { printf 'error: %s\n' "$*" >&2; exit 1; }

# ---------- kind helpers (download to cache dir if missing) -------------------

resolve_kind_bin() {
  if [ -n "$KIND_BIN" ] && [ -x "$KIND_BIN" ]; then printf '%s\n' "$KIND_BIN"; return 0; fi
  if command -v kind >/dev/null 2>&1; then command -v kind; return 0; fi
  return 1
}

kind_cmd() {
  local bin
  bin="$(resolve_kind_bin)" || fail "kind not available; run 'kind-stack.sh up' to auto-download"
  "$bin" "$@"
}

kubectl_cmd() { kubectl --context "$KUBECTL_CONTEXT" "$@"; }

ensure_kind() {
  if resolve_kind_bin >/dev/null 2>&1; then
    KIND_BIN="$(resolve_kind_bin)"
    return 0
  fi

  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$arch" in x86_64|amd64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) fail "unsupported arch $arch" ;; esac
  case "$os" in linux|darwin) ;; *) fail "unsupported OS $os" ;; esac

  mkdir -p "$KIND_CACHE_DIR"
  local url="https://kind.sigs.k8s.io/dl/$KIND_VERSION/kind-$os-$arch"
  local path="$KIND_CACHE_DIR/kind-$KIND_VERSION-$os-$arch"

  if [ ! -x "$path" ]; then
    log "downloading kind $KIND_VERSION → $path"
    curl -fsSL -o "$path" "$url"
    curl -fsSL -o "$path.sha256sum" "$url.sha256sum"
    local expected actual
    expected="$(awk '{print $1}' "$path.sha256sum")"
    if command -v sha256sum >/dev/null 2>&1; then
      actual="$(sha256sum "$path" | awk '{print $1}')"
    else
      actual="$(shasum -a 256 "$path" | awk '{print $1}')"
    fi
    [ "$actual" = "$expected" ] || fail "kind checksum mismatch ($actual vs $expected)"
    chmod +x "$path"
  fi

  KIND_BIN="$path"
}

require_prereqs() {
  local missing=""
  for t in bash docker kubectl curl jq perl; do
    command -v "$t" >/dev/null 2>&1 || missing="$missing $t"
  done
  [ -z "$missing" ] || fail "missing required tools:$missing"
  docker info >/dev/null 2>&1 || fail "docker daemon is not reachable"
}

cluster_exists() {
  resolve_kind_bin >/dev/null 2>&1 || return 1
  kind_cmd get clusters 2>/dev/null | grep -Fxq "$KIND_CLUSTER_NAME"
}

# ---------- manifest preparation (mirrors kind-smoke-test.yml) ----------------

prepare_manifests() {
  local out="$STATE_DIR/manifests"
  rm -rf "$out"
  mkdir -p "$out"
  cp -R "$REPO_ROOT/k8s/dev/." "$out/"
  perl -0pi -e "s#ghcr.io/denisvmedia/inventario:latest#$APP_IMAGE#g" \
    "$out/inventario/job-setup.yaml" \
    "$out/inventario/deployment.yaml"
  printf '%s' "$out"
}

apply_manifests() {
  local mdir="$1"
  for manifest in \
    "$mdir/namespace.yaml" \
    "$mdir/postgres/secret.yaml" \
    "$mdir/postgres/configmap.yaml" \
    "$mdir/postgres/pvc.yaml" \
    "$mdir/postgres/service.yaml" \
    "$mdir/postgres/deployment.yaml" \
    "$mdir/redis/service.yaml" \
    "$mdir/redis/deployment.yaml" \
    "$mdir/minio/secret.yaml" \
    "$mdir/minio/pvc.yaml" \
    "$mdir/minio/service.yaml" \
    "$mdir/minio/deployment.yaml" \
    "$mdir/minio/job.yaml" \
    "$mdir/inventario/configmap.yaml" \
    "$mdir/inventario/secret.yaml" \
    "$mdir/inventario/job-setup.yaml"; do
    log "apply ${manifest#"$mdir"/}"
    kubectl_cmd apply -f "$manifest"
  done
}

wait_for_endpoint() {
  local svc="$1" attempt=1 ip
  while [ "$attempt" -le 60 ]; do
    ip="$(kubectl_cmd get endpoints "$svc" -n "$K8S_NAMESPACE" -o jsonpath='{.subsets[0].addresses[0].ip}' 2>/dev/null || true)"
    if [ -n "$ip" ]; then log "service/$svc has endpoint $ip"; return 0; fi
    log "waiting for service/$svc... ($attempt/60)"
    attempt=$((attempt + 1))
    sleep 2
  done
  fail "timed out waiting for service/$svc"
}

wait_for_ready() {
  local mdir="$1"
  log "waiting for postgres rollout"
  kubectl_cmd rollout status deployment/postgres -n "$K8S_NAMESPACE" --timeout=180s
  log "waiting for redis rollout"
  kubectl_cmd rollout status deployment/redis -n "$K8S_NAMESPACE" --timeout=180s
  log "waiting for minio rollout"
  kubectl_cmd rollout status deployment/minio -n "$K8S_NAMESPACE" --timeout=180s
  wait_for_endpoint postgres
  wait_for_endpoint redis
  wait_for_endpoint minio
  log "waiting for minio-create-bucket job"
  kubectl_cmd wait --for=condition=complete job/minio-create-bucket -n "$K8S_NAMESPACE" --timeout=180s
  log "waiting for inventario-setup job"
  kubectl_cmd wait --for=condition=complete job/inventario-setup -n "$K8S_NAMESPACE" --timeout=300s

  log "applying inventario service + deployment"
  kubectl_cmd apply -f "$mdir/inventario/service.yaml"
  kubectl_cmd apply -f "$mdir/inventario/deployment.yaml"

  log "waiting for inventario rollout"
  kubectl_cmd rollout status deployment/inventario -n "$K8S_NAMESPACE" --timeout=300s
  wait_for_endpoint inventario
}

build_image() {
  log "building $APP_IMAGE (this can take a few minutes the first time)"
  ( cd "$REPO_ROOT" && docker build --target production -t "$APP_IMAGE" . )
}

load_image() {
  log "loading $APP_IMAGE into kind cluster $KIND_CLUSTER_NAME"
  kind_cmd load docker-image "$APP_IMAGE" --name "$KIND_CLUSTER_NAME"
}

# ---------- port-forward (background) -----------------------------------------
#
# When the script crashes mid-run, the OS may eventually reuse the PID stored
# in PORT_FORWARD_PID_FILE for an unrelated process. To avoid killing or
# observing the wrong PID, we stamp the cmdline at start time and only act
# on it if it still belongs to a kubectl port-forward process.

# port_forward_alive reports whether the PID in $PORT_FORWARD_PID_FILE still
# names a `kubectl port-forward` process (and not some unrelated re-use).
# Returns 0 iff the file exists, the PID is alive, AND its cmdline matches.
port_forward_alive() {
  [ -f "$PORT_FORWARD_PID_FILE" ] || return 1
  local pid
  pid="$(cat "$PORT_FORWARD_PID_FILE" 2>/dev/null || true)"
  [ -n "$pid" ] || return 1
  kill -0 "$pid" 2>/dev/null || return 1

  # /proc is the cheapest reliable check on Linux; ps -p ... -o args= is the
  # macOS / non-Linux fallback. cmdline arguments are NUL-separated; turn
  # those into spaces before grepping so the pattern stays simple.
  local cmd=""
  if [ -r "/proc/$pid/cmdline" ]; then
    cmd="$(tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null || true)"
  elif command -v ps >/dev/null 2>&1; then
    cmd="$(ps -p "$pid" -o args= 2>/dev/null || true)"
  fi
  case "$cmd" in
    *kubectl*port-forward*svc/inventario*) return 0 ;;
    *) return 1 ;;
  esac
}

stop_port_forward() {
  if port_forward_alive; then
    local pid
    pid="$(cat "$PORT_FORWARD_PID_FILE")"
    log "stopping port-forward (pid $pid)"
    kill "$pid" 2>/dev/null || true
  fi
  rm -f "$PORT_FORWARD_PID_FILE"
}

start_port_forward() {
  stop_port_forward
  log "starting port-forward on :$PORT_FORWARD_PORT (logs: $PORT_FORWARD_LOG)"
  : > "$PORT_FORWARD_LOG"
  nohup kubectl --context "$KUBECTL_CONTEXT" port-forward \
    -n "$K8S_NAMESPACE" "svc/inventario" "$PORT_FORWARD_PORT:3333" \
    > "$PORT_FORWARD_LOG" 2>&1 &
  echo $! > "$PORT_FORWARD_PID_FILE"

  local attempt=1
  while [ "$attempt" -le 30 ]; do
    if curl -fsS "http://127.0.0.1:$PORT_FORWARD_PORT/healthz" >/dev/null 2>&1; then
      log "port-forward is ready: http://127.0.0.1:$PORT_FORWARD_PORT"
      return 0
    fi
    if ! port_forward_alive; then
      cat "$PORT_FORWARD_LOG" >&2
      fail "port-forward exited unexpectedly"
    fi
    sleep 2
    attempt=$((attempt + 1))
  done
  cat "$PORT_FORWARD_LOG" >&2
  fail "timed out waiting for port-forward"
}

# ---------- subcommands -------------------------------------------------------

cmd_up() {
  require_prereqs
  ensure_kind

  if ! cluster_exists; then
    log "creating kind cluster $KIND_CLUSTER_NAME"
    kind_cmd create cluster --name "$KIND_CLUSTER_NAME" --wait 120s
    kubectl_cmd cluster-info
  else
    log "kind cluster $KIND_CLUSTER_NAME already exists; reusing"
  fi

  build_image
  load_image

  local mdir
  mdir="$(prepare_manifests)"
  apply_manifests "$mdir"
  wait_for_ready "$mdir"
  start_port_forward

  cat <<EOF

Stack is up.
  url:      http://127.0.0.1:$PORT_FORWARD_PORT
  email:    $ADMIN_EMAIL
  password: (default Admin123 from k8s/dev/inventario/secret.yaml; override via ADMIN_PASSWORD)
  cluster:  $KIND_CLUSTER_NAME
  context:  $KUBECTL_CONTEXT
  ns:       $K8S_NAMESPACE

Try:
  scripts/kind-stack.sh smoke      # verify endpoints
  scripts/kind-stack.sh logs inventario
  scripts/kind-stack.sh reload     # rebuild image + restart deployment
  scripts/kind-stack.sh down       # delete the cluster
EOF
}

cmd_down() {
  ensure_kind || true
  stop_port_forward
  if cluster_exists; then
    log "deleting kind cluster $KIND_CLUSTER_NAME"
    kind_cmd delete cluster --name "$KIND_CLUSTER_NAME"
  else
    log "no kind cluster $KIND_CLUSTER_NAME to delete"
  fi
}

cmd_restart() { cmd_down; cmd_up; }

cmd_status() {
  ensure_kind
  cluster_exists || fail "cluster $KIND_CLUSTER_NAME does not exist; run 'kind-stack.sh up' first"
  kubectl_cmd get all,jobs,pvc -n "$K8S_NAMESPACE" -o wide
  echo
  if port_forward_alive; then
    echo "port-forward: running on :$PORT_FORWARD_PORT (pid $(cat "$PORT_FORWARD_PID_FILE"))"
  else
    echo "port-forward: not running"
  fi
}

cmd_logs() {
  local component="${1:-inventario}"
  case "$component" in
    inventario) kubectl_cmd logs -n "$K8S_NAMESPACE" -l app.kubernetes.io/name=inventario,app.kubernetes.io/component=web --all-containers=true --tail=200 -f ;;
    postgres)   kubectl_cmd logs -n "$K8S_NAMESPACE" deployment/postgres --tail=200 -f ;;
    redis)      kubectl_cmd logs -n "$K8S_NAMESPACE" deployment/redis --tail=200 -f ;;
    minio)      kubectl_cmd logs -n "$K8S_NAMESPACE" deployment/minio --tail=200 -f ;;
    setup)      kubectl_cmd logs -n "$K8S_NAMESPACE" job/inventario-setup --all-containers=true --tail=500 ;;
    bucket)     kubectl_cmd logs -n "$K8S_NAMESPACE" job/minio-create-bucket --tail=500 ;;
    *) fail "unknown component '$component'; expected one of: inventario postgres redis minio setup bucket" ;;
  esac
}

cmd_reload() {
  ensure_kind
  cluster_exists || fail "cluster $KIND_CLUSTER_NAME does not exist; run 'kind-stack.sh up' first"
  build_image
  load_image
  log "rolling inventario deployment"
  # Forces pods to restart even though the image tag is the same — kubectl
  # recognises the env-var bump as a spec change.
  kubectl_cmd patch deployment/inventario -n "$K8S_NAMESPACE" \
    -p "{\"spec\":{\"template\":{\"metadata\":{\"annotations\":{\"kind-stack.sh/reload\":\"$(date +%s)\"}}}}}"
  kubectl_cmd rollout status deployment/inventario -n "$K8S_NAMESPACE" --timeout=300s
  log "reload complete"
}

cmd_port_forward() {
  ensure_kind
  cluster_exists || fail "cluster $KIND_CLUSTER_NAME does not exist; run 'kind-stack.sh up' first"
  start_port_forward
}

cmd_smoke() {
  local base="http://127.0.0.1:$PORT_FORWARD_PORT"
  log "GET $base/healthz"
  curl -fsS "$base/healthz" | jq -e '.status == "alive"' >/dev/null
  log "GET $base/readyz"
  curl -fsS "$base/readyz" | jq -e '.status == "ready" and .checks.database.status == "ok" and .checks.redis.status == "ok"' >/dev/null
  log "POST $base/api/v1/auth/login as $ADMIN_EMAIL"
  local login access csrf
  login="$(curl -fsS -X POST "$base/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}")"
  access="$(printf '%s' "$login" | jq -r '.access_token')"
  csrf="$(printf '%s' "$login" | jq -r '.csrf_token')"
  if [ -z "$access" ] || [ "$access" = "null" ]; then
    printf 'login response (token field redacted):\n%s\n' \
      "$(printf '%s' "$login" | jq '.access_token = (.access_token // null | if . == null then null else "<redacted>" end) | .csrf_token = (.csrf_token // null | if . == null then null else "<redacted>" end)' 2>/dev/null || printf '%s' "$login")" >&2
    fail "login failed: empty or null access_token"
  fi
  if [ -z "$csrf" ] || [ "$csrf" = "null" ]; then
    fail "login response missing csrf_token; the group-create call below cannot succeed without it"
  fi

  log "GET $base/api/v1/groups"
  local groups slug
  groups="$(curl -fsS "$base/api/v1/groups" \
    -H "Authorization: Bearer $access" \
    -H "Accept: application/vnd.api+json")"
  slug="$(printf '%s' "$groups" | jq -r '.data[0].attributes.slug // empty')"
  if [ -z "$slug" ]; then
    log "no group yet; creating one"
    local create
    create="$(curl -fsS -X POST "$base/api/v1/groups" \
      -H "Authorization: Bearer $access" \
      -H "Content-Type: application/vnd.api+json" \
      -H "X-CSRF-Token: $csrf" \
      -d '{"data":{"type":"groups","attributes":{"name":"Local Stack"}}}')"
    slug="$(printf '%s' "$create" | jq -r '.data.attributes.slug')"
  fi

  log "GET $base/api/v1/g/$slug/locations"
  curl -fsS "$base/api/v1/g/$slug/locations" \
    -H "Authorization: Bearer $access" \
    | jq -e --arg name "$EXPECTED_LOCATION_NAME" 'any(.data[]?; .attributes.name == $name)' >/dev/null

  log "smoke checks passed"
}

cmd_psql() {
  ensure_kind
  cluster_exists || fail "cluster $KIND_CLUSTER_NAME does not exist; run 'kind-stack.sh up' first"
  exec kubectl_cmd exec -it deployment/postgres -n "$K8S_NAMESPACE" -- \
    psql -U inventario -d inventario "$@"
}

cmd_shell() {
  ensure_kind
  cluster_exists || fail "cluster $KIND_CLUSTER_NAME does not exist; run 'kind-stack.sh up' first"
  local pod
  pod="$(kubectl_cmd get pod -n "$K8S_NAMESPACE" -l app.kubernetes.io/name=inventario,app.kubernetes.io/component=web -o jsonpath='{.items[0].metadata.name}')"
  [ -n "$pod" ] || fail "no inventario pod found"
  exec kubectl --context "$KUBECTL_CONTEXT" exec -it -n "$K8S_NAMESPACE" "$pod" -- /bin/sh
}

# ---------- entry point -------------------------------------------------------

main() {
  case "${1:-help}" in
    up)            shift; cmd_up "$@" ;;
    down)          shift; cmd_down "$@" ;;
    restart)       shift; cmd_restart "$@" ;;
    status)        shift; cmd_status "$@" ;;
    logs)          shift; cmd_logs "$@" ;;
    reload)        shift; cmd_reload "$@" ;;
    port-forward)  shift; cmd_port_forward "$@" ;;
    smoke)         shift; cmd_smoke "$@" ;;
    psql)          shift; cmd_psql "$@" ;;
    shell)         shift; cmd_shell "$@" ;;
    -h|--help|help) usage ;;
    *) usage; fail "unknown subcommand '${1:-}'" ;;
  esac
}

main "$@"
