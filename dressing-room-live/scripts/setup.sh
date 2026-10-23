#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p .runtime/pg
fetch(){ if [[ ! -f "$2" ]]; then curl --fail --location --retry 3 "$1" -o "$2.tmp"; mv "$2.tmp" "$2"; fi; }
check(){ echo "$1  $2" | sha256sum --check --status || { echo "Checksum mismatch: $2" >&2; exit 1; }; }
# x86_64 Linux, no root. PostgreSQL package uses host Arch/Omarchy libraries.
[[ "$(uname -sm)" == 'Linux x86_64' ]] || { echo 'This setup targets x86_64 Linux; see README for external runtime configuration.'; exit 1; }
fetch https://go.dev/dl/go1.26.1.linux-amd64.tar.gz .runtime/go.tar.gz
check 031f088e5d955bab8657ede27ad4e3bc5b7c1ba281f05f245bcc304f327c987a .runtime/go.tar.gz
[[ -x .runtime/go/bin/go ]] || tar -xzf .runtime/go.tar.gz -C .runtime
fetch https://stable-mirror.omarchy.org/extra/os/x86_64/postgresql-18.6-4-x86_64.pkg.tar.zst .runtime/postgresql.pkg.tar.zst
check 005afa838b458df780a9df689d10674decb6e03aa8005af7b703299a50d17f8e .runtime/postgresql.pkg.tar.zst
[[ -x .runtime/pg/usr/bin/postgres ]] || tar -xf .runtime/postgresql.pkg.tar.zst -C .runtime/pg
if ! .runtime/pg/usr/bin/postgres --version; then echo 'Missing host PostgreSQL libraries. Install the documented Arch dependencies or supply a PostgreSQL runtime; see README.'; exit 1; fi
fetch https://download.redis.io/releases/redis-7.4.2.tar.gz .runtime/redis.tar.gz
check 4ddebbf09061cbb589011786febdb34f29767dd7f89dbe712d2b68e808af6a1f .runtime/redis.tar.gz
[[ -d .runtime/redis-7.4.2 ]] || tar -xzf .runtime/redis.tar.gz -C .runtime
[[ -x .runtime/redis-7.4.2/src/redis-server ]] || make -C .runtime/redis-7.4.2 -j4 MALLOC=libc REDIS_CFLAGS= REDIS_LDFLAGS=
source scripts/env.sh
go mod download
(cd web && npm ci --no-audit --no-fund)
make build
printf 'Setup complete. Run make demo (accelerated) or make dev (24-hour holds).\n'
