# The Dressing Room / both briefs

Seven slides and a seven-minute Hinglish speaking script, combining the original Problem 21 with the Crisis Adaptation Brief. BIZZNOVATE branding appears on every slide. The opening uses a real hoodie photograph.

Open `dist/index.html` directly in a browser. This is a buildless, local HTML/CSS/JavaScript presentation. It does not require a login, dependency install or network for offline use.

## Present

- Left/right arrows or Space change slides. Keys 1–7 jump directly.
- Named buttons run each demonstration. R resets that slide, N shows notes, F opens fullscreen.
- Slide 1: crisis and physical-store solution, with actual hoodie photography.
- Slide 2: captions/comments/hashtags, product-code matching, a sample Trending Drop label and personalised reserved/picked-up Story PNG.
- Slide 3: item, size and quantity interest form, with demand by variant before launch.
- Slide 4: simultaneous same-size requests, one confirmed hold, digital receipt and strict 24-hour countdown.
- Slide 5: two-hour reminder job, editable draft and directions, available/held/sold states, expiry release, next-customer notice and staff metrics.
- Slide 6: proposed technology responsibilities, five-hour build order and requirement-to-slide summary.
- Slide 7: both supplied briefs, three research PDFs and official implementation references.
- Print produces seven landscape slide pages. The script and Q&A are separate rehearsal handouts.

## Handouts

- `dist/presenter-notes.html`: slide-by-slide Hinglish script, demo cues, and timings totalling 420 seconds.
- `dist/presentation-script.md`: downloadable plain-text script.
- `dist/judges-qa.html` and `.md`: 24 likely questions with short English answers.
- `dist/requirements-map.html` and `.md`: detailed mapping of both briefs to the seven slides.

## Demo scope

All customer data is fictional and local to the current page session. Refresh resets it. The presentation simulates scheduled jobs and database transactions, generates a real sample PNG locally, and prepares editable reminder text. It does not submit actual reservations, send SMS/Push messages or connect to social accounts. The directions link is an example store search; a production deployment needs the verified store location.

The original brief explicitly requires a strict 24-hour hold. The crisis update adds demand capture and staff operations. The interface demonstrates a jump to 22 hours after booking for the reminder example; server deadlines remain the proposed production authority.

Waitlist conversion means unique interested customers ever granted a confirmed hold divided by all unique interested customers. It does not measure paid sales. Requested-size charts count units; uncollected holds count active held orders.

## Proposed implementation

React provides the customer form and staff screen. Node.js validates requests and IDs. PostgreSQL stores per-item/per-size stock, interest, holds, deadlines, request results and an outbox. A conditional stock update, hold insert and outgoing event commit together before confirmation. A repeated request ID returns the saved result.

The worker checks stored deadlines, sends scheduled reminders, guards expiry so quantity returns once, and creates one priority notification for the next queued customer. Collection and expiry compete for the same active hold state. A periodic database sweep recovers missed jobs. Any queue claim window remains a product policy to agree; notification alone is not a confirmed hold.

Redis Lua can act as a fast admission gate and BullMQ can schedule background work. PostgreSQL remains authoritative. Redis TTL deletion alone does not restore database stock. HTTP 409 represents a stock conflict; 429 represents rate limiting. Capacity and revenue recovery are not measured claims.

## Verification

Run `node check.cjs` for demand quantities, stock by size, both race orders, duplicate requests, collection/expiry exclusivity, repeated expiry, reminder eligibility/deduplication, directions, queue notification and priority, seven slides, logo placements and local assets.

The frontend was also exercised in JSDOM for social matching, badge controls, demand capture, both winners, receipt display, staff metrics, reminder generation, expiry/pickup, next-customer notice, reset, print-state restoration, notes and navigation. JSDOM does not render layout. Browser preview surfaces were unavailable. Production safety requires parallel requests and failure tests against the real backend.

## Research and credits

1. Tu et al., *Speedy Transactions in Multicore In-Memory Databases*, SOSP 2013, §4.4. DOI: 10.1145/2517349.2522713. Competing-update validation principle, not an implementation of Silo.
2. Gray and Cheriton, *Leases: An Efficient Fault-Tolerant Mechanism for Distributed File Cache Consistency*, SOSP 1989, §§2, 5. DOI: 10.1145/74850.74870. Rights with deadlines, not the source of the retail hold duration.
3. Mathioudakis and Koudas, *TwitterMonitor: Trend Detection over the Twitter Stream*, SIGMOD 2010, §2. DOI: 10.1145/1807167.1807306. Baseline comparison principle; bundled PDF is the three-page author manuscript.

Official implementation references are linked on slide 7: PostgreSQL transaction isolation, Redis Lua scripting, Redis keyspace-notification limits and BullMQ delayed jobs.

Hoodie photo: Mediamodifier, https://unsplash.com/photos/white-hoodie-and-distressed-jeans-kJXGTOY1wLQ, free under the Unsplash License. It illustrates the product category and does not imply a store partnership. BIZZNOVATE logo and both briefs were supplied by the user. Technology logos use Devicon, with its license in `dist/assets/logos/LICENSE`.

## Live Go architecture update

Slide 6 now explains the implemented React + TypeScript, Go net/http, pgxpool/PostgreSQL, go-redis/Redis/Lua, PostgreSQL outbox, separate Go/Asynq worker and real Go HTTP load generator. Open `dist/index.html#slide-6` and select any technology for its role. Notes contain a short Hinglish explanation. `dist/architecture-explained.md` provides the detailed explanation and optional 90-second script.

The editable one-slide PowerPoint is `output/dressing-room-system-architecture-final.pptx`, with the full explanation embedded in speaker notes. The seven-slide HTML presentation remains interactive. Its earlier slides illustrate the flow locally; use the separate `../dressing-room-live` application for real transactions and measurements. The original `../dressing-room-offline.zip` remains unchanged. The updated presentation bundle is `../dressing-room-architecture-updated.zip`.
