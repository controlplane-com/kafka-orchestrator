package discovery

import (
	"os"
	"testing"
)

func TestParseBrokerIDFromHostname(t *testing.T) {
	tests := []struct {
		name        string
		hostname    string
		expected    int32
		expectError bool
	}{
		{
			name:        "valid kafka-0",
			hostname:    "kafka-0",
			expected:    0,
			expectError: false,
		},
		{
			name:        "valid kafka-1",
			hostname:    "kafka-1",
			expected:    1,
			expectError: false,
		},
		{
			name:        "valid kafka-99",
			hostname:    "kafka-99",
			expected:    99,
			expectError: false,
		},
		{
			name:        "multi-hyphen workload name",
			hostname:    "my-kafka-cluster-2",
			expected:    2,
			expectError: false,
		},
		{
			name:        "no hyphen",
			hostname:    "kafka",
			expected:    0,
			expectError: true,
		},
		{
			name:        "trailing hyphen",
			hostname:    "kafka-",
			expected:    0,
			expectError: true,
		},
		{
			name:        "non-numeric suffix",
			hostname:    "kafka-abc",
			expected:    0,
			expectError: true,
		},
		{
			name:        "empty string",
			hostname:    "",
			expected:    0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseBrokerIDFromHostname(tt.hostname)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestDiscoverBrokerID(t *testing.T) {
	// Test with HOSTNAME not set
	t.Run("hostname not set", func(t *testing.T) {
		originalHostname := os.Getenv("HOSTNAME")
		err := os.Unsetenv("HOSTNAME")
		if err != nil {
			t.Errorf("failed to unset HOSTNAME: %v", err)
			return
		}
		defer func() {
			if originalHostname != "" {
				err := os.Setenv("HOSTNAME", originalHostname)
				if err != nil {
					t.Errorf("failed to set HOSTNAME: %v", err)
					return
				}
			}
		}()

		_, err = DiscoverBrokerID()
		if err == nil {
			t.Errorf("expected error when HOSTNAME is not set")
		}
	})

	// Test with valid HOSTNAME
	t.Run("valid hostname", func(t *testing.T) {
		originalHostname := os.Getenv("HOSTNAME")
		err := os.Setenv("HOSTNAME", "kafka-5")
		if err != nil {
			t.Errorf("failed to set HOSTNAME: %v", err)
			return
		}
		defer func() {
			if originalHostname != "" {
				err := os.Setenv("HOSTNAME", originalHostname)
				if err != nil {
					t.Errorf("failed to set HOSTNAME: %v", err)
					return
				}
			} else {
				err := os.Unsetenv("HOSTNAME")
				if err != nil {
					t.Errorf("failed to unset HOSTNAME: %v", err)
					return
				}
			}
		}()

		result, err := DiscoverBrokerID()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if result != 5 {
			t.Errorf("expected 5, got %d", result)
		}
	})
}

func TestBuildBootstrapServers(t *testing.T) {
	tests := []struct {
		name         string
		workloadName string
		location     string
		gvcName      string
		replicaCount int
		port         int
		expected     string
	}{
		{
			name:         "single replica",
			workloadName: "kafka",
			location:     "aws-us-west-2",
			gvcName:      "test-gvc",
			replicaCount: 1,
			port:         9092,
			expected:     "replica-0.kafka.aws-us-west-2.test-gvc.cpln.local:9092",
		},
		{
			name:         "three replicas",
			workloadName: "kafka",
			location:     "gcp-us-east1",
			gvcName:      "prod-gvc",
			replicaCount: 3,
			port:         9092,
			expected:     "replica-0.kafka.gcp-us-east1.prod-gvc.cpln.local:9092,replica-1.kafka.gcp-us-east1.prod-gvc.cpln.local:9092,replica-2.kafka.gcp-us-east1.prod-gvc.cpln.local:9092",
		},
		{
			name:         "custom port",
			workloadName: "kafka",
			location:     "aws-eu-west-1",
			gvcName:      "test-gvc",
			replicaCount: 2,
			port:         9094,
			expected:     "replica-0.kafka.aws-eu-west-1.test-gvc.cpln.local:9094,replica-1.kafka.aws-eu-west-1.test-gvc.cpln.local:9094",
		},
		{
			name:         "zero replica count defaults to 1",
			workloadName: "kafka",
			location:     "aws-us-west-2",
			gvcName:      "test-gvc",
			replicaCount: 0,
			port:         9092,
			expected:     "replica-0.kafka.aws-us-west-2.test-gvc.cpln.local:9092",
		},
		{
			name:         "negative replica count defaults to 1",
			workloadName: "kafka",
			location:     "aws-us-west-2",
			gvcName:      "test-gvc",
			replicaCount: -1,
			port:         9092,
			expected:     "replica-0.kafka.aws-us-west-2.test-gvc.cpln.local:9092",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildBootstrapServers(tt.workloadName, tt.location, tt.gvcName, tt.replicaCount, tt.port)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestParseWorkloadNameFromLink(t *testing.T) {
	tests := []struct {
		name        string
		link        string
		expected    string
		expectError bool
	}{
		{
			name:        "valid full link",
			link:        "/org/gitops/gvc/igor-kafka/workload/kafka-fix-cluster",
			expected:    "kafka-fix-cluster",
			expectError: false,
		},
		{
			name:        "valid simple link",
			link:        "/workload/kafka",
			expected:    "kafka",
			expectError: false,
		},
		{
			name:        "workload name with hyphens",
			link:        "/org/my-org/gvc/my-gvc/workload/my-kafka-cluster",
			expected:    "my-kafka-cluster",
			expectError: false,
		},
		{
			name:        "missing /workload/ prefix",
			link:        "/org/gitops/gvc/igor-kafka/kafka-fix-cluster",
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty workload name",
			link:        "/org/gitops/gvc/igor-kafka/workload/",
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty string",
			link:        "",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseWorkloadNameFromLink(tt.link)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestDiscoverWorkloadName(t *testing.T) {
	t.Run("CPLN_WORKLOAD not set", func(t *testing.T) {
		original := os.Getenv("CPLN_WORKLOAD")
		if err := os.Unsetenv("CPLN_WORKLOAD"); err != nil {
			t.Fatalf("failed to unset CPLN_WORKLOAD: %v", err)
		}
		defer func() {
			if original != "" {
				if err := os.Setenv("CPLN_WORKLOAD", original); err != nil {
					t.Errorf("failed to restore CPLN_WORKLOAD: %v", err)
				}
			}
		}()

		_, err := DiscoverWorkloadName()
		if err == nil {
			t.Errorf("expected error when CPLN_WORKLOAD is not set")
		}
	})

	t.Run("valid CPLN_WORKLOAD", func(t *testing.T) {
		original := os.Getenv("CPLN_WORKLOAD")
		if err := os.Setenv("CPLN_WORKLOAD", "/org/gitops/gvc/test/workload/my-kafka"); err != nil {
			t.Fatalf("failed to set CPLN_WORKLOAD: %v", err)
		}
		defer func() {
			if original != "" {
				if err := os.Setenv("CPLN_WORKLOAD", original); err != nil {
					t.Errorf("failed to restore CPLN_WORKLOAD: %v", err)
				}
			} else {
				if err := os.Unsetenv("CPLN_WORKLOAD"); err != nil {
					t.Errorf("failed to unset CPLN_WORKLOAD: %v", err)
				}
			}
		}()

		result, err := DiscoverWorkloadName()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if result != "my-kafka" {
			t.Errorf("expected 'my-kafka', got %q", result)
		}
	})
}

func TestDiscoverGvcAlias(t *testing.T) {
	t.Run("CPLN_GVC_ALIAS not set", func(t *testing.T) {
		original := os.Getenv("CPLN_GVC_ALIAS")
		if err := os.Unsetenv("CPLN_GVC_ALIAS"); err != nil {
			t.Fatalf("failed to unset CPLN_GVC_ALIAS: %v", err)
		}
		defer func() {
			if original != "" {
				if err := os.Setenv("CPLN_GVC_ALIAS", original); err != nil {
					t.Errorf("failed to restore CPLN_GVC_ALIAS: %v", err)
				}
			}
		}()

		_, err := DiscoverGvcAlias()
		if err == nil {
			t.Errorf("expected error when CPLN_GVC_ALIAS is not set")
		}
	})

	t.Run("valid CPLN_GVC_ALIAS", func(t *testing.T) {
		original := os.Getenv("CPLN_GVC_ALIAS")
		if err := os.Setenv("CPLN_GVC_ALIAS", "abc123xyz"); err != nil {
			t.Fatalf("failed to set CPLN_GVC_ALIAS: %v", err)
		}
		defer func() {
			if original != "" {
				if err := os.Setenv("CPLN_GVC_ALIAS", original); err != nil {
					t.Errorf("failed to restore CPLN_GVC_ALIAS: %v", err)
				}
			} else {
				if err := os.Unsetenv("CPLN_GVC_ALIAS"); err != nil {
					t.Errorf("failed to unset CPLN_GVC_ALIAS: %v", err)
				}
			}
		}()

		result, err := DiscoverGvcAlias()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if result != "abc123xyz" {
			t.Errorf("expected 'abc123xyz', got %q", result)
		}
	})
}

func TestDiscoverGvcName(t *testing.T) {
	t.Run("CPLN_GVC not set", func(t *testing.T) {
		original := os.Getenv("CPLN_GVC")
		if err := os.Unsetenv("CPLN_GVC"); err != nil {
			t.Fatalf("failed to unset CPLN_GVC: %v", err)
		}
		defer func() {
			if original != "" {
				if err := os.Setenv("CPLN_GVC", original); err != nil {
					t.Errorf("failed to restore CPLN_GVC: %v", err)
				}
			}
		}()

		_, err := DiscoverGvcName()
		if err == nil {
			t.Errorf("expected error when CPLN_GVC is not set")
		}
	})

	t.Run("valid CPLN_GVC", func(t *testing.T) {
		original := os.Getenv("CPLN_GVC")
		if err := os.Setenv("CPLN_GVC", "my-gvc-name"); err != nil {
			t.Fatalf("failed to set CPLN_GVC: %v", err)
		}
		defer func() {
			if original != "" {
				if err := os.Setenv("CPLN_GVC", original); err != nil {
					t.Errorf("failed to restore CPLN_GVC: %v", err)
				}
			} else {
				if err := os.Unsetenv("CPLN_GVC"); err != nil {
					t.Errorf("failed to unset CPLN_GVC: %v", err)
				}
			}
		}()

		result, err := DiscoverGvcName()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if result != "my-gvc-name" {
			t.Errorf("expected 'my-gvc-name', got %q", result)
		}
	})
}

func TestDiscoverLocation(t *testing.T) {
	t.Run("CPLN_LOCATION not set", func(t *testing.T) {
		original := os.Getenv("CPLN_LOCATION")
		if err := os.Unsetenv("CPLN_LOCATION"); err != nil {
			t.Fatalf("failed to unset CPLN_LOCATION: %v", err)
		}
		defer func() {
			if original != "" {
				if err := os.Setenv("CPLN_LOCATION", original); err != nil {
					t.Errorf("failed to restore CPLN_LOCATION: %v", err)
				}
			}
		}()

		_, err := DiscoverLocation()
		if err == nil {
			t.Errorf("expected error when CPLN_LOCATION is not set")
		}
	})

	t.Run("valid CPLN_LOCATION", func(t *testing.T) {
		original := os.Getenv("CPLN_LOCATION")
		if err := os.Setenv("CPLN_LOCATION", "aws-us-west-2"); err != nil {
			t.Fatalf("failed to set CPLN_LOCATION: %v", err)
		}
		defer func() {
			if original != "" {
				if err := os.Setenv("CPLN_LOCATION", original); err != nil {
					t.Errorf("failed to restore CPLN_LOCATION: %v", err)
				}
			} else {
				if err := os.Unsetenv("CPLN_LOCATION"); err != nil {
					t.Errorf("failed to unset CPLN_LOCATION: %v", err)
				}
			}
		}()

		result, err := DiscoverLocation()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if result != "aws-us-west-2" {
			t.Errorf("expected 'aws-us-west-2', got %q", result)
		}
	})
}
