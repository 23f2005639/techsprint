#!/usr/bin/env bash
source "$(dirname "$0")/env.sh"
cd "$ROOT"
if [[ "${1:-}" == stop ]]; then
 if [[ -f .runtime/api.pid ]]; then kill "$(cat .runtime/api.pid)" 2>/dev/null || true; fi
 if [[ -f .runtime/worker.pid ]]; then kill "$(cat .runtime/worker.pid)" 2>/dev/null || true; fi
 redis-cli -h 127.0.0.1 -p 56379 shutdown || true
 pg_ctl -D "$ROOT/.runtime/pgdata" stop -m fast || true
 exit 0
fi
if [[ ! -f .runtime/pgdata/PG_VERSION ]]; then initdb -D "$ROOT/.runtime/pgdata" -L "$ROOT/.runtime/pg/usr/share/postgresql" --auth=trust --no-locale -E UTF8; fi
if ! pg_ctl -D "$ROOT/.runtime/pgdata" status >/dev/null; then
 pg_ctl -D "$ROOT/.runtime/pgdata" -l "$ROOT/.runtime/postgres.log" -o "-h 127.0.0.1 -p 55432 -k $ROOT/.runtime -c dynamic_library_path=$ROOT/.runtime/pg/usr/lib/postgresql" start
fi
if ! psql "postgres://$(id -un)@127.0.0.1:55432/postgres?sslmode=disable" -tAc "SELECT 1 FROM pg_database WHERE datname='dressing'" | rg -q 1; then
 createdb -h 127.0.0.1 -p 55432 dressing
fi
if ! redis-cli -h 127.0.0.1 -p 56379 ping >/dev/null 2>&1; then
 mkdir -p .runtime/redis-data
 redis-server --bind 127.0.0.1 --port 56379 --dir "$ROOT/.runtime/redis-data" --appendonly yes --daemonize yes --pidfile "$ROOT/.runtime/redis.pid" --logfile "$ROOT/.runtime/redis.log"
fi
