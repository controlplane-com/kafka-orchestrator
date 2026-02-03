package metrics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewCgroupV1Reader(t *testing.T) {
	logger := testLogger()
	reader := NewCgroupV1Reader(logger)

	if reader == nil {
		t.Error("expected non-nil reader")
		return
	}
	if reader.basePath != defaultCgroupV1BasePath {
		t.Errorf("expected basePath=%q, got %q", defaultCgroupV1BasePath, reader.basePath)
	}
}

func TestNewCgroupV1ReaderWithBasePath(t *testing.T) {
	logger := testLogger()
	customPath := "/custom/path"
	reader := NewCgroupV1ReaderWithBasePath(logger, customPath)

	if reader == nil {
		t.Error("expected non-nil reader")
		return
	}
	if reader.basePath != customPath {
		t.Errorf("expected basePath=%q, got %q", customPath, reader.basePath)
	}
}

func TestCgroupV1ReaderReadMemoryMetrics(t *testing.T) {
	logger := testLogger()

	// Get the path to testdata
	testdataPath := filepath.Join("testdata", "cgroupv1")
	absPath, err := filepath.Abs(testdataPath)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	reader := NewCgroupV1ReaderWithBasePath(logger, absPath)
	metrics, err := reader.ReadMemoryMetrics()
	if err != nil {
		t.Fatalf("failed to read memory metrics: %v", err)
	}

	// Verify the metrics match the testdata values
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

	// Verify derived metrics
	expectedWorkingSet := metrics.Usage - metrics.InactiveFile
	if metrics.WorkingSet != expectedWorkingSet {
		t.Errorf("expected WorkingSet=%d, got %d", expectedWorkingSet, metrics.WorkingSet)
	}

	if metrics.Limit > 0 {
		expectedOOMRatio := float64(metrics.WorkingSet) / float64(metrics.Limit)
		if metrics.OOMRatio != expectedOOMRatio {
			t.Errorf("expected OOMRatio=%f, got %f", expectedOOMRatio, metrics.OOMRatio)
		}

		expectedOOMFloorRatio := float64(metrics.RSS) / float64(metrics.Limit)
		if metrics.OOMFloorRatio != expectedOOMFloorRatio {
			t.Errorf("expected OOMFloorRatio=%f, got %f", expectedOOMFloorRatio, metrics.OOMFloorRatio)
		}
	}
}

func TestCgroupV1ReaderReadMemoryMetrics_FileNotFound(t *testing.T) {
	logger := testLogger()
	reader := NewCgroupV1ReaderWithBasePath(logger, "/nonexistent/path")

	_, err := reader.ReadMemoryMetrics()
	if err == nil {
		t.Error("expected error for nonexistent path, got nil")
	}
}

func TestCgroupV1ReaderReadMemoryMetrics_InvalidUsageValue(t *testing.T) {
	logger := testLogger()

	// Create temp directory with invalid usage file
	tmpDir := t.TempDir()

	// Write invalid usage file
	if err := os.WriteFile(filepath.Join(tmpDir, "memory.usage_in_bytes"), []byte("invalid"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	reader := NewCgroupV1ReaderWithBasePath(logger, tmpDir)
	_, err := reader.ReadMemoryMetrics()
	if err == nil {
		t.Error("expected error for invalid usage value, got nil")
	}
}

func TestCgroupV1ReaderReadMemoryMetrics_MissingLimitFile(t *testing.T) {
	logger := testLogger()

	// Create temp directory with only usage file
	tmpDir := t.TempDir()

	// Write valid usage file
	if err := os.WriteFile(filepath.Join(tmpDir, "memory.usage_in_bytes"), []byte("100\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	reader := NewCgroupV1ReaderWithBasePath(logger, tmpDir)
	_, err := reader.ReadMemoryMetrics()
	if err == nil {
		t.Error("expected error for missing limit file, got nil")
	}
}

func TestCgroupV1ReaderReadMemoryMetrics_MissingStatFile(t *testing.T) {
	logger := testLogger()

	// Create temp directory with usage and limit but no stat file
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "memory.usage_in_bytes"), []byte("100\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "memory.limit_in_bytes"), []byte("200\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	reader := NewCgroupV1ReaderWithBasePath(logger, tmpDir)
	_, err := reader.ReadMemoryMetrics()
	if err == nil {
		t.Error("expected error for missing stat file, got nil")
	}
}

func TestCgroupV1ReaderReadMemoryMetrics_ZeroLimit(t *testing.T) {
	logger := testLogger()

	// Create temp directory with zero limit (no limit set)
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "memory.usage_in_bytes"), []byte("100\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "memory.limit_in_bytes"), []byte("0\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "memory.stat"), []byte("rss 50\ninactive_file 10\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	reader := NewCgroupV1ReaderWithBasePath(logger, tmpDir)
	metrics, err := reader.ReadMemoryMetrics()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With zero limit, OOM ratios should be 0
	if metrics.OOMRatio != 0 {
		t.Errorf("expected OOMRatio=0 with zero limit, got %f", metrics.OOMRatio)
	}
	if metrics.OOMFloorRatio != 0 {
		t.Errorf("expected OOMFloorRatio=0 with zero limit, got %f", metrics.OOMFloorRatio)
	}
}

func TestCgroupV1ReaderReadMemoryStat_MalformedLines(t *testing.T) {
	logger := testLogger()
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "memory.usage_in_bytes"), []byte("100\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "memory.limit_in_bytes"), []byte("200\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	// Stat file with malformed lines
	statContent := "rss 50\ninvalid_line_without_value\n  \nthree fields here\ninactive_file notanumber\nactive_file 30\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "memory.stat"), []byte(statContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	reader := NewCgroupV1ReaderWithBasePath(logger, tmpDir)
	metrics, err := reader.ReadMemoryMetrics()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still parse valid lines
	if metrics.RSS != 50 {
		t.Errorf("expected RSS=50, got %d", metrics.RSS)
	}
	// inactive_file had invalid value, so should be 0
	if metrics.InactiveFile != 0 {
		t.Errorf("expected InactiveFile=0 (invalid parse), got %d", metrics.InactiveFile)
	}
}
