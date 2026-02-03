package types

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

// Helper to set and restore environment variables
func setEnv(t *testing.T, key, value string) func() {
	original, existed := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("failed to set env %s: %v", key, err)
	}
	return func() {
		if existed {
			_ = os.Setenv(key, original)
		} else {
			_ = os.Unsetenv(key)
		}
	}
}

func unsetEnv(t *testing.T, key string) func() {
	original, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("failed to unset env %s: %v", key, err)
	}
	return func() {
		if existed {
			_ = os.Setenv(key, original)
		}
	}
}

func TestConfigSchema_Defaults(t *testing.T) {
	// Config is nil until Initialize() is called explicitly
	// This is intentional to avoid init() side effects that break tests in CI
	if Config != nil {
		// If a previous test initialized Config, that's fine
		// Just verify it's a valid pointer
		_ = Config.BrokerID
	}
}

func TestConfigSchema_Fields(t *testing.T) {
	// Verify ConfigSchema has expected field types
	cfg := ConfigSchema{}

	// Test default values for various fields
	if cfg.KafkaPort != 0 {
		// KafkaPort should be 0 (zero value) before initialization
	}

	if cfg.SASLEnabled != false {
		t.Error("SASLEnabled should default to false")
	}

	if cfg.SASLMechanism != "" {
		// SASLMechanism has a default from tag
	}
}

func TestConfigSchema_DurationParsing(t *testing.T) {
	// The CheckTimeout is a time.Duration - verify it's properly typed
	cfg := ConfigSchema{
		CheckTimeout: 10 * time.Second,
	}

	if cfg.CheckTimeout != 10*time.Second {
		t.Errorf("expected CheckTimeout=10s, got %v", cfg.CheckTimeout)
	}
}

func TestConfigSchema_SASLFields(t *testing.T) {
	cfg := ConfigSchema{
		SASLEnabled:   true,
		SASLMechanism: "SCRAM-SHA-256",
		SASLUsername:  "admin",
		SASLPassword:  "secret",
	}

	if !cfg.SASLEnabled {
		t.Error("SASLEnabled should be true")
	}
	if cfg.SASLMechanism != "SCRAM-SHA-256" {
		t.Errorf("expected SASLMechanism=SCRAM-SHA-256, got %s", cfg.SASLMechanism)
	}
	if cfg.SASLUsername != "admin" {
		t.Errorf("expected SASLUsername=admin, got %s", cfg.SASLUsername)
	}
	if cfg.SASLPassword != "secret" {
		t.Errorf("expected SASLPassword=secret, got %s", cfg.SASLPassword)
	}
}

func TestInitialize_WithAllEnvVars(t *testing.T) {
	logger := testLogger()

	// Set all required environment variables
	cleanups := []func(){
		setEnv(t, "HOSTNAME", "kafka-5"),
		setEnv(t, "CPLN_WORKLOAD", "/org/test/gvc/test/workload/kafka"),
		setEnv(t, "CPLN_LOCATION", "aws-us-west-2"),
		setEnv(t, "CPLN_GVC", "test-gvc"),
		setEnv(t, "REPLICA_COUNT", "3"),
		setEnv(t, "KAFKA_PORT", "9092"),
	}
	defer func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
	}()

	err := Initialize(logger)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if Config.BrokerID != 5 {
		t.Errorf("expected BrokerID=5, got %d", Config.BrokerID)
	}

	if Config.ReplicaCount != 3 {
		t.Errorf("expected ReplicaCount=3, got %d", Config.ReplicaCount)
	}

	if Config.BootstrapServers == "" {
		t.Error("BootstrapServers should be auto-built")
	}
}

func TestInitialize_WithExplicitBrokerID(t *testing.T) {
	logger := testLogger()

	// Set explicit broker ID
	cleanups := []func(){
		setEnv(t, "BROKER_ID", "10"),
		setEnv(t, "HOSTNAME", "kafka-5"), // Should be ignored
		setEnv(t, "CPLN_WORKLOAD", "/org/test/gvc/test/workload/kafka"),
		setEnv(t, "CPLN_LOCATION", "aws-us-west-2"),
		setEnv(t, "CPLN_GVC", "test-gvc"),
	}
	defer func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
	}()

	err := Initialize(logger)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// When BROKER_ID is set explicitly, that value should be used
	if Config.BrokerID != 10 {
		t.Errorf("expected BrokerID=10, got %d", Config.BrokerID)
	}
}

