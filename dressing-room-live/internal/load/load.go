package load

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"
)

type Sample struct {
	Status int     `json:"status"`
	MS     float64 `json:"ms"`
}

func key() string { b := make([]byte, 16); rand.Read(b); return hex.EncodeToString(b) }
func Run(ctx context.Context, target, credential, scenario string, count, concurrency int) (map[string]any, error) {
	u, e := url.Parse(target)
	if e != nil || u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1") {
		return nil, fmt.Errorf("benchmark target must be loopback HTTP")
	}
	if count < 1 || count > 100000 || concurrency < 1 || concurrency > 500 {
		return nil, fmt.Errorf("invalid count/concurrency")
	}
	transport := &http.Transport{MaxConnsPerHost: concurrency + 2, MaxIdleConnsPerHost: concurrency + 2, DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		_, port, e := net.SplitHostPort(addr)
		if e != nil {
			return nil, e
		}
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", port))
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	post := func(path string, p any, k string) (int, map[string]any, error) {
		b, _ := json.Marshal(p)
		req, e := http.NewRequestWithContext(ctx, "POST", target+path, bytes.NewReader(b))
		if e != nil {
			return 0, nil, e
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", k)
		req.Header.Set("X-Staff-Key", credential)
		resp, e := client.Do(req)
		if e != nil {
			return 0, nil, e
		}
		defer resp.Body.Close()
		var m map[string]any
		json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&m)
		return resp.StatusCode, m, nil
	}
	s, run, e := post("/api/benchmarks/runs", map[string]any{"scenario": scenario, "requests": count, "concurrency": concurrency}, key())
	if e != nil {
		return nil, e
	}
	if s != 201 {
		return nil, fmt.Errorf("create run %d: %v", s, run)
	}
	id := run["id"].(string)
	variant := run["variant"].(string)
	jobs := make(chan int)
	out := make(chan Sample, count)
	start := time.Now()
	var wg sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				k := id + fmt.Sprint(i)
				customer := fmt.Sprintf("runner-%s-%d", id, i)
				if scenario == "duplicate" {
					k = id + "-duplicate"
					customer = "duplicate-" + id
				}
				t := time.Now()
				s, _, e := post("/api/reservations", map[string]any{"customer": customer, "variant": variant, "quantity": 1}, k)
				if e != nil {
					s = 0
				}
				out <- Sample{s, float64(time.Since(t).Microseconds()) / 1000}
			}
		}()
	}
	go func() {
		for i := 0; i < count; i++ {
			jobs <- i
		}
		close(jobs)
		wg.Wait()
		close(out)
	}()
	samples := []Sample{}
	all := []Sample{}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	flush := func() error {
		if len(samples) == 0 {
			return nil
		}
		s, m, e := post("/api/benchmarks/runs/"+id+"/samples", map[string]any{"samples": samples}, key())
		if e != nil {
			return e
		}
		if s != 200 {
			return fmt.Errorf("samples %d: %v", s, m)
		}
		samples = nil
		return nil
	}
	for collecting := true; collecting; {
		select {
		case sample, ok := <-out:
			if !ok {
				collecting = false
				break
			}
			samples = append(samples, sample)
			all = append(all, sample)
		case <-ticker.C:
			if e = flush(); e != nil {
				return nil, e
			}
		}
	}
	elapsed := time.Since(start).Seconds()
	if e = flush(); e != nil {
		return nil, e
	}
	s, result, e := post("/api/benchmarks/runs/"+id+"/finish", map[string]any{"elapsed": elapsed}, key())
	if e != nil {
		return nil, e
	}
	if s != 200 {
		return nil, fmt.Errorf("finish %d: %v", s, result)
	}
	verifyReq, e := http.NewRequestWithContext(ctx, "GET", target+"/api/invariants", nil)
	if e != nil {
		return nil, e
	}
	verifyReq.Header.Set("X-Staff-Key", credential)
	verifyResp, e := client.Do(verifyReq)
	if e != nil {
		return nil, e
	}
	defer verifyResp.Body.Close()
	var verified map[string]any
	if e = json.NewDecoder(verifyResp.Body).Decode(&verified); e != nil {
		return nil, e
	}
	if verifyResp.StatusCode != 200 {
		return nil, fmt.Errorf("invariant API status %d", verifyResp.StatusCode)
	}
	result["verified_inventory"] = verified
	for _, k := range []string{"no_duplicate_allocation", "protected_floor", "inventory_conserved"} {
		if verified[k] != true {
			return result, fmt.Errorf("authoritative invariant failed: %s", k)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].MS < all[j].MS })
	result["client_summary"] = map[string]any{"requests": len(all), "elapsed_seconds": elapsed, "rps": float64(len(all)) / elapsed, "p50": all[(len(all)-1)*50/100].MS, "p95": all[(len(all)-1)*95/100].MS, "p99": all[(len(all)-1)*99/100].MS}
	return result, nil
}
