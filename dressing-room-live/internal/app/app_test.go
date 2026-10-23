package app

import (
	"bytes"
	"context"
	"dressingroom/internal/load"
	"encoding/json"
	"fmt"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

func testApp(t *testing.T) *App {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run database-backed integration checks")
	}
	t.Setenv("DATABASE_URL", url)
	t.Setenv("DEMO_MODE", "1")
	a, e := Open(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(a.Close)
	return a
}
func fixture(t *testing.T, a *App, n int, display bool) string {
	t.Helper()
	v := "TEST-" + ID()
	ctx := context.Background()
	_, e := a.DB.Exec(ctx, "INSERT INTO variants(id,product,size,initial_units) VALUES($1,'DR01',$2,$3)", v, v, n)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < n; i++ {
		loc := "stockroom"
		if display && i == n-1 {
			loc = "display"
		}
		_, e = a.DB.Exec(ctx, "INSERT INTO units(code,variant,location,online) VALUES($1,$2,$3,$4)", fmt.Sprintf("%s-%d", v, i), v, loc, loc == "stockroom")
		if e != nil {
			t.Fatal(e)
		}
	}
	return v
}
func req(a *App, path string, p M, key string, staff bool) (int, M) {
	b, _ := json.Marshal(p)
	r := httptest.NewRequest("POST", "/api/"+path, bytes.NewReader(b))
	r.Header.Set("Idempotency-Key", key)
	if staff {
		r.Header.Set("X-Staff-Key", a.StaffKey)
	}
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	var m M
	json.Unmarshal(w.Body.Bytes(), &m)
	return w.Code, m
}
func reserve(a *App, v, key string) (int, M) {
	return req(a, "reservations", M{"customer": "test-" + key, "variant": v, "quantity": 1}, key, true)
}
func launch(t *testing.T, a *App) {
	t.Helper()
	_, e := a.DB.Exec(context.Background(), "UPDATE settings SET launched=true,paused=false WHERE id=1")
	if e != nil {
		t.Fatal(e)
	}
}
func expire(t *testing.T, a *App, h string) {
	t.Helper()
	_, e := a.DB.Exec(context.Background(), "UPDATE holds SET expires=now()-interval '1 second' WHERE id=$1", h)
	if e != nil {
		t.Fatal(e)
	}
}
func sweep(t *testing.T, a *App) {
	t.Helper()
	if e := a.Sweep(context.Background()); e != nil {
		t.Fatal(e)
	}
}
func assertStatus(t *testing.T, want, got int, m M) {
	t.Helper()
	if got != want {
		t.Fatalf("want %d got %d: %v", want, got, m)
	}
}
func assertInvariant(t *testing.T, a *App) {
	t.Helper()
	m := a.Invariants(context.Background())
	for _, k := range []string{"no_duplicate_allocation", "inventory_conserved", "protected_floor"} {
		if m[k] != true {
			t.Fatalf("%s: %v", k, m)
		}
	}
}
func TestConcurrentFloorAndExactlyOne(t *testing.T) {
	a := testApp(t)
	launch(t, a)
	for _, n := range []int{5, 3, 2} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			v := fixture(t, a, n, false)
			var wg sync.WaitGroup
			var mu sync.Mutex
			success := 0
			for i := 0; i < 100; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					s, m := reserve(a, v, ID())
					mu.Lock()
					defer mu.Unlock()
					if s == 201 {
						success++
					} else if s != 409 {
						t.Errorf("unexpected %d: %v", s, m)
					}
				}()
			}
			wg.Wait()
			if success != n-2 {
				t.Fatalf("%d units: %d winners", n, success)
			}
			var remain int
			a.DB.QueryRow(context.Background(), "SELECT count(*) FROM units WHERE variant=$1 AND state='available'", v).Scan(&remain)
			if remain != 2 {
				t.Fatal(remain)
			}
			assertInvariant(t, a)
		})
	}
}
func TestFittingCheckoutAndRaceOrders(t *testing.T) {
	a := testApp(t)
	launch(t, a)
	v := fixture(t, a, 5, true)
	code := v + "-4"
	s, f := req(a, "staff/fitting", M{"code": code, "customer": "walk-in"}, ID(), true)
	assertStatus(t, 201, s, f)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); reserve(a, v, ID()) }()
	}
	s, m := req(a, "staff/checkout", M{"hold": f["id"]}, ID(), true)
	assertStatus(t, 200, s, m)
	wg.Wait()
	assertInvariant(t, a)
	v = fixture(t, a, 3, false)
	s, h := reserve(a, v, ID())
	assertStatus(t, 201, s, h)
	units := h["units"].([]any)
	code = units[0].(map[string]any)["code"].(string)
	s, m = req(a, "staff/fitting", M{"code": code, "customer": "late scan"}, ID(), true)
	assertStatus(t, 409, s, m)
	v = fixture(t, a, 3, false)
	s, f = req(a, "staff/fitting", M{"code": v + "-0", "customer": "first scan"}, ID(), true)
	assertStatus(t, 201, s, f)
	s, m = reserve(a, v, ID())
	assertStatus(t, 409, s, m)
	assertInvariant(t, a)
}
func TestOverdueFittingAndPhysicalReturn(t *testing.T) {
	a := testApp(t)
	v := fixture(t, a, 2, true)
	s, h := req(a, "staff/fitting", M{"code": v + "-1", "customer": "overdue"}, ID(), true)
	assertStatus(t, 201, s, h)
	expire(t, a, h["id"].(string))
	sweep(t, a)
	sweep(t, a)
	var state string
	a.DB.QueryRow(context.Background(), "SELECT state FROM units WHERE code=$1", v+"-1").Scan(&state)
	if state != "fitting" {
		t.Fatal(state)
	}
	s, m := req(a, "staff/return", M{"hold": h["id"]}, ID(), true)
	assertStatus(t, 200, s, m)
	s, m = req(a, "staff/return", M{"hold": h["id"]}, ID(), true)
	assertStatus(t, 409, s, m)
	assertInvariant(t, a)
}
func TestDuplicateRequestAndFingerprint(t *testing.T) {
	a := testApp(t)
	launch(t, a)
	v := fixture(t, a, 5, false)
	key := ID()
	var wg sync.WaitGroup
	ids := make(chan string, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, m := reserve(a, v, key)
			if s != 201 {
				t.Errorf("%d %v", s, m)
				return
			}
			ids <- m["id"].(string)
		}()
	}
	wg.Wait()
	close(ids)
	id := ""
	for h := range ids {
		if id != "" && h != id {
			t.Fatal("different receipts")
		}
		id = h
	}
	s, m := req(a, "reservations", M{"customer": "different", "variant": v, "quantity": 1}, key, true)
	assertStatus(t, 409, s, m)
	assertInvariant(t, a)
}
func TestExpiryFIFOAndBelowFloor(t *testing.T) {
	a := testApp(t)
	launch(t, a)
	v := fixture(t, a, 3, false)
	s, h := reserve(a, v, ID())
	assertStatus(t, 201, s, h)
	for _, c := range []string{"fifo-first-" + v, "fifo-second-" + v} {
		s, m := req(a, "waitlist", M{"customer": c, "variant": v}, ID(), true)
		assertStatus(t, 201, s, m)
	}
	expire(t, a, h["id"].(string))
	sweep(t, a)
	sweep(t, a)
	var offered string
	var offers int
	a.DB.QueryRow(context.Background(), "SELECT customer FROM waitlist WHERE variant=$1 AND state='offered'", v).Scan(&offered)
	if offered != "fifo-first-"+v {
		t.Fatal(offered)
	}
	a.DB.QueryRow(context.Background(), "SELECT count(*) FROM holds WHERE variant=$1 AND kind='offer' AND state='active'", v).Scan(&offers)
	if offers != 1 {
		t.Fatal(offers)
	}
	var offer string
	a.DB.QueryRow(context.Background(), "SELECT hold FROM waitlist WHERE variant=$1 AND state='offered'", v).Scan(&offer)
	s, m := req(a, "claim", M{"hold": offer}, ID(), true)
	assertStatus(t, 200, s, m)
	v = fixture(t, a, 3, false)
	_, h = reserve(a, v, ID())
	s, f := req(a, "staff/fitting", M{"customer": "walk-in", "code": v + "-1"}, ID(), true)
	assertStatus(t, 201, s, f)
	req(a, "waitlist", M{"customer": "floor-waiter", "variant": v}, ID(), true)
	expire(t, a, h["id"].(string))
	sweep(t, a)
	a.DB.QueryRow(context.Background(), "SELECT count(*) FROM holds WHERE variant=$1 AND kind='offer'", v).Scan(&offers)
	if offers != 0 {
		t.Fatal("offered below floor")
	}
	assertInvariant(t, a)
}
func TestPickupVersusExpiryAndRecovery(t *testing.T) {
	a := testApp(t)
	launch(t, a)
	v := fixture(t, a, 3, false)
	_, h := reserve(a, v, ID())
	expire(t, a, h["id"].(string))
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); a.Sweep(context.Background()) }()
	go func() { defer wg.Done(); req(a, "staff/checkout", M{"hold": h["id"]}, ID(), true) }()
	wg.Wait()
	sweep(t, a)
	assertInvariant(t, a)
	v = fixture(t, a, 3, false)
	_, h = reserve(a, v, ID())
	a.DB.Exec(context.Background(), "UPDATE settings SET paused=true WHERE id=1")
	expire(t, a, h["id"].(string))
	sweep(t, a)
	var state string
	a.DB.QueryRow(context.Background(), "SELECT state FROM holds WHERE id=$1", h["id"]).Scan(&state)
	if state != "active" {
		t.Fatal(state)
	}
	a.DB.Exec(context.Background(), "UPDATE settings SET paused=false WHERE id=1")
	sweep(t, a)
	sweep(t, a)
	a.DB.QueryRow(context.Background(), "SELECT state FROM holds WHERE id=$1", h["id"]).Scan(&state)
	if state != "expired" {
		t.Fatal(state)
	}
	assertInvariant(t, a)
}
func TestRemindersDirectionsAndSocial(t *testing.T) {
	a := testApp(t)
	launch(t, a)
	v := fixture(t, a, 3, false)
	_, h := reserve(a, v, ID())
	a.DB.Exec(context.Background(), "UPDATE holds SET expires=now()+interval '5 seconds' WHERE id=$1", h["id"])
	sweep(t, a)
	sweep(t, a)
	var n int
	var nid string
	a.DB.QueryRow(context.Background(), "SELECT count(*) FROM notifications WHERE hold=$1 AND kind='reminder'", h["id"]).Scan(&n)
	if n != 1 {
		t.Fatal(n)
	}
	a.DB.QueryRow(context.Background(), "SELECT id FROM notifications WHERE hold=$1 AND kind='reminder'", h["id"]).Scan(&nid)
	if e := a.Deliver(context.Background(), nid); e != nil {
		t.Fatal(e)
	}
	var body string
	a.DB.QueryRow(context.Background(), "SELECT body FROM notifications WHERE id=$1 AND delivered", nid).Scan(&body)
	if !bytes.Contains([]byte(body), []byte("ground floor")) {
		t.Fatal(body)
	}
	s, m := req(a, "staff/checkout", M{"hold": h["id"]}, ID(), true)
	assertStatus(t, 200, s, m)
	if e := a.Deliver(context.Background(), nid); e != nil {
		t.Fatal(e)
	}
	posts := []Post{{ID: ID(), Caption: "DR01 hoodie", Period: "baseline", Reach: 10, Engagement: 1}, {ID: ID(), Caption: "no matching words", Period: "current", Reach: 1}}
	for i := 0; i < 5; i++ {
		posts = append(posts, Post{ID: ID(), Caption: "DR01 hoodie", Period: "current", Reach: 20, Engagement: 2})
	}
	s, m = req(a, "social/import", M{"posts": posts}, ID(), true)
	assertStatus(t, 201, s, m)
	s, m = req(a, "social/import", M{"posts": posts}, ID(), true)
	if m["imported"] != float64(0) {
		t.Fatal(m)
	}
	for _, p := range a.socialSummary(context.Background())["products"].([]M) {
		if p["id"] == "DR01" && p["trending"] != true {
			t.Fatal(p)
		}
	}
}
func TestOutboxRetainsOnRedisFailure(t *testing.T) {
	a := testApp(t)
	launch(t, a)
	v := fixture(t, a, 3, false)
	s, m := reserve(a, v, ID())
	assertStatus(t, 201, s, m)
	c := asynq.NewClient(asynq.RedisClientOpt{Addr: "127.0.0.1:1", DialTimeout: 10 * time.Millisecond})
	defer c.Close()
	if e := a.Dispatch(context.Background(), c); e == nil {
		t.Fatal("expected redis failure")
	}
	var n int
	a.DB.QueryRow(context.Background(), "SELECT count(*) FROM outbox WHERE NOT sent").Scan(&n)
	if n == 0 {
		t.Fatal("lost pending jobs")
	}
	assertInvariant(t, a)
}
func TestAuthorizationValidationAndTransfer(t *testing.T) {
	a := testApp(t)
	v := fixture(t, a, 3, true)
	s, m := req(a, "staff/fitting", M{"code": v + "-2", "customer": "x"}, ID(), false)
	assertStatus(t, 401, s, m)
	s, m = req(a, "interest", M{"customer": "x", "variant": v, "quantity": 1}, "", true)
	assertStatus(t, 400, s, m)
	s, m = req(a, "staff/transfer", M{"code": v + "-2", "location": "stockroom", "online": true}, ID(), true)
	assertStatus(t, 200, s, m)
	launch(t, a)
	s, m = reserve(a, v, ID())
	assertStatus(t, 201, s, m)
}
func TestMutateRollsBack(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	v := fixture(t, a, 3, false)
	_, _ = a.Mutate(ctx, ID(), "rollback", nil, func(tx pgx.Tx) (int, M, error) {
		tx.Exec(ctx, "UPDATE units SET state='sold' WHERE variant=$1", v)
		return 0, nil, fmt.Errorf("injected error")
	})
	var n int
	a.DB.QueryRow(ctx, "SELECT count(*) FROM units WHERE variant=$1 AND state='available'", v).Scan(&n)
	if n != 3 {
		t.Fatal(n)
	}
}
func TestWorkerRestartRecoversDeadlines(t *testing.T) {
	a := testApp(t)
	launch(t, a)
	v := fixture(t, a, 3, false)
	_, h := reserve(a, v, ID())
	// Start the real Asynq server, stop it, miss a deadline, then restart it.
	run := func() (context.CancelFunc, chan error) {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- a.Worker(ctx) }()
		return cancel, done
	}
	cancel, done := run()
	time.Sleep(150 * time.Millisecond)
	cancel()
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	expire(t, a, h["id"].(string))
	cancel, done = run()
	defer func() {
		cancel()
		if e := <-done; e != nil {
			t.Error(e)
		}
	}()
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		var state string
		a.DB.QueryRow(context.Background(), "SELECT state FROM holds WHERE id=$1", h["id"]).Scan(&state)
		if state == "expired" {
			assertInvariant(t, a)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("restarted worker did not recover expiry")
}

