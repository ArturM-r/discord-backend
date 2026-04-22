package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/you/discord-backend/internal/handler"
	"github.com/you/discord-backend/internal/store"
	"github.com/you/discord-backend/internal/ws"
)

func main() {
	_ = godotenv.Load()

	dsn := env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/discord?sslmode=disable")
	secret := env("JWT_SECRET", "change-me-in-production")
	addr := env("ADDR", ":8080")

	db, err := store.New(dsn)
	if err != nil {
		log.Fatalf("store.New: %v", err)
	}
	defer db.Close()

	hub := ws.NewHub()
	h := handler.New(db, hub, secret)
	srv := &http.Server{
		Addr:         addr,
		Handler:      h.Routes(secret),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
	log.Println("done")
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
