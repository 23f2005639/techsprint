#!/usr/bin/env bash
source "$(dirname "$0")/env.sh"
cd "$ROOT"
bash scripts/services.sh
make build
bin/worker > .runtime/worker.log 2>&1 &
worker_pid=$!
echo "$worker_pid" > .runtime/worker.pid
bin/api &
api_pid=$!
echo "$api_pid" > .runtime/api.pid
trap 'kill "$api_pid" "$worker_pid" 2>/dev/null || true' EXIT INT TERM
wait "$api_pid"
