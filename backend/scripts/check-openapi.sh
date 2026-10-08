#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
snapshot=$(mktemp -d)
trap 'rm -rf "$snapshot"' EXIT
sh scripts/generate-openapi.sh "$snapshot"
diff -ru api/openapi "$snapshot"
