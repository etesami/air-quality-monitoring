package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	metric "github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
	"github.com/etesami/air-quality-monitoring/svc-data-collector/api"
)

// validateData validates the fetched data and ensures it
// contains the status and data fields
func validateDataLocIds(data map[string]any) error {
	if data == nil {
		return fmt.Errorf("data is nil")
	}

	if v, ok := data["status"]; !ok || v.(string) != "ok" {
		return fmt.Errorf("invalid status: %v", v)
	}
	if _, ok := data["data"]; !ok {
		return fmt.Errorf("data field is missing")
	}
	return nil
}

// validateDataLocDetails validates the fetched data and ensures it
// contains the status and rxs fields
func validateDataLocDetails(data map[string]any) error {
	if data == nil {
		return fmt.Errorf("data is nil")
	}

	v, ok := data["rxs"]
	if !ok {
		return fmt.Errorf("key 'rxs' not found in data")
	}
	if vv, ok := v.(map[string]any)["status"]; !ok || vv.(string) != "ok" {
		return fmt.Errorf("invalid status: %v", vv)
	}
	return nil
}

// getLocationIds extracts the location IDs from the fetched data
func getLocationIds(data map[string]any) ([]string, error) {
	if data == nil {
		return nil, fmt.Errorf("data is nil")
	}

	var locationIds []string
	for _, item := range data["data"].([]any) {
		if v, ok := item.(map[string]any)["uid"]; ok {
			locationIds = append(locationIds, fmt.Sprintf("%d", int(v.(float64))))
		}
	}
	return locationIds, nil
}

// getLocationData fetches data for a specific location ID
func getLocationData(locationId, token string) (map[string]any, error) {
	url := fmt.Sprintf(
		"https://api.waqi.info/v2/feed/@%s/?token=%s",
		locationId, token)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data for location %s: %v", locationId, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch data for location %s: %d", locationId, resp.StatusCode)
	}

	var res map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("error decoding JSON for location %s: %v", locationId, err)
	}
	return res, nil
}

// sendToDataIngestionService sends the data to the data ingestion service
// It takes a gRPC client and the data to be sent as parameters
func sendToDataIngestionService(client pb.AirQualityMonitoringClient, data map[string]any) (int64, error) {

	jsonString, err := json.Marshal(data)
	if err != nil {
		return -1, fmt.Errorf("failed to marshal data: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	sentTimestamp := time.Now().UnixMilli()
	ack, err := client.SendDataToIngsetion(ctx, &pb.Data{
		Payload:       string(jsonString),
		SentTimestamp: strconv.Itoa(int(sentTimestamp)),
	})
	if err != nil {
		return -1, fmt.Errorf("send data not successful: %v", err)
	}
	if ack.Status != "ok" {
		return -1, fmt.Errorf("ack status not expected: %s", ack.Status)
	}
	rtt := time.Now().UnixMilli() - sentTimestamp
	log.Printf("Ack recevied. RTT: [%d]ms\n", rtt)
	return rtt, nil
}

// updateMetrics updates the metrics with the received RTT
func updateMetrics(metricList *metric.Metric, rtt int64) {
	metricList.Lock()
	defer metricList.Unlock()
	metricList.RttTimes = append(metricList.RttTimes, float64(rtt))
	metricList.SuccessCount++
}

// sendAndUpdateMetrics sends the data to the data ingestion service
// and updates the metrics with the received RTT
func sendAndUpdateMetrics(client pb.AirQualityMonitoringClient, locationData map[string]interface{}, metricList *metric.Metric) error {
	rtt, err := sendToDataIngestionService(client, locationData)
	if err != nil || rtt < 0 {
		metricList.Lock()
		defer metricList.Unlock()
		metricList.FailureCount++
		log.Println("Failed to send data to ingestion service")
		return fmt.Errorf("sending data to ingestion service: %w", err)
	}
	if rtt >= 0 {
		updateMetrics(metricList, rtt)
	}
	return nil
}

// processTicker processes the ticker event
func ProcessTicker(client pb.AirQualityMonitoringClient, locData *api.LocationData, metricList *metric.Metric) error {
	data, err := locData.CollectLocationsIds()
	if err != nil {
		return fmt.Errorf("fetching data: %w", err)
	}

	if err := validateDataLocIds(data); err != nil {
		return fmt.Errorf("validating data: %w", err)
	}

	locationIds, err := getLocationIds(data)
	if err != nil {
		return fmt.Errorf("getting location IDs: %w", err)
	}

	var wg sync.WaitGroup
	for _, locationId := range locationIds {
		locationData, err := getLocationData(locationId, locData.Token)
		if err != nil {
			log.Printf("Error getting location data for ID %s: %v", locationId, err)
			continue
		}

		if err := validateDataLocDetails(locationData); err != nil {
			log.Printf("Error validating location data for ID %s: %v", locationId, err)
			continue
		}

		wg.Add(1)
		go func(locationData map[string]interface{}) {
			defer wg.Done()
			if err := sendAndUpdateMetrics(client, locationData, metricList); err != nil {
				log.Printf("Error: %v", err)
			}
		}(locationData)
	}
	wg.Wait()
	return nil

}
