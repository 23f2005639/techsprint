# BIZZNOVATE · The Dressing Room Live

A local React + TypeScript storefront, separate staff console and full-page benchmark dashboard backed by Go `net/http`, PostgreSQL/pgxpool, Redis Lua and Go/Asynq. Existing `../dressing-room/` presentation and `../dressing-room-offline.zip` are preserved.

## Start

From this directory on the supplied x86_64 Omarchy/Arch machine:

```bash
make setup
make demo
```

Open these independently:

- http://127.0.0.1:8080 — storefront, interest, receipts, notifications and Story PNGs.
- http://127.0.0.1:8080/staff — scanned physical inventory, fitting, checkout, returns, placement, launch and worker controls.
- http://127.0.0.1:8080/benchmarks — real HTTP generator controls, SSE snapshots, latency, outcomes, invariants, service pulse and JSON/CSV export.

In another terminal:

```bash
make bench SCENARIO=race REQUESTS=100 CONCURRENCY=100
make bench SCENARIO=five REQUESTS=100 CONCURRENCY=100
make bench SCENARIO=floor REQUESTS=100 CONCURRENCY=100
make bench SCENARIO=duplicate REQUESTS=100 CONCURRENCY=100
make test
bash scripts/benchmarks.sh
python3 scripts/smoke.py
```

`make dev` uses normal timing. `make demo` uses explicitly labelled accelerated timing. Ctrl+C stops the API and worker; PostgreSQL and Redis keep their durable data. `make stop` stops all four processes. Logs and PIDs live in `.runtime/`. The demo starts in pre-drop mode on a fresh database; launch reservations from `/staff`. Benchmark creation launches reservations and creates a separate `BENCH-<run>` variant so it does not consume storefront stock.

## Runtime setup

No Docker, root service, cloud account or preinstalled Go is needed. Setup downloads and caches Go **1.26.1**, PostgreSQL **18.6-4** (Omarchy package) and Redis **7.4.2** source; checks pinned SHA-256 values, builds Redis locally, installs exact frontend dependencies from `package-lock.json`, and builds three Go executables. `go.sum` locks Go modules. Product images are already bundled and need no network at runtime.

Prerequisites on this host: Node/npm, Bash, make, GCC, Python 3, curl, tar/zstd, ripgrep and Arch PostgreSQL shared-library dependencies (`icu`, `liburing`, `numactl`, OpenSSL, LDAP, krb5, libxml2, zstd, lz4, systemd-libs). Setup checks the PostgreSQL executable before continuing. This runtime bootstrap targets x86_64 Linux with these host libraries; it is not a portable binary bundle for other distributions. To use an existing PostgreSQL/Redis instead, set the environment variables below and build with Go 1.26.1; `make services`/`make test` intentionally manage the supplied local runtimes.

All services bind to loopback: API **8080**, PostgreSQL **55432**, Redis **56379**. PostgreSQL uses local trust authentication on its loopback demo cluster; use a dedicated database with authenticated connections for any shared environment. This is a local demonstration with fictional customer aliases, not a multi-tenant deployment. Customer aliases select the local inbox; random receipt IDs act as capabilities. There is no production identity provider or payment processor.

| Variable | Default |
| --- | --- |
| `DATABASE_URL` | `postgres://<current-user>@127.0.0.1:55432/dressing?sslmode=disable` |
| `REDIS_ADDR` | `127.0.0.1:56379` |
| `REDIS_DB` | `0` (tests use `1`) |
| `STAFF_KEY` | `local-demo-staff` |
| `DEMO_MODE` | `0`; `make demo` sets `1` |

Use `STAFF_KEY='your-local-key' make demo` to change the privileged API credential, then enter the same key under **Local staff / demo credential** in the staff and benchmark views. The browser stores this key only for the session. The CLI reads `STAFF_KEY`. The local default is deliberately convenient for judging.

| Timing | Normal | Accelerated judging |
| --- | --- | --- |
| Online hold | 24 hours | 120 seconds |
| Reminder before expiry | 2 hours | 10 seconds |
| FIFO offer | 5 minutes | 20 seconds |
| Fitting overdue alert | 10 minutes | 30 seconds |

