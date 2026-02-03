package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
)

func TestLivenessHandler(t *testing.T) {
	logger := testLogger()

	tests := []struct {
		name           string
		brokerID       int32
		clientFactory  ClientFactory
		expectedStatus int
		expectHealthy  bool
	}{
		{
			name:     "healthy - broker found",
			brokerID: 0,
			clientFactory: func() (KafkaAdminClient, func(), error) {
				return &MockKafkaAdminClient{
					MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
						return kadm.Metadata{
							Brokers: []kadm.BrokerDetail{{NodeID: 0}, {NodeID: 1}},
						}, nil
					},
				}, func() {}, nil
			},
			expectedStatus: http.StatusOK,
			expectHealthy:  true,
		},
		{
			name:     "unhealthy - broker not found",
			brokerID: 5,
			clientFactory: func() (KafkaAdminClient, func(), error) {
				return &MockKafkaAdminClient{
					MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
						return kadm.Metadata{
							Brokers: []kadm.BrokerDetail{{NodeID: 0}, {NodeID: 1}},
						}, nil
					},
				}, func() {}, nil
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectHealthy:  false,
		},
		{
			name:     "unhealthy - client creation error",
			brokerID: 0,
			clientFactory: func() (KafkaAdminClient, func(), error) {
				return nil, nil, errors.New("connection refused")
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectHealthy:  false,
		},
		{
			name:     "unhealthy - metadata fetch error",
			brokerID: 0,
			clientFactory: func() (KafkaAdminClient, func(), error) {
				return &MockKafkaAdminClient{
					MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
						return kadm.Metadata{}, errors.New("timeout fetching metadata")
					},
				}, func() {}, nil
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectHealthy:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(tt.brokerID, "localhost:9092", 10*time.Second, SASLConfig{}, logger)
			checker.SetClientFactory(tt.clientFactory)

			req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
			w := httptest.NewRecorder()

			checker.LivenessHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response LivenessResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if tt.expectHealthy {
				if response.Status != "healthy" {
					t.Errorf("expected status 'healthy', got %q", response.Status)
				}
				if !response.BrokerFound {
					t.Error("expected BrokerFound to be true")
				}
			} else {
				if response.Status != "unhealthy" {
					t.Errorf("expected status 'unhealthy', got %q", response.Status)
				}
			}

			if response.BrokerID != tt.brokerID {
				t.Errorf("expected BrokerID %d, got %d", tt.brokerID, response.BrokerID)
			}
		})
	}
}

func TestCheckLiveness(t *testing.T) {
	logger := testLogger()
	ctx := context.Background()

	tests := []struct {
		name          string
		brokerID      int32
		clientFactory ClientFactory
		expectHealthy bool
		expectMessage string
	}{
		{
			name:     "healthy - broker found",
			brokerID: 0,
			clientFactory: func() (KafkaAdminClient, func(), error) {
				return &MockKafkaAdminClient{
					MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
						return kadm.Metadata{
							Brokers: []kadm.BrokerDetail{{NodeID: 0}},
						}, nil
					},
				}, func() {}, nil
			},
			expectHealthy: true,
			expectMessage: "",
		},
		{
			name:     "unhealthy - broker not found",
			brokerID: 5,
			clientFactory: func() (KafkaAdminClient, func(), error) {
				return &MockKafkaAdminClient{
					MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
						return kadm.Metadata{
							Brokers: []kadm.BrokerDetail{{NodeID: 0}},
						}, nil
					},
				}, func() {}, nil
			},
			expectHealthy: false,
			expectMessage: "broker not found in cluster metadata",
		},
		{
			name:     "unhealthy - client creation error",
			brokerID: 0,
			clientFactory: func() (KafkaAdminClient, func(), error) {
				return nil, nil, errors.New("connection refused")
			},
			expectHealthy: false,
			expectMessage: "connection refused",
		},
		{
			name:     "unhealthy - metadata error",
			brokerID: 0,
			clientFactory: func() (KafkaAdminClient, func(), error) {
				return &MockKafkaAdminClient{
					MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
						return kadm.Metadata{}, errors.New("network error")
					},
				}, func() {}, nil
			},
			expectHealthy: false,
			expectMessage: "failed to fetch metadata: network error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(tt.brokerID, "localhost:9092", 10*time.Second, SASLConfig{}, logger)
			checker.SetClientFactory(tt.clientFactory)

			result := checker.CheckLiveness(ctx)

			if result.Healthy != tt.expectHealthy {
				t.Errorf("expected Healthy=%v, got %v", tt.expectHealthy, result.Healthy)
			}

			if tt.expectMessage != "" && result.Message != tt.expectMessage {
				t.Errorf("expected Message=%q, got %q", tt.expectMessage, result.Message)
			}
		})
	}
}

func TestLivenessResponse(t *testing.T) {
	response := LivenessResponse{
		Status:       "healthy",
		BrokerID:     1,
		BrokerFound:  true,
		ErrorMessage: "",
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var unmarshaled LivenessResponse
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if unmarshaled.Status != response.Status {
		t.Errorf("expected Status=%q, got %q", response.Status, unmarshaled.Status)
	}
	if unmarshaled.BrokerID != response.BrokerID {
		t.Errorf("expected BrokerID=%d, got %d", response.BrokerID, unmarshaled.BrokerID)
	}
	if unmarshaled.BrokerFound != response.BrokerFound {
		t.Errorf("expected BrokerFound=%v, got %v", response.BrokerFound, unmarshaled.BrokerFound)
	}
}

func TestLivenessResponseWithError(t *testing.T) {
	response := LivenessResponse{
		Status:       "unhealthy",
		BrokerID:     0,
		BrokerFound:  false,
		ErrorMessage: "connection timeout",
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	// Verify error field is included in JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if _, ok := raw["error"]; !ok {
		t.Error("expected 'error' field in JSON when ErrorMessage is set")
	}
}
