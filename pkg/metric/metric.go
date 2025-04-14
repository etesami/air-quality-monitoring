package metric

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Buckets for processing histogram in milliseconds
	// customBuckets = []float64{0.01, 0.1, 0.5, 1, 2, 5, 10, 20, 50, 100}
	procTimeHistogram = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "processing_time_ms_histogram",
			Help:    "Histogram of processing times.",
			Buckets: prometheus.DefBuckets,
		},
	)
	rttTimeHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rtt_times_ms_histogram",
			Help:    "Histogram of round-trip times for different services.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service"},
	)
	procTime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "processing_time_ms",
			Help: "Gauge of processing times.",
		},
	)
	rTTTimes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rtt_times_ms",
			Help: "Gauge of round-trip times for different services.",
		},
		[]string{"service"})
)

func (m *Metric) RegisterMetrics() {
	prometheus.MustRegister(procTimeHistogram)
	prometheus.MustRegister(rttTimeHistogram)
	prometheus.MustRegister(procTime)
	prometheus.MustRegister(rTTTimes)
}

type Metric struct {
	mu sync.Mutex
}

func (m *Metric) AddProcessingTime(s string, time float64) {
	m.lock()
	defer m.unlock()
	procTimeHistogram.Observe(time)
	procTime.Set(time)
}

func (m *Metric) AddRttTime(s string, time float64) {
	m.lock()
	defer m.unlock()
	rttTimeHistogram.WithLabelValues(s).Observe(time)
	rTTTimes.WithLabelValues(s).Set(time)
}

func (m *Metric) lock() {
	m.mu.Lock()
}

func (m *Metric) unlock() {
	m.mu.Unlock()
}
