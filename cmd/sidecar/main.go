package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"gitlab.com/controlplane/controlplane/kafka-orchestrator/pkg/about"
	"gitlab.com/controlplane/controlplane/kafka-orchestrator/pkg/sidecar/types"
)

var logger *slog.Logger

func main() {
	// Initialize logger
	var level slog.Level
	err := level.UnmarshalText([]byte(types.Config.LogLevel))
	if err != nil {
		panic(err)
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
