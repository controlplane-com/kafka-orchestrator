# kafka-orchestrator

Kafka orchestration tools for Control Plane.

## Project Structure

```
kafka-orchestrator/
├── cmd/
│   └── sidecar/        # Kafka sidecar binary
├── pkg/
│   ├── about/          # Version information (shared across all commands)
│   └── sidecar/        # Sidecar-specific packages
│       ├── types/      # Configuration types
│       ├── health/     # Health check endpoints (franz-go)
│       ├── metrics/    # Cgroup memory metrics (Prometheus)
│       └── discovery/  # Auto-discovery for broker ID and bootstrap servers
```

## Building

```bash
# Build local image
make build-local

# Run tests
make test
```

## Configuration

The sidecar auto-discovers most configuration from Control Plane environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| BROKER_ID | No | auto from $HOSTNAME | Kafka broker ID (format: workload-N -> N) |
| WORKLOAD_NAME | No | auto from CPLN_WORKLOAD | Override workload name for bootstrap servers |
| LOCATION | No | auto from CPLN_LOCATION | Override location for bootstrap servers |
| GVC_NAME | No | auto from CPLN_GVC | Override GVC name for bootstrap servers |
| REPLICA_COUNT | No | 1 | Number of Kafka replicas for bootstrap server list |
| KAFKA_PORT | No | 9092 | Kafka broker port |
| BOOTSTRAP_SERVERS | No | auto-built | Explicit bootstrap servers (disables auto-build) |
| SASL_ENABLED | No | false | Enable SASL authentication |
| SASL_MECHANISM | No | PLAIN | SASL mechanism: PLAIN, SCRAM-SHA-256, or SCRAM-SHA-512 |
| SASL_USERNAME | No* | - | SASL username |
| SASL_PASSWORD | No* | - | SASL password (supports cpln://secret/ references) |
| CHECK_TIMEOUT | No | 10s | Health check timeout |
| PORT | No | 8080 | HTTP server port |

*Required if SASL_ENABLED is true

### Auto-Discovery

When running on Control Plane, the sidecar automatically discovers:
- **BROKER_ID**: Parsed from `$HOSTNAME` (e.g., `kafka-2` -> broker ID 2)
- **Workload name**: Parsed from `CPLN_WORKLOAD` (e.g., `/org/.../workload/kafka` -> `kafka`)
- **Location**: Read from `CPLN_LOCATION` (e.g., `aws-us-west-2`)
- **GVC name**: Read from `CPLN_GVC` (e.g., `my-gvc`)
- **Bootstrap servers**: Built as `replica-{i}.{workload}.{location}.{gvcName}.cpln.local:{port}`

## Endpoints

- `GET /health/live` - Liveness check (broker in metadata)
- `GET /health/ready` - Readiness check (full health validation)
- `GET /metrics` - Prometheus metrics
- `GET /about` - Version information

## Deployment

Control Plane workload manifests are in `deploy/`:

```bash
# Deploy Kafka with sidecar (stateful workload with volumeset)
cpln apply --file deploy/kafka-workload.yaml --gvc <your-gvc>

# Deploy sidecar only (for testing or existing Kafka)
cpln apply --file deploy/kafka-sidecar-only.yaml --gvc <your-gvc>
```

### Adding sidecar to existing Kafka workload

Add as a container in your stateful workload spec. The sidecar auto-discovers everything from Control Plane environment variables:

```yaml
containers:
  - name: kafka
    # ... your kafka container config
  - name: kafka-sidecar
    image: ghcr.io/controlplane-com/cpln-build/kafka-orchestrator:latest
    cpu: "100m"
    memory: 128Mi
    ports:
      - number: 8080
        protocol: http
    env:
      - name: REPLICA_COUNT
        value: "3"
```

For SASL-enabled clusters:

```yaml
env:
  - name: REPLICA_COUNT
    value: "3"
  - name: SASL_ENABLED
    value: "true"
  - name: SASL_MECHANISM
    value: "PLAIN"
  - name: SASL_USERNAME
    value: "admin"
  - name: SASL_PASSWORD
    value: "cpln://secret/kafka-secrets.admin-password"
```

For manual configuration (non-Control Plane or testing):

```yaml
env:
  - name: BROKER_ID
    value: "0"
  - name: BOOTSTRAP_SERVERS
    value: "localhost:9092"
```

Then configure probes on the Kafka container to use the sidecar:

```yaml
readinessProbe:
  httpGet:
    path: /health/ready
    port: 8080
livenessProbe:
  httpGet:
    path: /health/live
    port: 8080
```

## Types

DO NOT CREATE LOCAL TYPES UNLESS YOU CANNOT FIND ONE IN go-libs/schema.
