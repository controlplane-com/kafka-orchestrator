# Kafka Orchestrator

A Kafka sidecar designed exclusively for [Control Plane](https://controlplane.com) stateful workloads. Provides health checks, Prometheus metrics, and automatic broker discovery using Control Plane's environment and replica-direct DNS.

## Features

- **Health Checks** - Kubernetes-compatible liveness and readiness probes using [franz-go](https://github.com/twmb/franz-go)
- **Auto-Discovery** - Automatically discovers broker ID, bootstrap servers, and cluster topology from Control Plane environment
- **Prometheus Metrics** - Exposes cgroup memory metrics for OOM monitoring and capacity planning
- **SASL Support** - PLAIN, SCRAM-SHA-256, and SCRAM-SHA-512 authentication
- **Zero Config** - Works out of the box with sensible defaults from Control Plane environment

## How It Works

The sidecar runs alongside your Kafka container in a stateful workload. It:

1. **Discovers** the broker ID from the pod hostname (e.g., `kafka-2` -> broker ID `2`)
2. **Builds** bootstrap server addresses using Control Plane's replica-direct DNS
3. **Monitors** broker health via cluster metadata and log directory status
4. **Exposes** Prometheus metrics for memory pressure monitoring

```
┌─────────────────────────────────────────────────────┐
│                 Stateful Workload                   │
│  ┌─────────────┐    ┌───────────────────────────┐   │
│  │    Kafka    │    │     Kafka Sidecar         │   │
│  │   Broker    │◄───│  :8080/health/ready       │   │
│  │    :9092    │    │  :8080/health/live        │   │
│  │             │    │  :8080/metrics            │   │
│  └─────────────┘    └───────────────────────────┘   │
└─────────────────────────────────────────────────────┘
```

## Quick Start

Add the sidecar container to your stateful Kafka workload:

```yaml
kind: workload
name: kafka
spec:
  type: stateful
  containers:
    - name: kafka
      image: bitnami/kafka:latest
      cpu: "500m"
      memory: "1Gi"
      ports:
        - number: 9092
          protocol: tcp
      readinessProbe:
        httpGet:
          path: /health/ready
          port: 8080
      livenessProbe:
        httpGet:
          path: /health/live
          port: 8080
      # ... your kafka config
    - name: kafka-sidecar
      image: ghcr.io/controlplane-com/cpln-build/kafka-orchestrator:latest
      cpu: "100m"
      memory: "128Mi"
      ports:
        - number: 8080
          protocol: http
      env:
        - name: REPLICA_COUNT
          value: "3"
```

The Kafka container uses the sidecar's HTTP endpoints for health probes, allowing Kubernetes to properly manage broker lifecycle.

## Configuration

All configuration is via environment variables. Values are auto-discovered from Control Plane's injected environment (`CPLN_WORKLOAD`, `CPLN_GVC`, `CPLN_LOCATION`, `HOSTNAME`).

| Variable | Default | Description |
|----------|---------|-------------|
| `REPLICA_COUNT` | `1` | Number of Kafka replicas for bootstrap server list |
| `KAFKA_PORT` | `9092` | Kafka broker port |
| `PORT` | `8080` | HTTP server port |
| `CHECK_TIMEOUT` | `10s` | Health check timeout |
| `LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |

**SASL Authentication:**

| Variable | Default | Description |
|----------|---------|-------------|
| `SASL_ENABLED` | `false` | Enable SASL authentication |
| `SASL_MECHANISM` | `PLAIN` | `PLAIN`, `SCRAM-SHA-256`, or `SCRAM-SHA-512` |
| `SASL_USERNAME` | - | SASL username (required if enabled) |
| `SASL_PASSWORD` | - | SASL password (supports `cpln://secret/` references) |

**Advanced Overrides:**

| Variable | Default | Description |
|----------|---------|-------------|
| `BROKER_ID` | *from `$HOSTNAME`* | Override discovered broker ID |
| `WORKLOAD_NAME` | *from `CPLN_WORKLOAD`* | Override discovered workload name |
| `LOCATION` | *from `CPLN_LOCATION`* | Override discovered location |
| `GVC_NAME` | *from `CPLN_GVC`* | Override discovered GVC name |
| `BOOTSTRAP_SERVERS` | *auto-built* | Override auto-built bootstrap server list |

### Auto-Discovery

The sidecar automatically discovers configuration from Control Plane's environment:

| Value | Source | Example |
|-------|--------|---------|
| Broker ID | `$HOSTNAME` | `kafka-2` -> `2` |
| Workload name | `$CPLN_WORKLOAD` | `/org/.../workload/kafka` -> `kafka` |
| Location | `$CPLN_LOCATION` | `aws-us-west-2` |
| GVC name | `$CPLN_GVC` | `my-gvc` |

Bootstrap servers are built as:
```
replica-{i}.{workload}.{location}.{gvc}.cpln.local:{port}
```

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /health/live` | Liveness check - returns 200 if broker appears in cluster metadata |
| `GET /health/ready` | Readiness check - validates broker health, ISR status, and log directories |
| `GET /metrics` | Prometheus metrics endpoint |
| `GET /about` | Version and build information |

### Health Check Details

**Liveness (`/health/live`)**
- Connects to Kafka cluster
- Verifies this broker ID exists in cluster metadata
- Fast check suitable for frequent polling

**Readiness (`/health/ready`)**
- All liveness checks, plus:
- Verifies a controller is elected
- Checks for under-replicated partitions on this broker
- Validates log directories have no future partitions

## Metrics

The sidecar exposes cgroup memory metrics for monitoring OOM risk:

| Metric | Description |
|--------|-------------|
| `kafka_memory_usage_bytes` | Total memory usage |
| `kafka_memory_limit_bytes` | Container memory limit |
| `kafka_memory_rss_bytes` | Resident set size (non-reclaimable) |
| `kafka_memory_inactive_file_bytes` | Inactive file cache (reclaimable) |
| `kafka_memory_working_set_bytes` | Working set (`usage - inactive_file`) |
| `kafka_memory_oom_ratio` | OOM risk ratio (`working_set / limit`) |
| `kafka_memory_oom_floor_ratio` | OOM floor ratio (`rss / limit`) |

## Examples

### Basic 3-Node Cluster

Add the sidecar container to your workload's `spec.containers` array:

```yaml
- name: kafka-sidecar
  image: ghcr.io/controlplane-com/cpln-build/kafka-orchestrator:latest
  cpu: "100m"
  memory: "128Mi"
  ports:
    - number: 8080
      protocol: http
  env:
    - name: REPLICA_COUNT
      value: "3"
```

### SASL-Enabled Cluster

Add SASL configuration to the sidecar's environment variables:

```yaml
env:
  - name: REPLICA_COUNT
    value: "3"
  - name: SASL_ENABLED
    value: "true"
  - name: SASL_MECHANISM
    value: "SCRAM-SHA-512"
  - name: SASL_USERNAME
    value: "admin"
  - name: SASL_PASSWORD
    value: "cpln://secret/kafka-secrets.admin-password"
```

## Development

### Prerequisites

- Go 1.25+
- Docker (for testing)

### Build

```bash
# Run tests
make test

# Build multi-arch images
make push-image TAG=v1.0.0

# Security scan
make trivy-image TAG=v1.0.0
```

### Project Structure

```
kafka-orchestrator/
├── cmd/
│   └── sidecar/           # Main entrypoint and HTTP server
├── pkg/
│   ├── about/             # Version information
│   └── sidecar/
│       ├── types/         # Configuration schema
│       ├── health/        # Kafka health checks (franz-go)
│       ├── metrics/       # Cgroup memory metrics (Prometheus)
│       └── discovery/     # Auto-discovery for broker ID and topology
├── Dockerfile             # Multi-stage build (builder, tester, runner)
└── Makefile               # Build and test targets
```

## License

MIT License - see [LICENSE](LICENSE) for details.
