#!/usr/bin/env bash
#
# restore-longevity.sh — interactive Velero restore of the inv-vcl01-longevity
# namespace from a backup in Cloudflare R2 (#1865).
#
# Laptop orchestrator (same posture as bootstrap.sh): all cluster work runs on
# the VM over SSH against the in-cluster kubeconfig; nothing here needs a local
# kubeconfig or the velero CLI on the laptop. Invoked via
# `make -C infra restore-longevity VM=user@host`.
#
# Why this is more than `velero restore create`:
#   inv-vcl01-longevity is managed by an ArgoCD ApplicationSet (prune +
#   selfHeal). Velero File System Backup restores data into FRESHLY-created
#   PVCs — if a PVC already exists, Velero skips it and you get an empty volume.
#   So the namespace must be deleted and recreated by the RESTORE, not by
#   ArgoCD. But the moment the namespace (or its PVCs) goes away, ArgoCD's
#   application-controller selfHeals it back with empty PVCs, and the
#   applicationset-controller would re-template the Application. We therefore
#   FREEZE both ArgoCD controllers (scale to 0) for the duration of the
#   restore, then resume them — they re-adopt the Velero-restored resources
#   (which match git desired state) and roll the deployment forward to current
#   master on top of the restored data. The freeze pauses reconciliation for
#   ALL Applications cluster-wide for ~2-3 minutes; acceptable for a
#   deliberate DR operation. A trap guarantees the controllers come back even
#   if the restore aborts.

set -Eeuo pipefail

VM="${1:?usage: restore-longevity.sh user@host}"

NS=inv-vcl01-longevity
# `sudo env KUBECONFIG=...` (not a bare `sudo kubectl`) so the in-cluster
# kubeconfig is passed explicitly regardless of root's default env — robust
# against sudo's env_reset. velero defaults to the `velero` namespace.
REMOTE_KUBECTL="sudo env KUBECONFIG=/var/lib/vcluster/kubeconfig.yaml /usr/local/bin/kubectl"
REMOTE_VELERO="sudo env KUBECONFIG=/var/lib/vcluster/kubeconfig.yaml /usr/local/bin/velero"

note() { printf '\n==> %s\n' "$*" >&2; }
warn() { printf '\n[!] %s\n' "$*" >&2; }

# --- SSH preflight ---
note "SSH preflight: $VM"
ssh -o ConnectTimeout=10 -o BatchMode=yes "$VM" 'echo ok' >/dev/null

# --- velero CLI present? ---
if ! ssh "$VM" 'command -v velero' >/dev/null 2>&1; then
    echo "velero CLI not found on $VM." >&2
    echo "Run 'make -C infra bootstrap VM=$VM' first — it installs Velero + the CLI" >&2
    echo "when the velero.* keys are present in the sops bundle." >&2
    exit 1
fi

# --- List available backups ---
note "Backups available in the configured BackupStorageLocation:"
if ! ssh "$VM" "$REMOTE_VELERO backup get"; then
    echo "Failed to list backups. Check 'velero backup-location get' on the VM." >&2
    exit 1
fi

# --- Pick one ---
printf '\nEnter the backup name to restore from (Ctrl-C to abort): ' >&2
read -r BACKUP
[ -n "${BACKUP:-}" ] || { echo "No backup name entered; aborting." >&2; exit 1; }

# Validate the named backup exists and is Completed before we touch anything.
# Use kubectl for the lookup — the velero CLI's `-o` only does table/json/yaml,
# not jsonpath, so we query the Backup CRD directly.
PHASE=$(ssh "$VM" "$REMOTE_KUBECTL -n velero get backups.velero.io '$BACKUP' -o jsonpath='{.status.phase}'" 2>/dev/null || true)
if [ -z "$PHASE" ]; then
    echo "Backup '$BACKUP' not found. Re-run and copy a name from the list above." >&2
    exit 1
fi
if [ "$PHASE" != "Completed" ]; then
    warn "Backup '$BACKUP' has phase '$PHASE' (not 'Completed'). Restoring from it may be incomplete."
    printf "Continue anyway? Type 'yes': " >&2
    read -r GO
    [ "${GO:-}" = "yes" ] || { echo "Aborted." >&2; exit 1; }
fi

# --- Danger confirmation ---
warn "This will DELETE namespace '$NS' and restore it from backup '$BACKUP'."
warn "ArgoCD reconciliation is paused CLUSTER-WIDE for ~2-3 minutes during the restore."
printf "Type 'restore' to proceed: " >&2
read -r CONFIRM
[ "${CONFIRM:-}" = "restore" ] || { echo "Aborted." >&2; exit 1; }

# --- Freeze ArgoCD so it neither selfHeals an empty namespace nor re-templates
#     the Application mid-restore. Trap guarantees both controllers come back. ---
resume_argocd() {
    warn "Resuming ArgoCD controllers"
    ssh "$VM" "$REMOTE_KUBECTL -n argocd scale statefulset/argocd-application-controller --replicas=1" || true
    ssh "$VM" "$REMOTE_KUBECTL -n argocd scale deploy/argocd-applicationset-controller --replicas=1" || true
}
trap resume_argocd EXIT

note "Pausing ArgoCD controllers (applicationset + application)"
ssh "$VM" "$REMOTE_KUBECTL -n argocd scale deploy/argocd-applicationset-controller --replicas=0"
ssh "$VM" "$REMOTE_KUBECTL -n argocd scale statefulset/argocd-application-controller --replicas=0"
# `scale` returns when the spec is patched, not when the pod is gone — wait for
# the application-controller pod to actually terminate so it can't observe the
# namespace deletion below and kick off one last selfHeal (which would recreate
# empty PVCs and make the FSB restore a no-op). Best-effort: `|| true` so a
# label/selector drift just falls back to the (usually-sufficient) timing.
ssh "$VM" "$REMOTE_KUBECTL -n argocd wait --for=delete pod \
    -l app.kubernetes.io/name=argocd-application-controller --timeout=90s" || true

# --- Clean slate: delete the namespace so the restore recreates its PVCs.
#     --wait blocks until the namespace (and its finalizers) are fully gone. ---
note "Deleting namespace '$NS' (waiting for full teardown)"
ssh "$VM" "$REMOTE_KUBECTL delete namespace '$NS' --ignore-not-found --wait=true --timeout=180s"

# --- Restore ---
RESTORE_NAME="${BACKUP}-restore-$(date +%Y%m%d%H%M%S)"
note "Restoring '$NS' from backup '$BACKUP' as restore/'$RESTORE_NAME'"
# Capture the exit status instead of letting `set -e` short-circuit, so the
# describe below ALWAYS prints (it's where a partial-failure reason shows up)
# before the EXIT trap resumes ArgoCD onto the restored namespace.
rc=0
ssh "$VM" "$REMOTE_VELERO restore create '$RESTORE_NAME' --from-backup '$BACKUP' --wait" || rc=$?

note "Restore object status:"
ssh "$VM" "$REMOTE_VELERO restore describe '$RESTORE_NAME' --details" || \
    ssh "$VM" "$REMOTE_VELERO restore get" || true

if [ "$rc" -ne 0 ]; then
    warn "Restore did NOT complete cleanly (velero exit $rc). Inspect the describe output above"
    warn "before trusting the data in '$NS'. ArgoCD is being resumed regardless (leaving it"
    warn "paused is worse); re-run this restore once you've understood the failure."
fi

# resume_argocd fires here via the EXIT trap.
note "Done. ArgoCD will re-adopt the restored resources and roll '$NS' forward to current master."
echo "    Verify: KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n $NS get pods,pvc" >&2
