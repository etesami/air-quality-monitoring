package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
func sendToDataIngestionService(client pb.AirQualityMonitoringClient, data any) (int64, error) {

	byteData, err := json.Marshal(data)
	if err != nil {
		return -1, fmt.Errorf("failed to marshal data: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sentTimestamp := time.Now()
	ack, err := client.SendDataToIngsetion(ctx, &pb.Data{
		Payload:       string(byteData),
		SentTimestamp: fmt.Sprintf("%d", int(sentTimestamp.UnixMilli())),
	})
	if err != nil {
		return -1, fmt.Errorf("send data not successful: %v", err)
	}
	if ack.Status != "ok" {
		return -1, fmt.Errorf("ack status not expected: %s", ack.Status)
	}
	rtt := time.Since(sentTimestamp).Milliseconds()
	log.Printf("Sent [%d] bytes. Ack recevied. RTT: [%d]ms\n", len(byteData), rtt)
	return rtt, nil
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
	log.Printf("Location IDs to collect data from: %v\n", locationIds)

	var wg sync.WaitGroup
	for _, locationId := range locationIds {

		wg.Add(1)
		go func(locationId string, m *metric.Metric) {
			defer wg.Done()

			locationData, err := getLocationData(locationId, locData.Token)
			if err != nil {
				log.Printf("Error getting location data for ID %s: %v", locationId, err)
				return
			}

			start := time.Now()
			if err := validateDataLocDetails(locationData); err != nil {
				log.Printf("Error validating location data for ID %s: %v", locationId, err)
				m.Failure("collector")
				return
			}
			m.Sucess("collector")
			m.AddProcessingTime("collector", float64(time.Since(start).Milliseconds())/1000.0)

			rtt, err := sendToDataIngestionService(client, locationData["rxs"])
			if err != nil {
				log.Printf("Error: %v", err)
				m.Failure("toIngestion")
			}
			m.Sucess("toIngestion")
			m.AddRttTime("toIngestion", float64(rtt)/1000.0)

		}(locationId, metricList)
	}
	wg.Wait()
	return nil

}
