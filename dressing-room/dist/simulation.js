/* Educational model only. A production backend must enforce these transitions
   atomically in its database; a browser simulation cannot prove concurrency safety. */
(function (root) {
  'use strict';
  const HOLD_SECONDS = 24 * 60 * 60;
  function createInventory() { return { available: 1, version: 1, hold: null, queue: [], requests: Object.create(null), outbox: [] }; }
  // Models ONE indivisible database transaction; these JS statements are not a
  // distributed lock. A real SQL backend must atomically update stock and insert
  // the hold, outbox event and unique request ID before it reports success.
  function atomicBook(stock, customer, requestId, now = 0) {
    if (!requestId) return { ok: false, reason: 'missing-request-id' };
    if (Object.hasOwn(stock.requests, requestId)) {
      const previous = stock.requests[requestId];
      if (previous.customer !== customer) return { ok: false, reason: 'request-id-owner-mismatch' };
      return { ...previous, replay: true };
    }
    if (stock.queue.length && stock.queue[0] !== customer) return { ok: false, reason: 'waitlist-priority' };
    if (stock.available < 1) return { ok: false, reason: 'unavailable' };
    stock.available -= 1;
    stock.version += 1;
    stock.hold = { customer, status: 'held', expiresAt: now + HOLD_SECONDS, requestId };
    if (stock.queue[0] === customer) stock.queue.shift();
    const response = { ok: true, customer, requestId, expiresAt: stock.hold.expiresAt };
    stock.requests[requestId] = response;
    stock.outbox.push({ type: 'hold-confirmed', requestId });
    return response;
  }
  function createGate() { return { permits: 1, requests: new Set() }; }
  // Models a short, atomic Redis Lua admission script. It grants a provisional
  // permit, NOT a receipt. Redis failure must not bypass the database guard.
  function admit(gate, requestId) {
    if (gate.requests.has(requestId)) return true;
    if (gate.permits <= 0) return false;
    gate.permits -= 1; gate.requests.add(requestId); return true;
  }
  function simultaneousRace(first = 'Aarav', bypassRedis = false) {
    const names = first === 'Meera' ? ['Meera', 'Aarav'] : ['Aarav', 'Meera'];
    const stock = createInventory(), gate = createGate(), arrivals = {};
    for (const customer of names) {
      const requestId = `sample-${customer}`;
      const admitted = bypassRedis || admit(gate, requestId);
      arrivals[customer] = {
        arrival: '12:00:00.000', admitted,
        result: admitted ? atomicBook(stock, customer, requestId) : { ok: false, reason: 'gate-busy' }
      };
    }
    const winner = names.find(customer => arrivals[customer].result.ok);
    const loser = names.find(customer => customer !== winner);
    joinWaitlist(stock, loser);
    return { stock, arrivals, winner, loser };
  }
  function reserve(stock, customer, expectedVersion, now = 0) {
    if (stock.version !== expectedVersion) return { ok: false, reason: 'stale-version' };
    if (stock.available < 1) return { ok: false, reason: 'unavailable' };
    stock.available -= 1;
    stock.version += 1;
    stock.hold = { customer, status: 'held', expiresAt: now + HOLD_SECONDS };
    return { ok: true, expiresAt: stock.hold.expiresAt };
  }
  function joinWaitlist(stock, customer) {
    if (!stock.queue.includes(customer)) stock.queue.push(customer);
    return stock.queue.indexOf(customer) + 1;
  }
  function expire(stock, now) {
    if (!stock.hold || stock.hold.status !== 'held' || now < stock.hold.expiresAt) return false;
    stock.hold.status = 'expired';
    stock.available += 1;
    stock.version += 1;
    stock.outbox.push({ type: 'hold-expired', requestId: stock.hold.requestId || stock.hold.customer, next: stock.queue[0] || null });
    return true;
  }
  function collect(stock, customer, now) {
    if (!stock.hold || stock.hold.customer !== customer || stock.hold.status !== 'held' || now >= stock.hold.expiresAt) return false;
    stock.hold.status = 'collected';
    stock.version += 1;
    return true;
  }
  // A simple illustrative burst comparison, not TwitterMonitor's QueueBurst algorithm.
  function mentionSignal(baseline, recent) {
    if (baseline <= 0) return { ratio: null, label: 'Insufficient baseline' };
    const ratio = recent / baseline;
    return { ratio, label: ratio >= 3 ? 'Rising interest' : 'Normal activity' };
  }
  function matchProduct(text, products) {
    const normalized = text.toLowerCase().replace(/[^a-z0-9]/g, '');
    return products.filter(product => product.aliases.some(alias => normalized.includes(alias.toLowerCase().replace(/[^a-z0-9]/g, '')))).map(product => product.sku);
  }
  const model = { HOLD_SECONDS, createInventory, atomicBook, createGate, admit, simultaneousRace, reserve, joinWaitlist, expire, collect, mentionSignal, matchProduct };
  root.DropSimulation = model;
  if (typeof module !== 'undefined' && module.exports) module.exports = model;
})(typeof window !== 'undefined' ? window : globalThis);
