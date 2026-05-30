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
# Two posture bands:
#   - inv-vcl01-master/inventario-admin (admin.password) — HARD REQUIREMENT,
#     because the master ApplicationSet pins `secrets.existingSecret` to this
#     name and a missing Secret silently breaks master. A missing
#     admin.password makes the script exit non-zero (deferred to the end so
#     the optional sections still get applied — supports iterative bootstrap
#     with a partially-filled bundle).
#   - argocd/github-app-creds and tailscale/operator-oauth — best-effort:
#     a missing/empty section is skipped with a warning so a Phase-1 bundle
#     without those credentials can still bootstrap.
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

# --- inventario-admin (chart consumes via secrets.existingSecret) ---
# Materialized in BOTH persistent-namespace Applications that pin
# `secrets.existingSecret: inventario-admin`:
#   - inv-vcl01-master    — master ApplicationSet (#1883/#1885; #1885 replaced
#                           the previous static Application with a
#                           single-template ApplicationSet of the same name).
#   - inv-vcl01-longevity — the persistent, Velero-backed env (#1865).
# The chart's setup Job reads `SETUP_ADMIN_PASSWORD` from this Secret on first
# install (idempotent thereafter — the password is the seed value, not a
# runtime credential).
#
# When the bundle provides any of `jwt.secret`, `file_signing.key`, or
# `oauth_state.key`, this same Secret additionally carries the matching
# `INVENTARIO_RUN_JWT_SECRET` / `INVENTARIO_RUN_FILE_SIGNING_KEY` /
# `INVENTARIO_RUN_OAUTH_STATE_KEY`, pinning the apiserver's signing material
# across restarts (#1943). The chart loads the whole Secret via `envFrom`, so
# the keys reach the apiserver as-is — no chart change needed. All are OPTIONAL;
# an absent value leaves that key on the apiserver's random per-restart fallback
# (JWT: every redeploy logs users out and back-office MFA enrollment becomes
# undecryptable; file-signing: previously-issued signed file-download URLs stop
# validating; oauth-state: an in-flight OAuth sign-in fails state validation).
#
# Per-PR preview namespaces (`inv-vcl01-pr{N}`) are created dynamically by
# ArgoCD and so are NOT covered here; their ApplicationSet template
# (infra/argocd/applicationset-pr.yaml) sets a well-known dev password
# inline. See infra/SECRETS.md §4 for the master/PR split.
# admin.password is a hard requirement (see header) but we defer the exit so
# the optional GH App / Tailscale sections below can still apply on a
# partially-filled bundle; the final exit-non-zero at the bottom of the
# script ensures the missing field is surfaced loudly to bootstrap.sh
# (which runs under `set -e` and will halt the whole bootstrap).
ADMIN_PASSWORD=$(lookup "admin.password")
# Optional runtime JWT signing key for the persistent envs (master + longevity).
# When present it is injected into the inventario-admin Secret below as
# INVENTARIO_RUN_JWT_SECRET so the apiserver stops minting a fresh random secret
# on every restart. Absent it, warn (don't fail) — master ran disposably without
# a stable secret for a long time, so this stays best-effort.
JWT_SECRET=$(lookup "jwt.secret")
if [ -z "$JWT_SECRET" ]; then
    warn "jwt.secret missing in secrets bundle; inv-vcl01-master/longevity will use an EPHEMERAL per-restart JWT secret (every redeploy logs users out and back-office MFA enrollment won't survive a restart). Set jwt.secret to make it stable."
elif [ "${#JWT_SECRET}" -lt 32 ]; then
    # getJWTSecret() only accepts >=32 chars (plaintext) or >=64 hex chars;
    # anything shorter is silently ignored and a random per-restart secret is
    # generated. Drop it so we fall through to the ephemeral fallback loudly
    # rather than injecting a value the apiserver will discard.
    warn "jwt.secret is shorter than 32 chars; the apiserver ignores it and generates a random per-restart secret. Treating it as unset — use 'openssl rand -hex 32'."
    JWT_SECRET=""
fi
# Optional file-URL signing key for the persistent envs, same scheme as
# jwt.secret above. Absent it, signed file-download URLs break after a restart
# (the SPA re-fetches them, so it mostly self-heals); warn, don't fail.
FILE_SIGNING_KEY=$(lookup "file_signing.key")
if [ -z "$FILE_SIGNING_KEY" ]; then
    warn "file_signing.key missing in secrets bundle; inv-vcl01-master/longevity will use an EPHEMERAL per-restart file-signing key (previously-issued signed file-download URLs stop validating after a redeploy). Set file_signing.key to make it stable."
