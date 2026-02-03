package metrics

import (
	"log/slog"
	"os"
)

// CgroupVersion represents the cgroup version
type CgroupVersion int

const (
	CgroupUnknown CgroupVersion = iota
	CgroupV1
	CgroupV2
)

// MemoryMetrics holds memory metrics from cgroups
type MemoryMetrics struct {
	Usage         uint64  // Total memory usage
	Limit         uint64  // Memory limit
	RSS           uint64  // Resident set size (non-reclaimable)
	InactiveFile  uint64  // Inactive file (reclaimable page cache)
	WorkingSet    uint64  // Usage - InactiveFile
	OOMRatio      float64 // WorkingSet / Limit
	OOMFloorRatio float64 // RSS / Limit
}

// CgroupReader provides an interface for reading cgroup metrics
type CgroupReader interface {
	ReadMemoryMetrics() (*MemoryMetrics, error)
}

// DetectCgroupVersion detects the cgroup version in use
func DetectCgroupVersion() CgroupVersion {
	// Check for cgroup v2 first (unified hierarchy)
	if _, err := os.Stat("/sys/fs/cgroup/memory.current"); err == nil {
		return CgroupV2
	}

	// Check for cgroup v1
	if _, err := os.Stat("/sys/fs/cgroup/memory/memory.usage_in_bytes"); err == nil {
		return CgroupV1
	}

	return CgroupUnknown
}

// NewCgroupReader creates a new cgroup reader based on detected version
func NewCgroupReader(logger *slog.Logger) CgroupReader {
	version := DetectCgroupVersion()

	switch version {
	case CgroupV1:
		logger.Info("detected cgroup v1")
		return NewCgroupV1Reader(logger)
	case CgroupV2:
		logger.Info("detected cgroup v2")
		return NewCgroupV2Reader(logger)
	default:
		logger.Warn("cgroup version not detected, using v2 paths as default")
		return NewCgroupV2Reader(logger)
	}
}
