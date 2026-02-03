package health

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
)

// MockKafkaAdminClient is a mock implementation of KafkaAdminClient for testing
type MockKafkaAdminClient struct {
	MetadataFunc              func(ctx context.Context, topics ...string) (kadm.Metadata, error)
	DescribeBrokerLogDirsFunc func(ctx context.Context, broker int32, topics kadm.TopicsSet) (kadm.DescribedLogDirs, error)
}

func (m *MockKafkaAdminClient) Metadata(ctx context.Context, topics ...string) (kadm.Metadata, error) {
	if m.MetadataFunc != nil {
		return m.MetadataFunc(ctx, topics...)
	}
	return kadm.Metadata{}, nil
}

func (m *MockKafkaAdminClient) DescribeBrokerLogDirs(ctx context.Context, broker int32, topics kadm.TopicsSet) (kadm.DescribedLogDirs, error) {
	if m.DescribeBrokerLogDirsFunc != nil {
		return m.DescribeBrokerLogDirsFunc(ctx, broker, topics)
	}
	return kadm.DescribedLogDirs{}, nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewChecker(t *testing.T) {
	logger := testLogger()

	tests := []struct {
		name             string
		brokerID         int32
		bootstrapServers string
		checkTimeout     time.Duration
		saslConfig       SASLConfig
		expectedServers  []string
	}{
		{
			name:             "single server",
			brokerID:         0,
			bootstrapServers: "localhost:9092",
			checkTimeout:     10 * time.Second,
			saslConfig:       SASLConfig{},
			expectedServers:  []string{"localhost:9092"},
		},
		{
			name:             "multiple servers",
			brokerID:         1,
			bootstrapServers: "broker1:9092, broker2:9092, broker3:9092",
			checkTimeout:     5 * time.Second,
			saslConfig:       SASLConfig{Enabled: true, Mechanism: "PLAIN"},
			expectedServers:  []string{"broker1:9092", "broker2:9092", "broker3:9092"},
		},
		{
			name:             "servers with extra whitespace",
			brokerID:         2,
			bootstrapServers: "  broker1:9092  ,  broker2:9092  ",
			checkTimeout:     15 * time.Second,
			saslConfig:       SASLConfig{},
			expectedServers:  []string{"broker1:9092", "broker2:9092"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(tt.brokerID, tt.bootstrapServers, tt.checkTimeout, tt.saslConfig, logger)

			if checker.brokerID != tt.brokerID {
				t.Errorf("expected brokerID %d, got %d", tt.brokerID, checker.brokerID)
			}

			if checker.checkTimeout != tt.checkTimeout {
				t.Errorf("expected checkTimeout %v, got %v", tt.checkTimeout, checker.checkTimeout)
			}

			if len(checker.bootstrapServers) != len(tt.expectedServers) {
				t.Errorf("expected %d servers, got %d", len(tt.expectedServers), len(checker.bootstrapServers))
			}

			for i, server := range tt.expectedServers {
				if checker.bootstrapServers[i] != server {
					t.Errorf("expected server[%d] to be %q, got %q", i, server, checker.bootstrapServers[i])
				}
			}

			if checker.clientFactory == nil {
				t.Error("clientFactory should not be nil")
			}
		})
	}
}

func TestGetSASLOpt(t *testing.T) {
	logger := testLogger()

	tests := []struct {
		name        string
		mechanism   string
		expectError bool
	}{
		{
			name:        "PLAIN mechanism",
			mechanism:   "PLAIN",
			expectError: false,
		},
		{
			name:        "PLAIN lowercase",
			mechanism:   "plain",
			expectError: false,
		},
		{
			name:        "SCRAM-SHA-256",
			mechanism:   "SCRAM-SHA-256",
			expectError: false,
		},
		{
			name:        "SCRAM-SHA-512",
			mechanism:   "SCRAM-SHA-512",
			expectError: false,
		},
		{
			name:        "unsupported mechanism",
			mechanism:   "GSSAPI",
			expectError: true,
		},
		{
			name:        "empty mechanism",
			mechanism:   "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(0, "localhost:9092", 10*time.Second, SASLConfig{
				Enabled:   true,
				Mechanism: tt.mechanism,
				Username:  "user",
				Password:  "pass",
			}, logger)

			opt, err := checker.getSASLOpt()

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if opt == nil {
				t.Error("expected SASL option but got nil")
			}
		})
	}
}