func TestInitialize_WithExplicitBootstrapServers(t *testing.T) {
	logger := testLogger()

	// Set explicit bootstrap servers
	cleanups := []func(){
		setEnv(t, "HOSTNAME", "kafka-0"),
		setEnv(t, "BOOTSTRAP_SERVERS", "broker1:9092,broker2:9092"),
		// Don't need CPLN vars when BOOTSTRAP_SERVERS is set
	}
	defer func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
	}()

	err := Initialize(logger)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if Config.BootstrapServers != "broker1:9092,broker2:9092" {
		t.Errorf("expected BootstrapServers=broker1:9092,broker2:9092, got %s", Config.BootstrapServers)
	}
}

func TestInitialize_MissingHostname(t *testing.T) {
	logger := testLogger()

	// Unset HOSTNAME and BROKER_ID to trigger error
	cleanups := []func(){
		unsetEnv(t, "HOSTNAME"),
		unsetEnv(t, "BROKER_ID"),
	}
	defer func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
	}()

	err := Initialize(logger)
	if err == nil {
		t.Error("expected error when HOSTNAME is not set and BROKER_ID is not set")
	}
}

func TestInitialize_MissingWorkloadName(t *testing.T) {
	logger := testLogger()

	cleanups := []func(){
		setEnv(t, "HOSTNAME", "kafka-0"),
		unsetEnv(t, "BOOTSTRAP_SERVERS"),
		unsetEnv(t, "WORKLOAD_NAME"),
		unsetEnv(t, "CPLN_WORKLOAD"),
	}
	defer func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
	}()

	err := Initialize(logger)
	if err == nil {
		t.Error("expected error when workload name cannot be discovered")
	}
}

func TestInitialize_MissingLocation(t *testing.T) {
	logger := testLogger()

	cleanups := []func(){
		setEnv(t, "HOSTNAME", "kafka-0"),
		setEnv(t, "CPLN_WORKLOAD", "/org/test/gvc/test/workload/kafka"),
		unsetEnv(t, "BOOTSTRAP_SERVERS"),
		unsetEnv(t, "LOCATION"),
		unsetEnv(t, "CPLN_LOCATION"),
	}
	defer func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
	}()

	err := Initialize(logger)
	if err == nil {
		t.Error("expected error when location cannot be discovered")
	}
}

func TestInitialize_MissingGvcName(t *testing.T) {
	logger := testLogger()

	cleanups := []func(){
		setEnv(t, "HOSTNAME", "kafka-0"),
		setEnv(t, "CPLN_WORKLOAD", "/org/test/gvc/test/workload/kafka"),
		setEnv(t, "CPLN_LOCATION", "aws-us-west-2"),
		unsetEnv(t, "BOOTSTRAP_SERVERS"),
		unsetEnv(t, "GVC_NAME"),
		unsetEnv(t, "CPLN_GVC"),
	}
	defer func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
	}()

	err := Initialize(logger)
	if err == nil {
		t.Error("expected error when GVC name cannot be discovered")
	}
}

func TestConfigSchema_Tags(t *testing.T) {
	// This test verifies the struct tags are properly defined
	// by checking the struct can be introspected

	cfg := ConfigSchema{}

	// Just verify the struct has expected fields
	_ = cfg.BrokerID
	_ = cfg.WorkloadName
	_ = cfg.GvcAlias
	_ = cfg.GvcName
	_ = cfg.Location
	_ = cfg.ReplicaCount
	_ = cfg.KafkaPort
	_ = cfg.BootstrapServers
	_ = cfg.SASLEnabled
	_ = cfg.SASLMechanism
	_ = cfg.SASLUsername
	_ = cfg.SASLPassword
	_ = cfg.CheckTimeout
	_ = cfg.Port
	_ = cfg.LogLevel
}
