#!/usr/bin/env bash
#
# vm-install.sh — Inventario preview-env remote installer (#1853).
#
# Runs as root on a clean Ubuntu 26.04 VM. Brings up tailscaled, k3s (one
# node that is both server and agent, #2433), Tailscale Kubernetes Operator
# (#1855), and ArgoCD (#1858). Idempotent — re-running is safe.
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
#
# Every chart this script installs is pinned. An unpinned `helm upgrade
# --install` resolves to whatever the repo index says today, so `make
# bootstrap` and `make recover` on a clean VM stop being reproducible and an
# upstream release can break recovery at the worst possible moment (#1946).
#
# The `# renovate:` comments are read by the custom manager in renovate.json,
# so these bump through a reviewable PR rather than drifting silently or
# freezing forever.
#
# k3s v1.35 is the newest Kubernetes minor every chart below is tested on:
# Velero 1.18 is tested up to 1.35, Argo CD 3.6 on 1.34-1.37. Move to a newer
# minor only after Velero lists it, and one minor at a time on a running VM
# (`make upgrade` reruns the k3s installer when this pin changes).
# renovate: datasource=github-releases depName=k3s-io/k3s
K3S_VERSION="v1.35.10+k3s1"
# k3s ships kubectl but no helm CLI. v4 is what CI's setup-helm installs.
# renovate: datasource=github-releases depName=helm/helm
HELM_VERSION="v4.3.2"
# Velero (#1865): chart + matching CLI. The velero-plugin-for-aws image is
# pinned in infra/vm/helm-values/velero.yaml alongside the rest of the static
# config. Keep the CLI version aligned with the chart's appVersion so
# `make restore-longevity` speaks the same API as the in-cluster server.
# renovate: datasource=helm depName=velero registryUrl=https://vmware-tanzu.github.io/helm-charts
VELERO_CHART_VERSION="12.2.5"     # appVersion 1.18.2
VELERO_CLI_VERSION="v1.18.2"
# renovate: datasource=helm depName=tailscale-operator registryUrl=https://pkgs.tailscale.com/helmcharts
TS_OPERATOR_CHART_VERSION="1.104.7"
# renovate: datasource=helm depName=argo-cd registryUrl=https://argoproj.github.io/argo-helm
ARGOCD_CHART_VERSION="10.18.2"     # appVersion v3.6.2
# renovate: datasource=helm depName=reflector registryUrl=https://emberstack.github.io/helm-charts
REFLECTOR_CHART_VERSION="10.0.81"
TS_HOSTNAME="${TS_HOSTNAME:-inv-vcl01}"

note() { printf '\n==> %s\n' "$*" >&2; }
warn() { printf '\n[!] %s\n' "$*" >&2; }

