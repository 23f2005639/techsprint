# Executed verification · 23 October 2026

Story: a shopper captures demand or reserves a real unit through React → Go HTTP → PostgreSQL, staff scans and completes the same unit's hold, a separate Go worker recovers deadlines and delivers local inbox events, and real concurrent HTTP traffic drives the separate benchmark dashboard.

## Results

| Boundary | Evidence |
| --- | --- |
| Setup | `bash scripts/setup.sh` passed using pinned runtime checksums, `go.sum`, `npm ci` and production builds |
| API/worker startup | Real user-owned PostgreSQL 18.6, Redis 7.4.2, Go API and Asynq worker launched successfully on loopback |
| Concurrent database operations | Full integration suite passed: allocation floor, one winner, fitting/online orderings, physical returns, deduplication, FIFO/expiry, pickup, reminders and rollback |
| Asynq lifecycle | Actual server start/stop/restart test recovered an overdue hold; pause/resume and failed-Redis outbox retention also passed |
| Memory races | `go test -race -count=1 ./...` passed separately from stock invariants |
| React interactions | 9 Vitest/Testing Library tests passed: interest size/quantity, reservation, scan→existing-hold checkout, benchmark empty/start, live SSE snapshot, reserved and collected PNGs |
| Build | TypeScript typecheck and Vite production build passed; Go API, worker and load executables built |
| HTTP assets/routes | `python3 scripts/smoke.py` passed storefront, staff and benchmark shells, 8 local JPEGs, catalog, health and synthetic social site |
| Streaming/export | Live SSE response, second connection, persisted final result and CSV row count passed HTTP smoke checks |
| Browser visual QA | Blocked: the computer-use tool reported no available apps or browsers. No visual layout or browser-console check is claimed. React DOM interaction tests are not visual rendering tests. |

The startup check exposed concurrent schema initialization before the API could start; schema migration now takes a PostgreSQL transaction advisory lock. Interaction testing exposed number-input clearing selecting 12 instead of 2; the input now preserves its temporary edit value. Finalization replay testing verifies saved benchmark responses remain identical on retries. All fixes are in the delivered source.

## Standalone live load runs

`bash scripts/benchmarks.sh` executed four real HTTP scenarios with **100 requests / 100 goroutines each** against the running final build. These are observations on this machine, not performance guarantees. Each run independently called the protected authoritative invariant API after finishing. Every run passed duplicate-allocation, protected-floor and physical-inventory conservation checks.

| Scenario | Unique holds | Successful HTTP | Expected 409 conflicts | Unexpected failures | p95 latency | Observed RPS |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| race | 1 | 1 | 99 | 0 | 354.75ms | 259.8 |
| five | 3 | 3 | 97 | 0 | 297.26ms | 302.7 |
| floor | 0 | 0 | 100 | 0 | 298.07ms | 305.9 |
| duplicate | 1 | 100 | 0 | 0 | 22.77ms | 3718.9 |

Zero requests were rate-limited in these staff-authenticated runs. The duplicate run replays stored results, explaining its different throughput. Raw live results are in ignored `results/{scenario}.json`; a compact reproducible record with run IDs and all percentiles is delivered in `verification/benchmark-summary.json`. PostgreSQL also retains final snapshots and raw request samples for dashboard downloads.

## Practical limits

The inventory transaction lock serializes writes across API/worker processes and favors auditability over production throughput. Benchmark sample timing and final results are real; live RPS is based on arrival of client report batches. Initial inventory and hold-state checks are independent of Go memory-race checks. Instance totals include retained benchmark variants.

The synthetic scraper only reads the bundled XHTML page. Caption/comment/hashtag matching is keyword-based. Notification delivery is a local inbox, directions are illustrative, and no external social, messaging or payment integration is claimed. Setup targets this supplied x86_64 Omarchy host and its shared libraries. The original presentation and original offline ZIP were not edited.

## Storefront follow-up

After a reported Reserve-button issue, verified the running API had reservations open and online stock available. A live unauthenticated POST with the browser Origin returned 201 and a digital receipt identifying one physical unit. Missing-name feedback previously appeared above the collection, where it was easy to miss. The updated storefront shows feedback next to the clicked product, keeps the global notice visible, focuses the required name field, and scrolls successful reservations into view. Mutation keys use crypto.getRandomValues, supporting local preview contexts without randomUUID. Two regression interaction tests cover missing-name feedback/focus and successful receipt rendering without randomUUID; all nine frontend tests and the TypeScript/Vite build passed.
