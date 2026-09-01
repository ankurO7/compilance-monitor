package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ankurO7/compilance-monitor/internal/handlers"
	"github.com/ankurO7/compilance-monitor/internal/rules"
	"github.com/ankurO7/compilance-monitor/internal/store"
	"github.com/ankurO7/compilance-monitor/internal/worker"
)

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	st := store.NewInMemoryStore()

	ruleSet := []rules.Rule{
		rules.NewStructuringRule(),
		rules.NewVelocityRule(),
		rules.NewWatchlistRule([]string{
			"blocked-entity-1",
			"blocked-entity-2",
			"sanctioned-corp",
		}),
	}

	pool := worker.NewPool(8, 1000, st, ruleSet, newID)
	stopCh := make(chan struct{})
	pool.Start(stopCh)

	api := &handlers.API{Store: st, Pool: pool, IDFn: newID}
	mux := http.NewServeMux()
	api.Routes(mux)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("compliance-monitor listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("shutting down...")
	close(stopCh)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}