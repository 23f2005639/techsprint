#!/usr/bin/env bash
source "$(dirname "$0")/env.sh"
cd "$ROOT"
# A separate database keeps tests away from the running application's inventory.
if ! psql "postgres://$(id -un)@127.0.0.1:55432/postgres?sslmode=disable" -tAc "SELECT 1 FROM pg_database WHERE datname='dressing_test'" | rg -q 1; then createdb -h 127.0.0.1 -p 55432 dressing_test; fi
export TEST_DATABASE_URL="postgres://$(id -un)@127.0.0.1:55432/dressing_test?sslmode=disable"
export REDIS_DB=1
go test -count=1 ./...
go test -race -count=1 ./...
(cd web && npm test && npm run build)
