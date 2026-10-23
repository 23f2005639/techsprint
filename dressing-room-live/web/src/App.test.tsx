// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { App } from "./App";
const products = [
  { id: "DR01", name: "Cloud hoodie", price: 2499, photo: "hoodie-photo.jpg" },
];
let launched = false;
let calls: { path: string; body: any }[] = [];
const catalog = () => ({
  products,
  variants: ["S", "M", "L", "XL"].map((size) => ({
    id: "DR01-" + size,
    size,
    product: "DR01",
    available: 5,
    online: 3,
  })),
  settings: [{ launched }],
  accelerated: true,
});
beforeEach(() => {
  for (const name of ["localStorage", "sessionStorage"]) {
    const storage = new Map<string, string>();
    vi.stubGlobal(name, {
      getItem: (k: string) => storage.get(k) ?? null,
      setItem: (k: string, v: string) => storage.set(k, v),
      clear: () => storage.clear(),
    });
  }
  calls = [];
  launched = false;
  history.replaceState({}, "", "/");
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, opts?: RequestInit) => {
      const path = url.replace("/api/", "");
      if (opts?.method === "POST") {
        const body = JSON.parse(String(opts.body));
        calls.push({ path, body });
        return {
          ok: true,
          json: async () =>
            path === "staff/fitting"
              ? { id: "fitting-1" }
              : { id: "hold-1", message: "Saved" },
        };
      }
      const data: any =
        path === "catalog"
          ? catalog()
          : path === "demand"
            ? []
            : path === "social"
              ? { products: [] }
              : path === "inventory"
                ? {
                    units: [],
                    holds: [],
                    totals: { available: 5 },
                    settings: [{ launched: false, paused: false }],
                    waitlist: [],
                  }
                : path === "benchmarks/runs"
                  ? []
                  : [];
      return { ok: true, json: async () => data };
    }),
  );
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
it("captures actual customer, size and quantity from storefront controls", async () => {
  const user = userEvent.setup();
  render(<App />);
  await screen.findByText("Cloud hoodie");
  await user.type(screen.getByLabelText("Your name / local alias"), "Aarav");
  await user.click(screen.getByRole("button", { name: "L" }));
  const quantity = screen.getByLabelText("Quantity for Cloud hoodie");
  await user.clear(quantity);
  await user.type(quantity, "2");
  await user.click(
    screen.getByRole("button", { name: /Save pre-drop interest/ }),
  );
  await waitFor(() =>
    expect(calls).toContainEqual({
      path: "interest",
      body: { customer: "Aarav", variant: "DR01-L", quantity: 2 },
    }),
  );
});
it("reserves selected size when the drop is launched", async () => {
  launched = true;
  const user = userEvent.setup();
  render(<App />);
  await screen.findByText("Cloud hoodie");
  await user.type(screen.getByLabelText("Your name / local alias"), "Riya");
  await user.click(screen.getByRole("button", { name: /Reserve your size/ }));
  await waitFor(() =>
    expect(calls[0]).toEqual({
      path: "reservations",
      body: { customer: "Riya", variant: "DR01-M", quantity: 1 },
    }),
  );
});
it("staff fitting returns a hold used by checkout without reallocating stock", async () => {
  history.replaceState({}, "", "/staff");
  const user = userEvent.setup();
  render(<App />);
  await screen.findByText("Scan → fit → complete");
  await user.click(screen.getByRole("button", { name: "Start fitting hold" }));
  await waitFor(() =>
    expect(
      (screen.getByLabelText("Existing hold ID") as HTMLInputElement).value,
    ).toBe("fitting-1"),
  );
  await user.click(screen.getByRole("button", { name: "Complete checkout" }));
  expect(calls.map((c) => c.path)).toEqual(["staff/fitting", "staff/checkout"]);
  expect(calls[1].body).toEqual({ hold: "fitting-1" });
});
it("benchmark empty state has no invented measurements and starts chosen HTTP scenario", async () => {
  history.replaceState({}, "", "/benchmarks");
  const user = userEvent.setup();
  render(<App />);
  await screen.findByText("No benchmark running");
  expect(screen.queryByText("Client p95")).toBeNull();
  await user.selectOptions(screen.getByLabelText("Scenario"), "five");
  await user.click(screen.getByRole("button", { name: /Run Go benchmark/ }));
  await waitFor(() =>
    expect(calls[0]).toEqual({
      path: "benchmarks/start",
      body: { scenario: "five", requests: 100, concurrency: 100 },
    }),
  );
});
it.each(["active", "collected"])(
  "downloads a personalised %s Story PNG with correct dimensions",
  async (state) => {
    let dimensions: number[] = [];
    const text: string[] = [];
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockImplementation(
      function (this: HTMLCanvasElement) {
        dimensions = [this.width, this.height];
        return {
          fillStyle: "",
          font: "",
          fillRect: vi.fn(),
          fillText: (s: string) => text.push(s),
        } as any;
      },
    );
    vi.spyOn(HTMLCanvasElement.prototype, "toBlob").mockImplementation((cb) =>
      cb(new Blob(["png"], { type: "image/png" })),
    );
    URL.createObjectURL = vi.fn(() => "blob:story");
    URL.revokeObjectURL = vi.fn();
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
    const old = globalThis.fetch;
    vi.stubGlobal(
      "fetch",
      vi.fn(async (u: string, o?: RequestInit) =>
        u.startsWith("/api/holds")
          ? {
              ok: true,
              json: async () => [
                {
                  id: "abc",
                  customer: "Aarav",
                  variant: "DR01-M",
                  kind: "online",
                  state,
                  expires: new Date(Date.now() + 60000).toISOString(),
                },
              ],
            }
          : old(u, o),
      ),
    );
    const user = userEvent.setup();
    render(<App />);
    await user.click(
      await screen.findByRole("button", {
        name:
          state === "collected" ? /Collected Story PNG/ : /Reserved Story PNG/,
      }),
    );
    expect(dimensions).toEqual([1080, 1920]);
    expect(text).toContain("Aarav");
    expect(text).toContain(state === "collected" ? "COLLECTED." : "RESERVED.");
    vi.restoreAllMocks();
  },
);
it("renders observed benchmark samples and authoritative invariant results from SSE", async () => {
  history.replaceState({}, "", "/benchmarks");
  const run = {
    id: "run-1",
    scenario: "race",
    status: "running",
    requests: 100,
    concurrency: 100,
  };
  const snap = {
    run,
    metrics: {
      completed: 42,
      successful: 1,
      conflicts: 41,
      failures: 0,
      p50: 12.3,
      p95: 31.7,
      p99: 40.9,
      live_rps: 42,
    },
    inventory: {
      available: 2,
      online_held: 1,
      fitting_held: 0,
      protected: 2,
      offers: 0,
      sold: 0,
      no_duplicate_allocation: true,
      inventory_conserved: true,
      protected_floor: true,
    },
    pool: { acquired: 2, total: 4, max: 24 },
    events: [],
    worker: [{ healthy: true }],
    queue: {},
    outbox: [{ pending: 0 }],
  };
  vi.stubGlobal(
    "fetch",
    vi.fn(async (u: string) =>
      u.endsWith("/events")
        ? {
            ok: true,
            body: new ReadableStream({
              start(c) {
                c.enqueue(
                  new TextEncoder().encode(
                    "event: snapshot\ndata: " + JSON.stringify(snap) + "\n\n",
                  ),
                );
                c.close();
              },
            }),
          }
        : { ok: true, json: async () => (u.endsWith("/runs") ? [run] : snap) },
    ),
  );
  render(<App />);
  await screen.findByText("31.7");
  expect(screen.getAllByText("PASS")).toHaveLength(3);
  expect(screen.getByText("HEALTHY")).toBeTruthy();
  expect(screen.getByText("2 / 4 / 24")).toBeTruthy();
});