func TestBrokerInMetadata(t *testing.T) {
	logger := testLogger()
	ctx := context.Background()

	tests := []struct {
		name        string
		brokerID    int32
		brokers     []kadm.BrokerDetail
		metadataErr error
		expectFound bool
		expectError bool
	}{
		{
			name:     "broker found",
			brokerID: 1,
			brokers: []kadm.BrokerDetail{
				{NodeID: 0},
				{NodeID: 1},
				{NodeID: 2},
			},
			expectFound: true,
			expectError: false,
		},
		{
			name:     "broker not found",
			brokerID: 5,
			brokers: []kadm.BrokerDetail{
				{NodeID: 0},
				{NodeID: 1},
				{NodeID: 2},
			},
			expectFound: false,
			expectError: false,
		},
		{
			name:        "empty broker list",
			brokerID:    0,
			brokers:     []kadm.BrokerDetail{},
			expectFound: false,
			expectError: false,
		},
		{
			name:        "metadata error",
			brokerID:    0,
			brokers:     nil,
			metadataErr: errors.New("connection refused"),
			expectFound: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockKafkaAdminClient{
				MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
					if tt.metadataErr != nil {
						return kadm.Metadata{}, tt.metadataErr
					}
					return kadm.Metadata{Brokers: tt.brokers}, nil
				},
			}

			checker := NewChecker(tt.brokerID, "localhost:9092", 10*time.Second, SASLConfig{}, logger)

			found, err := checker.BrokerInMetadata(ctx, mockClient)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if found != tt.expectFound {
				t.Errorf("expected found=%v, got %v", tt.expectFound, found)
			}
		})
	}
}

func TestControllerElected(t *testing.T) {
	logger := testLogger()
	ctx := context.Background()

	tests := []struct {
		name          string
		controller    int32
		metadataErr   error
		expectElected bool
		expectError   bool
	}{
		{
			name:          "controller elected (ID 0)",
			controller:    0,
			expectElected: true,
			expectError:   false,
		},
		{
			name:          "controller elected (ID 5)",
			controller:    5,
			expectElected: true,
			expectError:   false,
		},
		{
			name:          "no controller elected",
			controller:    -1,
			expectElected: false,
			expectError:   false,
		},
		{
			name:          "metadata error",
			controller:    0,
			metadataErr:   errors.New("timeout"),
			expectElected: false,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockKafkaAdminClient{
				MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
					if tt.metadataErr != nil {
						return kadm.Metadata{}, tt.metadataErr
					}
					return kadm.Metadata{Controller: tt.controller}, nil
				},
			}

			checker := NewChecker(0, "localhost:9092", 10*time.Second, SASLConfig{}, logger)

			elected, err := checker.ControllerElected(ctx, mockClient)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if elected != tt.expectElected {
				t.Errorf("expected elected=%v, got %v", tt.expectElected, elected)
			}
		})
	}
}

func TestUnderReplicatedPartitions(t *testing.T) {
	logger := testLogger()
	ctx := context.Background()

	tests := []struct {
		name        string
		brokerID    int32
		topics      kadm.TopicDetails
		metadataErr error
		expectCount int
		expectError bool
	}{
		{
			name:     "all partitions fully replicated",
			brokerID: 0,
			topics: kadm.TopicDetails{
				"test-topic": kadm.TopicDetail{
					Partitions: kadm.PartitionDetails{
						0: {Partition: 0, Replicas: []int32{0, 1, 2}, ISR: []int32{0, 1, 2}},
						1: {Partition: 1, Replicas: []int32{0, 1, 2}, ISR: []int32{0, 1, 2}},
					},
				},
			},
			expectCount: 0,
			expectError: false,
		},
		{
			name:     "one under-replicated partition",
			brokerID: 0,
			topics: kadm.TopicDetails{
				"test-topic": kadm.TopicDetail{
					Partitions: kadm.PartitionDetails{
						0: {Partition: 0, Replicas: []int32{0, 1, 2}, ISR: []int32{1, 2}}, // broker 0 not in ISR
						1: {Partition: 1, Replicas: []int32{0, 1, 2}, ISR: []int32{0, 1, 2}},
					},
				},
			},
			expectCount: 1,
			expectError: false,
		},
		{
			name:     "broker not a replica - should not count",
			brokerID: 3,
			topics: kadm.TopicDetails{
				"test-topic": kadm.TopicDetail{
					Partitions: kadm.PartitionDetails{
						0: {Partition: 0, Replicas: []int32{0, 1, 2}, ISR: []int32{0, 1}},
					},
				},
			},
			expectCount: 0,
			expectError: false,
		},
		{
			name:     "multiple under-replicated partitions",
			brokerID: 0,
			topics: kadm.TopicDetails{
				"topic1": kadm.TopicDetail{
					Partitions: kadm.PartitionDetails{
						0: {Partition: 0, Replicas: []int32{0, 1}, ISR: []int32{1}},
					},
				},
				"topic2": kadm.TopicDetail{
					Partitions: kadm.PartitionDetails{
						0: {Partition: 0, Replicas: []int32{0, 2}, ISR: []int32{2}},
					},
				},
			},
			expectCount: 2,
			expectError: false,
		},
		{
			name:        "metadata error",
			brokerID:    0,
			topics:      nil,
			metadataErr: errors.New("connection lost"),
			expectCount: -1,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockKafkaAdminClient{
				MetadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
					if tt.metadataErr != nil {
						return kadm.Metadata{}, tt.metadataErr
					}
					return kadm.Metadata{Topics: tt.topics}, nil
				},
			}

			checker := NewChecker(tt.brokerID, "localhost:9092", 10*time.Second, SASLConfig{}, logger)

			count, err := checker.UnderReplicatedPartitions(ctx, mockClient)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if count != tt.expectCount {
				t.Errorf("expected count=%d, got %d", tt.expectCount, count)
			}
		})
	}
}

