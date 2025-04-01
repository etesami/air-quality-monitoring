package metric

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"sync"
)

type Metric struct {
	RttTimes        []float64 `json:"rtt_times"`
	ProcessingTimes []float64 `json:"processing_times"`
	FailureCount    int       `json:"failure_count"`
	SuccessCount    int       `json:"success_count"`
	mu              sync.Mutex
}

func (m *Metric) AddProcessingTime(time float64, success bool) {
	m.ProcessingTimes = append(m.ProcessingTimes, time)
}
func (m *Metric) AddRttTime(time float64, success bool) {
	m.RttTimes = append(m.RttTimes, time)
}

func (m *Metric) Lock() {
	m.mu.Lock()
}

func (m *Metric) Unlock() {
	m.mu.Unlock()
}

func minSlice(data []float64) float64 {
	if len(data) == 0 {
		return 0 // or handle empty slice case appropriately
	}
	minVal := data[0]
	for _, v := range data {
		if v < minVal {
			minVal = v
		}
	}
	return minVal
}

func maxSlice(data []float64) float64 {
	if len(data) == 0 {
		return 0 // or handle empty slice case appropriately
	}
	maxValue := data[0]
	for _, v := range data {
		if v > maxValue {
			maxValue = v
		}
	}
	return maxValue
}

func (m *Metric) MinTime(t string) float64 {
	if t == "rtt" {
		return minSlice(m.RttTimes)
	}
	return minSlice(m.ProcessingTimes)
}

func (m *Metric) MaxTime(t string) float64 {
	if t == "rtt" {
		return maxSlice(m.RttTimes)
	}
	return maxSlice(m.ProcessingTimes)
}

func (m *Metric) Percentiles(t string, percentiles []float64) map[float64]float64 {
	var times []float64
	if t == "rtt" {
		times = m.RttTimes
	} else {
		times = m.ProcessingTimes
	}
	sort.Float64s(times)
	results := make(map[float64]float64)
	for _, p := range percentiles {
		index := int(float64(len(times)-1) * p / 100)
		results[p] = times[index]
	}
	return results
}

func (m *Metric) Mean(t string) float64 {
	var times []float64
	if t == "rtt" {
		times = m.RttTimes
	} else {
		times = m.ProcessingTimes
	}
	sum := 0.0
	for _, time := range times {
		sum += time
	}
	return sum / float64(len(times))
}

func (m *Metric) Variance(t string) float64 {
	var times []float64
	if t == "rtt" {
		times = m.RttTimes
	} else {
		times = m.ProcessingTimes
	}
	mean := m.Mean(t)
	var sum float64
	for _, time := range times {
		sum += (time - mean) * (time - mean)
	}
	return sum / float64(len(times))
}

func (m *Metric) StdDev(t string) float64 {
	return math.Sqrt(m.Variance(t))
}

func (m *Metric) Count(t string) int {
	if t == "rtt" {
		return len(m.RttTimes)
	}
	return len(m.ProcessingTimes)
}

func (m *Metric) SuccessRate() int {
	return m.SuccessCount * 100 / (m.SuccessCount + m.FailureCount)
}

func (m *Metric) FailureRate() int {
	return m.FailureCount * 100 / (m.SuccessCount + m.FailureCount)
}

// metricRttHandler handles the /metrics/rtt endpoint
func (m *Metric) RttHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		queryType := r.URL.Query().Get("type")

		metric := "rtt"
		response, err := m.metricHandler(metric, queryType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

// metricsProcessingTimeHandler handles the /metrics/processing endpoint
func (m *Metric) ProcessingTimeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		queryType := r.URL.Query().Get("type")

		metric := "processing"
		response, err := m.metricHandler(metric, queryType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

// metricHandler handles the metric calculations based on the metric type and query type
func (m *Metric) metricHandler(metricType string, queryType string) (any, error) {
	var response interface{}
	switch queryType {
	case "mean":
		response = m.Mean(metricType)
	case "max":
		response = m.MaxTime(metricType)
	case "min":
		response = m.MinTime(metricType)
	case "success_rate":
		response = m.SuccessRate()
	case "failure_rate":
		response = m.FailureRate()
	case "count":
		response = m.Count(metricType)
	case "stddev":
		response = m.StdDev(metricType)
	case "variance":
		response = m.Variance(metricType)
	case "percentiles":
		percentiles := []float64{25, 50, 75, 90, 95, 99}
		response = m.Percentiles(metricType, percentiles)
	case "all":
		response = map[string]any{
			"mean":         m.Mean(metricType),
			"max":          m.MaxTime(metricType),
			"min":          m.MinTime(metricType),
			"success_rate": m.SuccessRate(),
			"failure_rate": m.FailureRate(),
			"count":        m.Count(metricType),
			"stddev":       m.StdDev(metricType),
			"variance":     m.Variance(metricType),
			"percentiles":  m.Percentiles(metricType, []float64{25, 50, 75, 90, 95, 99}),
		}
	default:
		return "", fmt.Errorf("invalid query type: %s", queryType)
	}
	return response, nil
}
