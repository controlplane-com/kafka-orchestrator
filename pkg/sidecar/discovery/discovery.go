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

// BuildBootstrapServers creates replica-direct hostnames for Kafka brokers.
// Format: replica-${replicaIndex}.${workloadName}.${location}.${gvcName}.cpln.local:${port}
func BuildBootstrapServers(workloadName, location, gvcName string, replicaCount int, port int) string {
	if replicaCount <= 0 {
		replicaCount = 1
	}

	servers := make([]string, replicaCount)
	for i := 0; i < replicaCount; i++ {
		servers[i] = fmt.Sprintf("replica-%d.%s.%s.%s.cpln.local:%d",
			i, workloadName, location, gvcName, port)
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
func DiscoverGvcAlias() (string, error) {
	gvcAlias := os.Getenv("CPLN_GVC_ALIAS")
	if gvcAlias == "" {
		return "", errors.New("CPLN_GVC_ALIAS environment variable not set")
	}
	return gvcAlias, nil
}

// DiscoverGvcName returns the GVC name from CPLN_GVC env var.
func DiscoverGvcName() (string, error) {
	gvcName := os.Getenv("CPLN_GVC")
	if gvcName == "" {
		return "", errors.New("CPLN_GVC environment variable not set")
	}
	return gvcName, nil
}

// DiscoverLocation returns the location from CPLN_LOCATION env var.
func DiscoverLocation() (string, error) {
	location := os.Getenv("CPLN_LOCATION")
	if location == "" {
		return "", errors.New("CPLN_LOCATION environment variable not set")
	}
	return location[strings.LastIndex(location, "/")+1:], nil
}
