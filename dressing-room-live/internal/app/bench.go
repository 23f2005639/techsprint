package app

import (
	"context"
	"dressingroom/internal/load"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (a *App) RunSnapshot(ctx context.Context, id string) M {
	runs := a.Query(ctx, "SELECT * FROM runs WHERE id=$1", id)
	if len(runs) == 0 {
		return M{"error": "Run not found"}
	}
	run := runs[0]
	if run["status"] == "finished" && run["final"] != nil {
		if final, ok := run["final"].(map[string]interface{}); ok {
			return final
		}
	}
	m := M{"run": run, "inventory": a.Invariants(ctx), "events": a.Query(ctx, "SELECT * FROM events ORDER BY id DESC LIMIT 50")}
	s := a.Query(ctx, `SELECT count(*)::int completed,count(*) FILTER(WHERE status=201)::int successful,count(*) FILTER(WHERE status=409)::int conflicts,count(*) FILTER(WHERE status=429)::int rate_limits,count(*) FILTER(WHERE status NOT IN (201,409,429))::int failures,coalesce(percentile_cont(0.5) WITHIN GROUP(ORDER BY ms),0) p50,coalesce(percentile_cont(0.95) WITHIN GROUP(ORDER BY ms),0) p95,coalesce(percentile_cont(0.99) WITHIN GROUP(ORDER BY ms),0) p99,count(*) FILTER(WHERE created>now()-interval '1 second')::int live_rps FROM samples WHERE run=$1`, id)
	if len(s) > 0 {
		m["metrics"] = s[0]
		h := a.Query(ctx, "SELECT count(*)::int unique_holds FROM holds WHERE variant=$1", "BENCH-"+id)
		if len(h) > 0 {
			s[0]["unique_holds"] = h[0]["unique_holds"]
		}
	}
	pool := a.DB.Stat()
	m["pool"] = M{"acquired": pool.AcquiredConns(), "total": pool.TotalConns(), "max": pool.MaxConns()}
	m["worker"] = a.Query(ctx, "SELECT worker_at,paused,(worker_at>now()-interval '5 seconds' AND NOT paused) healthy FROM settings")
	m["outbox"] = a.Query(ctx, "SELECT count(*) pending FROM outbox WHERE NOT sent")
	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: Env("REDIS_ADDR", "127.0.0.1:56379"), DB: a.Redis.Options().DB})
	defer inspector.Close()
	if q, e := inspector.GetQueueInfo("default"); e == nil {
		m["queue"] = M{"pending": q.Pending, "active": q.Active, "scheduled": q.Scheduled, "retry": q.Retry}
	} else {
		m["queue"] = M{"unavailable": true}
	}
	return m
}
func (a *App) benchAPI(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	path := strings.TrimPrefix(r.URL.Path, "/api/benchmarks/")
	parts := strings.Split(path, "/")
	if r.Method == "GET" {
		if path == "runs" {
			write(w, 200, a.Query(ctx, "SELECT id,scenario,requests,concurrency,status,started,finished FROM runs ORDER BY started DESC LIMIT 30"))
			return
		}
		if len(parts) < 2 {
			write(w, 404, M{"error": "Run not found"})
			return
		}
		id := parts[1]
		if len(parts) > 2 && parts[2] == "events" {
			f, ok := w.(http.Flusher)
			if !ok {
				write(w, 500, M{"error": "Streaming unavailable"})
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("X-Accel-Buffering", "no")
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			for {
				b, _ := json.Marshal(a.RunSnapshot(ctx, id))
				if _, e := fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", b); e != nil {
					return
				}
				f.Flush()
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}
		if len(parts) > 2 && parts[2] == "export.csv" {
			w.Header().Set("Content-Type", "text/csv")
			w.Header().Set("Content-Disposition", "attachment; filename="+id+".csv")
			c := csv.NewWriter(w)
			c.Write([]string{"run", "status", "latency_ms", "recorded_at"})
			for _, v := range a.Query(ctx, "SELECT * FROM samples WHERE run=$1 ORDER BY id", id) {
				c.Write([]string{id, fmt.Sprint(v["status"]), fmt.Sprint(v["ms"]), fmt.Sprint(v["created"])})
			}
			c.Flush()
			return
		}
		write(w, 200, a.RunSnapshot(ctx, id))
		return
	}
	if r.Method != "POST" {
		write(w, 405, M{"error": "POST required"})
		return
	}
	var p struct {
		Scenario    string `json:"scenario"`
		Requests    int    `json:"requests"`
		Concurrency int    `json:"concurrency"`
		Samples     []struct {
			Status int     `json:"status"`
			MS     float64 `json:"ms"`
		} `json:"samples"`
		Elapsed float64 `json:"elapsed"`
	}
	raw := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if e := raw.Decode(&p); e != nil {
		write(w, 400, M{"error": "Invalid JSON"})
		return
	}
	if path == "start" {
		if p.Requests < 1 || p.Requests > 100000 || p.Concurrency < 1 || p.Concurrency > 500 {
			write(w, 400, M{"error": "Requests 1–100000; concurrency 1–500"})
			return
		}
		if !a.Demo {
			write(w, 403, M{"error": "Benchmark fixtures require accelerated demo mode"})
			return
		}
		b, _ := json.Marshal(p)
		startNow := false
		s, m := a.Mutate(ctx, r.Header.Get("Idempotency-Key"), path, b, func(tx pgx.Tx) (int, M, error) {
			startNow = true
			return 202, M{"message": "Generator starting; awaiting observed requests"}, nil
		})
		if startNow && s == 202 {
			go func() {
				if _, e := load.Run(context.Background(), "http://127.0.0.1:8080", a.StaffKey, p.Scenario, p.Requests, p.Concurrency); e != nil {
					log.Printf("benchmark generator: %v", e)
					b, _ := json.Marshal(M{"error": e.Error()})
					a.DB.Exec(context.Background(), "INSERT INTO events(kind,body) VALUES('benchmark_error',$1)", b)
				}
			}()
		}
		write(w, s, m)
		return
	}
	// Read telemetry before taking the mutation lock, so pool readers never
	// wait for a connection while all other connections wait for this transaction.
	var finalSnapshot M
	expectedRequests := 0
	if len(parts) > 2 && parts[2] == "finish" {
		finalSnapshot = a.RunSnapshot(ctx, parts[1])
		expectedRequests = intFromRun(a, ctx, parts[1])
	}
	b, _ := json.Marshal(p)
	s, m := a.Mutate(ctx, r.Header.Get("Idempotency-Key"), path, b, func(tx pgx.Tx) (int, M, error) {
		if path == "runs" {
			if !a.Demo {
				return 403, M{"error": "Benchmark fixtures require DEMO_MODE=1"}, nil
			}
			if p.Requests < 1 || p.Requests > 100000 || p.Concurrency < 1 || p.Concurrency > 500 {
				return 400, M{"error": "Invalid request count or concurrency"}, nil
			}
			if p.Scenario != "race" && p.Scenario != "floor" && p.Scenario != "five" && p.Scenario != "duplicate" {
				return 400, M{"error": "Use race, five, floor or duplicate"}, nil
			}
			var active bool
			if e := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM runs WHERE status='running' AND started>now()-interval '10 minutes')").Scan(&active); e != nil {
				return 0, nil, e
			}
			if active {
				return 409, M{"error": "A benchmark is already running"}, nil
			}
			id := ID()
			v := "BENCH-" + id
			_, e := tx.Exec(ctx, "INSERT INTO variants(id,product,size) VALUES($1,'DR01',$2)", v, "run-"+id[:6])
			if e != nil {
				return 0, nil, e
			}
			n := 3
			if p.Scenario == "five" {
				n = 5
			}
			if p.Scenario == "floor" {
				n = 2
			}
			if _, e = tx.Exec(ctx, "UPDATE variants SET initial_units=$2 WHERE id=$1", v, n); e != nil {
				return 0, nil, e
			}
			for i := 1; i <= n; i++ {
				if _, e = tx.Exec(ctx, "INSERT INTO units(code,variant,location,online) VALUES($1,$2,'stockroom',true)", v+"-"+strconv.Itoa(i), v); e != nil {
					return 0, nil, e
				}
			}
			if _, e = tx.Exec(ctx, "INSERT INTO runs(id,scenario,requests,concurrency) VALUES($1,$2,$3,$4)", id, p.Scenario, p.Requests, p.Concurrency); e != nil {
				return 0, nil, e
			}
			if _, e = tx.Exec(ctx, "UPDATE settings SET launched=true WHERE id=1"); e != nil {
				return 0, nil, e
			}
			return 201, M{"id": id, "variant": v}, nil
		}
		if len(parts) < 3 {
			return 404, M{"error": "Unknown benchmark route"}, nil
		}
		id := parts[1]
		var status string
		if e := tx.QueryRow(ctx, "SELECT status FROM runs WHERE id=$1", id).Scan(&status); e != nil {
			return 404, M{"error": "Run not found"}, nil
		}
		if status != "running" {
			return 409, M{"error": "Run is already final"}, nil
		}
		if parts[2] == "samples" {
			for _, v := range p.Samples {
				if v.MS < 0 || v.MS > 3600000 {
					return 400, M{"error": "Invalid latency"}, nil
				}
				if _, e := tx.Exec(ctx, "INSERT INTO samples(run,status,ms) VALUES($1,$2,$3)", id, v.Status, v.MS); e != nil {
					return 0, nil, e
				}
			}
			return 200, M{"recorded": len(p.Samples)}, nil
		}
		if parts[2] == "finish" {
			if p.Elapsed <= 0 {
				return 400, M{"error": "Positive observed elapsed time required"}, nil
			}
			snapshot := finalSnapshot
			if metrics, ok := snapshot["metrics"].(M); ok {
				if int(metrics["completed"].(int32)) != expectedRequests {
					return 409, M{"error": "Run samples are incomplete"}, nil
				}
				metrics["rps"] = float64(metrics["completed"].(int32)) / p.Elapsed
			}
			if run, ok := snapshot["run"].(M); ok {
				run["status"] = "finished"
				run["finished"] = time.Now()
				delete(run, "final")
			}
			snapshot["elapsed_seconds"] = p.Elapsed
			b, _ := json.Marshal(snapshot)
			if _, e := tx.Exec(ctx, "UPDATE runs SET status='finished',finished=now(),final=$2 WHERE id=$1", id, b); e != nil {
				return 0, nil, e
			}
			return 200, snapshot, nil
		}
		return 404, M{"error": "Unknown benchmark route"}, nil
	})
	write(w, s, m)
}

func intFromRun(a *App, ctx context.Context, id string) int {
	var n int
	a.DB.QueryRow(ctx, "SELECT requests FROM runs WHERE id=$1", id).Scan(&n)
	return n
}
