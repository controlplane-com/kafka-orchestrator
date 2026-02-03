package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/controlplane-com/libs-go/pkg/config"
	"gitlab.com/controlplane/controlplane/kafka-orchestrator/pkg/about"
	"gitlab.com/controlplane/controlplane/kafka-orchestrator/pkg/sidecar/types"
)

var logger *slog.Logger

func main() {
	// Initialize logger with default level for startup
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Initialize configuration
	if err := types.Initialize(logger); err != nil {
		logger.Error("failed to initialize configuration", "error", err)
		os.Exit(1)
	}
	logger.Info("configuration loaded", "config", config.Summarize(types.Config))

	// Re-initialize logger with configured level
	var level slog.Level
	if err := level.UnmarshalText([]byte(types.Config.LogLevel)); err != nil {
		logger.Error("invalid log level", "error", err)
		os.Exit(1)
	}
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	logger.Info("starting kafka-sidecar",
		"version", about.Version,
		"epoch", about.Epoch,
		"build", about.Build,
	)

	// Create server
	server := NewServer(logger)

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("received shutdown signal", "signal", sig.String())
		cancel()
	}()

	// Start server
	if err := server.Start(ctx); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}

	logger.Info("kafka-sidecar stopped")
}
