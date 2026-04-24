package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/controlplane-com/kafka-orchestrator/pkg/about"
	"github.com/controlplane-com/kafka-orchestrator/pkg/sidecar/types"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestAboutHandler(t *testing.T) {
	logger := testLogger()

	// Create a server instance
	s := &Server{
		logger: logger,
	}

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	w := httptest.NewRecorder()

	// Call the handler
	s.aboutHandler(w, req)

	// Check the status code
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check the content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}

	// Parse the response body
	var response about.Ab
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify response fields match about.About
	if response.Version != about.About.Version {
		t.Errorf("expected Version=%q, got %q", about.About.Version, response.Version)
	}
	if response.Epoch != about.About.Epoch {
		t.Errorf("expected Epoch=%q, got %q", about.About.Epoch, response.Epoch)
	}
	if response.Build != about.About.Build {
		t.Errorf("expected Build=%q, got %q", about.About.Build, response.Build)
	}
	if response.Timestamp != about.About.Timestamp {
		t.Errorf("expected Timestamp=%q, got %q", about.About.Timestamp, response.Timestamp)
	}
}

func TestAboutHandlerJSONFields(t *testing.T) {
	logger := testLogger()

	s := &Server{
		logger: logger,
	}

	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	w := httptest.NewRecorder()

	s.aboutHandler(w, req)

	// Parse as generic map to verify JSON field names
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Check that expected fields are present
	expectedFields := []string{"version", "epoch", "build", "timestamp"}
	for _, field := range expectedFields {
		if _, ok := response[field]; !ok {
			t.Errorf("expected field %q in response, but not found", field)
		}
	}
}

func TestServerStruct(t *testing.T) {
	// Verify the Server struct has expected fields
	logger := testLogger()

	s := &Server{
		logger: logger,
	}

	if s.logger != logger {
		t.Error("logger field not set correctly")
	}
}

func withConfig(t *testing.T, cfg *types.ConfigSchema) {
	t.Helper()
	original := types.Config
	types.Config = cfg
	t.Cleanup(func() { types.Config = original })
}

func TestNewServer(t *testing.T) {
	withConfig(t, &types.ConfigSchema{
		BrokerID:         3,
		BootstrapServers: "localhost:9092",
		CheckTimeout:     10 * time.Second,
		SASLEnabled:      true,
		SASLMechanism:    "PLAIN",
		SASLUsername:     "user",
		SASLPassword:     "pass",
	})

	logger := testLogger()
	s := NewServer(logger)

	if s == nil {
		t.Fatal("expected non-nil Server")
	}
	if s.logger != logger {
		t.Error("logger not wired")
	}
	if s.healthChecker == nil {
		t.Error("healthChecker not wired")
	}
}

func findFreePort(t *testing.T) int {
	t.Helper()
	lc, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve free port: %v", err)
	}
	port := lc.Addr().(*net.TCPAddr).Port
	_ = lc.Close()
	return port
}

func waitForListener(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("server never started listening on %s", addr)
}

func TestServerStartAndShutdown(t *testing.T) {
	port := findFreePort(t)
	withConfig(t, &types.ConfigSchema{
		BrokerID:         0,
		BootstrapServers: "127.0.0.1:9092",
		CheckTimeout:     1 * time.Second,
		Port:             port,
	})

	logger := testLogger()
	s := NewServer(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Start(ctx)
	}()

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	waitForListener(t, addr, 3*time.Second)

	resp, err := http.Get(fmt.Sprintf("http://%s/about", addr))
	if err != nil {
		t.Fatalf("GET /about failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from /about, got %d", resp.StatusCode)
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Start returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for Start to return")
	}
}

func TestServerStartListenError(t *testing.T) {
	// Occupy a port, then configure the server to bind the same port to force
	// ListenAndServe to fail and exercise the error return path in Start.
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to occupy port: %v", err)
	}
	defer occupied.Close()
	port := occupied.Addr().(*net.TCPAddr).Port

	withConfig(t, &types.ConfigSchema{
		BrokerID:         0,
		BootstrapServers: "127.0.0.1:9092",
		CheckTimeout:     1 * time.Second,
		Port:             port,
	})

	logger := testLogger()
	s := NewServer(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	select {
	case err := <-func() chan error {
		ch := make(chan error, 1)
		go func() { ch <- s.Start(ctx) }()
		return ch
	}():
		if err == nil {
			t.Error("expected Start to return an error when port is in use")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return after bind failure")
	}
}
