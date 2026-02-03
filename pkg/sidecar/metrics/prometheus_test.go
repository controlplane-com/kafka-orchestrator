package metrics

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// MockCgroupReader is a mock implementation of CgroupReader for testing
type MockCgroupReader struct {
	Metrics *MemoryMetrics
	Err     error
}

func (m *MockCgroupReader) ReadMemoryMetrics() (*MemoryMetrics, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Metrics, nil
}

func TestNewCollector(t *testing.T) {
	logger := testLogger()
	collector := NewCollector(logger)

	if collector == nil {
		t.Error("expected non-nil collector")
		return
	}
	if collector.reader == nil {
		t.Error("expected non-nil reader")
	}
	if collector.logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestNewCollectorWithReader(t *testing.T) {
	logger := testLogger()
	mockReader := &MockCgroupReader{}
	collector := NewCollectorWithReader(logger, mockReader)

	if collector == nil {
		t.Error("expected non-nil collector")
		return
	}
	if collector.reader != mockReader {
		t.Error("expected collector to use provided reader")
	}
}

func TestCollectorDescribe(t *testing.T) {
	logger := testLogger()
	mockReader := &MockCgroupReader{}
	collector := NewCollectorWithReader(logger, mockReader)

	ch := make(chan *prometheus.Desc, 10)
	collector.Describe(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}

	// Should emit 8 metric descriptions
	if count != 8 {
		t.Errorf("expected 8 metric descriptions, got %d", count)
	}
}

func TestCollectorCollect_Success(t *testing.T) {
	logger := testLogger()
	mockReader := &MockCgroupReader{
		Metrics: &MemoryMetrics{
			Usage:         104857600,
			Limit:         268435456,
			RSS:           52428800,
			InactiveFile:  10485760,
			WorkingSet:    94371840,
			OOMRatio:      0.35,
			OOMFloorRatio: 0.19,
		},
	}
	collector := NewCollectorWithReader(logger, mockReader)

	ch := make(chan prometheus.Metric, 10)
	collector.Collect(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}

	// Should emit 8 metrics
	if count != 8 {
		t.Errorf("expected 8 metrics, got %d", count)
	}
}

func TestCollectorCollect_Error(t *testing.T) {
	logger := testLogger()
	mockReader := &MockCgroupReader{
		Err: errors.New("failed to read cgroup metrics"),
	}
	collector := NewCollectorWithReader(logger, mockReader)

	ch := make(chan prometheus.Metric, 10)
	collector.Collect(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}

	// Should emit 0 metrics on error
	if count != 0 {
		t.Errorf("expected 0 metrics on error, got %d", count)
	}
}

func TestCollectorCollect_ZeroValues(t *testing.T) {
	logger := testLogger()
	mockReader := &MockCgroupReader{
		Metrics: &MemoryMetrics{
			Usage:         0,
			Limit:         0,
			RSS:           0,
			InactiveFile:  0,
			WorkingSet:    0,
			OOMRatio:      0,
			OOMFloorRatio: 0,
		},
	}
	collector := NewCollectorWithReader(logger, mockReader)

	ch := make(chan prometheus.Metric, 10)
	collector.Collect(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}

	// Should still emit 8 metrics even with zero values
	if count != 8 {
		t.Errorf("expected 8 metrics with zero values, got %d", count)
	}
}

func TestCollectorMetricDescriptions(t *testing.T) {
	logger := testLogger()
	mockReader := &MockCgroupReader{}
	collector := NewCollectorWithReader(logger, mockReader)

	// Verify all metric descriptions are set
	if collector.usageDesc == nil {
		t.Error("usageDesc should not be nil")
	}
	if collector.limitDesc == nil {
		t.Error("limitDesc should not be nil")
	}
	if collector.rssDesc == nil {
		t.Error("rssDesc should not be nil")
	}
	if collector.inactiveFileDesc == nil {
		t.Error("inactiveFileDesc should not be nil")
	}
	if collector.workingSetDesc == nil {
		t.Error("workingSetDesc should not be nil")
	}
	if collector.nonReclaimableDesc == nil {
		t.Error("nonReclaimableDesc should not be nil")
	}
	if collector.oomRatioDesc == nil {
		t.Error("oomRatioDesc should not be nil")
	}
	if collector.oomFloorRatioDesc == nil {
		t.Error("oomFloorRatioDesc should not be nil")
	}
}

func TestCollectorRegister(t *testing.T) {
	logger := testLogger()
	mockReader := &MockCgroupReader{
		Metrics: &MemoryMetrics{},
	}
	collector := NewCollectorWithReader(logger, mockReader)

	// Create a new registry for testing
	registry := prometheus.NewRegistry()

	// Register with custom registry (not default)
	err := registry.Register(collector)
	if err != nil {
		t.Errorf("unexpected error registering collector: %v", err)
	}

	// Registering again should fail
	err = registry.Register(collector)
	if err == nil {
		t.Error("expected error when registering collector twice")
	}
}
