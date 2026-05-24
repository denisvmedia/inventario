#!/usr/bin/env bash
#
# destroy.sh — tear down vcluster + data on a preview-env VM (#1853).
#
# Stops and disables vcluster.service, wipes /etc/vcluster + /var/lib/vcluster,
# removes the /usr/local/bin/{vcluster,kubectl} symlinks the installer created.
# Tailscale membership (tailscaled + auth state) is intentionally preserved —
# `tailscale logout` is a separate, more destructive operation that breaks DNS
# for any other peer that has the host pinned.
#
# Idempotent. Re-running on a VM that has nothing to destroy is a no-op.
#
# Usage: bash infra/vm/destroy.sh user@host
#   (typically via `make destroy VM=user@host`)

set -Eeuo pipefail

VM="${1:?usage: destroy.sh user@host}"

note() { printf '\n==> %s\n' "$*" >&2; }

note "SSH preflight: $VM"
ssh -o ConnectTimeout=10 -o BatchMode=yes "$VM" 'echo ok' >/dev/null

note "Stopping vcluster.service and removing data"
ssh "$VM" 'set -e
    if systemctl list-unit-files vcluster.service >/dev/null 2>&1; then
        sudo systemctl disable --now vcluster.service 2>/dev/null || true
        sudo rm -f /etc/systemd/system/vcluster.service
        sudo systemctl daemon-reload
    fi
    sudo rm -rf /etc/vcluster /var/lib/vcluster
    sudo rm -f /usr/local/bin/vcluster /usr/local/bin/kubectl
    echo "vcluster + data removed"
'

note "Done. Tailscale state preserved (use \"sudo tailscale logout\" on the VM to revoke)."
