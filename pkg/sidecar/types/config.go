package types

import (
	"log/slog"
	"os"
	"time"

	"github.com/controlplane-com/kafka-orchestrator/pkg/sidecar/discovery"
	"github.com/controlplane-com/libs-go/pkg/config"
)

// Config holds the configuration for the Kafka sidecar
type ConfigSchema struct {
	// BrokerID is the Kafka broker ID
	// Auto-discovered from $HOSTNAME if not set (format: workload-N -> N)
	BrokerID int32 `cpln:"default:0;env:BROKER_ID"`

	// WorkloadName is the name of the workload for building per-pod hostnames.
	// Auto-discovered from CPLN_WORKLOAD if not set.
	WorkloadName string `cpln:"env:WORKLOAD_NAME"`

	// GvcAlias is the GVC alias (Kubernetes namespace) for building per-pod hostnames.
	// Auto-discovered from CPLN_GVC_ALIAS if not set.
	GvcAlias string `cpln:"env:GVC_ALIAS"`

	// ReplicaCount is the number of Kafka replicas for building bootstrap server list
	ReplicaCount int `cpln:"default:1;env:REPLICA_COUNT"`

	// KafkaPort is the Kafka broker port
	KafkaPort int `cpln:"default:9092;env:KAFKA_PORT"`

	// BootstrapServers is the Kafka bootstrap servers list. Auto-built from
	// WorkloadName/GvcAlias/ReplicaCount via the StatefulSet's headless Service per-pod
	// DNS if not set explicitly. We always use the in-cluster headless path because
	// the orchestrator only ever talks to brokers it's co-located with — there's no
	// cross-cluster or cross-location use case that would justify the cpln.local mesh
	// path, and that path's `-ext` Service readiness gating creates a chicken-and-egg
	// deadlock during cold start.
	BootstrapServers string `cpln:"env:BOOTSTRAP_SERVERS"`

	// SASL authentication configuration
	// SASLEnabled enables SASL authentication
	SASLEnabled bool `cpln:"default:false;env:SASL_ENABLED"`

	// SASLMechanism is the SASL mechanism (PLAIN, SCRAM-SHA-256, SCRAM-SHA-512)
	SASLMechanism string `cpln:"default:PLAIN;env:SASL_MECHANISM"`

	// SASLUsername is the SASL username
	SASLUsername string `cpln:"env:SASL_USERNAME"`

	// SASLPassword is the SASL password
	SASLPassword string `cpln:"env:SASL_PASSWORD;sensitive"`

	// CheckTimeout is the health check timeout duration
	CheckTimeout time.Duration `cpln:"default:10s;env:CHECK_TIMEOUT"`

	// Port is the HTTP server port
	Port int `cpln:"default:8080;env:PORT"`

	LogLevel string `cpln:"default:info;env:LOG_LEVEL"`
}

var Config *ConfigSchema

// Initialize initializes the configuration. Must be called before using Config.
func Initialize(logger *slog.Logger) error {
	Config = &ConfigSchema{}

	if err := config.ParseSchema(Config); err != nil {
		return err
	}

	// Auto-discover broker ID if BROKER_ID env var is not explicitly set
	if os.Getenv("BROKER_ID") == "" {
		brokerID, err := discovery.DiscoverBrokerID()
		if err != nil {
			return err
		}
		Config.BrokerID = brokerID
		logger.Info("auto-discovered broker ID from hostname",
			"brokerID", brokerID,
			"hostname", os.Getenv("HOSTNAME"))
	}

	// Auto-build bootstrap servers if not explicitly set
	if Config.BootstrapServers == "" {
		// Try to get workload name from config, or discover from CPLN_WORKLOAD
		workloadName := Config.WorkloadName
		if workloadName == "" {
			discovered, err := discovery.DiscoverWorkloadName()
			if err != nil {
				return err
			}
			workloadName = discovered
			logger.Info("discovered workload name from CPLN_WORKLOAD",
				"workloadName", workloadName)
		}

		gvcAlias := Config.GvcAlias
		if gvcAlias == "" {
			discovered, err := discovery.DiscoverGvcAlias()
			if err != nil {
				return err
			}
			gvcAlias = discovered
			logger.Info("discovered GVC alias from CPLN_GVC_ALIAS",
				"gvcAlias", gvcAlias)
		}

		Config.BootstrapServers = discovery.BuildBootstrapServers(
			workloadName,
			gvcAlias,
			Config.ReplicaCount,
			Config.KafkaPort,
		)
		logger.Info("auto-built bootstrap servers",
			"bootstrapServers", Config.BootstrapServers)
	}

	return nil
}
