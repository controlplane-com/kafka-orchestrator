package metrics

import (
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "kafka"
	subsystem = "memory"
)

// Collector implements prometheus.Collector for Kafka memory metrics
type Collector struct {
	reader CgroupReader
	logger *slog.Logger

	usageDesc          *prometheus.Desc
	limitDesc          *prometheus.Desc
	rssDesc            *prometheus.Desc
	inactiveFileDesc   *prometheus.Desc
	workingSetDesc     *prometheus.Desc
	nonReclaimableDesc *prometheus.Desc
	oomRatioDesc       *prometheus.Desc
	oomFloorRatioDesc  *prometheus.Desc
}

// NewCollector creates a new Prometheus collector for memory metrics
func NewCollector(logger *slog.Logger) *Collector {
	reader := NewCgroupReader(logger)
	return NewCollectorWithReader(logger, reader)
}

// NewCollectorWithReader creates a new Prometheus collector with a custom reader (for testing)
func NewCollectorWithReader(logger *slog.Logger, reader CgroupReader) *Collector {
	return &Collector{
		reader: reader,
		logger: logger,
		usageDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "usage_bytes"),
			"Total memory usage in bytes",
			nil, nil,
		),
		limitDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "limit_bytes"),
			"Memory limit in bytes (container limit)",
			nil, nil,
		),
		rssDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "rss_bytes"),
			"Resident set size (non-reclaimable memory) in bytes",
			nil, nil,
		),
		inactiveFileDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "inactive_file_bytes"),
			"Inactive file (reclaimable page cache) in bytes",
			nil, nil,
		),
		workingSetDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "working_set_bytes"),
			"Working set (usage - inactive_file) in bytes",
			nil, nil,
		),
		nonReclaimableDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "non_reclaimable_bytes"),
			"Non-reclaimable memory (RSS only) in bytes",
			nil, nil,
		),
		oomRatioDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "oom_ratio"),
			"OOM ratio (working_set / limit)",
			nil, nil,
		),
		oomFloorRatioDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "oom_floor_ratio"),
			"OOM floor ratio (rss / limit)",
			nil, nil,
		),
	}
}

// Describe implements prometheus.Collector
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.usageDesc
	ch <- c.limitDesc
	ch <- c.rssDesc
	ch <- c.inactiveFileDesc
	ch <- c.workingSetDesc
	ch <- c.nonReclaimableDesc
	ch <- c.oomRatioDesc
	ch <- c.oomFloorRatioDesc
}

// Collect implements prometheus.Collector
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	metrics, err := c.reader.ReadMemoryMetrics()
	if err != nil {
		c.logger.Error("failed to read memory metrics", "error", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(c.usageDesc, prometheus.GaugeValue, float64(metrics.Usage))
	ch <- prometheus.MustNewConstMetric(c.limitDesc, prometheus.GaugeValue, float64(metrics.Limit))
	ch <- prometheus.MustNewConstMetric(c.rssDesc, prometheus.GaugeValue, float64(metrics.RSS))
	ch <- prometheus.MustNewConstMetric(c.inactiveFileDesc, prometheus.GaugeValue, float64(metrics.InactiveFile))
	ch <- prometheus.MustNewConstMetric(c.workingSetDesc, prometheus.GaugeValue, float64(metrics.WorkingSet))
	ch <- prometheus.MustNewConstMetric(c.nonReclaimableDesc, prometheus.GaugeValue, float64(metrics.RSS))
	ch <- prometheus.MustNewConstMetric(c.oomRatioDesc, prometheus.GaugeValue, metrics.OOMRatio)
	ch <- prometheus.MustNewConstMetric(c.oomFloorRatioDesc, prometheus.GaugeValue, metrics.OOMFloorRatio)
}

// Register registers the collector with Prometheus
func (c *Collector) Register() error {
	return prometheus.Register(c)
}
