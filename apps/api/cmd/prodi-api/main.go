package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"prodi/api/internal/config"
	"prodi/api/internal/httpapi"
	"prodi/api/internal/hub"
	"prodi/api/internal/store"
	"prodi/api/internal/timer"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()

	database, err := store.Open(cfg.ResolveDatabasePath())
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	logger.Info("database ready", "path", cfg.DatabaseURL)

	taskRepo := store.NewTaskRepository(database)
	sessionRepo := store.NewSessionRepository(database)
	timerRepo := store.NewTimerStateRepository(database)
	settingsRepo := store.NewSettingsRepository(database)

	engine := timer.New(database, taskRepo, sessionRepo, timerRepo, settingsRepo, logger)

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := engine.Bootstrap(rootCtx); err != nil {
		logger.Error("bootstrap timer engine", "error", err)
		os.Exit(1)
	}
	if err := engine.EnsureActiveTask(rootCtx); err != nil {
		logger.Error("ensure active task", "error", err)
	}

	wsHub := hub.New(logger, nil)
	server := httpapi.New(cfg, database, taskRepo, sessionRepo, settingsRepo, engine, wsHub, logger)

	wsHub.SetHooks(hub.Hooks{
		OnConnect:    server.OnClientConnect,
		OnDisconnect: server.OnClientDisconnect,
	})
	wsHub.SetMessageHandler(server.HandleInbound)

	engine.SetBroadcast(func(eventType string, payload any) {
		wsHub.Broadcast(eventType, payload, time.Now().UTC())
	})

	go wsHub.Run(rootCtx)
	go engine.Run(rootCtx)

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		logger.Info("api listening",
			"addr", cfg.Addr,
			"websocket", cfg.WSPath,
			"cors", cfg.CORSOrigins,
		)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server stopped", "error", err)
			stop()
		}
	}()

	<-rootCtx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
	logger.Info("bye")
}
