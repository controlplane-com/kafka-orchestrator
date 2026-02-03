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
	defaultCgroupV2BasePath = "/sys/fs/cgroup"
)

// CgroupV2Reader reads memory metrics from cgroup v2
type CgroupV2Reader struct {
	logger   *slog.Logger
	basePath string
}

// NewCgroupV2Reader creates a new cgroup v2 reader
func NewCgroupV2Reader(logger *slog.Logger) *CgroupV2Reader {
	return &CgroupV2Reader{
		logger:   logger,
		basePath: defaultCgroupV2BasePath,
	}
}

// NewCgroupV2ReaderWithBasePath creates a new cgroup v2 reader with a custom base path (for testing)
func NewCgroupV2ReaderWithBasePath(logger *slog.Logger, basePath string) *CgroupV2Reader {
	return &CgroupV2Reader{
		logger:   logger,
		basePath: basePath,
	}
}

// ReadMemoryMetrics reads memory metrics from cgroup v2 files
func (r *CgroupV2Reader) ReadMemoryMetrics() (*MemoryMetrics, error) {
	metrics := &MemoryMetrics{}

	usagePath := r.basePath + "/memory.current"
	limitPath := r.basePath + "/memory.max"

	// Read usage (memory.current)
	usage, err := readUint64FromFile(usagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read memory.current: %w", err)
	}
	metrics.Usage = usage

	// Read limit (memory.max)
	limit, err := readUint64FromFile(limitPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read memory.max: %w", err)
	}
	metrics.Limit = limit

	// Read stats (anon -> rss, inactive_file)
	stats, err := r.readMemoryStat()
	if err != nil {
		return nil, fmt.Errorf("failed to read memory.stat: %w", err)
	}

	// In cgroup v2, "anon" is the equivalent of RSS (anonymous memory)
	metrics.RSS = stats["anon"]
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

// readMemoryStat parses the memory.stat file for cgroup v2
func (r *CgroupV2Reader) readMemoryStat() (map[string]uint64, error) {
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
