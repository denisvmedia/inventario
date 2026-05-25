#!/usr/bin/env bash
#
# vm-install.sh — Inventario preview-env remote installer (#1853).
#
# Runs as root on a clean Ubuntu 26.04 VM. Brings up tailscaled, vcluster
# standalone (single-VM = both control-plane and worker; no token+join, per
# #1867 spike), Tailscale Kubernetes Operator (#1855), and ArgoCD (#1858).
# Idempotent — re-running is safe.
#
# Argument: $1 = path to an upload tmp dir on the VM. May contain
#               secrets.plain.json (sops-decrypted bundle, schema per #1854).

set -Eeuo pipefail

REMOTE_TMP="${1:?usage: vm-install.sh /tmp/upload-dir}"
SECRETS_FILE="$REMOTE_TMP/secrets.plain.json"

if [ "$EUID" -ne 0 ]; then
    exec sudo -E "$0" "$@"
fi

# --- Pinned versions ---
VCLUSTER_VERSION="v0.34.0"
K8S_VERSION="v1.34.0"
TS_HOSTNAME="${TS_HOSTNAME:-inv-vcl01}"

note() { printf '\n==> %s\n' "$*" >&2; }
warn() { printf '\n[!] %s\n' "$*" >&2; }

# Look up a dotted key in the sops JSON bundle. Returns empty if absent.
sops_get() {
    local key="$1"
    [ -f "$SECRETS_FILE" ] || return 0
    jq -r --arg k "$key" '
        def lookup($parts; $v):
          if ($parts | length) == 0 then $v
          else lookup($parts[1:]; $v[$parts[0]] // "")
          end;
        lookup($k | split("."); .) // empty
    ' "$SECRETS_FILE" 2>/dev/null
}

# --- Force NTP resync (MUST run before apt) ---
# Snapshot-rollback gotcha: rolling back from a `qm snapshot --vmstate 1` snapshot
# restores the kernel time-of-day clock to the snapshot moment (T0). systemd-timesyncd
# is slow to step a large drift on its own, leaving the clock 10+ hours behind real time
# until next natural poll. Consequences:
#   - `apt-get update` rejects archive InRelease files with "is not valid yet"
#     (Valid-Until check on signed metadata; on Ubuntu 26.04 this hard-fails the
#     update and downstream `apt-get install` returns exit 100).
#   - Anything JWT-based fails with "iat in the past / token expired" (GitHub App
#     installation-token exchange, ArgoCD repo-server auth, etc.).
# Toggle NTP off+on to force an immediate step; sleep gives chronyd time to converge.
note "Forcing NTP resync (guards against snapshot-rollback clock drift)"
timedatectl set-ntp false 2>/dev/null || true
timedatectl set-ntp true 2>/dev/null || true
sleep 5

# --- apt prereqs (#1867 noted conntrack + socat missing on stock Ubuntu 26.04) ---
note "apt prereqs"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq curl jq ca-certificates conntrack socat

# --- tailscaled ---
if ! command -v tailscale >/dev/null; then
    note "Installing tailscale"
    curl -fsSL https://tailscale.com/install.sh | sh
fi
systemctl enable --now tailscaled

if ! tailscale status >/dev/null 2>&1; then
    AUTHKEY=$(sops_get "tailscale.auth_key")
    if [ -n "${AUTHKEY:-}" ]; then
        note "tailscale up --hostname=$TS_HOSTNAME"
        tailscale up --authkey="$AUTHKEY" --hostname="$TS_HOSTNAME" --ssh=false
    else
        warn "tailscaled not authenticated and no tailscale.auth_key in secrets."
        warn "Run manually:  sudo tailscale up --hostname=$TS_HOSTNAME"
    fi
else
    # Ensure hostname matches even on re-runs.
    CURRENT_HOSTNAME=$(tailscale status --self=true --peers=false --json 2>/dev/null \
        | jq -r '.Self.HostName // empty')
    if [ -n "$CURRENT_HOSTNAME" ] && [ "$CURRENT_HOSTNAME" != "$TS_HOSTNAME" ]; then
        warn "Tailscale hostname is '$CURRENT_HOSTNAME', expected '$TS_HOSTNAME' — leaving as-is."
        warn "Re-key with: sudo tailscale up --hostname=$TS_HOSTNAME --reset"
    fi
fi

# --- vcluster standalone (single-VM = CP + worker, no taint; verified in #1867) ---
if [ ! -f /etc/vcluster/vcluster.yaml ]; then
    note "Writing /etc/vcluster/vcluster.yaml (k8s $K8S_VERSION)"
    mkdir -p /etc/vcluster
    cat >/etc/vcluster/vcluster.yaml <<EOF
controlPlane:
  distro:
    k8s:
      version: $K8S_VERSION
EOF
fi

if ! systemctl is-active --quiet vcluster.service; then
    note "Installing vcluster standalone $VCLUSTER_VERSION"
    INSTALL_URL="https://github.com/loft-sh/vcluster/releases/download/${VCLUSTER_VERSION}/install-standalone.sh"
    curl -sfL "$INSTALL_URL" -o "$REMOTE_TMP/install-standalone.sh"
    bash "$REMOTE_TMP/install-standalone.sh" --vcluster-name standalone
fi

export KUBECONFIG=/var/lib/vcluster/kubeconfig.yaml
HELM=/var/lib/vcluster/bin/helm
KUBECTL=/usr/local/bin/kubectl

# --- Wait for kube-apiserver to be reachable ---
note "Waiting for kube-apiserver"
for i in $(seq 1 30); do
    if "$KUBECTL" get nodes >/dev/null 2>&1; then break; fi
    if [ "$i" -eq 30 ]; then echo "kube-apiserver never came up" >&2; exit 1; fi
    sleep 5
done

# --- Tailscale Kubernetes Operator (#1855) ---
TS_OAUTH_ID=$(sops_get "tailscale.oauth_client_id")
TS_OAUTH_SECRET=$(sops_get "tailscale.oauth_client_secret")
TS_OP_STATIC_VALUES="$REMOTE_TMP/helm-values/tailscale-operator.yaml"
if [ -n "${TS_OAUTH_ID:-}" ] && [ -n "${TS_OAUTH_SECRET:-}" ]; then
    [ -f "$TS_OP_STATIC_VALUES" ] || { echo "missing $TS_OP_STATIC_VALUES (bootstrap.sh upload)" >&2; exit 1; }
    note "Installing Tailscale Kubernetes Operator"
    "$KUBECTL" create namespace tailscale --dry-run=client -o yaml | "$KUBECTL" apply -f -
    "$HELM" repo add tailscale https://pkgs.tailscale.com/helmcharts >/dev/null 2>&1 || true
    "$HELM" repo update >/dev/null
    # Layer OAuth on top of static values via a second --values overlay (later
    # file wins on key conflict). Writing oauth to a temp file avoids leaking
    # the secret through `--set-string` in /proc/*/cmdline / process-args audit.
    # Emit values as chomped block scalars (`|-`) instead of double-quoted
    # strings — defensive against credentials containing YAML-sensitive chars
    # (`"`, `\`, newlines). Same pattern as infra/vm/scripts/apply-secrets.sh.
    TS_OAUTH_VALUES=$(umask 077 && mktemp "$REMOTE_TMP/ts-op-oauth.XXXXXX.yaml")
    trap 'rm -f "$TS_OAUTH_VALUES"' EXIT
    cat >"$TS_OAUTH_VALUES" <<EOF
oauth:
  clientId: |-
$(printf '%s' "$TS_OAUTH_ID" | sed 's/^/    /')
  clientSecret: |-
$(printf '%s' "$TS_OAUTH_SECRET" | sed 's/^/    /')
EOF
    "$HELM" upgrade --install tailscale-operator tailscale/tailscale-operator \
        --namespace tailscale \
        --values "$TS_OP_STATIC_VALUES" \
        --values "$TS_OAUTH_VALUES" \
        --wait --timeout 5m
    rm -f "$TS_OAUTH_VALUES"
    trap - EXIT
else
    warn "tailscale.oauth_client_{id,secret} not in secrets; skipping Tailscale Operator install."
    warn "Provide them in the sops bundle and re-run bootstrap. See infra/SECRETS.md."
fi

# --- ArgoCD (#1858 owns the ApplicationSet/AppProject/Application manifests) ---
note "Installing/upgrading ArgoCD"
"$KUBECTL" create namespace argocd --dry-run=client -o yaml | "$KUBECTL" apply -f -
"$HELM" repo add argo https://argoproj.github.io/argo-helm >/dev/null 2>&1 || true
"$HELM" repo update >/dev/null
"$HELM" upgrade --install argocd argo/argo-cd \
    --namespace argocd \
    --set 'configs.params.server\.insecure=true' \
    --set 'configs.params.applicationsetcontroller\.policy=sync' \
    --set 'applicationSet.enabled=true' \
    --wait --timeout 10m

note "vm-install.sh finished"
"$KUBECTL" get nodes -o wide
"$KUBECTL" get pods -A 2>/dev/null | head -30
