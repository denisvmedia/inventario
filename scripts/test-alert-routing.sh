#!/bin/bash
# test-alert-routing.sh — Check that every alert the chart ships lands on the
# receiver it is meant to land on.
#
# Usage:
#   ./scripts/test-alert-routing.sh
#
# Prerequisites: helm, ruby, and either amtool on PATH or docker.
#
# scripts/test-alert-rules.sh proves the expressions fire. This proves the
# other half: that what fires is routed somewhere. The two failures look
# nothing alike from the outside and both end in silence.
#
# The expected map below is deliberately exhaustive. An alert present in the
# rules and absent from the map fails here, which is the point — routing does
# not usually break because someone edits a route, it breaks because someone
# adds an alert and never decides where it goes.
set -euo pipefail

cd "$(dirname "$0")/.."

CONFIG=deploy/monitoring/alertmanager/alertmanager.yml

# alertname -> receiver. Severity is read from the rendered rules, not assumed,
# so an alert that changes severity and thereby changes route fails here too.
expected_receiver() {
  case "$1" in
    InventarioBackupStale)     echo backups ;;
    InventarioBackupNeverRan)  echo backups ;;
    InventarioTargetDown)      echo critical ;;
    InventarioHighErrorRate)   echo default ;;
    InventarioHighLatencyP95)  echo default ;;
    *)                         echo "__unmapped__" ;;
  esac
}

TMP=.tmp-alert-routing
CID=""
trap 'rm -rf "$TMP"; [ -n "$CID" ] && docker rm -f "$CID" > /dev/null 2>&1' EXIT

mkdir -p "$TMP"

if command -v amtool > /dev/null 2>&1; then
  amtool_() { amtool "$@"; }
else
  # The image ships amtool. The config is copied in rather than bind-mounted so
  # this also works against a remote Docker daemon.
  CID=$(docker run -d --entrypoint sleep prom/alertmanager:v0.28.1 600)
  docker cp "$CONFIG" "$CID:/tmp/alertmanager.yml" > /dev/null
  CONFIG=/tmp/alertmanager.yml
  amtool_() { docker exec "$CID" amtool "$@"; }
fi

amtool_ check-config "$CONFIG"

helm template inv helm/inventario \
  --set-string secrets.dbDsn='postgres://user:pass@pg-host:5432/inventario?sslmode=require' \
  --set metrics.prometheusRule.enabled=true \
  --set backup.enabled=true \
  --api-versions monitoring.coreos.com/v1 \
  | ruby -ryaml -e '
      docs = YAML.load_stream(STDIN.read).compact
      rule = docs.find { |d| d["kind"] == "PrometheusRule" }
      abort("the chart rendered no PrometheusRule") unless rule
      rule["spec"]["groups"].each do |g|
        g["rules"].to_a.each do |r|
          next unless r["alert"]
          puts "#{r["alert"]} #{(r["labels"] || {}).fetch("severity", "none")}"
        end
      end
    ' > "$TMP/alerts.txt"

[ -s "$TMP/alerts.txt" ] || { echo "the chart rendered no alerts" >&2; exit 1; }

failed=0
while read -r alert severity; do
  want=$(expected_receiver "$alert")
  if [ "$want" = "__unmapped__" ]; then
    echo "FAIL $alert is not in the expected map; decide where it pages and add it" >&2
    failed=1
    continue
  fi
  got=$(amtool_ config routes test --config.file="$CONFIG" \
    alertname="$alert" severity="$severity" | tr -d '[:space:]')
  if [ "$got" != "$want" ]; then
    echo "FAIL $alert (severity=$severity) routed to '$got', expected '$want'" >&2
    failed=1
  else
    printf 'ok   %-26s severity=%-8s -> %s\n' "$alert" "$severity" "$got"
  fi
done < "$TMP/alerts.txt"

exit "$failed"
