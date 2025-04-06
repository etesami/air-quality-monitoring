package metric

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Buckets for processing histogram in milliseconds
	// customBuckets = []float64{0.01, 0.1, 0.5, 1, 2, 5, 10, 20, 50, 100}
	processingTime = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "operation_processing_time_seconds",
			Help:    "Histogram of processing times.",
			Buckets: prometheus.DefBuckets,
		},
	)
	rttTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "operation_rtt_time_seconds",
			Help:    "Histogram of round-trip times for different services.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service"},
	)
)

func (m *Metric) RegisterMetrics() {
	prometheus.MustRegister(processingTime)
	prometheus.MustRegister(rttTime)
}

type Metric struct {
	mu sync.Mutex
}

func (m *Metric) AddProcessingTime(s string, time float64) {
	m.lock()
	defer m.unlock()
	processingTime.Observe(time)
}

func (m *Metric) AddRttTime(s string, time float64) {
	m.lock()
	defer m.unlock()
	rttTime.WithLabelValues(s).Observe(time)
}

func (m *Metric) lock() {
	m.mu.Lock()
}

func (m *Metric) unlock() {
	m.mu.Unlock()
}
