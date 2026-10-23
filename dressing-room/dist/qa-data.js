window.JUDGES_QA = [
  {
    "question": "What changed after the crisis brief?",
    "answer": "We added an item, size and quantity interest form before launch, a demand summary, and a staff dashboard for hold conversion, uncollected items and requested sizes. The original reservation, social and reminder features remain visible."
  },
  {
    "question": "How does this help when competitors are 60% cheaper?",
    "answer": "It helps the store use limited stock more carefully and recover missed pickups. It supports the store’s own streetwear identity. We have not measured revenue recovery or claimed that software removes the price gap."
  },
  {
    "question": "What if both customers click at the exact same millisecond?",
    "answer": "Both may reach the server together. A database transaction checks and reduces the same item-size stock row. One succeeds, then the other sees zero available. Arrival time alone does not decide correctness."
  },
  {
    "question": "What stops a duplicate click from booking twice?",
    "answer": "Each request has an ID. The server saves that ID and its result with the hold. Repeating it returns the same result instead of reducing stock again."
  },
  {
    "question": "What if the server crashes after changing stock?",
    "answer": "The proposed transaction saves stock, hold, request result and an outgoing-event record together. A failure before commit rolls back the transaction. After commit, the saved result and event allow recovery."
  },
  {
    "question": "Why use PostgreSQL if Redis is faster?",
    "answer": "PostgreSQL stores the confirmed stock and hold together. Redis can reduce traffic during a rush. A fast temporary permit still needs a successful database commit before the customer receives a confirmed hold."
  },
  {
    "question": "Why is the hold 24 hours?",
    "answer": "The original problem statement explicitly requires a strict 24-hour hold. The crisis update adds demand validation and staff operations; it does not replace that duration."
  },
  {
    "question": "What happens when a hold expires?",
    "answer": "The worker marks it expired, returns the quantity once, and creates a priority notification for the next customer in that item-size queue. The old receipt no longer authorizes pickup."
  },
  {
    "question": "What if the expiry job runs twice or is missed?",
    "answer": "A guarded status change allows only one release. A periodic check finds overdue holds if a queued job is missed. Redis key expiry is a hint rather than the only recovery mechanism."
  },
  {
    "question": "What if the customer closes the browser?",
    "answer": "The server stores the deadline and runs the worker. The browser displays a countdown, but closing it does not cancel or extend the hold."
  },
  {
    "question": "What if pickup and expiry happen together?",
    "answer": "Both operations must guard the same active hold in a database transaction. Collection is accepted only before the stored deadline. Only one final state can succeed, so the item cannot be sold and returned to stock together."
  },
  {
    "question": "Does the next customer automatically own the item?",
    "answer": "No. The next customer gets priority to claim it. The server must enforce queue order during claiming. The duration of that claim window is a product policy still to be agreed; we do not invent one from the brief."
  },
  {
    "question": "Is pre-drop interest a reservation?",
    "answer": "No. It records what people want before launch. Staff use those requests and actual supply to plan the drop. A confirmed hold starts only when stock is available and the reservation succeeds."
  },
  {
    "question": "How do you avoid fake or duplicate demand?",
    "answer": "The production form should identify customers, merge repeat interest for the same item-size, validate quantity, and rate-limit submissions. The presentation uses fictional entries; it does not claim a complete abuse-control system."
  },
  {
    "question": "What does waitlist conversion mean here?",
    "answer": "It is the number of unique interested customers who received a confirmed hold, divided by all unique interested customers. It measures interest-to-hold conversion, not paid sales. Requested-size charts count units separately."
  },
  {
    "question": "How do you decide an item is trending?",
    "answer": "Match captions, comments and hashtags to known product names, aliases or product codes. Compare matched mentions with a historical baseline. Our two-to-ten example is illustrative; thresholds and platform permissions need real data."
  },
  {
    "question": "What is the personalised badge?",
    "answer": "A customer can receive a Story-sized image with their name, product and reserved or collected status. The demo exports a 1080 × 1920 PNG. Production should generate it after a verified reservation or pickup."
  },
  {
    "question": "Are reminders automatic or staff-written?",
    "answer": "The original brief asks for scheduled SMS/Push reminders with directions. The update also asks for a ready-to-send staff reminder. The design supports both: a scheduled job and an editable draft. This local demo prepares a sample and does not send real messages."
  },
  {
    "question": "Can this handle 10,000 concurrent requests?",
    "answer": "That is a load-test scenario, not a measured result. We would test many real parallel requests against the backend, check that holds never exceed stock, and measure latency, errors and recovery under failure."
  },
  {
    "question": "What can you finish in five hours?",
    "answer": "Follow the revised build order: interest capture and summary, a safe hold with a receipt, then expiry release and staff operations. Social parsing can start with allowed sample inputs; live platform access and message delivery depend on available integrations."
  },
  {
    "question": "What is working in this presentation?",
    "answer": "The local demand form, chart, social text matching, sample badge export, reservation simulation, receipt countdown, pickup/expiry states, next-customer notice and editable reminder. The separate dressing-room-live application runs React/TypeScript, Go, PostgreSQL, Redis/Lua and Asynq. Slide animations remain educational simulations."
  },
  {
    "question": "How would you prove that it meets the brief?",
    "answer": "Run the same-size last-hoodie race in both orders. Check exactly one active hold, repeated-click safety, one-time expiry release, queue notification, pickup before deadline, and no release after collection. Then test the real database with parallel requests and failures."
  },
  {
    "question": "Why these research papers?",
    "answer": "Silo supports reasoning about competing updates, Leases supports rights with deadlines, and TwitterMonitor supports activity changes against a baseline. Official database docs support the actual implementation choices. We do not claim to reproduce the complete research systems."
  },
  {
    "question": "Does this turn the store into a full e-commerce business?",
    "answer": "The flow ends with physical store pickup. It keeps limited oversized streetwear drops and avoids assuming instant supply. Shipping, payment checkout and a full marketplace are outside this presentation’s scope."
  }
];
