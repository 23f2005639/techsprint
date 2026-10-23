THE DRESSING ROOM: SYSTEM ARCHITECTURE

THE PROBLEM
Online customers and walk-in shoppers compete for the same limited physical units. Two requests can arrive together, customers can repeat clicks, and a background job can be late. Staff may also have a unit in a fitting room. We need an official stock record, clear physical ownership and recovery after interruptions.

1. REACT + TYPESCRIPT: THE THREE SCREENS
React provides the storefront, the staff console at /staff and the separate benchmark dashboard at /benchmarks. TypeScript helps the interface handle data consistently. The storefront captures pre-drop product, size and quantity interest. After launch it creates reservations, shows digital receipts and countdowns, and displays the local notification inbox. Staff records physical scans, fittings, checkout and returns. The benchmark view shows actual observed requests and stock checks. The Go server serves the built frontend and local photographs on one loopback origin, http://127.0.0.1:8080.

2. GO net/http: THE API AND BUSINESS RULES
The API receives JSON requests. Business mutations require an Idempotency-Key. PostgreSQL stores the request fingerprint and response. A retry with the same key and request returns the original result. A changed body with the same key gets a conflict. Staff and benchmark controls require a staff key. For customer traffic, go-redis calls a Redis Lua counter to apply the request limit. Goroutines handle requests concurrently. The database determines the order of stock changes.

3. POSTGRESQL + pgxpool: THE OFFICIAL INVENTORY
Every physical unit has a scan code, product-size variant, location, publication flag and state. The API uses pgxpool for database connections. This local build takes a conservative PostgreSQL transaction advisory lock for stock-changing paths. It then validates eligibility, creates a hold, assigns the selected units, saves the idempotent response and writes the event and outbox entries in the same transaction. Confirmation follows commit. This design deliberately favors auditability over high write throughput.

Online allocation selects published stockroom units and must leave two available physical units in the same size. With five available units, at most three can be online-held. Display stock enters online allocation only after staff confirms stockroom placement and explicitly publishes it. Existing confirmed online holds remain protected. Walk-in fitting and purchase can consume the units protected from online allocation.

4. STAFF FITTING AND CHECKOUT
Staff scans an available physical unit before fitting. The unit becomes fitting-held. Checkout completes this existing hold and marks the same unit sold. It does not allocate another unit. When a fitting becomes overdue, the worker creates an alert but keeps the unit protected. Staff must confirm purchase or physical return. Expired online reservations cannot be checked out. Pickup and expiry use the same transaction lock and state validation so one final outcome wins.

5. REDIS + LUA AND GO-REDIS
Redis has two roles: request counting through Lua and storage for the Asynq job queue. It does not decide inventory eligibility. The local Redis process uses append-only persistence. A Redis outage leaves PostgreSQL stock rules active. New jobs remain in the PostgreSQL outbox until the queue can accept them again. The staff-authenticated benchmark bypasses the customer admission limit to isolate inventory contention.

6. POSTGRESQL OUTBOX, GO WORKER AND ASYNQ
The outbox is a durable list of work saved with each reservation. The separate Go worker dispatches outbox entries to Asynq with deterministic task IDs. It marks an entry sent after successful enqueue or recognition of the same existing task. Asynq schedules jobs and retries failures. The worker runs a one-second recovery sweep, with a periodic Asynq sweep every five seconds. Each action rechecks stored hold state before delivering a reminder or releasing stock. Repeated expiry work releases stock once.

Eligible released stock is offered to FIFO waiters in stored queue order. An offer protects a real unit while leaving the online floor intact. The customer must claim it within its window. When the floor blocks allocation, no online offer is created. Notification delivery is a deduplicated local inbox. Collected or expired holds suppress stale reminders.

7. GO HTTP LOAD GENERATOR AND LIVE SSE DASHBOARD
The generator sends real HTTP requests with bounded goroutine concurrency and one shared run ID. Its target is restricted to loopback HTTP. It measures response status and client-observed latency, reports measured samples and verifies final stock invariants through a protected API. PostgreSQL stores raw samples and final results. The API sends Server-Sent Events snapshots twice per second. The dashboard reconnects and fetches a current snapshot after disconnection. JSON and CSV downloads let judges inspect the evidence.

The supplied short runs use 100 requests and concurrency 100. The one-online-unit scenario produces one unique hold and 99 expected conflicts. Five available units produce three unique holds. Two available units block online holds. Repeating one identical request produces 100 successful HTTP responses for one unique hold and receipt. The dashboard checks duplicate allocation, the protected allocation floor and physical inventory conservation. Go's race detector separately checks shared-memory races. These local measurements do not establish production capacity.

OTHER FEATURES
The bundled synthetic social page supplies captions, comments and hashtags. A keyword matcher links posts to products and compares current mentions with the baseline. Fixture IDs deduplicate imports. Reach and engagement are explicitly synthetic values. Browser Canvas generates personalised reserved and collected Story PNGs. Store directions are illustrative. External Instagram access, BERT training, actual payments and external SMS/push delivery are outside this local build.

TIMING AND JUDGING
Normal online holds last 24 hours with reminders two hours before expiry, five-minute queue offers and ten-minute fitting alerts. Accelerated judging uses 120-second holds, reminders ten seconds before expiry, twenty-second offers and thirty-second fitting alerts. The UI labels accelerated mode. Present the customer reservation first, show its scan code in staff operations, complete the existing hold, then run the benchmark scenarios.

90-SECOND HINGLISH SPEAKING SCRIPT
Judges, problem ye hai ki online customer aur walk-in customer same physical stock ke liye compete karte hain. Duplicate click, simultaneous request ya missed expiry se wrong booking ho sakti hai. Left se architecture follow kijiye. React aur TypeScript hamare customer, staff aur benchmark screens banate hain. Go API request validate karti hai, aur request key repeat booking ko rokne ke liye saved result replay karti hai. PostgreSQL official stock register hai. Ek transaction mein hold, physical unit, request result aur outbox save hote hain. Five available units mein maximum three online hold ho sakte hain, kyunki two walk-in ke liye protected hain. Redis Lua request rush count karta hai, aur Asynq jobs Redis mein rakhta hai. Separate Go worker reminder, expiry aur FIFO offers process karta hai. Har action latest database state dobara check karta hai. Fitting overdue hone par item automatic release nahi hota, staff physical return confirm karta hai. Neeche Go load generator hundred real concurrent requests bhejta hai. Separate dashboard actual latency, outcomes aur three stock checks dikhata hai. Is tarah customer promise, physical inventory aur measured proof ek flow mein connected hain.

IMPLEMENTATION SOURCES
../dressing-room-live/internal/app/app.go
../dressing-room-live/internal/app/http.go
../dressing-room-live/internal/app/inventory.go
../dressing-room-live/internal/app/worker.go
../dressing-room-live/internal/load/load.go
../dressing-room-live/web/src/App.tsx
../dressing-room-live/verification/benchmark-summary.json
BIZZNOVATE logo supplied by the user. All architecture claims describe the inspected local implementation.
