package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hibiken/asynq"
	"log"
	"time"
)

func (a *App) Sweep(ctx context.Context) error {
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if e = lock(ctx, tx); e != nil {
		return e
	}
	var paused bool
	var draft string
	if e = tx.QueryRow(ctx, "SELECT paused,draft FROM settings WHERE id=1").Scan(&paused, &draft); e != nil {
		return e
	}
	if paused {
		return nil
	}
	rows, e := Rows(ctx, tx, "SELECT id,kind,customer,expires FROM holds WHERE state='active' ORDER BY created FOR UPDATE")
	if e != nil {
		return e
	}
	now := time.Now()
	for _, r := range rows {
		h := r["id"].(string)
		c := r["customer"].(string)
		kind := r["kind"].(string)
		ex := r["expires"].(time.Time)
		if ex.After(now) {
			if kind == "online" && ex.Sub(now) <= a.Reminder {
				if e = notify(ctx, tx, h, c, "reminder", draft+" Pickup: BIZZNOVATE demo counter, ground floor. Bring your receipt."); e != nil {
					return e
				}
			}
			continue
		}
		if kind == "fitting" {
			if e = notify(ctx, tx, h, c, "overdue", "Fitting overdue: staff must confirm purchase or physical return. Stock remains protected."); e != nil {
				return e
			}
			continue
		}
		if _, e = tx.Exec(ctx, "UPDATE holds SET state='expired' WHERE id=$1 AND state='active'", h); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, "UPDATE units SET state='available' WHERE code IN(SELECT unit FROM hold_units WHERE hold=$1)", h); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, "UPDATE waitlist SET state='expired' WHERE hold=$1", h); e != nil {
			return e
		}
		if e = event(ctx, tx, "expiry_release", M{"hold": h}); e != nil {
			return e
		}
	}
	wait, e := Rows(ctx, tx, "SELECT id,customer,variant FROM waitlist WHERE state='waiting' ORDER BY id FOR UPDATE")
	if e != nil {
		return e
	}
	for _, w := range wait {
		n, on, e := available(ctx, tx, w["variant"].(string))
		if e != nil {
			return e
		}
		if n <= 2 || on < 1 {
			continue
		}
		_, m, e := a.allocate(ctx, tx, w["customer"].(string), w["variant"].(string), 1, "offer")
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, "UPDATE waitlist SET state='offered',hold=$2 WHERE id=$1", w["id"], m["id"]); e != nil {
			return e
		}
	}
	_, e = tx.Exec(ctx, "UPDATE settings SET worker_at=now() WHERE id=1")
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (a *App) Deliver(ctx context.Context, id string) error {
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if e = lock(ctx, tx); e != nil {
		return e
	}
	var paused bool
	if e = tx.QueryRow(ctx, "SELECT paused FROM settings WHERE id=1").Scan(&paused); e != nil {
		return e
	}
	if paused {
		return fmt.Errorf("worker paused")
	}
	// Reminder and offer delivery are conditional on current stored state.
	_, e = tx.Exec(ctx, `UPDATE notifications n SET delivered=true WHERE n.id=$1 AND NOT delivered AND (n.kind NOT IN ('reminder','offer','online') OR EXISTS(SELECT 1 FROM holds h WHERE h.id=n.hold AND h.state='active' AND h.expires>now() AND (n.kind<>'offer' OR h.kind='offer')))`, id)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (a *App) Dispatch(ctx context.Context, c *asynq.Client) error {
	rows, e := Rows(ctx, a.DB, "SELECT id,kind,payload,run_at FROM outbox WHERE NOT sent ORDER BY id LIMIT 100")
	if e != nil {
		return e
	}
	for _, r := range rows {
		b, _ := json.Marshal(r["payload"])
		_, e = c.EnqueueContext(ctx, asynq.NewTask(r["kind"].(string), b), asynq.TaskID(fmt.Sprintf("outbox-%v", r["id"])), asynq.ProcessAt(r["run_at"].(time.Time)), asynq.MaxRetry(20), asynq.Retention(24*time.Hour))
		if e != nil && !errors.Is(e, asynq.ErrTaskIDConflict) {
			return e
		}
		if _, e = a.DB.Exec(ctx, "UPDATE outbox SET sent=true WHERE id=$1", r["id"]); e != nil {
			return e
		}
	}
	return nil
}
func (a *App) Worker(ctx context.Context) error {
	opt := asynq.RedisClientOpt{Addr: Env("REDIS_ADDR", "127.0.0.1:56379"), DB: a.Redis.Options().DB}
	c := asynq.NewClient(opt)
	defer c.Close()
	server := asynq.NewServer(opt, asynq.Config{Concurrency: 8})
	mux := asynq.NewServeMux()
	mux.HandleFunc("sweep", func(ctx context.Context, t *asynq.Task) error { return a.Sweep(ctx) })
	mux.HandleFunc("notify", func(ctx context.Context, t *asynq.Task) error {
		var p struct {
			ID string `json:"id"`
		}
		if e := json.Unmarshal(t.Payload(), &p); e != nil {
			return e
		}
		return a.Deliver(ctx, p.ID)
	})
	if e := server.Start(mux); e != nil {
		return e
	}
	defer server.Shutdown()
	scheduler := asynq.NewScheduler(opt, nil)
	if _, e := scheduler.Register("@every 5s", asynq.NewTask("sweep", nil)); e != nil {
		return e
	}
	if e := scheduler.Start(); e != nil {
		return e
	}
	defer scheduler.Shutdown()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
			if e := a.Sweep(ctx); e != nil {
				log.Printf("recovery sweep: %v", e)
			}
			if e := a.Dispatch(ctx, c); e != nil {
				log.Printf("outbox retained: %v", e)
			}
		}
	}
}