# Look up a dotted key in the sops JSON bundle. Returns empty if absent.
sops_get() {
    local key="$1"
    [ -f "$SECRETS_FILE" ] || return 0
    jq -r --arg k "$key" '
        # Walk the dotted path, but only descend when the current value is an
        # object. A missing intermediate key (e.g. the whole `velero` section
        # absent on a bundle that never configured backups) makes the parent
        # resolve to null/"" — indexing that with the next segment is a jq
        # RUNTIME ERROR ("Cannot index string", exit 5), which under the
        # caller`s `set -e` aborts the whole bootstrap. Guarding on object
        # type returns empty for any missing segment instead, matching this
        # helper`s documented "empty if absent" contract.
        def lookup($parts; $v):
          if ($parts | length) == 0 then $v
          elif ($v | type) == "object" then lookup($parts[1:]; $v[$parts[0]])
          else "" end;
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
# Toggle NTP off+on to force an immediate step, restart systemd-timesyncd so it
# polls a fresh server, then poll timedatectl until "System clock synchronized:
# yes" before continuing. A plain `sleep 5` after `set-ntp true` is NOT enough:
# on a fresh boot, timesyncd's first poll can take 15-30s to complete and apt
# will still see the old clock if we proceed immediately.
note "Forcing NTP resync (guards against snapshot-rollback clock drift)"
timedatectl set-ntp false 2>/dev/null || true
timedatectl set-ntp true 2>/dev/null || true
systemctl restart systemd-timesyncd 2>/dev/null || true
ntp_synced=no
for i in $(seq 1 30); do
    if timedatectl show -p NTPSynchronized --value 2>/dev/null | grep -qi yes; then
        printf '    NTP synchronized after %ss\n' "$i" >&2
        ntp_synced=yes
        break
    fi
    sleep 1
done
# Hard-fail if the clock never stepped — downstream apt-get update / JWT
# operations would otherwise produce confusing "not valid yet" / "iat in the
# past" errors that read as totally unrelated to clock skew. Better to stop
# here with a targeted message so the operator knows what to fix
# (NTP firewall block, missing systemd-timesyncd, broken /etc/systemd/timesyncd.conf).
if [ "$ntp_synced" != "yes" ]; then
    echo "NTP failed to synchronize within 30s. Current state:" >&2
    timedatectl status >&2
    echo "Refusing to continue — downstream apt + JWT operations require a correct clock." >&2
    exit 1
fi

# --- Wait for dpkg lock ---
# Ubuntu cloud images ship with unattended-upgrades enabled, and the first
# boot triggers a refresh that takes the /var/lib/dpkg/lock-frontend for a
# few minutes. If we race it, `apt-get update` exits 100 with "Could not get
# lock /var/lib/dpkg/lock-frontend. It is held by process N (unattended-upgr)"
# and the whole bootstrap aborts under `set -e`. Wait it out with a generous
# timeout — unattended-upgrades on a fresh VM typically completes in 1-3 min,
# but a particularly busy first boot has been seen to take 5+ min.
#
# Lock-holder detection uses `fuser` (psmisc package). It's preinstalled on
# every Ubuntu cloud image we target (psmisc is a dependency of
# ubuntu-minimal), but on a stripped base where it's missing we degrade
# gracefully: skip the explicit wait with a warning and let apt-get retry
# its own internal lock acquisition (which has a much shorter timeout, so
# we may still hit the original race — but that's strictly better than
# hard-exiting here under `set -e`).
note "Waiting for dpkg lock (unattended-upgrades may be running on first boot)"
if ! command -v fuser >/dev/null 2>&1; then
    warn "fuser not found (psmisc package missing on this base image)."
    warn "Skipping dpkg-lock wait — apt-get may briefly race unattended-upgrades."
else
    dpkg_timeout=600
    dpkg_elapsed=0
    while fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1 || \
          fuser /var/lib/dpkg/lock >/dev/null 2>&1 || \
          fuser /var/lib/apt/lists/lock >/dev/null 2>&1; do
        if [ "$dpkg_elapsed" -ge "$dpkg_timeout" ]; then
            echo "dpkg lock still held after ${dpkg_timeout}s. Holders:" >&2
            fuser -v /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock /var/lib/apt/lists/lock 2>&1 | head -20 >&2
            echo "Refusing to continue — apt operations will fail." >&2
            exit 1
        fi
        if [ "$((dpkg_elapsed % 30))" -eq 0 ]; then
            printf '    still locked after %ss, waiting...\n' "$dpkg_elapsed" >&2
        fi
        sleep 5
        dpkg_elapsed=$((dpkg_elapsed + 5))
    done
    [ "$dpkg_elapsed" -gt 0 ] && printf '    dpkg lock free after %ss\n' "$dpkg_elapsed" >&2
fi

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

# --- k3s (one node: server and agent on this VM) ---
# k3s cannot share the host with vcluster standalone: both run their own
# kubelet and containerd, on the same ports (10250 and 10010). vcluster writes
# kubelet.service and containerd.service next to its own unit, and they stay
# when it is stopped, so any of the three blocks the install. `make destroy`
# removes them and reboots the VM to stop the containers they leave behind.
if [ -e /etc/systemd/system/vcluster.service ] || systemctl is-active --quiet vcluster.service \
    || [ -e /etc/systemd/system/kubelet.service ] || [ -e /etc/systemd/system/containerd.service ]; then
    echo "A vcluster standalone install is on this VM (vcluster, kubelet or containerd unit); k3s cannot run beside it." >&2
    echo "Run 'make -C infra destroy VM=...' (it removes vcluster and reboots the VM), then bootstrap again." >&2
    exit 1
fi

# AppArmor: container processes must keep a plain profile label.
# Ubuntu 26.04 ships kernel.apparmor_restrict_unprivileged_unconfined=1. With
# it, runc stacks the container's profile with `unconfined` (audit message:
# "change_profile unprivileged unconfined converted to stacking"), so every
# container process is labeled cri-containerd.apparmor.d//&unconfined, while
# containerd's default profile lets a process signal only peers labeled
# cri-containerd.apparmor.d. The processes of one container then cannot signal
# each other: the demo PostgreSQL entrypoint fails in `pg_ctl stop` with
# "Permission denied", and the server restarts into recovery and stays there.
# With the sysctl at 0 containers get the plain label again. What it restricts
# is an unprivileged, unconfined host process switching AppArmor profiles (the
# aa-exec route around the unprivileged-userns restriction); this VM has one
# admin user and no other local accounts. The drop-in sorts after the package's
# /usr/lib/sysctl.d/10-apparmor.conf, so its value wins. Pods that started
# before the change keep the stacked label until they are recreated.
APPARMOR_SYSCTL_FILE=/proc/sys/kernel/apparmor_restrict_unprivileged_unconfined
if [ -e "$APPARMOR_SYSCTL_FILE" ]; then
    printf '%s\n' 'kernel.apparmor_restrict_unprivileged_unconfined = 0' >/etc/sysctl.d/60-inventario-apparmor.conf
    if [ "$(cat "$APPARMOR_SYSCTL_FILE")" != 0 ]; then
        note "Turning off kernel.apparmor_restrict_unprivileged_unconfined"
        sysctl -q -w kernel.apparmor_restrict_unprivileged_unconfined=0
    fi
fi

# /etc/rancher/k3s/config.yaml carries every server option, so the systemd
# unit the installer writes needs no flags. k3s reads it on each start.
#
# tls-san adds the host's tailnet FQDN to the apiserver serving cert, so a
# kubeconfig with server=https://<host>.<tailnet>.ts.net:6443 verifies
# without a tls-server-name override. k3s adds a new tls-san entry to the
# cert on restart, which is why a changed config restarts k3s below. Without
# tailscale.tailnet_name in the sops bundle there is no SAN, and bootstrap.sh
# falls back to an IP-based kubeconfig.
#
# The hostname comes from the live tailscaled, not the `TS_HOSTNAME`
# default, so the SAN matches whatever the VM is registered as (vm-install.sh
# only warns about a mismatch, it does not re-key). It falls back to
# `TS_HOSTNAME` while tailscaled is not authenticated yet. bootstrap.sh reads
# the same `.Self.HostName` for the kubeconfig server.
#
# Traefik and ServiceLB, which k3s bundles, stay enabled: they are the k3s
# setup PRODUCTION.md documents, and the previews should run on it. Every
# Ingress here uses the tailscale class, so Traefik routes nothing, but
# ServiceLB publishes it on ports 80 and 443 of the VM's own addresses. To
# run without them, add `disable: [traefik, servicelb]` to the config below.
TAILNET_NAME=$(sops_get "tailscale.tailnet_name")
LIVE_TS_HOSTNAME=$(tailscale status --self=true --peers=false --json 2>/dev/null \
    | jq -r '.Self.HostName // empty')
SAN_HOSTNAME="${LIVE_TS_HOSTNAME:-$TS_HOSTNAME}"
K3S_CONFIG=/etc/rancher/k3s/config.yaml
K3S_CONFIG_NEW="$REMOTE_TMP/k3s-config.yaml"
{
    echo 'write-kubeconfig-mode: "0600"'
    if [ -n "${TAILNET_NAME:-}" ]; then
        echo 'tls-san:'
        echo "  - ${SAN_HOSTNAME}.${TAILNET_NAME}.ts.net"
    fi
} >"$K3S_CONFIG_NEW"
if [ -n "${TAILNET_NAME:-}" ]; then
    note "k3s apiserver cert will include SAN: ${SAN_HOSTNAME}.${TAILNET_NAME}.ts.net"
else
    warn "tailscale.tailnet_name missing from sops bundle — apiserver cert won't include tailnet FQDN SAN."
    warn "bootstrap.sh will fall back to IP-based kubeconfig with tls-server-name override."
fi
mkdir -p "$(dirname "$K3S_CONFIG")"
K3S_CONFIG_CHANGED=no
if ! cmp -s "$K3S_CONFIG_NEW" "$K3S_CONFIG"; then
    note "Writing $K3S_CONFIG"
    install -m 0600 "$K3S_CONFIG_NEW" "$K3S_CONFIG"
    K3S_CONFIG_CHANGED=yes
fi
rm -f "$K3S_CONFIG_NEW"

# The installer runs when k3s is missing or not at K3S_VERSION; rerun on a
# working node it upgrades k3s in place. It comes from the release tag, so
# the script is pinned along with the binary it fetches (and checks against
# the release's sha256sum file); `+` in the tag is URL-encoded.
# INSTALL_K3S_SYMLINK=force makes /usr/local/bin/kubectl the k3s symlink even
# when another kubectl is on PATH, because every script here calls that path.
INSTALLED_K3S_VERSION=""
if [ -x /usr/local/bin/k3s ]; then
    INSTALLED_K3S_VERSION=$(/usr/local/bin/k3s --version 2>/dev/null | awk 'NR == 1 { print $3 }' || true)
fi
K3S_RESTART=no
if [ "$INSTALLED_K3S_VERSION" != "$K3S_VERSION" ]; then
    note "Installing k3s $K3S_VERSION (installed: ${INSTALLED_K3S_VERSION:-none})"
    K3S_INSTALL_URL="https://raw.githubusercontent.com/k3s-io/k3s/${K3S_VERSION//+/%2B}/install.sh"
    curl -sfL "$K3S_INSTALL_URL" -o "$REMOTE_TMP/k3s-install.sh"
    INSTALL_K3S_VERSION="$K3S_VERSION" INSTALL_K3S_SYMLINK=force sh "$REMOTE_TMP/k3s-install.sh"
elif [ "$K3S_CONFIG_CHANGED" = yes ]; then
    K3S_RESTART=yes
fi
if [ "$K3S_RESTART" = yes ] || ! systemctl is-active --quiet k3s.service; then
    note "Restarting k3s.service"
    systemctl restart k3s.service
fi

export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
HELM=/usr/local/bin/helm
KUBECTL=/usr/local/bin/kubectl

# --- helm CLI (pinned, checksum-verified) ---
if [ "$("$HELM" version --template '{{.Version}}' 2>/dev/null || true)" != "$HELM_VERSION" ]; then
    note "Installing helm $HELM_VERSION"
    HELM_TGZ="helm-${HELM_VERSION}-linux-amd64.tar.gz"
    curl -sfL "https://get.helm.sh/${HELM_TGZ}" -o "$REMOTE_TMP/${HELM_TGZ}"
    curl -sfL "https://get.helm.sh/${HELM_TGZ}.sha256sum" -o "$REMOTE_TMP/${HELM_TGZ}.sha256sum"
    (cd "$REMOTE_TMP" && sha256sum --check --quiet "${HELM_TGZ}.sha256sum")
    tar -xzf "$REMOTE_TMP/${HELM_TGZ}" -C "$REMOTE_TMP" linux-amd64/helm
    install -m 0755 "$REMOTE_TMP/linux-amd64/helm" "$HELM"
fi

# --- Wait for the node to be Ready ---
note "Waiting for the k3s node to be Ready"
for i in $(seq 1 60); do
    if "$KUBECTL" get nodes --no-headers 2>/dev/null | awk '$2 == "Ready" { found = 1 } END { exit !found }'; then break; fi
    if [ "$i" -eq 60 ]; then echo "k3s node never became Ready" >&2; "$KUBECTL" get nodes >&2 || true; exit 1; fi
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
        --version "$TS_OPERATOR_CHART_VERSION" \
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
    --version "$ARGOCD_CHART_VERSION" \
    --namespace argocd \
    --set 'configs.params.server\.insecure=true' \
    --set 'configs.params.applicationsetcontroller\.policy=sync' \
    --set 'applicationSet.enabled=true' \
    --wait --timeout 10m

# --- reflector (#1976 — copy the AI-vision API key into dynamic PR-preview namespaces) ---
# emberstack/reflector is a tiny controller that mirrors annotated Secrets into
# other namespaces. apply-secrets.sh creates a single sops-sourced
# `inventario-ai-vision` Secret (AI keys only) annotated for reflection into
# `inv-vcl01-pr[0-9]+`; reflector copies it into each per-PR namespace as ArgoCD
# creates them, so PR previews can run the real Anthropic provider (the static
# master/longevity envs get the key via apply-secrets.sh directly and don't need
# this). Installed unconditionally — harmless with no source Secret; the
# reflection annotations are what actually drive behavior.
note "Installing/upgrading reflector"
"$HELM" repo add emberstack https://emberstack.github.io/helm-charts >/dev/null 2>&1 || true
"$HELM" repo update >/dev/null
"$HELM" upgrade --install reflector emberstack/reflector \
    --version "$REFLECTOR_CHART_VERSION" \
    --namespace reflector --create-namespace \
    --wait --timeout 5m

# --- Velero (#1865 — daily, encrypted, off-VM backup of inv-vcl01-longevity to R2) ---
# Optional Phase-2 component. Gated on the velero.s3_* keys being present in
# the sops bundle; a bundle without them simply skips Velero (preview-env core
# is unaffected). The install is also wrapped so a Velero failure WARNS rather
# than aborts the whole bootstrap — backups are additive to, not load-bearing
# for, the preview environments.
VELERO_S3_BUCKET=$(sops_get "velero.s3_bucket")
VELERO_S3_ENDPOINT=$(sops_get "velero.s3_endpoint")
VELERO_S3_REGION=$(sops_get "velero.s3_region")
VELERO_S3_ACCESS_KEY=$(sops_get "velero.s3_access_key")
VELERO_S3_SECRET_KEY=$(sops_get "velero.s3_secret_key")
VELERO_ENCRYPTION_KEY=$(sops_get "velero.encryption_key")
VELERO_STATIC_VALUES="$REMOTE_TMP/helm-values/velero.yaml"
# R2 ignores the region but the AWS SDK the plugin/kopia use require a
# non-empty value; "auto" is Cloudflare's documented placeholder.
[ -n "${VELERO_S3_REGION:-}" ] || VELERO_S3_REGION="auto"

if [ -n "${VELERO_S3_BUCKET:-}" ] && [ -n "${VELERO_S3_ENDPOINT:-}" ] && \
   [ -n "${VELERO_S3_ACCESS_KEY:-}" ] && [ -n "${VELERO_S3_SECRET_KEY:-}" ]; then
    [ -f "$VELERO_STATIC_VALUES" ] || { echo "missing $VELERO_STATIC_VALUES (bootstrap.sh upload)" >&2; exit 1; }
    note "Installing/upgrading Velero (backup target: $VELERO_S3_BUCKET @ $VELERO_S3_ENDPOINT)"
    "$KUBECTL" create namespace velero --dry-run=client -o yaml | "$KUBECTL" apply -f -

    # cloud-credentials: AWS-style INI the velero-plugin-for-aws + the kopia
    # node-agent both read. Written via a temp file (umask 077) + `kubectl
    # create secret --from-file` so the keys never land in process args or
    # shell history. `apply` (not plain `create`) makes rotation idempotent.
    VELERO_CREDS_FILE=$(umask 077 && mktemp "$REMOTE_TMP/velero-creds.XXXXXX")
    trap 'rm -f "$VELERO_CREDS_FILE"' EXIT
    cat >"$VELERO_CREDS_FILE" <<EOF
[default]
aws_access_key_id=$VELERO_S3_ACCESS_KEY
aws_secret_access_key=$VELERO_S3_SECRET_KEY
EOF
    "$KUBECTL" -n velero create secret generic cloud-credentials \
        --from-file=cloud="$VELERO_CREDS_FILE" \
        --dry-run=client -o yaml | "$KUBECTL" apply -f -
    rm -f "$VELERO_CREDS_FILE"
    trap - EXIT

    # velero-repo-credentials: the kopia repository password. Pre-creating it
    # from the sops-managed velero.encryption_key (instead of letting Velero
    # auto-generate a random one on first backup) is what makes restore on a
    # FRESH VM possible — a fresh Velero install would otherwise mint a NEW
    # random password and be unable to decrypt the existing kopia repo in R2.
    # Treat velero.encryption_key like the age key: back it up, never rotate it
    # after the first backup (rotation orphans every prior backup).
    if [ -n "${VELERO_ENCRYPTION_KEY:-}" ]; then
        # Write the password to a temp file (umask 077) + `--from-file` rather
        # than `--from-literal` so it never lands in process args (/proc/*/cmdline)
        # — same hygiene as the cloud-credentials block above.
        VELERO_REPO_PASS_FILE=$(umask 077 && mktemp "$REMOTE_TMP/velero-repo-pass.XXXXXX")
        trap 'rm -f "$VELERO_REPO_PASS_FILE"' EXIT
        printf '%s' "$VELERO_ENCRYPTION_KEY" >"$VELERO_REPO_PASS_FILE"
        "$KUBECTL" -n velero create secret generic velero-repo-credentials \
            --from-file=repository-password="$VELERO_REPO_PASS_FILE" \
            --dry-run=client -o yaml | "$KUBECTL" apply -f -
        rm -f "$VELERO_REPO_PASS_FILE"
        trap - EXIT
    else
        warn "velero.encryption_key missing — Velero will auto-generate a random kopia repo password."
        warn "Cross-VM / fresh-VM restore will NOT work. Set velero.encryption_key and re-bootstrap BEFORE the first backup."
    fi

    # The BackupStorageLocation is a LIST, and Helm REPLACES (not deep-merges)
    # a list supplied by a later values file — so the whole BSL entry is
    # generated here with the sops-sourced bucket/endpoint/region. The static
    # slice (plugins, node-agent, schedule, snapshotsEnabled,
    # credentials.existingSecret) comes from helm-values/velero.yaml.
    # checksumAlgorithm:"" is the Cloudflare R2 workaround for the AWS SDK's
    # default trailing-checksum, which R2 rejects with XAmzContentSHA256Mismatch.
    # Values emitted as chomped block scalars (`|-`) — defensive against any
    # YAML-sensitive char in the bucket/endpoint, same pattern as the TS OAuth
    # overlay above and apply-secrets.sh.
    VELERO_BSL_VALUES=$(umask 077 && mktemp "$REMOTE_TMP/velero-bsl.XXXXXX.yaml")
    trap 'rm -f "$VELERO_BSL_VALUES"' EXIT
    cat >"$VELERO_BSL_VALUES" <<EOF
configuration:
  backupStorageLocation:
    - name: default
      provider: aws
      default: true
      bucket: |-
$(printf '%s' "$VELERO_S3_BUCKET" | sed 's/^/        /')
      credential:
        name: cloud-credentials
        key: cloud
      config:
        region: |-
$(printf '%s' "$VELERO_S3_REGION" | sed 's/^/          /')
        s3Url: |-
$(printf '%s' "$VELERO_S3_ENDPOINT" | sed 's/^/          /')
        s3ForcePathStyle: "true"
        checksumAlgorithm: ""
EOF
    "$HELM" repo add vmware-tanzu https://vmware-tanzu.github.io/helm-charts >/dev/null 2>&1 || true
    # Don't let a transient registry hiccup abort the whole bootstrap for an
    # additive component — fall back to the cached index. (set -e would
    # otherwise turn `helm repo update` failure into a hard exit here.)
    "$HELM" repo update >/dev/null || warn "helm repo update failed; using cached chart index for Velero"
    if "$HELM" upgrade --install velero vmware-tanzu/velero \
        --namespace velero \
        --version "$VELERO_CHART_VERSION" \
        --values "$VELERO_STATIC_VALUES" \
        --values "$VELERO_BSL_VALUES" \
        --wait --timeout 10m; then
        note "Velero installed. Verify the backend with: velero backup-location get"
    else
        warn "Velero helm install failed/timed out — preview-env core is UNAFFECTED."
        warn "Investigate: kubectl -n velero get pods; kubectl -n velero logs deploy/velero"
        warn "Common cause: node-agent can't reach the kubelet pods dir on this distro."
    fi
    rm -f "$VELERO_BSL_VALUES"
    trap - EXIT

    # Install the matching velero CLI on the VM so `make restore-longevity`
    # can drive `velero backup get` / `velero restore create` over SSH. Kept
    # failure-isolated (warn, don't abort) so a flaky GitHub release download
    # can't fail a bootstrap whose Velero server already came up.
    if ! command -v velero >/dev/null 2>&1 || \
       ! velero version --client-only 2>/dev/null | grep -q "$VELERO_CLI_VERSION"; then
        note "Installing velero CLI $VELERO_CLI_VERSION"
        VELERO_TGZ="$REMOTE_TMP/velero.tar.gz"
        if curl -sfL "https://github.com/vmware-tanzu/velero/releases/download/${VELERO_CLI_VERSION}/velero-${VELERO_CLI_VERSION}-linux-amd64.tar.gz" -o "$VELERO_TGZ" \
            && tar -xzf "$VELERO_TGZ" -C "$REMOTE_TMP" \
            && install -m 0755 "$REMOTE_TMP/velero-${VELERO_CLI_VERSION}-linux-amd64/velero" /usr/local/bin/velero; then
            note "velero CLI $VELERO_CLI_VERSION installed"
        else
            warn "velero CLI download/install failed — 'make restore-longevity' won't work until it's present on the VM."
        fi
    fi
else
    warn "velero.s3_{bucket,endpoint,access_key,secret_key} incomplete in secrets; skipping Velero install."
    warn "Fill the velero.* keys in the sops bundle and re-run bootstrap. See infra/SECRETS.md §\"Velero / Cloudflare R2\"."
fi

note "vm-install.sh finished"
"$KUBECTL" get nodes -o wide
"$KUBECTL" get pods -A 2>/dev/null | head -30
