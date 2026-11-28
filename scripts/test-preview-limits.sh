#!/bin/bash
# test-preview-limits.sh — Exercise `preview-manifests.py limits` on small
# manifests, one per case, and check that it passes or fails as it should.
#
# Usage:
#   ./scripts/test-preview-limits.sh
#
# Prerequisites: python3, bash. No helm: each case is a hand-written render.
#
# The check guards the pod policy in
# infra/vm/cluster-extras/preview-namespace-limits.yaml, which refuses a pod
# in a PR preview namespace unless every container and init container has a
# CPU and a memory limit at or below the ceiling. A render that passes the
# check but not the policy leaves the preview with pods that never start.
set -euo pipefail

cd "$(dirname "$0")/.."

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

quota='apiVersion: v1
kind: ResourceQuota
metadata:
  name: quota
spec:
  hard:
    requests.cpu: "2"
    requests.memory: 2Gi
    limits.cpu: "4"
    limits.memory: 4Gi
    requests.storage: 20Gi
    persistentvolumeclaims: "5"
    pods: "30"'

# limit_range DEFAULT-LINES [MAX-LINES]
limit_range() {
  printf -- '---\napiVersion: v1\nkind: LimitRange\nmetadata:\n  name: limits\nspec:\n  limits:\n    - type: Container\n'
  [ -n "$1" ] && printf '      default:\n%s\n' "$1"
  [ -n "${2:-}" ] && printf '      max:\n%s\n' "$2"
  return 0
}

# deployment INIT-RESOURCES APP-RESOURCES (each a resources: block or empty)
deployment() {
  cat <<EOF
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
spec:
  template:
    spec:
      initContainers:
        - name: migrate
          image: app
          command:
            - sh
            - -c
            - |
              echo resources:
$1
      containers:
        - name: app
          image: app
$2
EOF
}

both='          resources:
            limits:
              cpu: 500m
              memory: 512Mi
            requests:
              cpu: 50m'
cpu_only='          resources:
            limits:
              cpu: 500m'
memory_only='          resources:
            limits:
              memory: 512Mi'
flow='          resources:
            limits: {cpu: 500m, memory: 512Mi}'
over='          resources:
            limits:
              cpu: "3"
              memory: 512Mi'

failures=0
# check WANT DESCRIPTION — the manifest comes on stdin; WANT is pass or a
# fragment of the expected error.
check() {
  want="$1" desc="$2"
  cat > "$tmp/manifest.yaml"
  if out="$(python3 .github/scripts/preview-manifests.py limits "$tmp/manifest.yaml" 2>&1)"; then
    got=pass
  else
    got=fail
  fi
  if [ "$want" = pass ] && [ "$got" = pass ]; then
    printf 'ok   %s\n' "$desc"
  elif [ "$want" != pass ] && [ "$got" = fail ] && printf '%s' "$out" | grep -qF -- "$want"; then
    printf 'ok   %s\n' "$desc"
  else
    printf 'FAIL %s\n     want: %s\n     got:  %s\n' "$desc" "$want" "$out"
    failures=$((failures + 1))
  fi
}

{ echo "$quota"; deployment "$both" "$both"; } |
  check pass "explicit CPU and memory limits on every container"
{ echo "$quota"; deployment "$both" "$flow"; } |
  check pass "limits in flow style"
{ echo "$quota"; limit_range '        cpu: 500m
        memory: 512Mi'; deployment "" ""; } |
  check pass "no explicit limits, LimitRange defaults for both"
{ echo "$quota"; limit_range "" '        cpu: "2"
        memory: 2Gi'; deployment "" ""; } |
  check pass "no explicit limits, LimitRange max only (the API server copies it to default)"
{ echo "$quota"; deployment "$both" "$memory_only"; } |
  check "container app has no cpu limit" "container without a CPU limit, no LimitRange"
{ echo "$quota"; limit_range '        cpu: 500m'; deployment "$both" "$cpu_only"; } |
  check "container app has no memory limit" "container without a memory limit, LimitRange default for CPU only"
{ echo "$quota"; deployment "" "$both"; } |
  check "init container migrate has no cpu limit" "init container without limits, no LimitRange"
{ echo "$quota"; limit_range '        memory: 512Mi'; deployment "$cpu_only" "$both"; } |
  check pass "init container with a CPU limit and a memory default"
{ echo "$quota"; deployment "$both" "$over"; } |
  check "gets a cpu limit of \"3\", above the ceiling of 2" "container limit above the ceiling"
{ echo "$quota"; limit_range '        cpu: "3"
        memory: 512Mi'; deployment "" ""; } |
  check "the LimitRange sets a cpu limit of \"3\"" "LimitRange default above the ceiling"
{ echo "$quota" | sed 's/limits.cpu: "4"/limits.cpu: "8"/'; deployment "$both" "$both"; } |
  check "above the cap of 4" "quota above the cap"
{ echo "$quota" | grep -v 'pods:'; deployment "$both" "$both"; } |
  check "the ResourceQuota sets no pods" "quota without a pods cap"

if [ "$failures" -ne 0 ]; then
  echo "$failures case(s) failed" >&2
  exit 1
fi
