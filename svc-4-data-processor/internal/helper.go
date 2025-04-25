package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/etesami/air-quality-monitoring/api"
	"github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
	"github.com/etesami/air-quality-monitoring/pkg/utils"
	"google.golang.org/protobuf/proto"

	dpapi "github.com/etesami/air-quality-monitoring/api/data-processing"
)

type Server struct {
	pb.UnimplementedAirQualityMonitoringServer
	Metric *metric.Metric
	Client *pb.AirQualityMonitoringClient
}

// CheckConnection is a simple ping-pong method to respond for the health check
func (s Server) CheckConnection(ctx context.Context, recData *pb.Data) (*pb.Ack, error) {
	t := time.Now()
	ack := &pb.Ack{
		Status:                "pong",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     fmt.Sprintf("%d", int(t.UnixMilli())),
		AckSentTimestamp:      fmt.Sprintf("%d", int(t.UnixMilli())),
	}
	return ack, nil
}

func (s Server) SendDataToServer(ctx context.Context, recData *pb.Data) (*pb.Ack, error) {
	st := time.Now()
	recTimestamp := st.UnixMilli()
	log.Printf("Received at [%s]: [%d]\n", st.Format("2006-01-02 15:04:05"), len(recData.Payload))

	go func(data string, m *metric.Metric, start time.Time) {
		processedData, err := processData(data)
		if err != nil {
			log.Printf("Error processing data: %v", err)
		}

		if len(processedData) == 0 {
			log.Printf("No data to be sent to aggregated storage")
			return
		}
		log.Printf("Processed [%d] items.\n", len(processedData))

		procResBytes, err := json.Marshal(processedData)
		if err != nil {
			log.Printf("Error marshalling processed data: %v", err)
		}

		s.Metric.AddProcessingTime("processing", float64(time.Since(st).Milliseconds())/1000.0)

		// Sneding to the storage
		if *s.Client == nil {
			log.Printf("Client is not ready yet")
			return
		}
		if sentBytes, err := sendDataToStorage(*s.Client, string(procResBytes)); err != nil {
			log.Printf("Error sending data to storage: %v", err)
		} else {
			s.Metric.AddSentDataBytes("central-storage", float64(sentBytes))
		}
	}(recData.Payload, s.Metric, st)

	ack := &pb.Ack{
		Status:                "ok",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     fmt.Sprintf("%d", int(recTimestamp)),
		AckSentTimestamp:      fmt.Sprintf("%d", int(time.Now().UnixMilli())),
	}
	return ack, nil
}

// processData performs a few calculation along with enhancing data with additional information
// from api.weather.gov
func processData(res string) ([]dpapi.EnhancedDataResponse, error) {
	// Expect response to be a list of items
	msgList := make([]api.Msg, 0)
	if err := json.Unmarshal([]byte(res), &msgList); err != nil {
		return nil, fmt.Errorf("error unmarshalling JSON: %v", err)
	}
	log.Printf("Received [%d] items from local storage\n", len(msgList))

	var wg sync.WaitGroup
	respChan := make(chan dpapi.EnhancedDataResponse)
	for _, msg := range msgList {
		wg.Add(1)
		go func(m api.Msg) {
			defer wg.Done()

			geoAlerts, err := getAlertsforPoint(msg.City.Geo[0], msg.City.Geo[1])
			if err != nil {
				log.Printf("error getting alerts for point: %v", err)
				return
			}

			alert := &dpapi.Alert{}
			if geoAlerts == nil {
				log.Printf("no alerts found for point: %f, %f\n", msg.City.Geo[0], msg.City.Geo[1])
				alert = nil
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
				Alert: alert,
			}
			respChan <- procRes
		}(msg)
	}
	go func() {
		wg.Wait()
		close(respChan)
	}()

	procRespList := make([]dpapi.EnhancedDataResponse, 0)
	for response := range respChan {
		procRespList = append(procRespList, response)
	}
	return procRespList, nil
}

func getAlertsforPoint(lat, lng float64) (*dpapi.AlertRaw, error) {
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

func generateAlertStruct(res map[string]any) (*dpapi.AlertRaw, error) {
	if o, ok := res["features"].([]any); ok {
		if len(o) == 0 {
			return nil, nil
		}
		if f, ok := o[0].(map[string]any); ok {
			if p, ok := f["properties"].(map[string]any); ok {
				alert := &dpapi.AlertRaw{}
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
func sendDataToStorage(client pb.AirQualityMonitoringClient, data string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sentTimestamp := time.Now()
	res := &pb.Data{
		Payload:       data,
		SentTimestamp: fmt.Sprintf("%d", int(sentTimestamp.UnixMilli())),
	}
	ack, err := client.SendDataToServer(ctx, res)
	if err != nil {
		return 0, fmt.Errorf("send data not successful: %v", err)
	}
	if ack.Status != "ok" {
		return 0, fmt.Errorf("ack status not expected: %s", ack.Status)
	}

	bytesSent := proto.Size(res)
	log.Printf("Sent [%d] bytes. Ack recevied, status: [%s]\n", bytesSent, ack.Status)

	return bytesSent, nil
}

// processTicker processes the ticker event
func ProcessTicker(client *pb.AirQualityMonitoringClient, serverName string, m *metric.Metric) error {
	if *client == nil {
		log.Printf("Client is not ready yet")
		return nil
	}
	go func(m *metric.Metric) {
		ping := &pb.Data{
			Payload:       "ping",
			SentTimestamp: fmt.Sprintf("%d", int(time.Now().UnixMilli())),
		}
		pong, err := (*client).CheckConnection(context.Background(), ping)
		if err != nil {
			log.Printf("Error checking connection: %v", err)
			return
		}
		rtt, err := utils.CalculateRtt(ping.SentTimestamp, pong.ReceivedTimestamp, pong.AckSentTimestamp, time.Now())
		if err != nil {
			log.Printf("Error calculating RTT: %v", err)
			return
		}
		m.AddRttTime(serverName, float64(rtt)/1000.0)
		log.Printf("RTT to [%s] service: [%.2f] ms\n", serverName, float64(rtt)/1000.0)
	}(m)
	return nil
}