## Inventory contract

Each of the eight illustrative streetwear products has S/M/L/XL variants and six individual scan-coded units per size: five published stockroom units and one unpublished display unit. Prices are illustrative INR. Photos and source links are in `web/public/assets/credits.txt`.

Online allocation counts *all available physical units* in that variant, selects only published stockroom units, and requires at least two available units after the entire quantity is allocated. At the floor the storefront says **Available in-store only**. The floor protects walk-ins from online allocations; staff fitting/purchase can legitimately consume protected units. It never cancels an existing online hold.

All stock changes take one PostgreSQL transaction advisory lock, then lock/select the unit or hold, validate its state, change stock, persist the hold, save the idempotent response and insert the event/outbox records before commit. This conservative global lock makes this local demonstration easy to audit across API and worker processes. It serializes business mutations; throughput is measured honestly and is not a production scalability claim. Go goroutines issue concurrent requests; PostgreSQL enforces correctness.

Staff must scan an available unit to start fitting. Display units are eligible for fitting but never for online allocation. Staff must confirm placement into the stockroom and explicitly publish an available unit before it can enter the online pool. Overdue fitting generates an alert without releasing stock. Checkout updates the existing active fitting or online hold. Physical return releases a fitting unit exactly once. Expired online holds cannot be checked out. Pickup and expiry are serialized and produce one terminal state.

Idempotency keys are mandatory for POST business operations. Reusing the same key and identical body/path replays the saved response; a changed request gets 409. The Redis Lua admission gate allows 200 unauthenticated business requests per second per IP. Redis failure is surfaced in `X-Rate-Limit-Status` and does not bypass PostgreSQL eligibility rules. Staff-authenticated benchmark traffic bypasses the admission gate to isolate stock contention; zero rate-limit outcomes are real observations, not an omitted counter.

FIFO waiters are processed by increasing stored queue ID. Offers allocate one eligible unit while preserving the floor, so an offer has actual stock protection. A new ordinary online request cannot leapfrog an existing waiter. An offer is claimed using its unguessable hold ID. Unclaimed offers expire; queue progression requires capacity above the floor. Offer expiry and ordinary expiry use the same guarded state transition.

## Durable background work

The API only inserts PostgreSQL outbox rows; it does not need Redis to commit a hold. The worker dispatches rows with deterministic Asynq task IDs and marks them sent only after enqueue succeeds (including a known identical task ID). Asynq provides scheduled execution and retries. Local Redis uses append-only persistence.

The separate Go worker runs a one-second authoritative recovery sweep and an Asynq periodic scheduler every five seconds. Every reminder, offer delivery, deadline and release rechecks stored hold state. Notifications are a local inbox, deduplicated by `(hold,kind)`. Collected/expired holds suppress stale reminders. Fitting is never automatically returned. Staff pause suspends sweeps and notification delivery; dispatch can continue. Resume or actual worker restart recovers overdue online releases and does not double-release stock. Outbox events remain pending during Redis failure.

## Benchmark interpretation

`cmd/load` generates real HTTP requests using bounded goroutines and one run ID. Targets are restricted to loopback HTTP, redirects are refused and socket dialing is pinned to loopback. The same generator powers the dashboard launch button. Four automated load scenarios are supplied; the wider 14-scenario acceptance matrix is in [REQUIREMENTS.md](REQUIREMENTS.md) and [JUDGING.md](JUDGING.md).

Client-observed latency includes response body consumption. The generator sends actual status/latency batches every 200ms and stores a final snapshot in PostgreSQL. p50/p95/p99 use PostgreSQL continuous percentiles; the CLI's client summary uses nearest sampled ranks, so small numerical differences are expected. Final average RPS is observed completed requests divided by run wall time. Live RPS uses server arrival time of measured sample batches and is labelled as such. No benchmark values are synthesized.

