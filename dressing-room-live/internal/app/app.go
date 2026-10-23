package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

//go:embed schema.sql
var schema string

type App struct {
	DB                                         *pgxpool.Pool
	Redis                                      *redis.Client
	HoldTime, Reminder, OfferTime, FittingTime time.Duration
	StaffKey                                   string
	Demo                                       bool
}
type M = map[string]any

func Env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func ID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func Open(ctx context.Context) (*App, error) {
	cfg, e := pgxpool.ParseConfig(Env("DATABASE_URL", "postgres://human@127.0.0.1:55432/dressing?sslmode=disable"))
	if e != nil {
		return nil, e
	}
	cfg.MaxConns = 24
	db, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		return nil, e
	}
	if e = db.Ping(ctx); e != nil {
		return nil, e
	}
	redisDB, _ := strconv.Atoi(Env("REDIS_DB", "0"))
	a := &App{DB: db, Redis: redis.NewClient(&redis.Options{Addr: Env("REDIS_ADDR", "127.0.0.1:56379"), DB: redisDB, DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second}), StaffKey: Env("STAFF_KEY", "local-demo-staff"), Demo: Env("DEMO_MODE", "0") == "1", HoldTime: 24 * time.Hour, Reminder: 2 * time.Hour, OfferTime: 5 * time.Minute, FittingTime: 10 * time.Minute}
	if a.Demo {
		a.HoldTime = 120 * time.Second
		a.Reminder = 10 * time.Second
		a.OfferTime = 20 * time.Second
		a.FittingTime = 30 * time.Second
	}
	tx, e := db.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	if e = lock(ctx, tx); e != nil {
		return nil, e
	}
	if _, e = tx.Exec(ctx, schema); e != nil {
		return nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return a, a.seed(ctx)
}
func (a *App) Close() { a.DB.Close(); a.Redis.Close() }

type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func Rows(ctx context.Context, q querier, s string, args ...any) ([]M, error) {
	rows, e := q.Query(ctx, s, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToMap)
}
func (a *App) Query(ctx context.Context, s string, args ...any) []M {
	r, e := Rows(ctx, a.DB, s, args...)
	if e != nil {
		log.Printf("query: %v", e)
		return []M{}
	}
	if r == nil {
		return []M{}
	}
	return r
}
func event(ctx context.Context, tx pgx.Tx, kind string, v any) error {
	b, _ := json.Marshal(v)
	_, e := tx.Exec(ctx, "INSERT INTO events(kind,body) VALUES($1,$2)", kind, b)
	return e
}
func job(ctx context.Context, tx pgx.Tx, kind string, v any, at time.Time) error {
	b, _ := json.Marshal(v)
	_, e := tx.Exec(ctx, "INSERT INTO outbox(kind,payload,run_at) VALUES($1,$2,$3)", kind, b, at)
	return e
}
func notify(ctx context.Context, tx pgx.Tx, h, c, k, body string) error {
	id := ID()
	r, e := tx.Exec(ctx, "INSERT INTO notifications(id,hold,customer,kind,body) VALUES($1,NULLIF($2,''),$3,$4,$5) ON CONFLICT(hold,kind) DO NOTHING", id, h, c, k, body)
	if e != nil {
		return e
	}
	if r.RowsAffected() > 0 {
		return job(ctx, tx, "notify", M{"id": id}, time.Now())
	}
	return nil
}
func lock(ctx context.Context, tx pgx.Tx) error {
	_, e := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(210021)")
	return e
}

// A single transaction lock is deliberately conservative for this local demo. All
// stock-changing paths use it, including sweeps, staff actions and queue claims.
func (a *App) Mutate(ctx context.Context, key, path string, body []byte, fn func(pgx.Tx) (int, M, error)) (int, M) {
	if key == "" || len(key) > 200 {
		return 400, M{"error": "Idempotency-Key (1–200 characters) required"}
	}
	sum := sha256.Sum256(append([]byte(path), body...))
	fp := hex.EncodeToString(sum[:])
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return 503, M{"error": "Database unavailable"}
	}
	defer tx.Rollback(ctx)
	if e = lock(ctx, tx); e != nil {
		return 503, M{"error": "Database unavailable"}
	}
	var oldfp string
	var status int
	var saved []byte
	e = tx.QueryRow(ctx, "SELECT fingerprint,status,body FROM requests WHERE key=$1", key).Scan(&oldfp, &status, &saved)
	if e == nil {
		if fp != oldfp {
			return 409, M{"error": "Idempotency key reused with a different request"}
		}
		var m M
		json.Unmarshal(saved, &m)
		return status, m
	}
	if e != pgx.ErrNoRows {
		return 500, M{"error": "Request lookup failed"}
	}
	status, m, e := fn(tx)
	if e != nil {
		log.Printf("mutation %s: %v", path, e)
		return 500, M{"error": "Transaction failed"}
	}
	b, _ := json.Marshal(m)
	if _, e = tx.Exec(ctx, "INSERT INTO requests VALUES($1,$2,$3,$4)", key, fp, status, b); e != nil {
		return 500, M{"error": "Request save failed"}
	}
	eventPayload := any(m)
	if strings.HasSuffix(path, "/finish") {
		eventPayload = M{"run": path, "state": "finished"}
	}
	if e = event(ctx, tx, path, eventPayload); e != nil {
		return 500, M{"error": "Event save failed"}
	}
	if e = tx.Commit(ctx); e != nil {
		return 500, M{"error": "Commit failed"}
	}
	return status, m
}
func (a *App) seed(ctx context.Context) error {
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if e = lock(ctx, tx); e != nil {
		return e
	}
	names := []string{"Cloud heavyweight hoodie", "Midnight cargo pants", "Concrete oversized tee", "Afterhours varsity jacket", "Orbit sneakers", "Signal crossbody bag", "District denim jacket", "Static knit beanie"}
	prices := []int{2499, 2199, 999, 3999, 3499, 1499, 2999, 799}
	photos := []string{"hoodie-photo.jpg", "cargo.jpg", "tee.jpg", "varsity.jpg", "sneaker.jpg", "bag.jpg", "denim.jpg", "beanie.jpg"}
	for i, n := range names {
		p := fmt.Sprintf("DR%02d", i+1)
		_, e = tx.Exec(ctx, "INSERT INTO products VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING", p, n, prices[i], photos[i], "Illustrative category photograph · see asset credits", p+" "+n)
		if e != nil {
			return e
		}
		for _, s := range []string{"S", "M", "L", "XL"} {
			v := p + "-" + s
			r, e := tx.Exec(ctx, "INSERT INTO variants(id,product,size,initial_units) VALUES($1,$2,$3,6) ON CONFLICT DO NOTHING", v, p, s)
			if e != nil {
				return e
			}
			if r.RowsAffected() == 0 {
				continue
			}
			for u := 1; u <= 6; u++ {
				loc := "stockroom"
				if u == 6 {
					loc = "display"
				}
				_, e = tx.Exec(ctx, "INSERT INTO units(code,variant,location,online) VALUES($1,$2,$3,$4)", fmt.Sprintf("%s-%02d", v, u), v, loc, u != 6)
				if e != nil {
					return e
				}
			}
		}
	}
	return tx.Commit(ctx)
}
