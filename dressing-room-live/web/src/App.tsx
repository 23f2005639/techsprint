import { useCallback, useEffect, useRef, useState } from "react";
type Row = Record<string, any>;
const credential = () =>
  sessionStorage.getItem("staffKey") || "local-demo-staff";
// getRandomValues also works in local HTTP previews where randomUUID may
// be unavailable. Each mutation still gets a cryptographically random key.
function requestKey() {
  return Array.from(crypto.getRandomValues(new Uint8Array(16)), (value) =>
    value.toString(16).padStart(2, "0"),
  ).join("");
}
async function api(path: string, data?: unknown, staff = false) {
  let r: Response;
  try {
    r = await fetch("/api/" + path, {
      method: data === undefined ? "GET" : "POST",
      headers: {
        "Content-Type": "application/json",
        ...(staff ? { "X-Staff-Key": credential() } : {}),
        ...(data !== undefined ? { "Idempotency-Key": requestKey() } : {}),
      },
      body: data === undefined ? undefined : JSON.stringify(data),
    });
  } catch {
    throw Error(
      "Cannot reach the local app. Run make demo in dressing-room-live, then open http://127.0.0.1:8080 and refresh.",
    );
  }
  const b = await r.json();
  if (!r.ok) throw Error(b.error || "Request failed");
  return b;
}
function usePoll<T>(fetcher: () => Promise<T>, ms = 2000) {
  const [data, setData] = useState<T>();
  const [error, setError] = useState("");
  const refresh = useCallback(async () => {
    try {
      setData(await fetcher());
      setError("");
    } catch (e) {
      setError(String(e));
    }
  }, [fetcher]);
  useEffect(() => {
    refresh();
    const id = setInterval(refresh, ms);
    return () => clearInterval(id);
  }, [refresh, ms]);
  return { data, error, refresh };
}
const catalogFetch = () => api("catalog");
const inventoryFetch = () => api("inventory", undefined, true);
const demandFetch = () => api("demand");
const socialFetch = () => api("social");
const runsFetch = () => api("benchmarks/runs", undefined, true);
function download(name: string, text: string, type = "application/json") {
  const url = URL.createObjectURL(new Blob([text], { type }));
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
function Story({ hold }: { hold: Row }) {
  return (
    <button
      className="ghost"
      onClick={() => {
        const c = document.createElement("canvas");
        c.width = 1080;
        c.height = 1920;
        const x = c.getContext("2d")!;
        x.fillStyle = "#10110e";
        x.fillRect(0, 0, 1080, 1920);
        x.fillStyle = "#d5ff42";
        x.font = "bold 70px sans-serif";
        x.fillText("BIZZNOVATE", 80, 180);
        x.font = "bold 110px sans-serif";
        x.fillText(
          hold.state === "collected" ? "COLLECTED." : "RESERVED.",
          80,
          790,
        );
        x.fillStyle = "#f4f3eb";
        x.font = "48px sans-serif";
        x.fillText(String(hold.customer).slice(0, 28), 80, 910);
        x.fillText(String(hold.variant).slice(0, 28), 80, 1000);
        x.font = "32px monospace";
        x.fillText("THE DRESSING ROOM / LOCAL DROP", 80, 1660);
        x.fillText(String(hold.id).slice(0, 20), 80, 1740);
        c.toBlob((blob) => {
          if (!blob) return;
          const u = URL.createObjectURL(blob),
            a = document.createElement("a");
          a.href = u;
          a.download = `${hold.state}-story.png`;
          a.click();
          setTimeout(() => URL.revokeObjectURL(u), 1000);
        }, "image/png");
      }}
    >
      ↓ {hold.state === "collected" ? "Collected" : "Reserved"} Story PNG
    </button>
  );
}
function Countdown({ expires }: { expires: string }) {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, []);
  const s = Math.max(0, Math.floor((Date.parse(expires) - now) / 1000));
  return (
    <span>
      {s === 0
        ? "Deadline reached"
        : `${Math.floor(s / 3600)}h ${Math.floor(s / 60) % 60}m ${s % 60}s`}
    </span>
  );
}
function Header() {
  const path = location.pathname;
  return (
    <>
      <header>
        <a href="/" className="brand">
          <img src="/assets/bizznovate.png" alt="BIZZNOVATE" />
          <span>THE DRESSING ROOM</span>
        </a>
        <nav aria-label="Main navigation">
          <a className={path === "/" ? "selected" : ""} href="/">
            The drop
          </a>
          <a className={path === "/staff" ? "selected" : ""} href="/staff">
            Staff
          </a>
          <a
            className={path === "/benchmarks" ? "selected" : ""}
            href="/benchmarks"
          >
            Live benchmarks ↗
          </a>
        </nav>
      </header>
    </>
  );
}
function Credential() {
  const [key, setKey] = useState(credential());
  return (
    <details className="credential">
      <summary>Local staff / demo credential</summary>
      <label>
        Staff key
        <input
          type="password"
          value={key}
          onChange={(e) => {
            setKey(e.target.value);
            sessionStorage.setItem("staffKey", e.target.value);
          }}
        />
      </label>
      <small>
        Local default: local-demo-staff. Set STAFF_KEY before starting services
        to change it.
      </small>
    </details>
  );
}
export function App() {
  return (
    <>
      <Header />
      {location.pathname === "/benchmarks" ? (
        <Benchmarks />
      ) : location.pathname === "/staff" ? (
        <Staff />
      ) : (
        <Store />
      )}
      <footer>
        BIZZNOVATE / THE DRESSING ROOM{" "}
        <span>Local live build · React / Go / PostgreSQL / Redis / Asynq</span>
      </footer>
    </>
  );
}
function Store() {
  const { data: cat, error } = usePoll<Row>(catalogFetch);
  const { data: demand } = usePoll<Row[]>(demandFetch);
  const { data: social } = usePoll<Row>(socialFetch);
  const [customer, setCustomer] = useState(
    localStorage.getItem("customer") || "",
  );
  const [size, setSize] = useState<Record<string, string>>({});
  const [qty, setQty] = useState<Record<string, number>>({});
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState("");
  const [receipt, setReceipt] = useState<Row>();
  const customerInput = useRef<HTMLInputElement>(null);
  const receiptPanel = useRef<HTMLElement>(null);
  const [feedback, setFeedback] = useState<{ id: string; text: string }>();
  useEffect(() => {
    if (receipt)
      receiptPanel.current?.scrollIntoView?.({
        behavior: "smooth",
        block: "start",
      });
  }, [receipt]);
  const myFetch = useCallback(
    () => api("holds?customer=" + encodeURIComponent(customer)),
    [customer],
  );
  const inboxFetch = useCallback(
    () => api("notifications?customer=" + encodeURIComponent(customer)),
    [customer],
  );
  const { data: holds, refresh } = usePoll<Row[]>(myFetch);
  const { data: inbox } = usePoll<Row[]>(inboxFetch);
  const act = async (path: string, data: Row, id: string) => {
    setBusy(id);
    try {
      setFeedback(undefined);
      if (!customer.trim()) {
        customerInput.current?.focus();
        throw Error(
          "Enter your name in the customer field above, then choose Reserve your size.",
        );
      }
      const result = await api(path, { customer: customer.trim(), ...data });
      setFeedback({
        id,
        text:
          result.message ||
          (path === "waitlist"
            ? "You joined the FIFO waitlist."
            : "Reservation confirmed. Your receipt is below the collection."),
      });
      setNotice(
        result.message ||
          "Your unit is reserved. Bring your digital receipt to pickup.",
      );
      if (result.receipt) setReceipt(await api("receipts/" + result.id));
      refresh();
    } catch (e) {
      const message = e instanceof Error ? e.message : String(e);
      setNotice(message);
      setFeedback({ id, text: message });
    } finally {
      setBusy("");
    }
  };
  const launched = cat?.settings?.[0]?.launched;
  return (
    <main>
      <div className="eyebrow">
        BIZZNOVATE / DROP 001{" "}
        <span className="live">
          ● {launched ? "RESERVATIONS LIVE" : "PRE-DROP INTEREST"}
        </span>
      </div>
      <section className="hero">
        <div>
          <p className="eyebrow">STREETWEAR. REAL STOCK. YOUR SIZE.</p>
          <h1>
            Claim the fit.
            <br />
            <em>Keep it yours.</em>
          </h1>
          <p className="intro">
            Pick your size. Reserve a real unit. Try it on in store.
            <br />
            The last two available units stay for walk-ins.
          </p>
          <a className="button" href="#collection">
            Explore the drop <span>↘</span>
          </a>
        </div>
        <div className="hero-image">
          <img
            src="/assets/hoodie-photo.jpg"
            alt="White hoodie streetwear inspiration"
          />
          <span>01 / CLOUD HEAVYWEIGHT</span>
        </div>
      </section>
      <div className="policy">
        <strong>
          {cat?.accelerated
            ? "ACCELERATED JUDGING MODE · 120s holds / 10s reminders / 20s queue offers / 30s fitting alerts"
            : "24-hour reservations · reminder 2 hours before expiry"}
        </strong>
        <span>Two units protected for walk-ins, in every size.</span>
      </div>
      <section className="customer">
        <label>
          Your name / local alias
          <input
            ref={customerInput}
            placeholder="Enter your name before reserving, e.g. Aarav"
            value={customer}
            onChange={(e) => {
              setCustomer(e.target.value);
              localStorage.setItem("customer", e.target.value);
            }}
          />
        </label>
        <p>
          Use the same alias for your receipts and local notification inbox.
          Demo data stays on this machine.
        </p>
      </section>
      {(notice || error) && (
        <div role="status" className="notice">
          {notice || error}
          <button className="text-button" onClick={() => setNotice("")}>
            Dismiss
          </button>
        </div>
      )}
      <section id="collection">
        <div className="section-title">
          <h2>
            The collection <sup>08</sup>
          </h2>
          <span>Illustrative INR pricing / S–XL</span>
        </div>
        <div className="products">
          {cat?.products?.map((p: Row) => {
            const sz = size[p.id] || "M",
              v = cat.variants.find((v: Row) => v.id === p.id + "-" + sz);
            const trend = social?.products?.find(
              (t: Row) => t.id === p.id,
            )?.trending;
            return (
              <article className="product" key={p.id}>
                <div className="photo">
                  <img src={"/assets/" + p.photo} alt={p.name} loading="lazy" />
                  <span className="code">{p.id}</span>
                  {trend && <span className="trend">↗ TRENDING DROP</span>}
                </div>
                <div className="product-title">
                  <h3>{p.name}</h3>
                  <span>₹{p.price.toLocaleString("en-IN")}</span>
                </div>
                <div
                  className="size-row"
                  role="group"
                  aria-label={"Size for " + p.name}
                >
                  {["S", "M", "L", "XL"].map((s) => (
                    <button
                      aria-pressed={sz === s}
                      key={s}
                      onClick={() => setSize({ ...size, [p.id]: s })}
                    >
                      {s}
                    </button>
                  ))}
                  <label className="quantity">
                    Qty
                    <input
                      aria-label={"Quantity for " + p.name}
                      type="number"
                      min="1"
                      max="10"
                      value={qty[p.id] ?? 1}
                      onChange={(e) =>
                        setQty({ ...qty, [p.id]: Number(e.target.value) })
                      }
                    />
                  </label>
                </div>
                <p className="stock">
                  {v?.online > 0
                    ? `${v.online} online · ${v.available} available in store`
                    : "Available in-store only"}{" "}
                  <span>2-unit floor</span>
                </p>
                <button
                  disabled={
                    busy === p.id ||
                    (launched && !(v?.online >= (qty[p.id] || 1)))
                  }
                  onClick={() =>
                    act(
                      launched ? "reservations" : "interest",
                      { variant: v.id, quantity: qty[p.id] || 1 },
                      p.id,
                    )
                  }
                >
                  {busy === p.id
                    ? "Saving…"
                    : launched
                      ? "Reserve your size ↗"
                      : "Save pre-drop interest ↗"}
                </button>
                <button
                  className="text-button"
                  disabled={busy === p.id}
                  onClick={() => act("waitlist", { variant: v.id }, p.id)}
                >
                  Join FIFO waitlist
                </button>
                {feedback && feedback.id === p.id && (
                  <p className="product-feedback" role="status">
                    {feedback.text}
                  </p>
                )}
                {launched && !(v?.online >= (qty[p.id] || 1)) && (
                  <p className="product-feedback">
                    This quantity cannot be reserved online. Try another size,
                    lower the quantity, join the waitlist, or visit the store.
                  </p>
                )}
              </article>
            );
          })}
        </div>
      </section>
      <section className="two-col">
        <div className="panel">
          <p className="eyebrow">YOUR DROP</p>
          <h2>Receipts & holds</h2>
          {!holds?.length && <p>Your reservations will appear here.</p>}
          {holds?.map((h) => (
            <div className="hold" key={h.id}>
              <div>
                <strong>{h.variant}</strong>
                <span className="pill">
                  {h.kind} / {h.state}
                </span>
              </div>
              <small>{h.id}</small>
              {h.state === "active" && (
                <p>
                  <Countdown expires={h.expires} />
                  {h.kind === "fitting" &&
                    " · Stock protected until physical return"}
                </p>
              )}
              <div className="actions">
                <button
                  className="ghost"
                  onClick={async () =>
                    setReceipt(await api("receipts/" + h.id))
                  }
                >
                  Digital receipt
                </button>
                {h.kind === "offer" && h.state === "active" && (
                  <button onClick={() => act("claim", { hold: h.id }, h.id)}>
                    Claim offer
                  </button>
                )}
                {(h.kind === "online" || h.state === "collected") && (
                  <Story hold={h} />
                )}
              </div>
            </div>
          ))}
        </div>
        <div className="panel">
          <p className="eyebrow">LOCAL DELIVERY</p>
          <h2>Notification inbox</h2>
          {!inbox?.length && (
            <p>Confirmed holds, reminders and queue offers arrive here.</p>
          )}
          {inbox?.map((n) => (
            <article className="notification" key={n.id}>
              <strong>{n.kind}</strong>
              <p>{n.body}</p>
              <small>{new Date(n.created).toLocaleString()}</small>
            </article>
          ))}
        </div>
      </section>
      {receipt && (
        <section
          ref={receiptPanel}
          className="panel receipt"
          aria-label="Digital receipt"
        >
          <button className="text-button" onClick={() => setReceipt(undefined)}>
            Close ×
          </button>
          <p className="eyebrow">DIGITAL RECEIPT / {receipt.hold.state}</p>
          <h2>{receipt.hold.name}</h2>
          <p>
            {receipt.hold.customer} · Size {receipt.hold.size} · ₹
            {receipt.hold.price} per unit
          </p>
          <code>{receipt.hold.id}</code>
          <p>Units: {receipt.units.map((u: Row) => u.unit).join(", ")}</p>
          <p>{receipt.directions}</p>
          <button
            onClick={() =>
              download(
                "receipt-" + receipt.hold.id + ".json",
                JSON.stringify(receipt, null, 2),
              )
            }
          >
            Download receipt JSON
          </button>
          <Story hold={receipt.hold} />
        </section>
      )}
      <section className="two-col">
        <div className="panel">
          <h2>Before the doors open</h2>
          <p>Real saved interest, grouped by requested size and quantity.</p>
          {demand?.length ? (
            demand.map((d) => (
              <div className="row" key={d.variant}>
                <span>{d.variant}</span>
                <strong>
                  {d.quantity} units / {d.customers} people
                </strong>
              </div>
            ))
          ) : (
            <p>No interest captured yet.</p>
          )}
        </div>
        <div className="panel">
          <h2>Social signals, honestly labelled</h2>
          <p>
            Synthetic social-site imports. Keyword product matching. A spike
            requires at least 3 current posts and 2× the baseline.
          </p>
          {social?.products
            ?.filter((s: Row) => s.current || s.baseline)
            .map((s: Row) => (
              <div className="row" key={s.id}>
                <span>
                  {s.id} · {s.baseline} → {s.current} posts
                </span>
                <strong>
                  {s.reach} fixture reach / {s.engagement} interactions
                </strong>
              </div>
            ))}
          <small>
            Illustrative category photography; products are demo items.{" "}
            <a href="/assets/credits.txt">Photo sources & credits</a>.
          </small>
        </div>
      </section>
    </main>
  );
}
function Staff() {
  const { data, refresh, error } = usePoll<Row>(inventoryFetch);
  const { data: demand } = usePoll<Row[]>(demandFetch);
  const [code, setCode] = useState("DR01-M-06");
  const [customer, setCustomer] = useState("Walk-in");
  const [hold, setHold] = useState("");
  const [draft, setDraft] = useState("");
  const [message, setMessage] = useState("");
  const [location_, setLocation] = useState("stockroom");
  const [online, setOnline] = useState(false);
  const [pending, setPending] = useState(false);
  const controls = data?.settings?.[0];
  const act = async (path: string, p: Row) => {
    setPending(true);
    try {
      const m = await api(path, p, true);
      setMessage(m.message || m.state || "Saved");
      if (m.id) setHold(m.id);
      refresh();
    } catch (e) {
      setMessage(String(e));
    } finally {
      setPending(false);
    }
  };
  return (
    <main>
      <div className="eyebrow">OPERATIONS / PHYSICAL INVENTORY</div>
      <div className="section-title">
        <h1>At the counter.</h1>
        <span className="live">● STAFF CONSOLE</span>
      </div>
      <Credential />
      {(error || message) && (
        <div className="notice" role="status">
          {message || error}
        </div>
      )}
      <Inventory data={data?.totals} />
      <div className="two-col">
        <section className="panel">
          <h2>Scan → fit → complete</h2>
          <p>
            Scan an available physical unit before fitting. An overdue fitting
            stays protected until purchase or confirmed physical return.
          </p>
          <label>
            Unit scan code
            <input value={code} onChange={(e) => setCode(e.target.value)} />
          </label>
          <label>
            Customer
            <input
              value={customer}
              onChange={(e) => setCustomer(e.target.value)}
            />
          </label>
          <button
            disabled={pending}
            onClick={() => act("staff/fitting", { code, customer })}
          >
            Start fitting hold
          </button>
          <label>
            Existing hold ID
            <input value={hold} onChange={(e) => setHold(e.target.value)} />
          </label>
          <div className="actions">
            <button
              disabled={pending || !hold}
              onClick={() => act("staff/checkout", { hold })}
            >
              Complete checkout
            </button>
            <button
              className="ghost"
              disabled={pending || !hold}
              onClick={() => act("staff/return", { hold })}
            >
              Confirm physical return
            </button>
          </div>
          <hr />
          <h3>Confirm physical placement</h3>
          <label>
            Destination
            <select
              value={location_}
              onChange={(e) => setLocation(e.target.value)}
            >
              <option value="stockroom">Stockroom</option>
              <option value="display">Display</option>
            </select>
          </label>
          <label className="check">
            <input
              type="checkbox"
              checked={online}
              onChange={(e) => setOnline(e.target.checked)}
            />
            Publish surplus online after placement
          </label>
          <button
            className="ghost"
            disabled={pending}
            onClick={() =>
              act("staff/transfer", { code, location: location_, online })
            }
          >
            Confirm placement of scanned unit
          </button>
        </section>
        <section className="panel">
          <h2>Drop & delivery controls</h2>
          <p>
            {controls?.launched ? "Drop is live" : "Pre-drop interest capture"}{" "}
            · Worker{" "}
            {controls?.paused
              ? "paused"
              : controls?.worker_at
                ? "last seen " +
                  new Date(controls.worker_at).toLocaleTimeString()
                : "not yet seen"}
          </p>
          <div className="actions">
            <button
              disabled={pending}
              onClick={() =>
                act("staff/settings", {
                  launched: !controls?.launched,
                  paused: !!controls?.paused,
                })
              }
            >
              {controls?.launched ? "Close reservations" : "Launch drop"}
            </button>
            <button
              className="ghost"
              disabled={pending}
              onClick={() =>
                act("staff/settings", {
                  launched: !!controls?.launched,
                  paused: !controls?.paused,
                })
              }
            >
              {controls?.paused ? "Resume worker" : "Pause worker processing"}
            </button>
          </div>
          <label>
            Editable reminder draft
            <textarea
              placeholder={controls?.draft}
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
            />
          </label>
          <button
            className="ghost"
            disabled={pending}
            onClick={() =>
              act("staff/settings", {
                launched: !!controls?.launched,
                paused: !!controls?.paused,
                draft,
              })
            }
          >
            Save reminder draft
          </button>
          <hr />
          <h3>Synthetic social import</h3>
          <p>
            Scrape captions, comments and hashtags from the bundled synthetic
            page. Baseline, hoodie spike and unrelated noise use stable fixture
            IDs make repeated imports deduplicate.
          </p>
          <button className="ghost" onClick={() => act("social/scrape", {})}>
            Scrape synthetic social site
          </button>
          <p>
            <a href="/api/social/site" target="_blank" rel="noreferrer">
              View bundled synthetic social page ↗
            </a>
          </p>
          <h3>Pre-drop demand</h3>
          {demand?.map((d) => (
            <div className="row" key={d.variant}>
              <span>{d.variant}</span>
              <strong>{d.quantity} units</strong>
            </div>
          ))}
        </section>
      </div>
      <section className="panel">
        <h2>Active & recent holds</h2>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Hold / customer</th>
                <th>Variant</th>
                <th>State</th>
                <th>Deadline</th>
                <th>Action</th>
              </tr>
            </thead>
            <tbody>
              {data?.holds?.map((h: Row) => (
                <tr key={h.id}>
                  <td>
                    <code>{h.id.slice(0, 12)}</code>
                    <br />
                    {h.customer}
                  </td>
                  <td>{h.variant}</td>
                  <td>
                    {h.kind} / {h.state}
                  </td>
                  <td>
                    <Countdown expires={h.expires} />
                  </td>
                  <td>
                    <button className="ghost" onClick={() => setHold(h.id)}>
                      Use hold
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
      <section className="panel">
        <h2>
          Physical units <small>Click a scan code to use it</small>
        </h2>
        <div className="table-wrap units">
          <table>
            <thead>
              <tr>
                <th>Scan code</th>
                <th>Variant</th>
                <th>Location</th>
                <th>Online eligible</th>
                <th>State</th>
              </tr>
            </thead>
            <tbody>
              {data?.units?.map((u: Row) => (
                <tr key={u.code}>
                  <td>
                    <button
                      className="text-button"
                      onClick={() => setCode(u.code)}
                    >
                      {u.code}
                    </button>
                  </td>
                  <td>{u.variant}</td>
                  <td>{u.location}</td>
                  <td>
                    {u.online ? "Published" : "Staff confirmation needed"}
                  </td>
                  <td>
                    <span className="pill">{u.state}</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
      <section className="panel">
        <h2>FIFO waitlist</h2>
        {data?.waitlist?.map((w: Row) => (
          <div className="row" key={w.id}>
            <span>
              #{w.id} · {w.customer} / {w.variant}
            </span>
            <strong>{w.state}</strong>
          </div>
        ))}
      </section>
    </main>
  );
}
function Inventory({ data }: { data?: Row }) {
  return (
    <div className="metrics">
      {[
        ["Available", "available"],
        ["Online held", "online_held"],
        ["Fitting held", "fitting_held"],
        ["Walk-in protected¹", "protected"],
        ["Queue offers", "offers"],
        ["Sold", "sold"],
      ].map(([label, k]) => (
        <div className="metric" key={k}>
          <small>{label}</small>
          <strong>{data?.[k] ?? "—"}</strong>
        </div>
      ))}
    </div>
  );
}
function Chart({ points }: { points: Row[] }) {
  const max = Math.max(1, ...points.map((p) => p.p95 || 0));
  return (
    <div className="chart">
      <svg
        viewBox="0 0 900 150"
        role="img"
        aria-label="Observed p95 latency history"
      >
        <path d="M0 149H900 M0 75H900 M0 1H900" className="gridline" />
        <polyline
          points={points
            .map(
              (p, i) =>
                `${(i * 900) / Math.max(points.length - 1, 1)},${145 - ((p.p95 || 0) / max) * 135}`,
            )
            .join(" ")}
          fill="none"
          stroke="#d5ff42"
          strokeWidth="3"
        />
      </svg>
      <small>
        CLIENT p95 / {max.toFixed(1)} ms scale · latest 60 snapshots
      </small>
    </div>
  );
}
function Benchmarks() {
  const { data: runs, error, refresh } = usePoll<Row[]>(runsFetch);
  const [selected, setSelected] = useState("");
  const [snapshot, setSnapshot] = useState<Row>();
  const [history, setHistory] = useState<Row[]>([]);
  const [connected, setConnected] = useState(false);
  const [message, setMessage] = useState("");
  const [scenario, setScenario] = useState("race");
  const [requests, setRequests] = useState(100);
  const [concurrency, setConcurrency] = useState(100);
  const [starting, setStarting] = useState(false);
  const id = selected || runs?.[0]?.id;
  useEffect(() => {
    if (!id) return;
    const controller = new AbortController();
    setSnapshot(undefined);
    setHistory([]);
    let retry: ReturnType<typeof setTimeout>;
    const stream = async () => {
      try {
        setSnapshot(await api("benchmarks/runs/" + id, undefined, true));
        const r = await fetch("/api/benchmarks/runs/" + id + "/events", {
          headers: { "X-Staff-Key": credential() },
          signal: controller.signal,
        });
        if (!r.ok || !r.body) throw Error("Stream unavailable");
        setConnected(true);
        const reader = r.body.getReader(),
          decoder = new TextDecoder();
        let buf = "";
        while (!controller.signal.aborted) {
          const { value, done } = await reader.read();
          if (done) break;
          buf += decoder.decode(value, { stream: true });
          let split;
          while ((split = buf.indexOf("\n\n")) >= 0) {
            const event = buf.slice(0, split);
            buf = buf.slice(split + 2);
            const line = event.split("\n").find((l) => l.startsWith("data: "));
            if (line) {
              const s = JSON.parse(line.slice(6));
              setSnapshot(s);
              if (s.run?.status === "running")
                setHistory((h) => [...h, s.metrics].slice(-60));
            }
          }
        }
      } catch (e) {
        if (!controller.signal.aborted)
          setMessage(
            "Connection interrupted; fetching current snapshot and reconnecting.",
          );
      } finally {
        setConnected(false);
        if (!controller.signal.aborted) retry = setTimeout(stream, 1000);
      }
    };
    stream();
    return () => {
      controller.abort();
      clearTimeout(retry);
    };
  }, [id]);
  const m = snapshot?.metrics,
    run = snapshot?.run,
    inv = snapshot?.inventory;
  const start = async () => {
    setStarting(true);
    try {
      await api("benchmarks/start", { scenario, requests, concurrency }, true);
      setSelected("");
      setMessage(
        "Go HTTP generator starting. Measurements appear when requests complete.",
      );
      refresh();
    } catch (e) {
      setMessage(String(e));
    } finally {
      setStarting(false);
    }
  };
  const exportCSV = async () => {
    const r = await fetch("/api/benchmarks/runs/" + id + "/export.csv", {
      headers: { "X-Staff-Key": credential() },
    });
    if (!r.ok) {
      setMessage("CSV download failed");
      return;
    }
    download(id + ".csv", await r.text(), "text/csv");
  };
  return (
    <main className="bench">
      <div className="eyebrow">OBSERVABILITY / REAL CONCURRENT HTTP</div>
      <div className="section-title">
        <h1>Prove the promise.</h1>
        <span className="live">
          ● {connected ? "STREAM CONNECTED" : "AWAITING STREAM"}
        </span>
      </div>
      <p className="intro">
        One unit. Concurrent requests. An inventory ledger that has the final
        word.
      </p>
      <Credential />
      <section className="run-controls panel">
        <label>
          Scenario
          <select
            value={scenario}
            onChange={(e) => setScenario(e.target.value)}
          >
            <option value="race">One online unit · race</option>
            <option value="five">Five units · three online winners</option>
            <option value="floor">Two units · walk-in floor</option>
            <option value="duplicate">Duplicate request · one hold</option>
          </select>
        </label>
        <label>
          Requests
          <input
            type="number"
            min="1"
            max="100000"
            value={requests}
            onChange={(e) => setRequests(Number(e.target.value))}
          />
        </label>
        <label>
          Concurrency
          <input
            type="number"
            min="1"
            max="500"
            value={concurrency}
            onChange={(e) => setConcurrency(Number(e.target.value))}
          />
        </label>
        <button
          disabled={starting || run?.status === "running"}
          onClick={start}
        >
          {starting ? "Starting…" : "Run Go benchmark ↗"}
        </button>
      </section>
      {(message || error) && (
        <p className="notice" role="status">
          {message || error}
        </p>
      )}
      {!id ? (
        <section className="empty">
          <span>◎</span>
          <h2>No benchmark running</h2>
          <p>
            Start a scenario here or run{" "}
            <code>make bench SCENARIO=race REQUESTS=100 CONCURRENCY=100</code>.
            <br />
            All charts wait for real observed requests.
          </p>
        </section>
      ) : (
        <>
          <div className="section-title">
            <div>
              <span className="pill">
                {run?.status || "Loading snapshot"} / {run?.scenario}
              </span>
              <p className="mono">
                {id} · {run?.requests} requests / {run?.concurrency} concurrent
              </p>
            </div>
            <label>
              Run history
              <select value={id} onChange={(e) => setSelected(e.target.value)}>
                {runs?.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.scenario} · {r.id.slice(0, 8)} · {r.status}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <div className="metrics">
            <div className="metric accent">
              <small>
                {run?.status === "finished"
                  ? "Observed average RPS"
                  : "Live requests / sec²"}
              </small>
              <strong>
                {Number(
                  run?.status === "finished" ? m?.rps || 0 : m?.live_rps || 0,
                ).toFixed(1)}
              </strong>
            </div>
            {["p50", "p95", "p99"].map((k) => (
              <div className="metric" key={k}>
                <small>Client {k}</small>
                <strong>
                  {Number(m?.[k] || 0).toFixed(1)}
                  <i> ms</i>
                </strong>
              </div>
            ))}
            <div className="metric">
              <small>Completed requests</small>
              <strong>
                {m?.completed ?? 0}
                <i> / {run?.requests}</i>
              </strong>
            </div>
          </div>
          <div className="panel">
            <div className="section-title">
              <h2>Latency, observed</h2>
              <small>500ms snapshots / bounded history</small>
            </div>
            <Chart points={history.length ? history : m ? [m] : []} />
            <div className="outcomes">
              {[
                ["Successful HTTP holds³", "successful"],
                ["Expected stock / channel conflicts", "conflicts"],
                ["Rate limits", "rate_limits"],
                ["Unexpected failures", "failures"],
              ].map(([label, k]) => (
                <div key={k}>
                  <strong>{m?.[k] ?? 0}</strong>
                  <small>{label}</small>
                </div>
              ))}
            </div>
          </div>
          <p className="footnote">
            {m?.unique_holds ?? "—"} unique holds recorded for this run.
          </p>
          <h2>Authoritative inventory</h2>
          <Inventory data={inv} />
          <p className="footnote">
            ¹ Protected is a subset of available, capped at two per size. ² Live
            rate uses server receipt time of client sample batches. ³ Duplicate
            requests can return multiple successes for one unique hold.
          </p>
          <div className="two-col">
            <section className="panel">
              <h2>Invariant checks</h2>
              {[
                ["No duplicate unit allocation", "no_duplicate_allocation"],
                ["Protected-floor compliance", "protected_floor"],
                ["Inventory conservation", "inventory_conserved"],
              ].map(([label, k]) => (
                <div className="row" key={k}>
                  <span>{label}</span>
                  <strong className={inv?.[k] ? "good" : "bad"}>
                    {inv?.[k] === true
                      ? "PASS"
                      : inv?.[k] === false
                        ? "FAIL"
                        : "PENDING"}
                  </strong>
                </div>
              ))}
              <p>Checks read PostgreSQL state and allocation audit records.</p>
            </section>
            <section className="panel">
              <h2>Service pulse</h2>
              <div className="row">
                <span>PostgreSQL pool acquired / total / max</span>
                <strong>
                  {snapshot?.pool?.acquired} / {snapshot?.pool?.total} /{" "}
                  {snapshot?.pool?.max}
                </strong>
              </div>
              <div className="row">
                <span>Asynq pending / active / scheduled / retry</span>
                <strong>
                  {snapshot?.queue?.unavailable
                    ? "Redis unavailable"
                    : `${snapshot?.queue?.pending ?? 0} / ${snapshot?.queue?.active ?? 0} / ${snapshot?.queue?.scheduled ?? 0} / ${snapshot?.queue?.retry ?? 0}`}
                </strong>
              </div>
              <div className="row">
                <span>Worker heartbeat</span>
                <strong>
                  {snapshot?.worker?.[0]?.healthy
                    ? "HEALTHY"
                    : snapshot?.worker?.[0]?.paused
                      ? "PAUSED"
                      : "STALE / OFFLINE"}
                </strong>
              </div>
              <div className="row">
                <span>Outbox awaiting dispatch</span>
                <strong>{snapshot?.outbox?.[0]?.pending ?? "—"}</strong>
              </div>
            </section>
          </div>
          <section className="panel">
            <div className="section-title">
              <h2>Event timeline</h2>
              <div className="actions">
                <button
                  className="ghost"
                  onClick={() =>
                    download(id + ".json", JSON.stringify(snapshot, null, 2))
                  }
                >
                  ↓ JSON snapshot
                </button>
                <button className="ghost" onClick={exportCSV}>
                  ↓ Request CSV
                </button>
              </div>
            </div>
            <p>Latest 50 application events, including expiry recovery.</p>
            <div className="timeline">
              {snapshot?.events?.map((e: Row) => (
                <div className="row" key={e.id}>
                  <small>{new Date(e.created).toLocaleTimeString()}</small>
                  <strong>{e.kind}</strong>
                  <code>{JSON.stringify(e.body).slice(0, 180)}</code>
                </div>
              ))}
            </div>
          </section>
        </>
      )}
    </main>
  );
}
