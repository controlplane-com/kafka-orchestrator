package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"gitlab.com/controlplane/controlplane/kafka-orchestrator/pkg/about"
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
