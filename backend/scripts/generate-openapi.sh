#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
go tool swag init --generalInfo main.go --dir cmd/api,internal --parseInternal --output "${1:-api/openapi}" --outputTypes json,yaml
