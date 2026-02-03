package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/controlplane-com/libs-go/pkg/config"
	"github.com/controlplane-com/libs-go/pkg/web"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"gitlab.com/controlplane/controlplane/kafka-orchestrator/pkg/about"
	"gitlab.com/controlplane/controlplane/kafka-orchestrator/pkg/sidecar/health"
	"gitlab.com/controlplane/controlplane/kafka-orchestrator/pkg/sidecar/metrics"
	"gitlab.com/controlplane/controlplane/kafka-orchestrator/pkg/sidecar/types"
)

// Server represents the HTTP server for the sidecar
type Server struct {
	logger        *slog.Logger
	healthChecker *health.Checker
	httpServer    *http.Server
}

// NewServer creates a new sidecar server
func NewServer(logger *slog.Logger) *Server {
	saslConfig := health.SASLConfig{
		Enabled:   types.Config.SASLEnabled,
		Mechanism: types.Config.SASLMechanism,
		Username:  types.Config.SASLUsername,
		Password:  types.Config.SASLPassword,
	}

	healthChecker := health.NewChecker(
		types.Config.BrokerID,
		types.Config.BootstrapServers,
		types.Config.CheckTimeout,
		saslConfig,
		logger,
	)

	return &Server{
		logger:        logger,
		healthChecker: healthChecker,
	}
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	router := mux.NewRouter()

	fmt.Println(config.Summarize(types.Config))

	// Health endpoints
	router.HandleFunc("/health/live", s.healthChecker.LivenessHandler).Methods("GET")
	router.HandleFunc("/health/ready", s.healthChecker.ReadinessHandler).Methods("GET")

	// Metrics endpoint
	metricsCollector := metrics.NewCollector(s.logger)
	if err := metricsCollector.Register(); err != nil {
		s.logger.Warn("failed to register metrics collector", "error", err)
	}
	router.Handle("/metrics", promhttp.Handler()).Methods("GET")

	// About endpoint
	router.HandleFunc("/about", s.aboutHandler).Methods("GET")

	addr := fmt.Sprintf(":%d", types.Config.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		return s.Shutdown()
	case err := <-errCh:
		return err
	}
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.logger.Info("shutting down HTTP server")
	return s.httpServer.Shutdown(ctx)
}

// aboutHandler returns version information
func (s *Server) aboutHandler(w http.ResponseWriter, _ *http.Request) {
	_, _ = web.ReturnResponse(w, about.About)
}
