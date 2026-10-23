package app

import (
	"crypto/subtle"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

func write(w http.ResponseWriter, s int, m any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(s)
	json.NewEncoder(w).Encode(m)
}
func (a *App) staff(r *http.Request) bool {
	return subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Staff-Key")), []byte(a.StaffKey)) == 1
}

var rate = redis.NewScript(`local n=redis.call('INCR',KEYS[1]); if n==1 then redis.call('PEXPIRE',KEYS[1],1000) end; return n`)

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", a.api)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Clean(r.URL.Path)
		if strings.HasPrefix(p, "/assets/") {
			http.FileServer(http.Dir("web/dist")).ServeHTTP(w, r)
			return
		}
		if p != "/" && p != "/staff" && p != "/benchmarks" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "web/dist/index.html")
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != "GET" && r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+r.Host {
			write(w, 403, M{"error": "Same-origin requests required"})
			return
		}
		mux.ServeHTTP(w, r)
	})
}
func (a *App) api(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	path := strings.TrimPrefix(r.URL.Path, "/api/")
	priv := strings.HasPrefix(path, "staff") || strings.HasPrefix(path, "benchmarks") || path == "invariants" || path == "inventory" || strings.HasPrefix(path, "social/") && path != "social/site"
	if priv && !a.staff(r) {
		write(w, 401, M{"error": "Staff/demo credential required"})
		return
	}
	if strings.HasPrefix(path, "benchmarks") {
		a.benchAPI(w, r)
		return
	}
	if r.Method == "GET" {
		switch {
		case path == "health":
			write(w, 200, M{"ok": a.DB.Ping(ctx) == nil, "accelerated": a.Demo, "hold_seconds": a.HoldTime.Seconds()})
		case path == "catalog":
			write(w, 200, M{"products": a.Query(ctx, "SELECT * FROM products ORDER BY id"), "variants": a.Query(ctx, `SELECT v.*,count(u.*) FILTER(WHERE state='available')::int available, greatest(least(count(u.*) FILTER(WHERE state='available')-2,count(u.*) FILTER(WHERE state='available' AND online AND location='stockroom')),0)::int online FROM variants v LEFT JOIN units u ON u.variant=v.id GROUP BY v.id ORDER BY v.id`), "settings": a.Query(ctx, "SELECT launched FROM settings"), "accelerated": a.Demo})
		case path == "demand":
			write(w, 200, a.Query(ctx, "SELECT variant,sum(quantity)::int quantity,count(DISTINCT customer)::int customers FROM demand GROUP BY variant ORDER BY quantity DESC"))
		case path == "inventory":
			write(w, 200, M{"units": a.Query(ctx, "SELECT * FROM units ORDER BY code"), "holds": a.Query(ctx, "SELECT * FROM holds ORDER BY created DESC LIMIT 200"), "totals": a.Invariants(ctx), "settings": a.Query(ctx, "SELECT * FROM settings"), "waitlist": a.Query(ctx, "SELECT * FROM waitlist ORDER BY id"), "outbox": a.Query(ctx, "SELECT count(*) pending FROM outbox WHERE NOT sent")})
		case path == "invariants":
			write(w, 200, a.Invariants(ctx))
		case path == "notifications":
			write(w, 200, a.Query(ctx, "SELECT * FROM notifications WHERE customer=$1 AND delivered ORDER BY created DESC LIMIT 100", r.URL.Query().Get("customer")))
		case path == "holds":
			write(w, 200, a.Query(ctx, "SELECT * FROM holds WHERE customer=$1 ORDER BY created DESC LIMIT 100", r.URL.Query().Get("customer")))
		case strings.HasPrefix(path, "receipts/"):
			h := strings.TrimPrefix(path, "receipts/")
			rs := a.Query(ctx, "SELECT h.*,p.name,p.price,v.size FROM holds h JOIN variants v ON v.id=h.variant JOIN products p ON p.id=v.product WHERE h.id=$1", h)
			if len(rs) == 0 {
				write(w, 404, M{"error": "Receipt not found"})
				return
			}
			write(w, 200, M{"hold": rs[0], "units": a.Query(ctx, "SELECT unit FROM hold_units WHERE hold=$1", h), "directions": "BIZZNOVATE demo counter · ground floor · bring your receipt. Illustrative store; confirm venue with staff."})
		case path == "social/site":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(socialSite)
		case path == "social":
			write(w, 200, a.socialSummary(ctx))
		default:
			write(w, 404, M{"error": "API route not found"})
		}
		return
	}
	if r.Method != "POST" {
		write(w, 405, M{"error": "GET or POST required"})
		return
	}
	body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if e != nil {
		write(w, 400, M{"error": "Request too large"})
		return
	}
	var p struct {
		Customer string `json:"customer"`
		Variant  string `json:"variant"`
		Quantity int    `json:"quantity"`
		Code     string `json:"code"`
		Hold     string `json:"hold"`
		Action   string `json:"action"`
		Location string `json:"location"`
		Online   bool   `json:"online"`
		Launched bool   `json:"launched"`
		Paused   bool   `json:"paused"`
		Draft    string `json:"draft"`
		Posts    []Post `json:"posts"`
	}
	if e = json.Unmarshal(body, &p); e != nil {
		write(w, 400, M{"error": "Invalid JSON"})
		return
	}
	if !a.staff(r) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		n, e := rate.Run(ctx, a.Redis, []string{"rate:" + host}).Int()
		if e != nil {
			w.Header().Set("X-Rate-Limit-Status", "redis-unavailable")
		} else if n > 200 {
			write(w, 429, M{"error": "Please retry shortly"})
			return
		}
	}
	s, m := a.Mutate(ctx, r.Header.Get("Idempotency-Key"), path, body, func(tx pgx.Tx) (int, M, error) {
		switch path {
		case "reservations":
			var launched bool
			if e := tx.QueryRow(ctx, "SELECT launched FROM settings WHERE id=1").Scan(&launched); e != nil {
				return 0, nil, e
			}
			if !launched {
				return 409, M{"error": "Drop not launched; save your interest"}, nil
			}
			return a.allocate(ctx, tx, p.Customer, p.Variant, p.Quantity, "online")
		case "interest":
			if p.Customer == "" || p.Quantity < 1 || p.Quantity > 10 {
				return 400, M{"error": "Customer and quantity 1–10 required"}, nil
			}
			var exists bool
			if e := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM variants WHERE id=$1)", p.Variant).Scan(&exists); e != nil {
				return 0, nil, e
			}
			if !exists {
				return 400, M{"error": "Unknown variant"}, nil
			}
			id := ID()
			_, e := tx.Exec(ctx, "INSERT INTO demand(id,customer,variant,quantity) VALUES($1,$2,$3,$4)", id, p.Customer, p.Variant, p.Quantity)
			return 201, M{"id": id, "message": "Interest saved before launch"}, e
		case "waitlist":
			if p.Customer == "" {
				return 400, M{"error": "Customer required"}, nil
			}
			var exists bool
			if e := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM variants WHERE id=$1)", p.Variant).Scan(&exists); e != nil {
				return 0, nil, e
			}
			if !exists {
				return 400, M{"error": "Unknown variant"}, nil
			}
			var id int64
			e := tx.QueryRow(ctx, "INSERT INTO waitlist(customer,variant) VALUES($1,$2) ON CONFLICT(customer,variant) WHERE state='waiting' DO UPDATE SET customer=EXCLUDED.customer RETURNING id", p.Customer, p.Variant).Scan(&id)
			if e != nil {
				return 0, nil, e
			}
			return 201, M{"id": id, "message": "FIFO waitlist joined"}, job(ctx, tx, "sweep", M{}, time.Now())
		case "claim":
			return a.finish(ctx, tx, p.Hold, "claim")
		case "staff/fitting":
			return a.fitting(ctx, tx, p.Customer, p.Code)
		case "staff/checkout":
			return a.finish(ctx, tx, p.Hold, "checkout")
		case "staff/return":
			return a.finish(ctx, tx, p.Hold, "return")
		case "staff/transfer":
			if p.Location != "display" && p.Location != "stockroom" {
				return 400, M{"error": "Choose stockroom or display"}, nil
			}
			r, e := tx.Exec(ctx, "UPDATE units SET location=$2,online=$3 WHERE code=$1 AND state='available'", p.Code, p.Location, p.Online && p.Location == "stockroom")
			if e != nil {
				return 0, nil, e
			}
			if r.RowsAffected() == 0 {
				return 409, M{"error": "Scan an available unit before transfer"}, nil
			}
			return 200, M{"code": p.Code, "message": "Physical placement confirmed"}, job(ctx, tx, "sweep", M{}, time.Now())
		case "staff/settings":
			if len(p.Draft) > 1000 {
				return 400, M{"error": "Draft too long"}, nil
			}
			_, e := tx.Exec(ctx, "UPDATE settings SET launched=$1,paused=$2,draft=CASE WHEN $3='' THEN draft ELSE $3 END WHERE id=1", p.Launched, p.Paused, p.Draft)
			return 200, M{"message": "Controls saved"}, e
		case "social/scrape":
			posts, e := ScrapeSocial()
			if e != nil {
				return 0, nil, e
			}
			return a.importSocial(ctx, tx, posts)
		case "social/import":
			return a.importSocial(ctx, tx, p.Posts)
		default:
			return 404, M{"error": "Mutation route not found"}, nil
		}
	})
	write(w, s, m)
}
