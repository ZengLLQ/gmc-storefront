package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gmc/api/internal/gen/openapi"
	"gmc/api/internal/platform"
	"gmc/api/internal/server"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		runHealthcheck()
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := platform.LoadConfig()
	if err != nil {
		logger.Error("failed to load configuration", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		logger.Error("invalid DATABASE_URL", "err", err)
		os.Exit(1)
	}
	poolCfg.MaxConns = 20
	poolCfg.MinConns = 2
	poolCfg.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		logger.Error("failed to connect to postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	storage, err := platform.NewLocalStorage(cfg.UploadDir)
	if err != nil {
		logger.Error("failed to initialize local upload storage", "err", err)
		os.Exit(1)
	}

	metrics := &platform.MetricsCollector{}
	srv := server.NewServer(pool, cfg, storage, metrics)
	srv.StartAutoCancelWorker(ctx)

	r := BuildRouter(srv, cfg, metrics)

	addr := fmt.Sprintf(":%d", cfg.APIPort)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		logger.Info("starting GMC API server", "addr", addr, "env", cfg.AppEnv)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down GMC API server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
	}
	logger.Info("GMC API server stopped cleanly")
}

func runHealthcheck() {
	port := strings.TrimSpace(os.Getenv("API_PORT"))
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
	os.Exit(0)
}

func BuildRouter(srv *server.Server, cfg *platform.Config, metrics *platform.MetricsCollector) http.Handler {
	r := chi.NewRouter()
	r.Use(platform.StandardMiddlewares(cfg, metrics))

	r.Get("/metrics", srv.ServeMetrics)
	r.Get("/uploads/{id}/{size}", srv.ServeUploads)

	openapi.HandlerWithOptions(srv, openapi.ChiServerOptions{
		BaseRouter: r,
		ErrorHandlerFunc: func(w http.ResponseWriter, req *http.Request, err error) {
			platform.WriteError(w, req, platform.NewAppError(http.StatusBadRequest, openapi.BadRequest, err.Error()))
		},
	})

	return r
}
