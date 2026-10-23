#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export PATH="$ROOT/.runtime/go/bin:$ROOT/.runtime/pg/usr/bin:$ROOT/.runtime/redis-7.4.2/src:$PATH"
export GOPATH="$ROOT/.runtime/gopath"
export GOCACHE="$ROOT/.runtime/gocache"
export DATABASE_URL="${DATABASE_URL:-postgres://$(id -un)@127.0.0.1:55432/dressing?sslmode=disable}"
export REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:56379}"
export STAFF_KEY="${STAFF_KEY:-local-demo-staff}"
