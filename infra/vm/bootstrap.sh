#!/usr/bin/env bash
#
# bootstrap.sh — laptop orchestrator for the Inventario preview-env VM (#1853).
#
# Brings up a clean Ubuntu 26.04 VM with tailscaled, vcluster standalone,
# Tailscale Operator, and ArgoCD. Reads secrets from the sops bundle
# (introduced in #1854) and applies ArgoCD manifests (introduced in #1858)
# at the end. Idempotent — re-runs are safe and act as upgrades.
#
# Usage: bash infra/vm/bootstrap.sh user@host
#   (typically invoked via `make bootstrap VM=user@host`)

set -Eeuo pipefail

VM="${1:?usage: bootstrap.sh user@host}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"

SECRETS_FILE="$REPO_ROOT/infra/vm/secrets/secrets.enc.yaml"
ARGOCD_DIR="$REPO_ROOT/infra/argocd"
VM_INSTALL="$SCRIPT_DIR/vm-install.sh"
APPLY_SECRETS="$SCRIPT_DIR/scripts/apply-secrets.sh"

note() { printf '\n==> %s\n' "$*" >&2; }
warn() { printf '\n[!] %s\n' "$*" >&2; }

# --- SSH preflight ---
note "SSH preflight: $VM"
ssh -o ConnectTimeout=10 -o BatchMode=yes "$VM" 'echo ok' >/dev/null

# --- Decrypt sops bundle locally (when present) ---
# Bundle schema lives in #1854; until that lands, bootstrap can still run and
# bring services up — just without auth-key, OAuth, or GH App credentials.
SECRETS_JSON=""
if [ -f "$SECRETS_FILE" ]; then
    command -v sops >/dev/null || { echo "sops required to decrypt $SECRETS_FILE" >&2; exit 1; }
    note "Decrypting $SECRETS_FILE"
    SECRETS_JSON=$(sops -d --output-type json "$SECRETS_FILE")
else
    warn "$SECRETS_FILE missing (lands in #1854)."
    warn "Tailscale will need manual 'sudo tailscale up' on the VM."
    warn "Tailscale Operator and ArgoCD GH-App integration won't have credentials."
fi

# --- Upload installer (and decrypted secrets if any) to a remote tmp dir ---
REMOTE_TMP=$(ssh "$VM" 'mktemp -d /tmp/inv-bootstrap.XXXXXX')
cleanup() { ssh "$VM" "rm -rf $REMOTE_TMP" 2>/dev/null || true; }
trap cleanup EXIT

note "Uploading installer to $VM:$REMOTE_TMP"
scp -q "$VM_INSTALL" "$VM":"$REMOTE_TMP/vm-install.sh"

if [ -n "$SECRETS_JSON" ]; then
    ssh "$VM" "umask 077 && cat > $REMOTE_TMP/secrets.plain.json" <<<"$SECRETS_JSON"
fi

# --- Run remote installer ---
note "Running vm-install.sh on $VM (first run takes 3-5 minutes)"
ssh "$VM" "sudo bash $REMOTE_TMP/vm-install.sh $REMOTE_TMP"

# --- Apply k8s Secrets generated from the sops bundle (#1854) ---
if [ -n "$SECRETS_JSON" ]; then
    if [ -x "$APPLY_SECRETS" ]; then
        note "Applying k8s Secrets to argocd/tailscale/inv-system namespaces"
        SECRETS_JSON="$SECRETS_JSON" bash "$APPLY_SECRETS" "$VM"
    else
        warn "$APPLY_SECRETS not executable; skipping k8s Secret apply step."
    fi
fi

# --- Apply ArgoCD manifests (AppProject, ApplicationSet, master Application) (#1858) ---
if [ -d "$ARGOCD_DIR" ] && compgen -G "$ARGOCD_DIR/*.yaml" >/dev/null; then
    note "Applying ArgoCD manifests from $ARGOCD_DIR"
    for m in "$ARGOCD_DIR"/*.yaml; do
        ssh "$VM" 'sudo /usr/local/bin/kubectl apply -f -' < "$m"
    done
else
    warn "$ARGOCD_DIR has no manifests yet (lands in #1858)."
    warn "ArgoCD is installed but no AppProject/ApplicationSet/Application is registered."
fi

# --- Copy kubeconfig back, rewrite server to tailnet IP ---
LAPTOP_KUBECONFIG="$HOME/.kube/inv-vcl01.config"
note "Copying kubeconfig to $LAPTOP_KUBECONFIG"
mkdir -p "$HOME/.kube"
ssh "$VM" 'sudo cat /var/lib/vcluster/kubeconfig.yaml' >"$LAPTOP_KUBECONFIG"
chmod 600 "$LAPTOP_KUBECONFIG"

TS_IP=$(ssh "$VM" 'tailscale ip -4 2>/dev/null | head -1' || true)
if [ -n "$TS_IP" ]; then
    # macOS sed and GNU sed differ; perl is portable.
    perl -pi -e "s|https://127\.0\.0\.1:|https://${TS_IP}:|g; s|https://localhost:|https://${TS_IP}:|g" \
        "$LAPTOP_KUBECONFIG"

    # The vcluster kube-apiserver TLS cert is signed for 127.0.0.1, 10.96.0.1,
    # and the in-cluster API IP — NOT for the tailnet IP. Without tls-server-name,
    # kubectl from the laptop would fail with "certificate is valid for 127.0.0.1
    # ..., not <tailnet IP>". Tell kubectl to verify against 127.0.0.1 (a SAN
    # that IS in the cert) while still dialling the tailnet IP. Idempotent.
    if ! grep -q '^    tls-server-name:' "$LAPTOP_KUBECONFIG"; then
        perl -i -pe '$_ .= "    tls-server-name: 127.0.0.1\n" if /^    server: https:/;' \
            "$LAPTOP_KUBECONFIG"
    fi

    note "kubeconfig server rewritten to https://${TS_IP}:<port> (tls-server-name: 127.0.0.1)"
else
    warn "Couldn't fetch tailnet IP. kubeconfig still points at 127.0.0.1 —"
    warn "either ssh-tunnel port 443/6443 or edit the server: field manually."
fi

note "Bootstrap complete. Verify:"
echo "    KUBECONFIG=$LAPTOP_KUBECONFIG kubectl get nodes,pods -A"
echo "    ssh $VM 'tailscale status | head'"
