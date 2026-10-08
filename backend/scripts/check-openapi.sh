#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
snapshot=$(mktemp -d)
trap 'rm -rf "$snapshot"' EXIT
sh scripts/generate-openapi.sh "$snapshot"
diff -u api/openapi/swagger.json "$snapshot/swagger.json"
diff -u api/openapi/swagger.yaml "$snapshot/swagger.yaml"
