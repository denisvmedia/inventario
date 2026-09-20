#!/usr/bin/env bash
#
# Tail every pod in a namespace for as long as this script runs, one file
# per pod.
#
# The Job controller deletes a Job's pod once its backoff limit is
# exceeded, so the one pod whose logs matter most is exactly the one a
# post-hoc `kubectl logs` can no longer reach — the init-data failure in
# #2243 had to be reconstructed from sibling pods. Attaching while the pod
# is alive is the only way to keep it.
#
# Usage: capture-pod-logs.sh <namespace> <output-dir>
set -u

ns="${1:?namespace required}"
out="${2:?output directory required}"
mkdir -p "$out"

# Re-attach until the pod is gone: `kubectl logs -f` exits early while a
# pod is still pending or between init containers. Each attach replays the
# log from the start, so a file may repeat itself; a duplicated diagnostic
# beats a missing one.
tail_pod() {
  local pod="$1"
  while kubectl get pod "$pod" -n "$ns" >/dev/null 2>&1; do
    kubectl logs -f "$pod" -n "$ns" --all-containers=true --prefix \
      >>"$out/${pod}.log" 2>>"$out/${pod}.attach-errors" || true
    sleep 1
  done
}

declare -A seen=()
while true; do
  for pod in $(kubectl get pods -n "$ns" \
    -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null); do
    if [ -z "${seen[$pod]:-}" ]; then
      seen["$pod"]=1
      tail_pod "$pod" &
    fi
  done
  sleep 2
done
