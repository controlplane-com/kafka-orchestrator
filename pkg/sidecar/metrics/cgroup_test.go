package metrics

import (
	"log/slog"
	"os"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestDetectCgroupVersion(t *testing.T) {
	// This test checks the actual system state - results depend on the environment
	version := DetectCgroupVersion()

	// Just verify it returns a valid enum value
	if version != CgroupUnknown && version != CgroupV1 && version != CgroupV2 {
		t.Errorf("DetectCgroupVersion returned invalid version: %d", version)
	}
}

func TestCgroupVersionConstants(t *testing.T) {
	// Verify the constants have expected values
	if CgroupUnknown != 0 {
		t.Errorf("expected CgroupUnknown to be 0, got %d", CgroupUnknown)
	}
	if CgroupV1 != 1 {
		t.Errorf("expected CgroupV1 to be 1, got %d", CgroupV1)
	}
	if CgroupV2 != 2 {
		t.Errorf("expected CgroupV2 to be 2, got %d", CgroupV2)
	}
}

func TestNewCgroupReader(t *testing.T) {
	logger := testLogger()
	reader := NewCgroupReader(logger)

	if reader == nil {
		t.Error("expected non-nil reader")
	}

	// Verify the reader implements CgroupReader interface
	var _ CgroupReader = reader
}

func TestMemoryMetrics(t *testing.T) {
	metrics := &MemoryMetrics{
		Usage:         104857600, // 100 MB
		Limit:         268435456, // 256 MB
		RSS:           52428800,  // 50 MB
		InactiveFile:  10485760,  // 10 MB
		WorkingSet:    94371840,  // 90 MB
		OOMRatio:      0.35,
		OOMFloorRatio: 0.19,
	}

	if metrics.Usage != 104857600 {
		t.Errorf("expected Usage=104857600, got %d", metrics.Usage)
	}
	if metrics.Limit != 268435456 {
		t.Errorf("expected Limit=268435456, got %d", metrics.Limit)
	}
	if metrics.RSS != 52428800 {
		t.Errorf("expected RSS=52428800, got %d", metrics.RSS)
	}
	if metrics.InactiveFile != 10485760 {
		t.Errorf("expected InactiveFile=10485760, got %d", metrics.InactiveFile)
	}
	if metrics.WorkingSet != 94371840 {
		t.Errorf("expected WorkingSet=94371840, got %d", metrics.WorkingSet)
	}
}
