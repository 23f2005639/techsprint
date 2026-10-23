package main

import (
	"context"
	"dressingroom/internal/app"
	"log"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	a, e := app.Open(ctx)
	if e != nil {
		log.Fatal(e)
	}
	defer a.Close()
	if e = a.Worker(ctx); e != nil {
		log.Fatal(e)
	}
}
