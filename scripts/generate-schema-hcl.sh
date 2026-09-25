#!/bin/bash
# generate-schema-hcl.sh — Render the schema the Go annotations declare into a
# single readable, diffable HCL file.
#
# Usage:
#   ./scripts/generate-schema-hcl.sh
#
# Prerequisites: go
#
# The Go annotations under go/models/ stay the source of truth. This artifact
# is derived from them, so it is never edited by hand — the drift check in
# .github/workflows/schema-artifact.yml regenerates it and fails on a diff.
#
# Why it exists: 541 column declarations spread across 46 model files and
# interleaved with Go struct fields cannot be read as a schema, and diffing the
# schema between two commits means diffing 46 files of mixed Go and
# annotations. One file gives both (#2420).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
GO_DIR="${REPO_ROOT}/go"
MODELS_DIR="./models"
OUT_FILE="${GO_DIR}/schema/schema.hcl"

# The CLI version comes from go.mod so the export always matches the Ptah the
# library code is built against, and `go run pkg@version` resolves outside the
# main module, so this adds nothing to go.mod/go.sum. The HCL renderer lives in
# an internal package upstream, which is why this goes through the CLI rather
# than a wrapper in inventool.
PTAH_VERSION="$(cd "${GO_DIR}" && go list -m -f '{{.Version}}' ptah.run)"

echo "Exporting the schema with ptah ${PTAH_VERSION}..."

# The six warnings about opaque SQL function bodies are expected: the plpgsql
# bodies of the RLS helpers are carried as raw strings by the annotations too,
# so the export is as structured as the source is.
(cd "${GO_DIR}" && go run "ptah.run/cmd/ptah@${PTAH_VERSION}" \
  schema export --to hcl --root-dir "${MODELS_DIR}" --out "${OUT_FILE}")

echo ""
echo "Wrote ${OUT_FILE}"
