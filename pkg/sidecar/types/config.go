package types

import (
	"log/slog"
	"os"
	"time"

	"github.com/controlplane-com/libs-go/pkg/config"
	"gitlab.com/controlplane/controlplane/kafka-orchestrator/pkg/sidecar/discovery"
)

// Config holds the configuration for the Kafka sidecar
type ConfigSchema struct {
	// BrokerID is the Kafka broker ID
	// Auto-discovered from $HOSTNAME if not set (format: workload-N -> N)
	BrokerID int32 `cpln:"default:0;env:BROKER_ID"`

	// WorkloadName is the name of the workload for building replica-direct hostnames
	// Auto-discovered from CPLN_WORKLOAD if not set
	WorkloadName string `cpln:"env:WORKLOAD_NAME"`

	// GvcAlias is the GVC alias for building replica-direct hostnames (legacy)
	// Auto-discovered from CPLN_GVC_ALIAS if not set
	GvcAlias string `cpln:"env:GVC_ALIAS"`

	// GvcName is the GVC name for building replica-direct hostnames
	// Auto-discovered from CPLN_GVC if not set
	GvcName string `cpln:"env:GVC_NAME"`

	// Location is the location for building replica-direct hostnames
	// Auto-discovered from CPLN_LOCATION if not set
	Location string `cpln:"env:LOCATION"`

	// ReplicaCount is the number of Kafka replicas for building bootstrap server list
	ReplicaCount int `cpln:"default:1;env:REPLICA_COUNT"`

	// KafkaPort is the Kafka broker port
	KafkaPort int `cpln:"default:9092;env:KAFKA_PORT"`

	// BootstrapServers is the Kafka bootstrap servers
	// Auto-built from WorkloadName/GvcAlias/ReplicaCount if not set
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

func init() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	if err := Initialize(logger); err != nil {
		logger.Error("failed to initialize configuration", "error", err)
		os.Exit(1)
	}
	logger.Info("configuration loaded",
		"config", config.Summarize(Config))
}

// Initialize initializes the configuration. This is separated from init() for testability.
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

		// Try to get location from config, or discover from CPLN_LOCATION
		location := Config.Location
		if location == "" {
			discovered, err := discovery.DiscoverLocation()
			if err != nil {
				return err
			}
			location = discovered
			logger.Info("discovered location from CPLN_LOCATION",
				"location", location)
		}

		// Try to get GVC name from config, or discover from CPLN_GVC
		gvcName := Config.GvcName
		if gvcName == "" {
			discovered, err := discovery.DiscoverGvcName()
			if err != nil {
				return err
			}
			gvcName = discovered
			logger.Info("discovered GVC name from CPLN_GVC",
				"gvcName", gvcName)
		}

		Config.BootstrapServers = discovery.BuildBootstrapServers(
			workloadName,
			location,
			gvcName,
			Config.ReplicaCount,
			Config.KafkaPort,
		)
		logger.Info("auto-built bootstrap servers",
			"bootstrapServers", Config.BootstrapServers)
	}

	return nil
}
