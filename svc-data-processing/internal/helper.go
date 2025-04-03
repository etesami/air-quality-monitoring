package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/etesami/air-quality-monitoring/api"
	"github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"

	dpapi "github.com/etesami/air-quality-monitoring/api/data-processing"
	loapi "github.com/etesami/air-quality-monitoring/api/local-storage"
)

type AlertRaw struct {
	AreaDesc    string `json:"areaDesc,omitempty"`
	Sent        string `json:"sent,omitempty"`
	Effective   string `json:"effective,omitempty"`
	Expires     string `json:"expires,omitempty"`
	Ends        string `json:"ends,omitempty"`
	Status      string `json:"status,omitempty"`
	Certainty   string `json:"certainty,omitempty"`
	Urgency     string `json:"urgency,omitempty"`
	Event       string `json:"event,omitempty"`
	Headline    string `json:"headline,omitempty"`
	Description string `json:"description,omitempty"`
	Instruction string `json:"instruction,omitempty"`
	Severity    string `json:"severity,omitempty"`
}

// requestNewData requests new data from the local storage service
func requestNewData(ctx context.Context, client pb.AirQualityMonitoringClient) (int64, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check context if we have a last call time
	// If so, use that as the start time
	var startTime string
	if lastCall := ctx.Value("lastCall"); lastCall != nil {
		sTime := lastCall.(time.Time)
		startTime = sTime.Format(time.RFC3339)
	} else {
		startTime = time.Now().Add(-8 * time.Hour).Format(time.RFC3339)
	}

	reqBody := loapi.DataRequest{
		StartTime: startTime,
		EndTime:   time.Now().Format(time.RFC3339),
	}
	reqByte, err := json.Marshal(reqBody)
	if err != nil {
		return -1, "", fmt.Errorf("error marshalling JSON: %v", err)
	}

	sentTimestamp := time.Now()
	res, err := client.ReceiveDataFromLocalStorage(ctx, &pb.Data{
		Payload:       string(reqByte),
		SentTimestamp: fmt.Sprintf("%d", int(sentTimestamp.UnixMilli())),
	})
	if err != nil {
		return -1, "", fmt.Errorf("error requesting data from local storage: %v", err)
	}
	rtt := time.Since(sentTimestamp).Milliseconds()
	if len(res.Payload) == 0 {
		log.Printf("No data received from local storage. RTT: [%d]ms\n", rtt)
		return rtt, "", nil
	}
	log.Printf("Response from local storage recevied. RTT: [%d]ms, len: [%d]\n", rtt, len(res.Payload))
	return rtt, res.Payload, nil
}

// processData performs a few calculation along with enhancing data with additional information
// from api.weather.gov
func processData(res string) ([]dpapi.EnhancedDataResponse, error) {
	// Expect response to be a list of items
	msgList := make([]api.Msg, 0)
	if err := json.Unmarshal([]byte(res), &msgList); err != nil {
		return nil, fmt.Errorf("error unmarshalling JSON: %v", err)
	}

	procRespList := make([]dpapi.EnhancedDataResponse, 0)
	for _, msg := range msgList {
		geoAlerts, err := getAlertsforPoint(msg.City.Geo[0], msg.City.Geo[1])
		if err != nil {
			log.Printf("error getting alerts for point: %v", err)
			continue
		}
		alert := &dpapi.Alert{}
		if geoAlerts == nil {
			log.Printf("no alerts found for point: %f, %f\n", msg.City.Geo[0], msg.City.Geo[1])
		} else {
			alert.AlertDesc = geoAlerts.Description
			alert.AlertEffective = geoAlerts.Effective
			alert.AlertExpires = geoAlerts.Expires
			alert.AlertStatus = geoAlerts.Status
			alert.AlertCertainty = geoAlerts.Certainty
			alert.AlertUrgency = geoAlerts.Urgency
			alert.AlertSeverity = geoAlerts.Severity
			alert.AlertHeadline = geoAlerts.Headline
			alert.AlertDescription = geoAlerts.Description
			alert.AlertEvent = geoAlerts.Event
		}
		procRes := dpapi.EnhancedDataResponse{
			City: dpapi.City{
				Idx:      int64(msg.Idx),
				CityName: msg.City.Name,
				Lat:      msg.City.Geo[0],
				Lng:      msg.City.Geo[1],
			},
			AirQualityData: dpapi.AirQualityData{
				Timestamp:   msg.Time.ISO,
				Aqi:         int64(msg.Aqi),
				DewPoint:    int64(msg.IAQI.H.V),
				Humidity:    int64(msg.IAQI.H.V),
				Pressure:    int64(msg.IAQI.P.V),
				Temperature: int64(msg.IAQI.T.V),
				WindSpeed:   int64(msg.IAQI.W.V),
				WindGust:    int64(msg.IAQI.WG.V),
				PM25:        int64(msg.IAQI.PM25.V),
			},
			Alert: *alert,
		}
		procRespList = append(procRespList, procRes)
	}

	return procRespList, nil
}

