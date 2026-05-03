package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	version = "unknown"
	commit  = "unknown"
)

func main() {
	cfg := loadConfig()
	srv := buildServer(cfg)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		log.Printf("vexd %s (%s) starting on :%s", version, commit, cfg.port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("vexd: listen: %v", err)
		}
	}()

	<-stop
	log.Println("vexd: shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("vexd: shutdown: %v", err)
	}

	log.Println("vexd: stopped")
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("error getting user home directory: %v", err)
	}
	return home
}
