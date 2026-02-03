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

func TestReadinessHandler(t *testing.T) {
	logger := testLogger()

	tests := []struct {
		name           string
		brokerID       int32
		clientFactory  ClientFactory
		expectedStatus int
		expectHealthy  bool
	}{
		{
			name:     "healthy - all checks pass",
			brokerID: 0,
			clientFactory: func() (KafkaAdminClient, func(), error) {
				return &MockKafkaAdminClient{
					MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
						return kadm.Metadata{
							Brokers:    []kadm.BrokerDetail{{NodeID: 0}, {NodeID: 1}},
							Controller: 1,
							Topics: kadm.TopicDetails{
								"test": kadm.TopicDetail{
									Partitions: kadm.PartitionDetails{
										0: {Partition: 0, Replicas: []int32{0, 1}, ISR: []int32{0, 1}},
									},
								},
							},
						}, nil
					},
					DescribeBrokerLogDirsFunc: func(ctx context.Context, broker int32, topics kadm.TopicsSet) (kadm.DescribedLogDirs, error) {
						return kadm.DescribedLogDirs{
							"/var/kafka-logs": kadm.DescribedLogDir{Dir: "/var/kafka-logs"},
						}, nil
					},
				}, func() {}, nil
			},
			expectedStatus: http.StatusOK,
			expectHealthy:  true,
		},
		{
			name:     "unhealthy - broker not registered",
			brokerID: 5,
			clientFactory: func() (KafkaAdminClient, func(), error) {
				return &MockKafkaAdminClient{
					MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
						return kadm.Metadata{
							Brokers:    []kadm.BrokerDetail{{NodeID: 0}, {NodeID: 1}},
							Controller: 1,
						}, nil
					},
				}, func() {}, nil
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectHealthy:  false,
		},
		{
			name:     "unhealthy - no controller elected",
			brokerID: 0,
			clientFactory: func() (KafkaAdminClient, func(), error) {
				return &MockKafkaAdminClient{
					MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
						return kadm.Metadata{
							Brokers:    []kadm.BrokerDetail{{NodeID: 0}},
							Controller: -1,
						}, nil
					},
				}, func() {}, nil
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectHealthy:  false,
		},
		{
			name:     "unhealthy - under-replicated partitions",
			brokerID: 0,
			clientFactory: func() (KafkaAdminClient, func(), error) {
				return &MockKafkaAdminClient{
					MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
						return kadm.Metadata{
							Brokers:    []kadm.BrokerDetail{{NodeID: 0}, {NodeID: 1}},
							Controller: 1,
							Topics: kadm.TopicDetails{
								"test": kadm.TopicDetail{
									Partitions: kadm.PartitionDetails{
										0: {Partition: 0, Replicas: []int32{0, 1}, ISR: []int32{1}}, // broker 0 not in ISR
									},
								},
							},
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
			name:     "unhealthy - metadata error",
			brokerID: 0,
			clientFactory: func() (KafkaAdminClient, func(), error) {
				return &MockKafkaAdminClient{
					MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
						return kadm.Metadata{}, errors.New("timeout")
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

			req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
			w := httptest.NewRecorder()

			checker.ReadinessHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response ReadinessResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if tt.expectHealthy {
				if response.Status != "healthy" {
					t.Errorf("expected status 'healthy', got %q", response.Status)
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

func TestReadinessHandlerLogDirError(t *testing.T) {
	logger := testLogger()

	checker := NewChecker(0, "localhost:9092", 10*time.Second, SASLConfig{}, logger)
	checker.SetClientFactory(func() (KafkaAdminClient, func(), error) {
		return &MockKafkaAdminClient{
			MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
				return kadm.Metadata{
					Brokers:    []kadm.BrokerDetail{{NodeID: 0}},
					Controller: 0,
					Topics:     kadm.TopicDetails{},
				}, nil
			},
			DescribeBrokerLogDirsFunc: func(ctx context.Context, broker int32, topics kadm.TopicsSet) (kadm.DescribedLogDirs, error) {
				return kadm.DescribedLogDirs{}, errors.New("log dir error")
			},
		}, func() {}, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	checker.ReadinessHandler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

func TestCheckReadiness(t *testing.T) {
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
			name:     "healthy - all checks pass",
			brokerID: 0,
			clientFactory: func() (KafkaAdminClient, func(), error) {
				return &MockKafkaAdminClient{
					MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
						return kadm.Metadata{
							Brokers:    []kadm.BrokerDetail{{NodeID: 0}},
							Controller: 0,
							Topics:     kadm.TopicDetails{},
						}, nil
					},
					DescribeBrokerLogDirsFunc: func(ctx context.Context, broker int32, topics kadm.TopicsSet) (kadm.DescribedLogDirs, error) {
						return kadm.DescribedLogDirs{}, nil
					},
				}, func() {}, nil
			},
			expectHealthy: true,
			expectMessage: "",
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
			name:     "unhealthy - broker not registered",
			brokerID: 5,
			clientFactory: func() (KafkaAdminClient, func(), error) {
				return &MockKafkaAdminClient{
					MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
						return kadm.Metadata{
							Brokers:    []kadm.BrokerDetail{{NodeID: 0}},
							Controller: 0,
						}, nil
					},
				}, func() {}, nil
			},
			expectHealthy: false,
			expectMessage: "broker not registered in cluster metadata",
		},
		{
			name:     "unhealthy - no controller elected",
			brokerID: 0,
			clientFactory: func() (KafkaAdminClient, func(), error) {
				return &MockKafkaAdminClient{
					MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
						return kadm.Metadata{
							Brokers:    []kadm.BrokerDetail{{NodeID: 0}},
							Controller: -1,
						}, nil
					},
				}, func() {}, nil
			},
			expectHealthy: false,
			expectMessage: "no controller elected",
		},
		{
			name:     "unhealthy - under-replicated partitions",
			brokerID: 0,
			clientFactory: func() (KafkaAdminClient, func(), error) {
				return &MockKafkaAdminClient{
					MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
						return kadm.Metadata{
							Brokers:    []kadm.BrokerDetail{{NodeID: 0}},
							Controller: 0,
							Topics: kadm.TopicDetails{
								"test": kadm.TopicDetail{
									Partitions: kadm.PartitionDetails{
										0: {Partition: 0, Replicas: []int32{0}, ISR: []int32{}},
									},
								},
							},
						}, nil
					},
				}, func() {}, nil
			},
			expectHealthy: false,
			expectMessage: "broker has under-replicated partitions",
		},
		{
			name:     "unhealthy - log dirs unhealthy",
			brokerID: 0,
			clientFactory: func() (KafkaAdminClient, func(), error) {
				return &MockKafkaAdminClient{
					MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
						return kadm.Metadata{
							Brokers:    []kadm.BrokerDetail{{NodeID: 0}},
							Controller: 0,
							Topics:     kadm.TopicDetails{},
						}, nil
					},
					DescribeBrokerLogDirsFunc: func(ctx context.Context, broker int32, topics kadm.TopicsSet) (kadm.DescribedLogDirs, error) {
						return kadm.DescribedLogDirs{
							"/var/kafka-logs": kadm.DescribedLogDir{
								Dir: "/var/kafka-logs",
								Topics: kadm.DescribedLogDirTopics{
									"test": {
										0: {Topic: "test", Partition: 0, IsFuture: true},
									},
								},
							},
						}, nil
					},
				}, func() {}, nil
			},
			expectHealthy: false,
			expectMessage: "log directories unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(tt.brokerID, "localhost:9092", 10*time.Second, SASLConfig{}, logger)
			checker.SetClientFactory(tt.clientFactory)

			result := checker.CheckReadiness(ctx)

			if result.Healthy != tt.expectHealthy {
				t.Errorf("expected Healthy=%v, got %v", tt.expectHealthy, result.Healthy)
			}

			if tt.expectMessage != "" && result.Message != tt.expectMessage {
				t.Errorf("expected Message=%q, got %q", tt.expectMessage, result.Message)
			}
		})
	}
}

func TestReadinessResponse(t *testing.T) {
	response := ReadinessResponse{
		Status:                    "healthy",
		BrokerID:                  1,
		BrokerRegistered:          true,
		ControllerElected:         true,
		UnderReplicatedPartitions: 0,
		LogDirsHealthy:            true,
		ErrorMessage:              "",
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var unmarshaled ReadinessResponse
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if unmarshaled.Status != response.Status {
		t.Errorf("expected Status=%q, got %q", response.Status, unmarshaled.Status)
	}
	if unmarshaled.BrokerID != response.BrokerID {
		t.Errorf("expected BrokerID=%d, got %d", response.BrokerID, unmarshaled.BrokerID)
	}
	if unmarshaled.BrokerRegistered != response.BrokerRegistered {
		t.Errorf("expected BrokerRegistered=%v, got %v", response.BrokerRegistered, unmarshaled.BrokerRegistered)
	}
	if unmarshaled.ControllerElected != response.ControllerElected {
		t.Errorf("expected ControllerElected=%v, got %v", response.ControllerElected, unmarshaled.ControllerElected)
	}
	if unmarshaled.UnderReplicatedPartitions != response.UnderReplicatedPartitions {
		t.Errorf("expected UnderReplicatedPartitions=%d, got %d", response.UnderReplicatedPartitions, unmarshaled.UnderReplicatedPartitions)
	}
	if unmarshaled.LogDirsHealthy != response.LogDirsHealthy {
		t.Errorf("expected LogDirsHealthy=%v, got %v", response.LogDirsHealthy, unmarshaled.LogDirsHealthy)
	}
}

func TestReadinessResponseWithError(t *testing.T) {
	response := ReadinessResponse{
		Status:       "unhealthy",
		BrokerID:     0,
		ErrorMessage: "broker not registered",
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
