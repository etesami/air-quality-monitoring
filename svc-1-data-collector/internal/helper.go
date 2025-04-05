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
	utils "github.com/etesami/air-quality-monitoring/pkg/utils"
)

type LocationData struct {
	Lat1  float64 `json:"lat1"`
	Lng1  float64 `json:"lng1"`
	Lat2  float64 `json:"lat2"`
	Lng2  float64 `json:"lng2"`
	Token string  `json:"token"`
}

func (l *LocationData) collectLocationsIds() (map[string]any, error) {
	url := fmt.Sprintf(
		"https://api.waqi.info/v2/map/bounds?latlng=%f,%f,%f,%f&token=%s",
		l.Lat1, l.Lng1, l.Lat2, l.Lng2, l.Token)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch data: %d", resp.StatusCode)
	}

	var res map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("error Decoding JSON: %v", err)
	}
	return res, nil
}

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
func sendToDataIngestionService(client pb.AirQualityMonitoringClient, data any) error {

	byteData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sentTimestamp := time.Now()
	ack, err := client.SendDataToServer(ctx, &pb.Data{
		Payload:       string(byteData),
		SentTimestamp: fmt.Sprintf("%d", int(sentTimestamp.UnixMilli())),
	})
	if err != nil {
		return fmt.Errorf("send data not successful: %v", err)
	}
	log.Printf("Sent [%d] bytes. Ack recevied, status: [%s]\n", len(byteData), ack.Status)

	return nil
}

// processTicker processes the ticker event
func ProcessTicker(client pb.AirQualityMonitoringClient, serverName string, locData *LocationData, metricList *metric.Metric) error {

	go func(m *metric.Metric) {
		pong, err := client.CheckConnection(context.Background(), &pb.Data{
			Payload:       "ping",
			SentTimestamp: fmt.Sprintf("%d", int(time.Now().UnixMilli())),
		})
		if err != nil {
			log.Printf("Error checking connection: %v", err)
			return
		}
		rtt, err := utils.CalculateRtt(time.Now(), pong.ReceivedTimestamp, time.Now(), pong.AckSentTimestamp)
		if err != nil {
			log.Printf("Error calculating RTT: %v", err)
			return
		}
		m.AddRttTime(serverName, float64(rtt)/1000.0)
		log.Printf("RTT to ingestion service: [%.2f] ms\n", float64(rtt)/1000.0)

	}(metricList)

	data, err := locData.collectLocationsIds()
	if err != nil {
		return fmt.Errorf("fetching data: %w", err)
	}

	// processing time starts
	var processingTime time.Duration
	st := time.Now()
	if err := validateDataLocIds(data); err != nil {
		return fmt.Errorf("validating data: %w", err)
	}

	locationIds, err := getLocationIds(data)
	if err != nil {
		return fmt.Errorf("getting location IDs: %w", err)
	}
	log.Printf("Received [%d] location IDs. \n", len(locationIds))
	processingTime = time.Since(st)

	var wg sync.WaitGroup
	for _, locationId := range locationIds {

		wg.Add(1)
		go func(locationId string, m *metric.Metric, pt time.Duration) {
			defer wg.Done()

			locationData, err := getLocationData(locationId, locData.Token)
			if err != nil {
				log.Printf("Error getting location data for ID %s: %v", locationId, err)
				return
			}

			st := time.Now()
			if err := validateDataLocDetails(locationData); err != nil {
				log.Printf("Error validating location data for ID %s: %v", locationId, err)
				return
			}
			pt += time.Since(st)
			m.AddProcessingTime("collector", float64(pt)/1000.0)

			if err := sendToDataIngestionService(client, locationData["rxs"]); err != nil {
				log.Printf("Error sending data to ingestion service: %v", err)
			}

		}(locationId, metricList, processingTime)
	}

	wg.Wait()
	return nil
}
