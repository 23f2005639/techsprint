# Requirement and acceptance checklist

| Requirement / scenario | Implementation | Verification |
| --- | --- | --- |
| 1. Catalog, interest, normal reservation | Eight product photos, sizes, INR pricing, demand table, live launch controls | Storefront interaction tests, API integration tests |
| 2. Five units → at most three online holds | Authoritative available count; allocation audit | 100 concurrent DB requests; `five` HTTP scenario |
| 3. One online-eligible hoodie → one winner | Three physical units, two-unit floor; same live reservation API | 100 concurrent DB requests; `race` HTTP scenario |
| 4. Two units block online, allow walk-in | Online 409; staff scanned fitting/checkout | `floor` HTTP scenario and walk-in checkout test |
| 5. Displayed fitting/checkout with online load | Location/online flags; scanned unit; existing hold checkout | `TestFittingCheckoutAndRaceOrders` |
| 6. Online wins before scan | Held unit rejects fitting; reverse ordering also checked | `TestFittingCheckoutAndRaceOrders` |
| 7. Overdue fitting / confirmed physical return | Alert without release; explicit guarded return | `TestOverdueFittingAndPhysicalReturn` |
| 8. Duplicate request → one hold/receipt | Fingerprinted stored idempotent results | 20 concurrent duplicate DB calls, 100-request `duplicate` HTTP scenario |
| 9. Expiry once / eligible FIFO claim | Guarded expiry, protected stock offer, claim deadline | `TestExpiryFIFOAndBelowFloor` |
| 10. Below-floor expiry creates no offer | Offer uses same online allocation rules | `TestExpiryFIFOAndBelowFloor` |
| 11. Pickup versus expiry → one final state | Shared transaction lock and stored state checks | `TestPickupVersusExpiryAndRecovery`, pickup-first/stale reminder check |
| 12. Worker pause/restart recovery | Outbox, Asynq retries/scheduler, periodic recovery sweep | Pause/resume test, actual `TestWorkerRestartRecoversDeadlines`, Redis failure retention test |
| 13. Social baseline/spike/noise/dedup | Bundled XHTML scraper plus JSON import; caption/comment/hashtag keyword matching; clearly labelled fixture metrics | `TestRemindersDirectionsAndSocial` |
| 14. Reminders, directions, Story PNGs | Editable draft; conditional inbox delivery; personalised reserved/collected 1080×1920 PNGs | Reminder/directions/stale-state DB tests; PNG interaction tests |
| Staff inventory / placement | Individual scan code, variant, location, state, publish confirmation | Authorization and transfer tests, staff fitting/checkout interaction |
| Durable mutations | PG transaction updates hold/units/request/outbox/event; rollback on failure | Concurrent integration and rollback test |
| Redis outage safety | SQL eligibility remains authoritative; failed enqueue retains outbox | Redis failure test |
| Independent benchmark route | `/benchmarks` full page; `/staff` separate | React empty/start/live SSE tests; HTTP route smoke checks |
| Genuine benchmark observations | Go HTTP generator, bounded goroutines, loopback target restriction | Four HTTP load scenarios; target/bounds unit tests |
| Live metrics and recovery | 500ms SSE, 60 points, 50 events, reconnect/current snapshot | SSE interaction test and live HTTP SSE smoke check |
| Persistence and export | PG run final JSON + raw sample CSV | Persisted-result integration test and HTTP export smoke check |
| Inventory invariants | Unique active unit ownership; initial inventory ledger; floor allocation audit | Assertions throughout DB suite; independent memory-race test pass |
| Setup and delivery | Pinned Go/PG/Redis cache, locks, local processes, source/assets/docs | `make setup`, `make build`, `make test`, live benchmark runs |
| Original presentation retained | Existing presentation directory and offline ZIP untouched | Workspace diff/file checks |

External BERT training, live Instagram access, external SMS/push and actual payments are explicitly outside scope. Local alias-based customer views and the shared staff demo key are deliberate judging defaults. Browser visual rendering was unavailable in the execution environment; DOM interaction tests and HTTP checks provide the executed frontend evidence.
