#!/usr/bin/env bash
#
# destroy.sh — tear down k3s + data on a preview-env VM (#1853, #2433).
#
# Runs the k3s-uninstall.sh the k3s installer left on the VM: it stops
# k3s.service and every container, unmounts pod volumes, removes the CNI
# interfaces and iptables rules, and deletes /etc/rancher/k3s,
# /var/lib/rancher/k3s (including the local-path PVC data) and
# /var/lib/kubelet. The helm CLI that vm-install.sh adds goes too.
#
# A vcluster standalone install, if the VM has one, is removed as well.
# vcluster.service runs with KillMode=process, so stopping it leaves its
# containers, mounts and network interfaces behind (including the Tailscale
# proxies, which would keep their tailnet hostnames). The script therefore
# disables the unit, reboots the VM, waits for it to come back, and then
# deletes the files vcluster's own installer deletes for a reset.
#
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

note "Removing k3s and its data"
ssh "$VM" 'set -e
    if [ -x /usr/local/bin/k3s-uninstall.sh ]; then
        sudo /usr/local/bin/k3s-uninstall.sh >/dev/null 2>&1
        echo "k3s + data removed"
    else
        echo "k3s not installed"
    fi
    sudo rm -f /usr/local/bin/helm
'

if ssh "$VM" '[ -e /etc/systemd/system/vcluster.service ] || [ -d /var/lib/vcluster ]'; then
    note "Removing vcluster standalone"
    # Always attempt stop/disable — systemctl is idempotent and tolerates a
    # missing unit when we ignore the error.
    ssh "$VM" 'set -e
        sudo systemctl disable --now vcluster.service 2>/dev/null || true
        sudo rm -f /etc/systemd/system/vcluster.service
        sudo systemctl daemon-reload
    '

    # Wait for a new boot ID rather than for SSH alone: sshd can still answer
    # for a few seconds after `systemctl reboot` returns.
    note "Rebooting $VM to stop the containers vcluster.service left running"
    boot_id=$(ssh "$VM" 'cat /proc/sys/kernel/random/boot_id')
    ssh "$VM" 'sudo systemctl reboot' || true
    for i in $(seq 1 60); do
        sleep 5
        new_boot_id=$(ssh -o ConnectTimeout=5 -o BatchMode=yes "$VM" \
            'cat /proc/sys/kernel/random/boot_id' 2>/dev/null || true)
        if [ -n "$new_boot_id" ] && [ "$new_boot_id" != "$boot_id" ]; then
            break
        fi
        if [ "$i" -eq 60 ]; then
            echo "$VM did not come back within 5 minutes of the reboot." >&2
            echo "Re-run 'make destroy' once it is reachable; the vcluster files are still on disk." >&2
            exit 1
        fi
    done

    ssh "$VM" 'set -e
        sudo rm -rf /etc/vcluster /var/lib/vcluster /etc/kubernetes /var/lib/kubelet \
            /var/run/kubernetes /var/lib/etcd /root/.kube /root/.vcluster
        sudo rm -f /usr/local/bin/vcluster /usr/local/bin/kubectl
        echo "vcluster + data removed"
    '
fi

note "Done. Tailscale state preserved (use \"sudo tailscale logout\" on the VM to revoke)."
