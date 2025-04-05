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
	RttTimes        map[string][]float64 `json:"rtt_times"`
	ProcessingTimes map[string][]float64 `json:"processing_times"`
	mu              sync.Mutex
}

func (m *Metric) AddProcessingTime(s string, time float64) {
	m.lock()
	defer m.unlock()
	if m.ProcessingTimes == nil {
		m.ProcessingTimes = make(map[string][]float64)
	}
	if _, ok := m.ProcessingTimes[s]; !ok {
		m.ProcessingTimes[s] = []float64{}
	}
	m.ProcessingTimes[s] = append(m.ProcessingTimes[s], time)
}

func (m *Metric) AddRttTime(s string, time float64) {
	m.lock()
	defer m.unlock()
	if m.RttTimes == nil {
		m.RttTimes = make(map[string][]float64)
	}
	if _, ok := m.RttTimes[s]; !ok {
		m.RttTimes[s] = []float64{}
	}
	m.RttTimes[s] = append(m.RttTimes[s], time)
}

func (m *Metric) lock() {
	m.mu.Lock()
}

func (m *Metric) unlock() {
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

func (m *Metric) MinTime(s string, t string) float64 {
	if t == "rtt" {
		return minSlice(m.RttTimes[s])
	}
	return minSlice(m.ProcessingTimes[s])
}

func (m *Metric) MaxTime(s string, t string) float64 {
	if t == "rtt" {
		return maxSlice(m.RttTimes[s])
	}
	return maxSlice(m.ProcessingTimes[s])
}

func (m *Metric) Percentiles(s string, t string, percentiles []float64) map[float64]float64 {
	var times []float64
	if t == "rtt" {
		times = m.RttTimes[s]
	} else {
		times = m.ProcessingTimes[s]
	}
	sort.Float64s(times)
	results := make(map[float64]float64)
	for _, p := range percentiles {
		index := int(float64(len(times)-1) * p / 100)
		results[p] = times[index]
	}
	return results
}

func (m *Metric) Mean(s string, t string) float64 {
	var times []float64
	if t == "rtt" {
		times = m.RttTimes[s]
	} else {
		times = m.ProcessingTimes[s]
	}
	sum := 0.0
	for _, time := range times {
		sum += time
	}
	return sum / float64(len(times))
}

func (m *Metric) Variance(s string, t string) float64 {
	var times []float64
	if t == "rtt" {
		times = m.RttTimes[s]
	} else {
		times = m.ProcessingTimes[s]
	}
	mean := m.Mean(s, t)
	var sum float64
	for _, time := range times {
		sum += (time - mean) * (time - mean)
	}
	return sum / float64(len(times))
}

func (m *Metric) StdDev(s string, t string) float64 {
	return math.Sqrt(m.Variance(s, t))
}

func (m *Metric) Count(s string, t string) int {
	if t == "rtt" {
		return len(m.RttTimes[s])
	}
	return len(m.ProcessingTimes[s])
}

// metricRttHandler handles the /metrics/rtt endpoint
func (m *Metric) RttHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		queryType := r.URL.Query().Get("type")
		svcName := r.URL.Query().Get("service")

		metric := "rtt"
		response, err := m.metricHandler(svcName, metric, queryType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

// IndexHandler handles the /metrics endpoint
// and returns the names of all services and available metrics
func (m *Metric) IndexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		services := make([]string, 0, len(m.RttTimes))
		for service := range m.RttTimes {
			services = append(services, service)
		}
		for service := range m.ProcessingTimes {
			if _, ok := m.RttTimes[service]; !ok {
				services = append(services, service)
			}
		}
		sort.Strings(services)
		metrics := []string{"rtt", "processing"}
		types := []string{"mean", "max", "min", "count", "stddev", "variance", "percentiles"}
		sort.Strings(types)

		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{
			"services": services,
			"metrics":  metrics,
			"types":    types,
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
		svcName := r.URL.Query().Get("service")

		metric := "processing"
		response, err := m.metricHandler(svcName, metric, queryType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

func buildResponse(s string, tName string, t any) map[string]any {
	if slice, ok := t.([]float64); ok {
		return map[string]any{
			"service": s,
			"metrics": map[string]any{
				tName: slice,
			},
		}
	}
	return map[string]any{
		"service": s,
		"metrics": map[string]any{
			tName: t,
		},
	}
}

// metricHandler handles the metric calculations based on the metric type and query type
func (m *Metric) metricHandler(s string, metricType string, queryType string) (any, error) {
	var response any
	switch queryType {
	case "mean":
		response = buildResponse(s, "mean", m.Mean(s, metricType))
	case "max":
		response = buildResponse(s, "max", m.MaxTime(s, metricType))
	case "min":
		response = buildResponse(s, "min", m.MinTime(s, metricType))
	case "count":
		response = buildResponse(s, "count", m.Count(s, metricType))
	case "stddev":
		response = buildResponse(s, "stddev", m.StdDev(s, metricType))
	case "variance":
		response = buildResponse(s, "variance", m.Variance(s, metricType))
	case "percentiles":
		percentiles := []float64{25, 50, 75, 90, 95, 99}
		response = buildResponse(s, "percentiles", m.Percentiles(s, metricType, percentiles))
	case "all":
		// TODO: Fix precentiles
		// prc := m.Percentiles(s, metricType, []float64{25, 50, 75, 90, 95, 99})
		response = map[string]any{
			"service": s,
			"metrics": map[string]any{
				"mean":     m.Mean(s, metricType),
				"max":      m.MaxTime(s, metricType),
				"min":      m.MinTime(s, metricType),
				"count":    m.Count(s, metricType),
				"stddev":   m.StdDev(s, metricType),
				"variance": m.Variance(s, metricType),
			},
		}
	default:
		return "", fmt.Errorf("invalid query type: [%s] for service [%s]", queryType, s)
	}
	return response, nil
}
