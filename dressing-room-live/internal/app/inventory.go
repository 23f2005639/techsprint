package app

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"time"
)

func available(ctx context.Context, tx pgx.Tx, v string) (int, int, error) {
	var n, on int
	e := tx.QueryRow(ctx, "SELECT count(*),count(*) FILTER(WHERE location='stockroom' AND online) FROM units WHERE variant=$1 AND state='available'", v).Scan(&n, &on)
	return n, on, e
}
func (a *App) allocate(ctx context.Context, tx pgx.Tx, c, v string, qty int, kind string) (int, M, error) {
	if c == "" || len(c) > 80 || qty < 1 || qty > 10 {
		return 400, M{"error": "Customer and quantity 1–10 required"}, nil
	}
	var exists bool
	if e := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM variants WHERE id=$1)", v).Scan(&exists); e != nil {
		return 0, nil, e
	}
	if !exists {
		return 400, M{"error": "Unknown size variant"}, nil
	}
	if kind == "online" {
		var queued bool
		if e := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM waitlist WHERE variant=$1 AND state='waiting')", v).Scan(&queued); e != nil {
			return 0, nil, e
		}
		if queued {
			return 409, M{"error": "FIFO queue has priority; join the waitlist"}, nil
		}
	}
	n, on, e := available(ctx, tx, v)
	if e != nil {
		return 0, nil, e
	}
	if n-qty < 2 || on < qty {
		return 409, M{"error": "Available in-store only", "available": n}, nil
	}
	rows, e := Rows(ctx, tx, "SELECT code FROM units WHERE variant=$1 AND state='available' AND location='stockroom' AND online ORDER BY code LIMIT $2 FOR UPDATE", v, qty)
	if e != nil {
		return 0, nil, e
	}
	h := ID()
	duration := a.HoldTime
	if kind == "offer" {
		duration = a.OfferTime
	}
	expires := time.Now().Add(duration)
	_, e = tx.Exec(ctx, "INSERT INTO holds(id,customer,variant,kind,state,expires) VALUES($1,$2,$3,$4,'active',$5)", h, c, v, kind, expires)
	if e != nil {
		return 0, nil, e
	}
	for _, r := range rows {
		if _, e = tx.Exec(ctx, "UPDATE units SET state=$1 WHERE code=$2", kind, r["code"]); e != nil {
			return 0, nil, e
		}
		if _, e = tx.Exec(ctx, "INSERT INTO hold_units VALUES($1,$2)", h, r["code"]); e != nil {
			return 0, nil, e
		}
	}
	if _, e = tx.Exec(ctx, "INSERT INTO audit(hold,available_after) VALUES($1,$2)", h, n-qty); e != nil {
		return 0, nil, e
	}
	if e = job(ctx, tx, "sweep", M{"hold": h}, expires); e != nil {
		return 0, nil, e
	}
	if kind == "online" {
		if e = job(ctx, tx, "sweep", M{"hold": h}, expires.Add(-a.Reminder)); e != nil {
			return 0, nil, e
		}
	}
	if e = notify(ctx, tx, h, c, kind, "Your "+kind+" hold is ready. Receipt "+h+". Pickup: BIZZNOVATE demo counter, ground floor. Bring your receipt; this is an illustrative local store."); e != nil {
		return 0, nil, e
	}
	return 201, M{"id": h, "customer": c, "variant": v, "state": "active", "kind": kind, "expires": expires, "units": rows, "receipt": "/api/receipts/" + h}, nil
}
func (a *App) fitting(ctx context.Context, tx pgx.Tx, c, code string) (int, M, error) {
	if c == "" {
		return 400, M{"error": "Customer required"}, nil
	}
	var state, v string
	e := tx.QueryRow(ctx, "SELECT state,variant FROM units WHERE code=$1 FOR UPDATE", code).Scan(&state, &v)
	if e == pgx.ErrNoRows {
		return 404, M{"error": "Scan code not found"}, nil
	}
	if e != nil {
		return 0, nil, e
	}
	if state != "available" {
		return 409, M{"error": "Unit already held or sold"}, nil
	}
	h := ID()
	ex := time.Now().Add(a.FittingTime)
	_, e = tx.Exec(ctx, "INSERT INTO holds(id,customer,variant,kind,state,expires) VALUES($1,$2,$3,'fitting','active',$4)", h, c, v, ex)
	if e != nil {
		return 0, nil, e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO hold_units VALUES($1,$2)", h, code); e != nil {
		return 0, nil, e
	}
	if _, e = tx.Exec(ctx, "UPDATE units SET state='fitting' WHERE code=$1", code); e != nil {
		return 0, nil, e
	}
	return 201, M{"id": h, "kind": "fitting", "expires": ex}, job(ctx, tx, "sweep", M{"hold": h}, ex)
}
func (a *App) finish(ctx context.Context, tx pgx.Tx, h, action string) (int, M, error) {
	var kind, state, c string
	var ex time.Time
	e := tx.QueryRow(ctx, "SELECT kind,state,customer,expires FROM holds WHERE id=$1 FOR UPDATE", h).Scan(&kind, &state, &c, &ex)
	if e == pgx.ErrNoRows {
		return 404, M{"error": "Hold not found"}, nil
	}
	if e != nil {
		return 0, nil, e
	}
	if state != "active" {
		return 409, M{"error": "Hold already " + state}, nil
	}
	target, unit := "collected", "sold"
	if action == "return" {
		if kind != "fitting" {
			return 409, M{"error": "Only a fitting unit can be physically returned"}, nil
		}
		target = "returned"
		unit = "available"
	} else if action == "claim" {
		if kind != "offer" || !ex.After(time.Now()) {
			return 409, M{"error": "Offer expired or not claimable"}, nil
		}
		ex = time.Now().Add(a.HoldTime)
		_, e = tx.Exec(ctx, "UPDATE holds SET kind='online',expires=$2 WHERE id=$1", h, ex)
		if e != nil {
			return 0, nil, e
		}
		_, e = tx.Exec(ctx, "UPDATE units SET state='online' WHERE code IN(SELECT unit FROM hold_units WHERE hold=$1)", h)
		if e != nil {
			return 0, nil, e
		}
		if e = job(ctx, tx, "sweep", M{"hold": h}, ex.Add(-a.Reminder)); e != nil {
			return 0, nil, e
		}
		return 200, M{"id": h, "kind": "online", "expires": ex}, job(ctx, tx, "sweep", M{"hold": h}, ex)
	} else if kind == "offer" || kind == "online" && !ex.After(time.Now()) {
		return 409, M{"error": "Hold expired or offer not claimed"}, nil
	}
	if _, e = tx.Exec(ctx, "UPDATE holds SET state=$2 WHERE id=$1", h, target); e != nil {
		return 0, nil, e
	}
	if _, e = tx.Exec(ctx, "UPDATE units SET state=$2 WHERE code IN(SELECT unit FROM hold_units WHERE hold=$1)", h, unit); e != nil {
		return 0, nil, e
	}
	if e = notify(ctx, tx, h, c, target, fmt.Sprintf("Receipt %s: %s. Thank you from BIZZNOVATE.", h, target)); e != nil {
		return 0, nil, e
	}
	if e = job(ctx, tx, "sweep", M{}, time.Now()); e != nil {
		return 0, nil, e
	}
	return 200, M{"id": h, "state": target}, nil
}
func (a *App) Invariants(ctx context.Context) M {
	rows := a.Query(ctx, `SELECT (SELECT count(*) FROM units) total,(SELECT count(*) FROM units WHERE state='available') available,(SELECT count(*) FROM units WHERE state='online') online_held,(SELECT count(*) FROM units WHERE state='fitting') fitting_held,(SELECT count(*) FROM units WHERE state='offer') offers,(SELECT count(*) FROM units WHERE state='sold') sold`)
	m := M{}
	if len(rows) > 0 {
		m = rows[0]
	}
	p := a.Query(ctx, "SELECT coalesce(sum(least(n,2)),0)::int protected FROM (SELECT count(*) n FROM units WHERE state='available' GROUP BY variant) z")
	if len(p) > 0 {
		m["protected"] = p[0]["protected"]
	}
	bad := a.Query(ctx, `SELECT count(*) n FROM (SELECT hu.unit FROM hold_units hu JOIN holds h ON h.id=hu.hold WHERE h.state='active' GROUP BY hu.unit HAVING count(*)>1) z`)
	mismatch := a.Query(ctx, `SELECT count(*) n FROM units u WHERE (u.state IN ('online','fitting','offer') AND NOT EXISTS(SELECT 1 FROM hold_units hu JOIN holds h ON h.id=hu.hold WHERE hu.unit=u.code AND h.state='active' AND h.kind=u.state)) OR (u.state IN ('available','sold') AND EXISTS(SELECT 1 FROM hold_units hu JOIN holds h ON h.id=hu.hold WHERE hu.unit=u.code AND h.state='active'))`)
	floor := a.Query(ctx, "SELECT count(*) n FROM audit WHERE available_after<2")
	m["no_duplicate_allocation"] = len(bad) > 0 && bad[0]["n"] == int64(0)
	physical := a.Query(ctx, "SELECT count(*) n FROM variants v WHERE initial_units<>(SELECT count(*) FROM units u WHERE u.variant=v.id)")
	m["inventory_conserved"] = len(mismatch) > 0 && mismatch[0]["n"] == int64(0) && len(physical) > 0 && physical[0]["n"] == int64(0)
	m["protected_floor"] = len(floor) > 0 && floor[0]["n"] == int64(0)
	return m
}
