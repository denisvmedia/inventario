#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd -P)"

KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-inventario-local-smoke}"
K8S_NAMESPACE="${K8S_NAMESPACE:-inventario-dev}"
APP_IMAGE="${APP_IMAGE:-inventario:kind-local}"
ARTIFACTS_DIR="${ARTIFACTS_DIR:-/tmp/kind-smoke-local-$(date +%Y%m%d-%H%M%S)}"
KIND_VERSION="${KIND_VERSION:-v0.29.0}"
KIND_CACHE_DIR="${KIND_CACHE_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/inventario-kind-smoke}"
KEEP_CLUSTER="${KEEP_CLUSTER:-false}"
DELETE_EXISTING_CLUSTER="${DELETE_EXISTING_CLUSTER:-true}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123}"
EXPECTED_LOCATION_NAME="${EXPECTED_LOCATION_NAME:-Home}"

PORT_FORWARD_LOG="$ARTIFACTS_DIR/inventario-port-forward.log"
PORT_FORWARD_PID_FILE="$ARTIFACTS_DIR/inventario-port-forward.pid"
RUN_LOG="$ARTIFACTS_DIR/run.log"
MANIFESTS_DIR=""
KUBECTL_CONTEXT="kind-$KIND_CLUSTER_NAME"
KIND_BIN="${KIND_BIN:-}"

usage() {
  cat <<EOF
Reproduce the local kind smoke flow used by .github/workflows/kind-smoke-test.yml.

Prerequisites:
  - bash, docker, kubectl, curl, jq, perl
  - if kind is missing, the script downloads kind $KIND_VERSION into $KIND_CACHE_DIR

Usage:
  bash scripts/kind-smoke-repro.sh

Optional environment overrides:
  KIND_CLUSTER_NAME        kind cluster name (default: $KIND_CLUSTER_NAME)
  APP_IMAGE                local image tag (default: $APP_IMAGE)
  ARTIFACTS_DIR            diagnostics directory (default: timestamped /tmp path)
  KEEP_CLUSTER             true to keep the kind cluster after the run
  DELETE_EXISTING_CLUSTER  false to fail instead of replacing an existing cluster name
EOF
}

log() {
  printf '\n[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

is_true() {
  case "$1" in
    1|[Tt][Rr][Uu][Ee]|[Yy]|[Yy][Ee][Ss]) return 0 ;;
    *) return 1 ;;
  esac
}

resolve_kind_bin() {
  if [ -n "$KIND_BIN" ] && [ -x "$KIND_BIN" ]; then
    printf '%s\n' "$KIND_BIN"
    return 0
  fi

  if command -v kind >/dev/null 2>&1; then
    command -v kind
    return 0
  fi

  return 1
}

kind_available() {
  resolve_kind_bin >/dev/null 2>&1
}

cluster_exists() {
  kind_available || return 1
  kind_cmd get clusters 2>/dev/null | grep -Fxq "$KIND_CLUSTER_NAME"
}

kubectl_cmd() {
  kubectl --context "$KUBECTL_CONTEXT" "$@"
}

kind_cmd() {
  local kind_bin
  kind_bin="$(resolve_kind_bin)" || {
    echo "kind is not available; run ensure_kind first or install kind" >&2
    return 1
  }

  "$kind_bin" "$@"
}

