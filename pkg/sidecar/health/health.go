package health

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

// KafkaAdminClient defines the interface for Kafka admin operations.
// This enables mocking in tests.
type KafkaAdminClient interface {
	Metadata(ctx context.Context, topics ...string) (kadm.Metadata, error)
	DescribeBrokerLogDirs(ctx context.Context, broker int32, topics kadm.TopicsSet) (kadm.DescribedLogDirs, error)
}

// SASLConfig holds SASL authentication configuration
type SASLConfig struct {
	Enabled   bool
	Mechanism string // PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
	Username  string
	Password  string
}

// ClientFactory creates Kafka admin clients. Allows injection for testing.
type ClientFactory func() (KafkaAdminClient, func(), error)

// Checker provides health check functionality for Kafka brokers
type Checker struct {
	brokerID         int32
	bootstrapServers []string
	checkTimeout     time.Duration
	saslConfig       SASLConfig
	logger           *slog.Logger
	clientFactory    ClientFactory
}

// NewChecker creates a new health checker
func NewChecker(brokerID int32, bootstrapServers string, checkTimeout time.Duration, saslConfig SASLConfig, logger *slog.Logger) *Checker {
	servers := strings.Split(bootstrapServers, ",")
	for i := range servers {
		servers[i] = strings.TrimSpace(servers[i])
	}
	c := &Checker{
		brokerID:         brokerID,
		bootstrapServers: servers,
		checkTimeout:     checkTimeout,
		saslConfig:       saslConfig,
		logger:           logger,
	}
	// Set default client factory
	c.clientFactory = c.defaultClientFactory
	return c
}

// SetClientFactory allows overriding the client factory for testing
func (c *Checker) SetClientFactory(factory ClientFactory) {
	c.clientFactory = factory
}

// defaultClientFactory creates a new Kafka admin client using franz-go
func (c *Checker) defaultClientFactory() (KafkaAdminClient, func(), error) {
	opts := []kgo.Opt{
		kgo.SeedBrokers(c.bootstrapServers...),
	}

	// Add SASL authentication if enabled
	if c.saslConfig.Enabled {
		saslOpt, err := c.getSASLOpt()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to configure SASL: %w", err)
		}
		opts = append(opts, saslOpt)
	}

	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create kafka client: %w", err)
	}

	adm := kadm.NewClient(cl)
	return adm, cl.Close, nil
}

// getSASLOpt returns the appropriate SASL option based on mechanism
func (c *Checker) getSASLOpt() (kgo.Opt, error) {
	mechanism := strings.ToUpper(c.saslConfig.Mechanism)

	switch mechanism {
	case "PLAIN":
		auth := plain.Auth{
			User: c.saslConfig.Username,
			Pass: c.saslConfig.Password,
		}
		return kgo.SASL(auth.AsMechanism()), nil

	case "SCRAM-SHA-256":
		auth := scram.Auth{
			User: c.saslConfig.Username,
			Pass: c.saslConfig.Password,
		}
		return kgo.SASL(auth.AsSha256Mechanism()), nil

	case "SCRAM-SHA-512":
		auth := scram.Auth{
			User: c.saslConfig.Username,
			Pass: c.saslConfig.Password,
		}
		return kgo.SASL(auth.AsSha512Mechanism()), nil

	default:
		return nil, fmt.Errorf("unsupported SASL mechanism: %s (supported: PLAIN, SCRAM-SHA-256, SCRAM-SHA-512)", mechanism)
	}
}

// CheckResult represents the result of a health check
type CheckResult struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

// BrokerInMetadata checks if the broker is present in cluster metadata
func (c *Checker) BrokerInMetadata(ctx context.Context, adm KafkaAdminClient) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.checkTimeout)
	defer cancel()

	metadata, err := adm.Metadata(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to fetch metadata: %w", err)
	}

	for _, broker := range metadata.Brokers {
		if broker.NodeID == c.brokerID {
			return true, nil
		}
	}

	return false, nil
}

// ControllerElected checks if a controller has been elected
func (c *Checker) ControllerElected(ctx context.Context, adm KafkaAdminClient) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.checkTimeout)
	defer cancel()

	metadata, err := adm.Metadata(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to fetch metadata: %w", err)
	}

	return metadata.Controller >= 0, nil
}

// UnderReplicatedPartitions returns the count of under-replicated partitions for this broker
func (c *Checker) UnderReplicatedPartitions(ctx context.Context, adm KafkaAdminClient) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.checkTimeout)
	defer cancel()

	metadata, err := adm.Metadata(ctx)
	if err != nil {
		return -1, fmt.Errorf("failed to fetch metadata: %w", err)
	}

	underReplicated := 0
	for _, topic := range metadata.Topics {
		for _, partition := range topic.Partitions {
			// Check if this broker is a replica for this partition
			isReplica := false
			for _, replica := range partition.Replicas {
				if replica == c.brokerID {
					isReplica = true
					break
				}
			}

			if !isReplica {
				continue
			}

			// Check if this broker is in the ISR
			inISR := false
			for _, isr := range partition.ISR {
				if isr == c.brokerID {
					inISR = true
					break
				}
			}

			if !inISR {
				underReplicated++
			}
		}
	}

	return underReplicated, nil
}

// LogDirsHealthy checks if log directories are healthy (no future partitions)
func (c *Checker) LogDirsHealthy(ctx context.Context, adm KafkaAdminClient) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.checkTimeout)
	defer cancel()

	// DescribeBrokerLogDirs returns DescribedLogDirs which is map[string]DescribedLogDir
	logDirs, err := adm.DescribeBrokerLogDirs(ctx, c.brokerID, nil)
	if err != nil {
		return false, fmt.Errorf("failed to describe log dirs: %w", err)
	}

	// Check if there was an error for any directory
	if err := logDirs.Error(); err != nil {
		c.logger.Warn("error describing log dirs", "error", err)
		return false, nil
	}

	// Check each partition for future replicas
	var foundFuture bool
	logDirs.EachPartition(func(p kadm.DescribedLogDirPartition) {
		if p.IsFuture {
			c.logger.Warn("found future partition",
				"topic", p.Topic,
				"partition", p.Partition,
				"dir", p.Dir)
			foundFuture = true
		}
	})

	return !foundFuture, nil
}
