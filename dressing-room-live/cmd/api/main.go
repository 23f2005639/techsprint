package main

import (
	"context"
	"dressingroom/internal/app"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	a, e := app.Open(ctx)
	if e != nil {
		log.Fatal(e)
	}
	defer a.Close()
	s := &http.Server{Addr: "127.0.0.1:8080", Handler: a.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		c, x := context.WithTimeout(context.Background(), 5*time.Second)
		defer x()
		s.Shutdown(c)
	}()
	log.Println("BIZZNOVATE http://127.0.0.1:8080 · /staff · /benchmarks")
	if e = s.ListenAndServe(); e != http.ErrServerClosed {
		log.Fatal(e)
	}
}
