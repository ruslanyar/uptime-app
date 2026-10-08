#!/bin/sh
# Run from backend/. The committed generated package must already be up to date.
set -eu
version=$(cat .sqlc-version)
if [ "$(sqlc version)" != "v$version" ]; then
  echo "sqlc v$version is required" >&2
  exit 1
fi
snapshot=$(mktemp -d)
trap 'rm -rf "$snapshot"' EXIT
cp -R internal/storage/postgres/sqlc "$snapshot/before"
sqlc compile
sqlc generate
diff -ru "$snapshot/before" internal/storage/postgres/sqlc
# The second pass catches generators that change their own output on rerun.
sqlc generate
diff -ru "$snapshot/before" internal/storage/postgres/sqlc