func TestBenchmarkRealHTTPAndPersistedResults(t *testing.T) {
	a := testApp(t)
	launch(t, a)
	server := httptest.NewServer(a.Handler())
	defer server.Close()
	for _, scenario := range []string{"race", "five", "floor", "duplicate"} {
		t.Run(scenario, func(t *testing.T) {
			result, e := load.Run(context.Background(), server.URL, a.StaffKey, scenario, 100, 100)
			if e != nil {
				t.Fatal(e)
			}
			run := result["run"].(map[string]any)
			id := run["id"].(string)
			var active, total int
			a.DB.QueryRow(context.Background(), "SELECT count(*) FROM holds WHERE variant=$1", "BENCH-"+id).Scan(&active)
			want := 1
			if scenario == "five" {
				want = 3
			}
			if scenario == "floor" {
				want = 0
			}
			if active != want {
				t.Fatalf("%s: %d unique holds, want %d", scenario, active, want)
			}
			a.DB.QueryRow(context.Background(), "SELECT count(*) FROM samples WHERE run=$1", id).Scan(&total)
			if total != 100 {
				t.Fatal(total)
			}
			snapshot := a.RunSnapshot(context.Background(), id)
			if snapshot["run"].(map[string]any)["status"] != "finished" {
				t.Fatal(snapshot)
			}
			assertInvariant(t, a)
		})
	}
}
func TestPickupWinsAndStaleReminderIsSuppressed(t *testing.T) {
	a := testApp(t)
	launch(t, a)
	v := fixture(t, a, 3, false)
	_, h := reserve(a, v, ID())
	a.DB.Exec(context.Background(), "UPDATE holds SET expires=now()+interval '5 seconds' WHERE id=$1", h["id"])
	sweep(t, a)
	var nid string
	a.DB.QueryRow(context.Background(), "SELECT id FROM notifications WHERE hold=$1 AND kind='reminder'", h["id"]).Scan(&nid)
	s, m := req(a, "staff/checkout", M{"hold": h["id"]}, ID(), true)
	assertStatus(t, 200, s, m)
	expire(t, a, h["id"].(string))
	sweep(t, a)
	if e := a.Deliver(context.Background(), nid); e != nil {
		t.Fatal(e)
	}
	var delivered bool
	a.DB.QueryRow(context.Background(), "SELECT delivered FROM notifications WHERE id=$1", nid).Scan(&delivered)
	if delivered {
		t.Fatal("stale reminder delivered")
	}
	var sold int
	a.DB.QueryRow(context.Background(), "SELECT count(*) FROM units WHERE variant=$1 AND state='sold'", v).Scan(&sold)
	if sold != 1 {
		t.Fatal(sold)
	}
	assertInvariant(t, a)
}
func TestWalkInCheckoutAtFloor(t *testing.T) {
	a := testApp(t)
	launch(t, a)
	v := fixture(t, a, 2, true)
	s, m := reserve(a, v, ID())
	assertStatus(t, 409, s, m)
	s, h := req(a, "staff/fitting", M{"customer": "floor-buyer", "code": v + "-1"}, ID(), true)
	assertStatus(t, 201, s, h)
	s, m = req(a, "staff/checkout", M{"hold": h["id"]}, ID(), true)
	assertStatus(t, 200, s, m)
	assertInvariant(t, a)
}
func TestSyntheticSiteScrape(t *testing.T) {
	posts, e := ScrapeSocial()
	if e != nil {
		t.Fatal(e)
	}
	if len(posts) != 7 {
		t.Fatalf("scraped %d posts", len(posts))
	}
	if posts[2].Caption == "" || len(posts[2].Comments) == 0 || len(posts[4].Hashtags) == 0 {
		t.Fatal("missing scraped fields")
	}
	a := testApp(t)
	s, m := req(a, "social/scrape", M{}, ID(), true)
	assertStatus(t, 201, s, m)
	s, m = req(a, "social/scrape", M{}, ID(), true)
	assertStatus(t, 201, s, m)
	if m["imported"] != float64(0) {
		t.Fatal(m)
	}
}
func TestBenchmarkFinishIsIdempotent(t *testing.T) {
	a := testApp(t)
	s, run := req(a, "benchmarks/runs", M{"scenario": "floor", "requests": 1, "concurrency": 1}, ID(), true)
	assertStatus(t, 201, s, run)
	id := run["id"].(string)
	req(a, "benchmarks/runs/"+id+"/samples", M{"samples": []M{{"status": 409, "ms": 1}}}, ID(), true)
	key := ID()
	s, m := req(a, "benchmarks/runs/"+id+"/finish", M{"elapsed": 0.01}, key, true)
	assertStatus(t, 200, s, m)
	s, again := req(a, "benchmarks/runs/"+id+"/finish", M{"elapsed": 0.01}, key, true)
	assertStatus(t, 200, s, again)
	b, _ := json.Marshal(m)
	c, _ := json.Marshal(again)
	if !bytes.Equal(b, c) {
		t.Fatal("finish response changed on replay")
	}
}

func TestRedisThrottleAndOutageCannotBypassFloor(t *testing.T) {
	a := testApp(t)
	launch(t, a)
	ctx := context.Background()
	v := fixture(t, a, 3, false)
	s, m := reserve(a, v, ID())
	assertStatus(t, 201, s, m)
	if e := a.Redis.Set(ctx, "rate:192.0.2.1", 200, time.Second).Err(); e != nil {
		t.Fatal(e)
	}
	defer a.Redis.Del(ctx, "rate:192.0.2.1")
	s, m = req(a, "interest", M{"customer": "limited", "variant": v, "quantity": 1}, ID(), false)
	assertStatus(t, 429, s, m)
	original := a.Redis
	broken := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 10 * time.Millisecond, MaxRetries: -1})
	a.Redis = broken
	s, m = req(a, "reservations", M{"customer": "redis-down", "variant": v, "quantity": 1}, ID(), false)
	a.Redis = original
	broken.Close()
	assertStatus(t, 409, s, m)
	assertInvariant(t, a)
}