it("shows missing-name feedback at the clicked product and focuses the name field", async () => {
  launched = true;
  const user = userEvent.setup();
  render(<App />);
  const product = (await screen.findByText("Cloud hoodie")).closest("article")!;
  await user.click(
    within(product).getByRole("button", { name: /Reserve your size/ }),
  );
  expect(within(product).getByRole("status").textContent).toContain(
    "Enter your name",
  );
  expect(document.activeElement).toBe(
    screen.getByLabelText("Your name / local alias"),
  );
  expect(calls).toHaveLength(0);
});
it("reserves without randomUUID and shows the actual digital receipt", async () => {
  launched = true;
  const randomUUID = vi.spyOn(crypto, "randomUUID").mockImplementation(() => {
    throw Error("Unavailable in this preview");
  });
  const old = globalThis.fetch;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, options?: RequestInit) => {
      if (url === "/api/reservations") {
        expect(new Headers(options?.headers).get("Idempotency-Key")).toMatch(
          /^[a-f0-9]{32}$/,
        );
        return {
          ok: true,
          json: async () => ({
            id: "receipt-check",
            receipt: "/api/receipts/receipt-check",
          }),
        };
      }
      if (url === "/api/receipts/receipt-check")
        return {
          ok: true,
          json: async () => ({
            hold: {
              id: "receipt-check",
              name: "Cloud hoodie",
              customer: "Riya",
              size: "M",
              price: 2499,
              state: "active",
              kind: "online",
              variant: "DR01-M",
            },
            units: [{ unit: "DR01-M-01" }],
            directions: "Bring your receipt to the ground-floor counter.",
          }),
        };
      return old(url, options);
    }),
  );
  const user = userEvent.setup();
  render(<App />);
  await screen.findByText("Cloud hoodie");
  await user.type(screen.getByLabelText("Your name / local alias"), "Riya");
  await user.click(screen.getByRole("button", { name: /Reserve your size/ }));
  const receipt = await screen.findByRole("region", {
    name: "Digital receipt",
  });
  expect(receipt.textContent).toContain("receipt-check");
  expect(receipt.textContent).toContain("DR01-M-01");
  randomUUID.mockRestore();
});
