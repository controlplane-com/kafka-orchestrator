package metrics

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

const (
	defaultCgroupV1BasePath = "/sys/fs/cgroup/memory"
)

// CgroupV1Reader reads memory metrics from cgroup v1
type CgroupV1Reader struct {
	logger   *slog.Logger
	basePath string
}

// NewCgroupV1Reader creates a new cgroup v1 reader
func NewCgroupV1Reader(logger *slog.Logger) *CgroupV1Reader {
	return &CgroupV1Reader{
		logger:   logger,
		basePath: defaultCgroupV1BasePath,
	}
}

// NewCgroupV1ReaderWithBasePath creates a new cgroup v1 reader with a custom base path (for testing)
func NewCgroupV1ReaderWithBasePath(logger *slog.Logger, basePath string) *CgroupV1Reader {
	return &CgroupV1Reader{
		logger:   logger,
		basePath: basePath,
	}
}

// ReadMemoryMetrics reads memory metrics from cgroup v1 files
func (r *CgroupV1Reader) ReadMemoryMetrics() (*MemoryMetrics, error) {
	metrics := &MemoryMetrics{}

	usagePath := r.basePath + "/memory.usage_in_bytes"
	limitPath := r.basePath + "/memory.limit_in_bytes"

	// Read usage
	usage, err := readUint64FromFile(usagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read memory usage: %w", err)
	}
	metrics.Usage = usage

	// Read limit
	limit, err := readUint64FromFile(limitPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read memory limit: %w", err)
	}
	metrics.Limit = limit

	// Read stats (rss and inactive_file)
	stats, err := r.readMemoryStat()
	if err != nil {
		return nil, fmt.Errorf("failed to read memory stat: %w", err)
	}

	metrics.RSS = stats["rss"]
	metrics.InactiveFile = stats["inactive_file"]

	// Calculate derived metrics
	if metrics.Usage >= metrics.InactiveFile {
		metrics.WorkingSet = metrics.Usage - metrics.InactiveFile
	}

	if metrics.Limit > 0 {
		metrics.OOMRatio = float64(metrics.WorkingSet) / float64(metrics.Limit)
		metrics.OOMFloorRatio = float64(metrics.RSS) / float64(metrics.Limit)
	}

	return metrics, nil
}

// readMemoryStat parses the memory.stat file
func (r *CgroupV1Reader) readMemoryStat() (map[string]uint64, error) {
	statPath := r.basePath + "/memory.stat"
	file, err := os.Open(statPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stats := make(map[string]uint64)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}

		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			r.logger.Warn("failed to parse stat value",
				"key", fields[0],
				"value", fields[1],
				"error", err)
			continue
		}

		stats[fields[0]] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

// readUint64FromFile reads a uint64 value from a file
func readUint64FromFile(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	value := strings.TrimSpace(string(data))

	// Handle "max" value which indicates no limit
	if value == "max" {
		return 0, nil
	}

	return strconv.ParseUint(value, 10, 64)
}