ensure_kind() {
  if [ -n "$KIND_BIN" ] && [ -x "$KIND_BIN" ]; then
    log "Using kind from KIND_BIN=$KIND_BIN"
    kind_cmd version
    return 0
  fi

  if command -v kind >/dev/null 2>&1; then
    KIND_BIN="$(command -v kind)"
    log "Using installed kind at $KIND_BIN"
    kind_cmd version
    return 0
  fi

  os_name="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch_name="$(uname -m)"

  case "$os_name" in
    darwin|linux) ;;
    *)
      echo "Unsupported OS for automatic kind download: $os_name" >&2
      exit 1
      ;;
  esac

  case "$arch_name" in
    x86_64|amd64) arch_name="amd64" ;;
    arm64|aarch64) arch_name="arm64" ;;
    *)
      echo "Unsupported architecture for automatic kind download: $arch_name" >&2
      exit 1
      ;;
  esac

  mkdir -p "$KIND_CACHE_DIR"
  kind_url="https://kind.sigs.k8s.io/dl/$KIND_VERSION/kind-$os_name-$arch_name"
  kind_path="$KIND_CACHE_DIR/kind-$KIND_VERSION-$os_name-$arch_name"
  checksum_path="$KIND_CACHE_DIR/kind-$KIND_VERSION-$os_name-$arch_name.sha256sum"

  if [ ! -x "$kind_path" ]; then
    log "kind is not installed; downloading $KIND_VERSION for $os_name/$arch_name"
    curl -fsSL -o "$kind_path" "$kind_url"
    curl -fsSL -o "$checksum_path" "$kind_url.sha256sum"
    expected_sha="$(awk '{print $1}' "$checksum_path")"

    if command -v shasum >/dev/null 2>&1; then
      actual_sha="$(shasum -a 256 "$kind_path" | awk '{print $1}')"
    elif command -v sha256sum >/dev/null 2>&1; then
      actual_sha="$(sha256sum "$kind_path" | awk '{print $1}')"
    else
      echo "Neither shasum nor sha256sum is available to verify the downloaded kind binary" >&2
      exit 1
    fi

    if [ "$actual_sha" != "$expected_sha" ]; then
      echo "Downloaded kind checksum mismatch: expected $expected_sha, got $actual_sha" >&2
      exit 1
    fi

    chmod +x "$kind_path"
  fi

  KIND_BIN="$kind_path"
  log "Using downloaded kind at $KIND_BIN"
  kind_cmd version
}

collect_diagnostics() {
  mkdir -p "$ARTIFACTS_DIR"
  log "Collecting diagnostics in $ARTIFACTS_DIR"

  if [ -n "$MANIFESTS_DIR" ] && [ -d "$MANIFESTS_DIR" ]; then
    rm -rf "$ARTIFACTS_DIR/manifests"
    cp -R "$MANIFESTS_DIR" "$ARTIFACTS_DIR/manifests"
  fi

  if ! kind_available; then
    log "kind is unavailable; skipping kubectl/kind diagnostics"
    return 0
  fi

  if ! cluster_exists; then
    log "Cluster $KIND_CLUSTER_NAME is not present; skipping kubectl/kind diagnostics"
    return 0
  fi

  kubectl_cmd get all,pvc,svc,endpoints,jobs -n "$K8S_NAMESPACE" -o wide \
    > "$ARTIFACTS_DIR/kubectl-get.txt" 2>&1 || true
  kubectl_cmd get events -n "$K8S_NAMESPACE" --sort-by=.metadata.creationTimestamp \
    > "$ARTIFACTS_DIR/kubectl-events.txt" 2>&1 || true
  kubectl_cmd describe deployments,pods,jobs,pvc,svc -n "$K8S_NAMESPACE" \
    > "$ARTIFACTS_DIR/kubectl-describe.txt" 2>&1 || true

  for resource in \
    deployment/inventario \
    deployment/postgres \
    deployment/redis \
    deployment/minio \
    job/minio-create-bucket \
    job/inventario-setup; do
    safe_name="$(printf '%s' "$resource" | tr '/' '_')"
    kubectl_cmd logs "$resource" -n "$K8S_NAMESPACE" --all-containers=true \
      > "$ARTIFACTS_DIR/${safe_name}.log" 2>&1 || true
  done

  pods="$(kubectl_cmd get pods -n "$K8S_NAMESPACE" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true)"
  if [ -n "$pods" ]; then
    while IFS= read -r pod_name; do
      [ -n "$pod_name" ] || continue
      kubectl_cmd logs pod/"$pod_name" -n "$K8S_NAMESPACE" --all-containers=true \
        > "$ARTIFACTS_DIR/pod_${pod_name}.log" 2>&1 || true
      kubectl_cmd logs pod/"$pod_name" -n "$K8S_NAMESPACE" --all-containers=true --previous \
        > "$ARTIFACTS_DIR/pod_${pod_name}_previous.log" 2>&1 || true
    done <<EOF
$pods
EOF
  fi

  kind_cmd export logs --name "$KIND_CLUSTER_NAME" "$ARTIFACTS_DIR/kind-logs" \
    > "$ARTIFACTS_DIR/kind-export.log" 2>&1 || true
}