func TestLogDirsHealthy(t *testing.T) {
	logger := testLogger()
	ctx := context.Background()

	tests := []struct {
		name          string
		brokerID      int32
		logDirs       kadm.DescribedLogDirs
		logDirsErr    error
		expectHealthy bool
		expectError   bool
	}{
		{
			name:     "healthy - no future partitions",
			brokerID: 0,
			logDirs: kadm.DescribedLogDirs{
				"/var/kafka-logs": kadm.DescribedLogDir{
					Dir: "/var/kafka-logs",
					Topics: kadm.DescribedLogDirTopics{
						"test-topic": {
							0: {Topic: "test-topic", Partition: 0, IsFuture: false},
						},
					},
				},
			},
			expectHealthy: true,
			expectError:   false,
		},
		{
			name:     "unhealthy - future partition found",
			brokerID: 0,
			logDirs: kadm.DescribedLogDirs{
				"/var/kafka-logs": kadm.DescribedLogDir{
					Dir: "/var/kafka-logs",
					Topics: kadm.DescribedLogDirTopics{
						"test-topic": {
							0: {Topic: "test-topic", Partition: 0, IsFuture: true},
						},
					},
				},
			},
			expectHealthy: false,
			expectError:   false,
		},
		{
			name:          "describe log dirs error",
			brokerID:      0,
			logDirs:       kadm.DescribedLogDirs{},
			logDirsErr:    errors.New("broker not available"),
			expectHealthy: false,
			expectError:   true,
		},
		{
			name:     "log dir has error",
			brokerID: 0,
			logDirs: kadm.DescribedLogDirs{
				"/var/kafka-logs": kadm.DescribedLogDir{
					Dir: "/var/kafka-logs",
					Err: errors.New("kafka storage error"),
				},
			},
			expectHealthy: false,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockKafkaAdminClient{
				DescribeBrokerLogDirsFunc: func(ctx context.Context, broker int32, topics kadm.TopicsSet) (kadm.DescribedLogDirs, error) {
					if tt.logDirsErr != nil {
						return kadm.DescribedLogDirs{}, tt.logDirsErr
					}
					return tt.logDirs, nil
				},
			}

			checker := NewChecker(tt.brokerID, "localhost:9092", 10*time.Second, SASLConfig{}, logger)

			healthy, err := checker.LogDirsHealthy(ctx, mockClient)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if healthy != tt.expectHealthy {
				t.Errorf("expected healthy=%v, got %v", tt.expectHealthy, healthy)
			}
		})
	}
}

func TestCheckResult(t *testing.T) {
	tests := []struct {
		name     string
		result   CheckResult
		expected CheckResult
	}{
		{
			name:     "healthy result",
			result:   CheckResult{Healthy: true},
			expected: CheckResult{Healthy: true, Message: ""},
		},
		{
			name:     "unhealthy result with message",
			result:   CheckResult{Healthy: false, Message: "broker not found"},
			expected: CheckResult{Healthy: false, Message: "broker not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.Healthy != tt.expected.Healthy {
				t.Errorf("expected Healthy=%v, got %v", tt.expected.Healthy, tt.result.Healthy)
			}
			if tt.result.Message != tt.expected.Message {
				t.Errorf("expected Message=%q, got %q", tt.expected.Message, tt.result.Message)
			}
		})
	}
}

func TestSASLConfig(t *testing.T) {
	config := SASLConfig{
		Enabled:   true,
		Mechanism: "SCRAM-SHA-256",
		Username:  "admin",
		Password:  "secret",
	}

	if !config.Enabled {
		t.Error("expected Enabled to be true")
	}
	if config.Mechanism != "SCRAM-SHA-256" {
		t.Errorf("expected Mechanism to be SCRAM-SHA-256, got %s", config.Mechanism)
	}
	if config.Username != "admin" {
		t.Errorf("expected Username to be admin, got %s", config.Username)
	}
	if config.Password != "secret" {
		t.Errorf("expected Password to be secret, got %s", config.Password)
	}
}
