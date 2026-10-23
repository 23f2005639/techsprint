package main

import (
	"context"
	"dressingroom/internal/load"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	target := flag.String("target", "http://127.0.0.1:8080", "loopback target")
	scenario := flag.String("scenario", "race", "race, five, floor, duplicate")
	count := flag.Int("requests", 100, "requests")
	concurrency := flag.Int("concurrency", 100, "goroutines")
	flag.Parse()
	key := os.Getenv("STAFF_KEY")
	if key == "" {
		key = "local-demo-staff"
	}
	m, e := load.Run(context.Background(), *target, key, *scenario, *count, *concurrency)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	json.NewEncoder(os.Stdout).Encode(m)
	if inv, ok := m["inventory"].(map[string]any); ok {
		for _, k := range []string{"no_duplicate_allocation", "protected_floor", "inventory_conserved"} {
			if inv[k] != true {
				os.Exit(2)
			}
		}
	}
}