`/api/benchmarks/runs/{id}/events` emits a snapshot twice per second. The React dashboard reconnects, fetches a current snapshot after disconnect, and keeps at most 60 latency points and 50 application events. Final snapshots and raw request samples remain in PostgreSQL; the dashboard offers JSON snapshots and request CSV. `make bench` additionally saves `results/latest.json`. Events are the latest application events across the local instance; completed run snapshots freeze the final observations. Inventory totals include all physical variants, including isolated benchmark fixtures; protected stock is a subset of available and must not be added to it.

For duplicate scenarios, successful *HTTP responses* can be 100 while only one unique hold exists. Stock correctness checks count unique active unit ownership, compare physical counts to each variant's initial inventory ledger, match active holds to unit state, and check every online allocation audit left two units. These are independent of Go's memory race detector. A long run may outlive 120-second demo holds; use short judging runs when expecting a fixed unique-winner count.

## API map

All requests/responses are JSON except the benchmark SSE/CSV routes. All POSTs require `Idempotency-Key`. Privileged routes require `X-Staff-Key`.

| API | Purpose |
| --- | --- |
| `GET /api/health`, `/api/catalog`, `/api/demand`, `/api/social` | Health, stock eligibility, interest, synthetic signal summaries |
| `POST /api/interest`, `/api/reservations`, `/api/waitlist` | `{customer,variant,quantity}` (waitlist needs no quantity) |
| `GET /api/holds?customer=...`, `/api/notifications?customer=...` | Local customer state and delivered inbox |
| `GET /api/receipts/{hold}` | Digital receipt and illustrative pickup directions |
| `POST /api/claim` | `{hold}` FIFO offer capability |
| `GET /api/inventory`, `/api/invariants` | Protected staff inventory and authoritative checks |
| `POST /api/staff/fitting` | `{code,customer}` |
| `POST /api/staff/checkout`, `/api/staff/return` | `{hold}` |
| `POST /api/staff/transfer` | `{code,location,online}` confirms physical placement |
| `POST /api/staff/settings` | `{launched,paused,draft}` |
| `POST /api/social/scrape` | Scrape the bundled local synthetic site; no arbitrary network target |
| `POST /api/social/import` | `{posts:[{id,caption,period,reach,engagement}]}`; baseline/current |
| `POST /api/benchmarks/start` | Start real generator `{scenario,requests,concurrency}` |
| `GET/POST /api/benchmarks/runs` | Run history / create isolated run fixture |
| `GET /api/benchmarks/runs/{id}` | Current or persisted final snapshot |
| `GET /api/benchmarks/runs/{id}/events`, `/export.csv` | SSE / raw samples CSV |
| `POST /api/benchmarks/runs/{id}/samples`, `/finish` | Protected generator reporting |

400 validation, 401 staff credential, 409 stock/channel/state/idempotency conflicts, 429 admission throttling; unexpected errors are distinct and never represented as stock conflicts.

## Verification and delivery

`make test` creates/uses the separate `dressing_test` PostgreSQL database and Redis DB 1. It runs real concurrent database integration tests, real HTTP benchmark scenarios, actual Asynq stop/restart recovery, `go test -race`, React Testing Library/Vitest interactions and the TypeScript/Vite production build. Tests require the local services and fail on DB startup errors; directly invoking `go test` without `TEST_DATABASE_URL` skips database integration tests. Read [VERIFICATION.md](VERIFICATION.md) for executed checks and measured results.

Included: Go source and locks, React/TypeScript source and lock, SQL schema/seed, bundled photographs, synthetic import fixture, pinned setup scripts, benchmark CLI, tests, checklist and judging guide. Runtime caches, compiled binaries, dependency directories and generated results are ignored by Git. No source file or runtime download modifies the existing presentation or ZIP.

Synthetic social-site scraping parses the bundled `internal/app/social-site.html` XHTML fixture (view it at `/api/social/site`), extracting captions, comments and hashtags and deduplicating by post ID. It uses code/category keyword matching, baseline/spike/noise analysis and clearly labelled fixture reach/engagement. External Instagram, SMS/push services, BERT training, live payment collection and production auth are outside this local build. Pickup directions are explicitly illustrative because no real store address was supplied.
