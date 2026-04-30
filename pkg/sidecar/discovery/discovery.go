package discovery

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DiscoverBrokerID extracts the replica index from the hostname.
// Hostname format: ${workloadName}-${replicaIndex}
// Example: "kafka-2" -> brokerID = 2
func DiscoverBrokerID() (int32, error) {
	hostname := os.Getenv("HOSTNAME")
	if hostname == "" {
		return 0, errors.New("HOSTNAME environment variable not set")
	}

	return ParseBrokerIDFromHostname(hostname)
}

// ParseBrokerIDFromHostname extracts the replica index from a hostname string.
// Hostname format: ${workloadName}-${replicaIndex}
// Example: "kafka-2" -> brokerID = 2
func ParseBrokerIDFromHostname(hostname string) (int32, error) {
	lastHyphen := strings.LastIndex(hostname, "-")
	if lastHyphen == -1 {
		return 0, fmt.Errorf("invalid hostname format (no hyphen): %s", hostname)
	}
	if lastHyphen == len(hostname)-1 {
		return 0, fmt.Errorf("invalid hostname format (trailing hyphen): %s", hostname)
	}

	indexStr := hostname[lastHyphen+1:]
	index, err := strconv.ParseInt(indexStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("failed to parse replica index from hostname %s: %w", hostname, err)
	}

	return int32(index), nil
}

// BuildBootstrapServers creates per-pod hostnames using the Kubernetes headless
// Service that backs the StatefulSet. The orchestrator only ever talks to the
// brokers it lives next to (same cluster, same GVC), so we always use the
// in-cluster headless DNS path — never the cpln.local mesh path. The headless
// Service is published with publishNotReadyAddresses=true (required for KRaft
// peer discovery on cold start), which means DNS resolves the pod IPs even
// before any replica is Ready and the orchestrator can break the readiness
// chicken-and-egg.
//
// Format: ${workloadName}-${i}.${workloadName}.${gvcAlias}.svc.cluster.local:${port}
//
// gvcAlias here is the Control Plane GVC's parent identifier (the value
// injected as $CPLN_GVC_ALIAS, which is the Kubernetes namespace), not the GVC
// name.
func BuildBootstrapServers(workloadName, gvcAlias string, replicaCount int, port int) string {
	if replicaCount <= 0 {
		replicaCount = 1
	}

	servers := make([]string, replicaCount)
	for i := 0; i < replicaCount; i++ {
		servers[i] = fmt.Sprintf("%s-%d.%s.%s.svc.cluster.local:%d",
			workloadName, i, workloadName, gvcAlias, port)
	}

	return strings.Join(servers, ",")
}

// DiscoverWorkloadName extracts the workload name from CPLN_WORKLOAD env var.
// CPLN_WORKLOAD format: /org/{org}/gvc/{gvc}/workload/{workloadName}
// Example: "/org/gitops/gvc/igor-kafka/workload/kafka-fix-cluster" -> "kafka-fix-cluster"
func DiscoverWorkloadName() (string, error) {
	cplnWorkload := os.Getenv("CPLN_WORKLOAD")
	if cplnWorkload == "" {
		return "", errors.New("CPLN_WORKLOAD environment variable not set")
	}

	return ParseWorkloadNameFromLink(cplnWorkload)
}

// ParseWorkloadNameFromLink extracts the workload name from a CPLN workload link.
// Format: /org/{org}/gvc/{gvc}/workload/{workloadName}
// Example: "/org/gitops/gvc/igor-kafka/workload/kafka-fix-cluster" -> "kafka-fix-cluster"
func ParseWorkloadNameFromLink(link string) (string, error) {
	// Find the last segment after "/workload/"
	const prefix = "/workload/"
	idx := strings.LastIndex(link, prefix)
	if idx == -1 {
		return "", fmt.Errorf("invalid CPLN_WORKLOAD format (missing /workload/): %s", link)
	}

	name := link[idx+len(prefix):]
	if name == "" {
		return "", fmt.Errorf("invalid CPLN_WORKLOAD format (empty workload name): %s", link)
	}

	// Remove any trailing path segments (shouldn't exist, but be safe)
	if slashIdx := strings.Index(name, "/"); slashIdx != -1 {
		name = name[:slashIdx]
	}

	return name, nil
}

// DiscoverGvcAlias returns the GVC alias from CPLN_GVC_ALIAS env var.
// This is the Kubernetes namespace name (Control Plane's GVC parent identifier),
// used as the in-cluster DNS namespace for headless Service per-pod records.
func DiscoverGvcAlias() (string, error) {
	gvcAlias := os.Getenv("CPLN_GVC_ALIAS")
	if gvcAlias == "" {
		return "", errors.New("CPLN_GVC_ALIAS environment variable not set")
	}
	return gvcAlias, nil
}
