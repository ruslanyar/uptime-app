#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../../backend"
redocly=../node_modules/.bin/redocly
if [ ! -x "$redocly" ]; then
  echo "Install documentation tools with npm ci from the repository root." >&2
  exit 1
fi
"$redocly" build-docs api/openapi/swagger.json --output api/openapi/redoc.html --disableGoogleFont