elif [ "${#FILE_SIGNING_KEY}" -lt 32 ]; then
    warn "file_signing.key is shorter than 32 chars; the apiserver ignores it and generates a random per-restart key. Treating it as unset — use 'openssl rand -hex 32'."
    FILE_SIGNING_KEY=""
fi
# Optional OAuth state-signing key for the persistent envs, same scheme. Only
# matters when OAuth sign-in is enabled; absent it, an in-flight OAuth flow that
# crosses a restart/replica fails state validation. Warn, don't fail.
OAUTH_STATE_KEY=$(lookup "oauth_state.key")
if [ -z "$OAUTH_STATE_KEY" ]; then
    warn "oauth_state.key missing in secrets bundle; inv-vcl01-master/longevity will use an EPHEMERAL per-restart OAuth state key (an OAuth sign-in spanning a redeploy/replica fails state validation). Set oauth_state.key to make it stable."
elif [ "${#OAUTH_STATE_KEY}" -lt 32 ]; then
    warn "oauth_state.key is shorter than 32 chars; the apiserver ignores it and generates a random per-restart key. Treating it as unset — use 'openssl rand -hex 32'."
    OAUTH_STATE_KEY=""
fi
ADMIN_MISSING=0
if [ -z "$ADMIN_PASSWORD" ]; then
    warn "admin.password missing in secrets bundle; required by master + longevity ApplicationSets via secrets.existingSecret"
    warn "Continuing with optional sections; will exit non-zero at the end so the issue can't be missed."
    ADMIN_MISSING=1
else
    for ns in inv-vcl01-master inv-vcl01-longevity; do
        note "Applying $ns/inventario-admin"
        # Emit values as YAML block scalars so secrets with special chars
        # (':', '{', '#', newlines) don't break manifest parsing or open an
        # injection path. Same pattern as the github private key block below.
        # Grouped so the optional INVENTARIO_RUN_JWT_SECRET key can be appended
        # to the same stringData map before a single apply.
        {
            cat <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: $ns
---
apiVersion: v1
kind: Secret
metadata:
  name: inventario-admin
  namespace: $ns
type: Opaque
stringData:
  SETUP_ADMIN_PASSWORD: |-
$(printf '%s' "$ADMIN_PASSWORD" | sed 's/^/    /')
EOF
            if [ -n "$JWT_SECRET" ]; then
                cat <<EOF
  INVENTARIO_RUN_JWT_SECRET: |-
$(printf '%s' "$JWT_SECRET" | sed 's/^/    /')
EOF
            fi
            if [ -n "$FILE_SIGNING_KEY" ]; then
                cat <<EOF
  INVENTARIO_RUN_FILE_SIGNING_KEY: |-
$(printf '%s' "$FILE_SIGNING_KEY" | sed 's/^/    /')
EOF
            fi
            if [ -n "$OAUTH_STATE_KEY" ]; then
                cat <<EOF
  INVENTARIO_RUN_OAUTH_STATE_KEY: |-
$(printf '%s' "$OAUTH_STATE_KEY" | sed 's/^/    /')
EOF
            fi
        } | remote_apply
    done
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
    # All values use chomped block scalars (`|-`) — keeps multi-line PEM intact
    # while stripping the spurious trailing newline that a `|` clip would add.
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
  githubAppPrivateKey: |-
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
    # Block scalars for the same reason as inv-vcl01-master/inventario-admin above.
    cat <<EOF | remote_apply
apiVersion: v1
kind: Secret
metadata:
  name: operator-oauth
  namespace: tailscale
type: Opaque
stringData:
  client_id: |-
$(printf '%s' "$TS_ID" | sed 's/^/    /')
  client_secret: |-
$(printf '%s' "$TS_SECRET" | sed 's/^/    /')
EOF
else
    warn "tailscale.oauth_client_{id,secret} missing; skipping tailscale/operator-oauth"
fi

# --- deferred fatal: admin.password ---
# See the admin block above for rationale (master ApplicationSet pins
# secrets.existingSecret = inventario-admin; a missing Secret silently breaks
# master). Optional sections above ran first so iterative bootstrap with a
# partially-filled bundle still applies what it can. bootstrap.sh runs under
# `set -e`, so the non-zero exit halts the rest of the run.
if [ "$ADMIN_MISSING" = 1 ]; then
    warn "Bootstrap incomplete: admin.password is required (see warning above). Fill the field in the sops bundle and re-run."
    exit 1
fi