func getAlertsforPoint(lat, lng float64) (*AlertRaw, error) {
	url := fmt.Sprintf("https://api.weather.gov/alerts?point=%f,%f", lat, lng)

	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "(skycluster.io, ehsan.etesami@utoronto.ca)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch data: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	var res map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("error decoding JSON: %w", err)
	}
	alert, err := generateAlertStruct(res)
	if err != nil {
		return nil, fmt.Errorf("error generating alert struct: %v", err)
	}
	return alert, nil
}

func generateAlertStruct(res map[string]any) (*AlertRaw, error) {
	if o, ok := res["features"].([]any); ok {
		if len(o) == 0 {
			return nil, nil
		}
		if f, ok := o[0].(map[string]any); ok {
			if p, ok := f["properties"].(map[string]any); ok {
				alert := &AlertRaw{}
				// Marshal the properties to get bytes
				pBytes, err := json.Marshal(p)
				if err != nil {
					return nil, fmt.Errorf("error marshalling properties: %v", err)
				}
				// Unmarshal the bytes into the Alert struct
				if err := json.Unmarshal(pBytes, alert); err != nil {
					return nil, fmt.Errorf("error unmarshalling JSON: %v", err)
				}
				return alert, nil
			}
		}
	}
	return nil, fmt.Errorf("failed to extract properties from response")
}

// sendDataToStorage sends the processed data to the storage service
func sendDataToStorage(client pb.AirQualityMonitoringClient, data string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sentTimestamp := time.Now()
	ack, err := client.SendToAggregatedStorage(ctx, &pb.Data{
		Payload:       data,
		SentTimestamp: fmt.Sprintf("%d", int(sentTimestamp.UnixMilli())),
	})
	if err != nil {
		return -1, fmt.Errorf("send data not successful: %v", err)
	}
	if ack.Status != "ok" {
		return -1, fmt.Errorf("ack status not expected: %s", ack.Status)
	}
	rtt := time.Since(sentTimestamp).Milliseconds()
	log.Printf("Sent [%d] bytes to local storage. Ack recevied. RTT: [%d]ms\n", len(data), rtt)
	return rtt, nil
}

// processTicker processes the ticker event
func ProcessTicker(ctx context.Context, clientLocal, clientAggr pb.AirQualityMonitoringClient, m *metric.Metric) error {
	// TODO: think about the rtt, this time includes the processing time of the remote service
	_, recData, err := requestNewData(ctx, clientLocal)
	if err != nil {
		log.Printf("Error requesting new data: %v", err)
		return err
	}
	if len(recData) == 0 {
		log.Printf("No data received from local storage service")
		return nil
	}

	processedData, err := processData(recData)
	if err != nil {
		log.Printf("Error processing data: %v", err)
	}

	procResBytes, err := json.Marshal(processedData)
	if err != nil {
		log.Printf("Error marshalling processed data: %v", err)
	}
	log.Printf("Processed data: %s\n", string(procResBytes))

	// send processed data to the aggregated storage service

	go func(d string) {
		// TODO: handle RTT
		if _, err := sendDataToStorage(clientAggr, d); err != nil {
			log.Printf("Error sending data to storage: %v", err)
		}
	}(string(procResBytes))

	return nil
}
