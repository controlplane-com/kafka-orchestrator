package metrics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewCgroupV2Reader(t *testing.T) {
	logger := testLogger()
	reader := NewCgroupV2Reader(logger)

	if reader == nil {
		t.Error("expected non-nil reader")
		return
	}
	if reader.basePath != defaultCgroupV2BasePath {
		t.Errorf("expected basePath=%q, got %q", defaultCgroupV2BasePath, reader.basePath)
	}
}

func TestNewCgroupV2ReaderWithBasePath(t *testing.T) {
	logger := testLogger()
	customPath := "/custom/path"
	reader := NewCgroupV2ReaderWithBasePath(logger, customPath)

	if reader == nil {
		t.Error("expected non-nil reader")
		return
	}
	if reader.basePath != customPath {
		t.Errorf("expected basePath=%q, got %q", customPath, reader.basePath)
	}
}

func TestCgroupV2ReaderReadMemoryMetrics(t *testing.T) {
	logger := testLogger()

	// Get the path to testdata
	testdataPath := filepath.Join("testdata", "cgroupv2")
	absPath, err := filepath.Abs(testdataPath)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	reader := NewCgroupV2ReaderWithBasePath(logger, absPath)
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
	// In cgroup v2, RSS is from "anon" field
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

func TestCgroupV2ReaderReadMemoryMetrics_FileNotFound(t *testing.T) {
	logger := testLogger()
	reader := NewCgroupV2ReaderWithBasePath(logger, "/nonexistent/path")

	_, err := reader.ReadMemoryMetrics()
	if err == nil {
		t.Error("expected error for nonexistent path, got nil")
	}
}

func TestCgroupV2ReaderReadMemoryMetrics_MaxValue(t *testing.T) {
	logger := testLogger()

	// Create temp directory with "max" limit (no limit set)
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "memory.current"), []byte("100\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "memory.max"), []byte("max\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "memory.stat"), []byte("anon 50\ninactive_file 10\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	reader := NewCgroupV2ReaderWithBasePath(logger, tmpDir)
	metrics, err := reader.ReadMemoryMetrics()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// "max" should be parsed as 0
	if metrics.Limit != 0 {
		t.Errorf("expected Limit=0 for 'max', got %d", metrics.Limit)
	}

	// With zero limit, OOM ratios should be 0
	if metrics.OOMRatio != 0 {
		t.Errorf("expected OOMRatio=0 with max limit, got %f", metrics.OOMRatio)
	}
}

func TestCgroupV2ReaderReadMemoryMetrics_InvalidCurrentValue(t *testing.T) {
	logger := testLogger()

	// Create temp directory with invalid current file
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "memory.current"), []byte("invalid"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	reader := NewCgroupV2ReaderWithBasePath(logger, tmpDir)
	_, err := reader.ReadMemoryMetrics()
	if err == nil {
		t.Error("expected error for invalid current value, got nil")
	}
}

func TestCgroupV2ReaderReadMemoryMetrics_MissingMaxFile(t *testing.T) {
	logger := testLogger()

	// Create temp directory with only current file
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "memory.current"), []byte("100\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	reader := NewCgroupV2ReaderWithBasePath(logger, tmpDir)
	_, err := reader.ReadMemoryMetrics()
	if err == nil {
		t.Error("expected error for missing max file, got nil")
	}
}

func TestCgroupV2ReaderReadMemoryMetrics_MissingStatFile(t *testing.T) {
	logger := testLogger()

	// Create temp directory with current and max but no stat file
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "memory.current"), []byte("100\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "memory.max"), []byte("200\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	reader := NewCgroupV2ReaderWithBasePath(logger, tmpDir)
	_, err := reader.ReadMemoryMetrics()
	if err == nil {
		t.Error("expected error for missing stat file, got nil")
	}
}

func TestCgroupV2ReaderReadMemoryMetrics_UsageLessThanInactiveFile(t *testing.T) {
	logger := testLogger()

	// Edge case: inactive_file > usage (should result in WorkingSet = 0)
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "memory.current"), []byte("50\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "memory.max"), []byte("200\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "memory.stat"), []byte("anon 30\ninactive_file 100\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	reader := NewCgroupV2ReaderWithBasePath(logger, tmpDir)
	metrics, err := reader.ReadMemoryMetrics()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// WorkingSet should be 0 when usage < inactive_file
	if metrics.WorkingSet != 0 {
		t.Errorf("expected WorkingSet=0 when usage < inactive_file, got %d", metrics.WorkingSet)
	}
}

func TestCgroupV2ReaderReadMemoryStat_MalformedLines(t *testing.T) {
	logger := testLogger()
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "memory.current"), []byte("100\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "memory.max"), []byte("200\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	// Stat file with malformed lines
	statContent := "anon 50\ninvalid_line_without_value\n  \nthree fields here\ninactive_file notanumber\nactive_file 30\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "memory.stat"), []byte(statContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	reader := NewCgroupV2ReaderWithBasePath(logger, tmpDir)
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
