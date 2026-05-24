#!/usr/bin/env bash
#
# apply-secrets.sh — translate the sops bundle into k8s Secrets and apply them
# to the in-cluster namespaces consumed by the chart, the Tailscale Operator,
# and ArgoCD's repo-creds layer.
#
# Invoked by bootstrap.sh after vm-install.sh has brought the cluster up.
# Expects SECRETS_JSON in the environment (decrypted JSON from sops --output-type json).
#
# Schema lives in infra/vm/secrets/secrets.example.yaml; the real encrypted bundle
# at infra/vm/secrets/secrets.enc.yaml is owned by #1854.
#
# This script is best-effort: if a section is missing from the bundle (e.g. GitHub
# App not yet configured), the corresponding Secret apply is skipped with a warning.
#
# Usage: SECRETS_JSON='{...}' bash apply-secrets.sh user@host

set -Eeuo pipefail

VM="${1:?usage: apply-secrets.sh user@host}"
: "${SECRETS_JSON:?SECRETS_JSON env var required}"

note() { printf '\n==> %s\n' "$*" >&2; }
warn() { printf '\n[!] %s\n' "$*" >&2; }

# Look up a dotted key, returning empty string if any segment is missing.
lookup() {
    local key="$1"
    jq -r --arg k "$key" '
        def lookup($parts; $v):
          if ($parts | length) == 0 then $v
          else lookup($parts[1:]; $v[$parts[0]] // "")
          end;
        lookup($k | split("."); .) // empty
    ' <<<"$SECRETS_JSON" 2>/dev/null
}

# Pipe a manifest through `kubectl apply -f -` on the remote VM.
remote_apply() {
    ssh "$VM" 'sudo /usr/local/bin/kubectl apply -f -'
}

# --- inv-system / inventario-admin (chart consumes via existingSecret) ---
ADMIN_EMAIL=$(lookup "admin.email")
ADMIN_PASSWORD=$(lookup "admin.password")
if [ -n "$ADMIN_EMAIL" ] && [ -n "$ADMIN_PASSWORD" ]; then
    note "Applying inv-system/inventario-admin"
    # Emit values as YAML block scalars so passwords with special chars (':', '{', '#', newlines)
    # don't break manifest parsing or open an injection path. Same pattern as
    # the github private key block below.
    cat <<EOF | remote_apply
apiVersion: v1
kind: Namespace
metadata:
  name: inv-system
---
apiVersion: v1
kind: Secret
metadata:
  name: inventario-admin
  namespace: inv-system
type: Opaque
stringData:
  email: |
$(printf '%s' "$ADMIN_EMAIL" | sed 's/^/    /')
  password: |
$(printf '%s' "$ADMIN_PASSWORD" | sed 's/^/    /')
EOF
else
    warn "admin.{email,password} missing in secrets; skipping inv-system/inventario-admin"
fi

# --- argocd / github-app-creds (repo-creds + ApplicationSet PR-generator) ---
GH_APP_ID=$(lookup "github.app_id")
GH_INSTALL_ID=$(lookup "github.app_installation_id")
GH_PRIVATE_KEY=$(lookup "github.app_private_key")
GH_URL=$(lookup "github.url")
[ -n "$GH_URL" ] || GH_URL="https://github.com/denisvmedia"
if [ -n "$GH_APP_ID" ] && [ -n "$GH_INSTALL_ID" ] && [ -n "$GH_PRIVATE_KEY" ]; then
    note "Applying argocd/github-app-creds"
    # PEM may contain newlines; embed via stringData so kubectl handles escaping.
    cat <<EOF | remote_apply
apiVersion: v1
kind: Secret
metadata:
  name: github-app-creds
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repo-creds
type: Opaque
stringData:
  type: git
  url: $GH_URL
  githubAppID: "$GH_APP_ID"
  githubAppInstallationID: "$GH_INSTALL_ID"
  githubAppPrivateKey: |
$(printf '%s' "$GH_PRIVATE_KEY" | sed 's/^/    /')
EOF
else
    warn "github.app_{id,installation_id,private_key} incomplete in secrets; skipping argocd/github-app-creds"
fi

# --- tailscale / operator-oauth (consumed by the TS-op helm chart in vm-install.sh) ---
# The chart's helm install also takes OAuth via --set-string flags, but a Secret
# keeps the bundle the source of truth for rotation. Optional.
TS_ID=$(lookup "tailscale.oauth_client_id")
TS_SECRET=$(lookup "tailscale.oauth_client_secret")
if [ -n "$TS_ID" ] && [ -n "$TS_SECRET" ]; then
    note "Applying tailscale/operator-oauth"
    # Block scalars for the same reason as inv-system/inventario-admin above.
    cat <<EOF | remote_apply
apiVersion: v1
kind: Secret
metadata:
  name: operator-oauth
  namespace: tailscale
type: Opaque
stringData:
  client_id: |
$(printf '%s' "$TS_ID" | sed 's/^/    /')
  client_secret: |
$(printf '%s' "$TS_SECRET" | sed 's/^/    /')
EOF
else
    warn "tailscale.oauth_client_{id,secret} missing; skipping tailscale/operator-oauth"
fi