cleanup() {
  status=$?
  set +e

  if [ -f "$PORT_FORWARD_PID_FILE" ]; then
    kill "$(cat "$PORT_FORWARD_PID_FILE")" 2>/dev/null || true
  fi

  if [ "$status" -ne 0 ]; then
    collect_diagnostics
  fi

  if is_true "$KEEP_CLUSTER"; then
    log "KEEP_CLUSTER=true; leaving cluster $KIND_CLUSTER_NAME running"
  elif kind_available && cluster_exists; then
    log "Deleting kind cluster $KIND_CLUSTER_NAME"
    kind_cmd delete cluster --name "$KIND_CLUSTER_NAME" >/dev/null 2>&1 || true
  fi

  if [ -n "$MANIFESTS_DIR" ] && [ -d "$MANIFESTS_DIR" ]; then
    rm -rf "$MANIFESTS_DIR"
  fi

  if [ "$status" -eq 0 ]; then
    log "Kind smoke reproduction completed successfully"
    log "Artifacts written to $ARTIFACTS_DIR"
  else
    log "Kind smoke reproduction failed"
    log "Diagnostics written to $ARTIFACTS_DIR"
  fi

  exit "$status"
}

wait_for_endpoint() {
  service_name="$1"
  attempt=1
  while [ "$attempt" -le 60 ]; do
    endpoint_ip="$(kubectl_cmd get endpoints "$service_name" -n "$K8S_NAMESPACE" -o jsonpath='{.subsets[0].addresses[0].ip}' 2>/dev/null || true)"
    if [ -n "$endpoint_ip" ]; then
      log "Service $service_name has endpoint $endpoint_ip"
      return 0
    fi
    log "Waiting for service/$service_name endpoints... ($attempt/60)"
    attempt=$((attempt + 1))
    sleep 2
  done

  log "Timed out waiting for service/$service_name endpoints"
  return 1
}

require_prerequisites() {
  missing_tools=""
  for tool_name in bash docker kubectl curl jq perl; do
    if ! command -v "$tool_name" >/dev/null 2>&1; then
      missing_tools="$missing_tools $tool_name"
    fi
  done

  if [ -n "$missing_tools" ]; then
    printf 'Missing required tools:%s\n' "$missing_tools" >&2
    exit 1
  fi

  docker info >/dev/null 2>&1 || {
    echo "Docker daemon is not reachable" >&2
    exit 1
  }
}

prepare_manifests() {
  MANIFESTS_DIR="$(mktemp -d)"
  cp -R "$REPO_ROOT/k8s/dev/." "$MANIFESTS_DIR/"
  perl -0pi -e "s#ghcr.io/denisvmedia/inventario:latest#$APP_IMAGE#g" \
    "$MANIFESTS_DIR/inventario/job-setup.yaml" \
    "$MANIFESTS_DIR/inventario/deployment.yaml"
  rm -rf "$ARTIFACTS_DIR/manifests"
  cp -R "$MANIFESTS_DIR" "$ARTIFACTS_DIR/manifests"
}

apply_manifests() {
  for manifest in \
    "$MANIFESTS_DIR/namespace.yaml" \
    "$MANIFESTS_DIR/postgres/secret.yaml" \
    "$MANIFESTS_DIR/postgres/configmap.yaml" \
    "$MANIFESTS_DIR/postgres/pvc.yaml" \
    "$MANIFESTS_DIR/postgres/service.yaml" \
    "$MANIFESTS_DIR/postgres/deployment.yaml" \
    "$MANIFESTS_DIR/redis/service.yaml" \
    "$MANIFESTS_DIR/redis/deployment.yaml" \
    "$MANIFESTS_DIR/minio/secret.yaml" \
    "$MANIFESTS_DIR/minio/pvc.yaml" \
    "$MANIFESTS_DIR/minio/service.yaml" \
    "$MANIFESTS_DIR/minio/deployment.yaml" \
    "$MANIFESTS_DIR/minio/job.yaml" \
    "$MANIFESTS_DIR/inventario/configmap.yaml" \
    "$MANIFESTS_DIR/inventario/secret.yaml" \
    "$MANIFESTS_DIR/inventario/job-setup.yaml"; do
    log "Applying ${manifest#$MANIFESTS_DIR/}"
    kubectl_cmd apply -f "$manifest"
  done
}

