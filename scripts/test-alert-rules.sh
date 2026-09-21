#!/bin/bash
# test-alert-rules.sh — Run promtool's unit tests against the alert rules the
# chart actually renders.
#
# Usage:
#   ./scripts/test-alert-rules.sh
#
# Prerequisites: helm, ruby, and either promtool on PATH or docker.
#
# The rules are rendered from the chart rather than read from a checked-in
# copy, so the test covers what ships. helm-lint already checks that the chart
# and the dev compose stack carry the same alert *set*; this checks that the
# expressions fire.
set -euo pipefail

cd "$(dirname "$0")/.."

TMP=.tmp-alert-rules
mkdir -p "$TMP"
trap 'rm -rf "$TMP"' EXIT

# Backups are opt-in, so their alerts only render with backup.enabled.
helm template inv helm/inventario \
  --set-string secrets.dbDsn='postgres://user:pass@pg-host:5432/inventario?sslmode=require' \
  --set metrics.prometheusRule.enabled=true \
  --set backup.enabled=true \
  --api-versions monitoring.coreos.com/v1 \
  | ruby -ryaml -e '
      docs = YAML.load_stream(STDIN.read).compact
      rule = docs.find { |d| d["kind"] == "PrometheusRule" }
      abort("the chart rendered no PrometheusRule") unless rule
      File.write(ARGV[0], { "groups" => rule["spec"]["groups"] }.to_yaml)
    ' "$TMP/rendered.rules.yml"

TESTS=deploy/monitoring/prometheus/tests

if command -v promtool > /dev/null 2>&1; then
  promtool check rules "$TMP/rendered.rules.yml"
  promtool test rules "$TESTS"/*.test.yml
else
  # The image ships promtool; mounting the repo keeps the relative rule_files
  # paths in the test files working either way.
  docker run --rm -v "$PWD:/work" -w /work --entrypoint promtool \
    prom/prometheus:v3.7.3 check rules "$TMP/rendered.rules.yml"
  docker run --rm -v "$PWD:/work" -w /work --entrypoint promtool \
    prom/prometheus:v3.7.3 test rules "$TESTS"/backup-alerts.test.yml
fi
