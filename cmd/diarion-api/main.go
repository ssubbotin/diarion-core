// Package main implements diarion-api, the Diarion HTTP API service.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/diarion/diarion-core/internal/auth/oauth"
	"github.com/diarion/diarion-core/internal/auth/session"
	"github.com/diarion/diarion-core/internal/config"
	"github.com/diarion/diarion-core/internal/db"
	"github.com/diarion/diarion-core/internal/db/dbgen"
	"github.com/diarion/diarion-core/internal/handlers/agents"
	"github.com/diarion/diarion-core/internal/handlers/auth"
	"github.com/diarion/diarion-core/internal/handlers/entries"
	"github.com/diarion/diarion-core/internal/handlers/me"
	"github.com/diarion/diarion-core/internal/markdown"
	"github.com/diarion/diarion-core/internal/signing"
	authmw "github.com/diarion/diarion-core/internal/middleware/auth"
	diarionredis "github.com/diarion/diarion-core/internal/store/redis"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		slog.Error("diarion-api fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config load: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := db.Migrate(ctx, cfg.DatabaseURL); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db pool: %w", err)
	}
	defer pool.Close()

	rClient, err := diarionredis.NewClient(ctx, cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	defer func() { _ = rClient.Close() }()

	queries := dbgen.New(pool)
	secureCookie := strings.HasPrefix(cfg.BaseURL, "https://")
	sessions := session.NewManager(queries, secureCookie)

	provider := oauth.NewGoogleProvider(
		cfg.GoogleOAuthClientID,
		cfg.GoogleOAuthClientSecret,
		strings.TrimRight(cfg.BaseURL, "/")+"/auth/google/callback",
	)

	authHandlers := auth.New(provider, sessions, queries, rClient)
	meHandlers := me.New(queries)
	agentHandlers := agents.New(queries, cfg.DiarionMasterKey)

	verifier := signing.NewVerifier(signing.NewDBKeyFetcher(queries))
	entryHandlers := entries.New(queries, verifier, markdown.Render)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(authmw.Middleware(queries, sessions))

	r.Get("/healthz", healthz)
	r.Get("/readyz", readyz(pool, rClient))

	r.Get("/auth/google", authHandlers.GoogleStart)
	r.Get("/auth/google/callback", authHandlers.GoogleCallback)
	r.Post("/auth/logout", authHandlers.Logout)
	r.Post("/auth/logout-all", authHandlers.LogoutAll)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/me", meHandlers.Get)
		r.Post("/me/tokens", meHandlers.CreateToken)
		r.Get("/me/tokens", meHandlers.ListTokens)
		r.Delete("/me/tokens/{id}", meHandlers.RevokeToken)
		agentHandlers.Register(r)
		entryHandlers.Register(r)
	})

	srv := &http.Server{
		Addr:              cfg.APIListen,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown", "err", err)
		}
	}()

	slog.Info("diarion-api starting", "addr", cfg.APIListen, "base_url", cfg.BaseURL, "secure_cookies", secureCookie)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server: %w", err)
	}
	slog.Info("diarion-api exited cleanly")
	return nil
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func readyz(pool *pgxpool.Pool, rClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pingCtx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
		defer cancel()
		if err := pool.Ping(pingCtx); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"db": err.Error()})
			return
		}
		if err := rClient.Ping(pingCtx).Err(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"redis": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	}
}