wait_for_readiness() {
  log "Waiting for postgres rollout"
  kubectl_cmd rollout status deployment/postgres -n "$K8S_NAMESPACE" --timeout=180s
  log "Waiting for redis rollout"
  kubectl_cmd rollout status deployment/redis -n "$K8S_NAMESPACE" --timeout=180s
  log "Waiting for minio rollout"
  kubectl_cmd rollout status deployment/minio -n "$K8S_NAMESPACE" --timeout=180s
  wait_for_endpoint postgres
  wait_for_endpoint redis
  wait_for_endpoint minio
  log "Waiting for minio-create-bucket job"
  kubectl_cmd wait --for=condition=complete job/minio-create-bucket -n "$K8S_NAMESPACE" --timeout=180s
  log "Waiting for inventario-setup job"
  kubectl_cmd wait --for=condition=complete job/inventario-setup -n "$K8S_NAMESPACE" --timeout=300s

  log "Applying inventario service and deployment"
  kubectl_cmd apply -f "$MANIFESTS_DIR/inventario/service.yaml"
  kubectl_cmd apply -f "$MANIFESTS_DIR/inventario/deployment.yaml"

  log "Waiting for inventario rollout"
  kubectl_cmd rollout status deployment/inventario -n "$K8S_NAMESPACE" --timeout=300s
  wait_for_endpoint inventario
}

start_port_forward() {
  log "Starting port-forward to service/inventario"
  nohup kubectl --context "$KUBECTL_CONTEXT" port-forward -n "$K8S_NAMESPACE" svc/inventario 3333:3333 \
    > "$PORT_FORWARD_LOG" 2>&1 &
  echo $! > "$PORT_FORWARD_PID_FILE"

  attempt=1
  while [ "$attempt" -le 30 ]; do
    if curl -fsS http://127.0.0.1:3333/healthz >/dev/null 2>&1; then
      log "Port-forward is ready"
      return 0
    fi
    if ! kill -0 "$(cat "$PORT_FORWARD_PID_FILE")" 2>/dev/null; then
      log "kubectl port-forward exited unexpectedly"
      return 1
    fi
    log "Waiting for port-forward... ($attempt/30)"
    attempt=$((attempt + 1))
    sleep 2
  done

  log "Timed out waiting for port-forward"
  return 1
}

run_smoke_checks() {
  base_url="http://127.0.0.1:3333"

  log "Checking /healthz"
  health_response="$(curl -fsS "$base_url/healthz")"
  printf '%s' "$health_response" | jq -e '.status == "alive"' >/dev/null

  log "Checking /readyz"
  ready_response="$(curl -fsS "$base_url/readyz")"
  printf '%s' "$ready_response" | jq -e '.status == "ready" and .checks.database.status == "ok" and .checks.redis.status == "ok"' >/dev/null

  log "Logging in as $ADMIN_EMAIL"
  login_response="$(curl -fsS -X POST "$base_url/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}")"
  access_token="$(printf '%s' "$login_response" | jq -r '.access_token')"
  [ -n "$access_token" ] && [ "$access_token" != "null" ]

  log "Checking seeded locations"
  locations_response="$(curl -fsS "$base_url/api/v1/locations" \
    -H "Authorization: Bearer $access_token")"
  printf '%s' "$locations_response" | jq -e --arg expected_name "$EXPECTED_LOCATION_NAME" 'any(.data[]?; .attributes.name == $expected_name)' >/dev/null
}

main() {
  if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
    usage
    exit 0
  fi

  mkdir -p "$ARTIFACTS_DIR"
  : > "$RUN_LOG"
  exec > >(tee -a "$RUN_LOG") 2>&1
  trap cleanup EXIT

  log "Repository root: $REPO_ROOT"
  log "Artifacts directory: $ARTIFACTS_DIR"
  log "Command: bash scripts/kind-smoke-repro.sh"

  require_prerequisites
  ensure_kind

  if cluster_exists; then
    if is_true "$DELETE_EXISTING_CLUSTER"; then
      log "Deleting existing cluster $KIND_CLUSTER_NAME before starting"
      kind_cmd delete cluster --name "$KIND_CLUSTER_NAME"
    else
      echo "Cluster $KIND_CLUSTER_NAME already exists; set DELETE_EXISTING_CLUSTER=true to replace it" >&2
      exit 1
    fi
  fi

  log "Building application image $APP_IMAGE"
  (
    cd "$REPO_ROOT"
    docker build --target production -t "$APP_IMAGE" .
  )

  log "Creating kind cluster $KIND_CLUSTER_NAME"
  kind_cmd create cluster --name "$KIND_CLUSTER_NAME" --wait 120s
  kubectl_cmd cluster-info

  log "Loading $APP_IMAGE into kind"
  kind_cmd load docker-image "$APP_IMAGE" --name "$KIND_CLUSTER_NAME"

  log "Preparing temporary manifests"
  prepare_manifests

  log "Applying manifests in CI order"
  apply_manifests

  log "Waiting for workloads to become ready"
  wait_for_readiness

  start_port_forward
  run_smoke_checks
}

main "$@"