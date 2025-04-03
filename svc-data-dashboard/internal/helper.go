package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
)

type DataType string

const (
	RequestPoints DataType = "points"
)

type DataRequest struct {
	StartTime   string   `json:"startTime,omitempty"`
	EndTime     string   `json:"endTime,omitempty"`
	LAT         float64  `json:"lat,omitempty"`
	LNG         float64  `json:"lng,omitempty"`
	RequestType DataType `json:"requestType,omitempty"`
}

type City struct {
	Idx      int64   `json:"idx,omitempty"`
	CityName string  `json:"cityName,omitempty"`
	Lat      float64 `json:"lat,omitempty"`
	Lng      float64 `json:"lng,omitempty"`
}

type Alert1 struct {
	AlertDesc        string `json:"alertDesc,omitempty"`
	AlertEffective   string `json:"alertEffective,omitempty"`
	AlertExpires     string `json:"alertExpires,omitempty"`
	AlertStatus      string `json:"alertStatus,omitempty"`
	AlertCertainty   string `json:"alertCertainty,omitempty"`
	AlertUrgency     string `json:"alertUrgency,omitempty"`
	AlertSeverity    string `json:"alertSeverity,omitempty"`
	AlertHeadline    string `json:"alertHeadline,omitempty"`
	AlertDescription string `json:"alertDescription,omitempty"`
	AlertEvent       string `json:"alertEvent,omitempty"`
}

// This is a modifed version of the original msg struct
// with fewer fields
type AirQualityData struct {
	Timestamp   string `json:"timestamp,omitempty"`
	Aqi         int64  `json:"aqi,omitempty"`
	DewPoint    int64  `json:"dewPoint,omitempty"`
	Humidity    int64  `json:"humidity,omitempty"`
	Pressure    int64  `json:"pressure,omitempty"`
	Temperature int64  `json:"temperature,omitempty"`
	WindSpeed   int64  `json:"windSpeed,omitempty"`
	WindGust    int64  `json:"windGust,omitempty"`
	PM25        int64  `json:"pm25,omitempty"`
	PM10        int64  `json:"pm10,omitempty"`
}

type Response struct {
	City           City             `json:"city,omitempty"`
	AirQualityData []AirQualityData `json:"airQualityData,omitempty"`
	Alert          []Alert1         `json:"alert,omitempty"`
}

// requestNewData requests new data from the local storage service
func requestNewData(client pb.AirQualityMonitoringClient) (int64, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	reqBody := DataRequest{
		RequestType: RequestPoints,
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

// processTicker processes the ticker event
// fetch data from the aggregated storage service in fixed intervals and show some statistics
func ProcessTicker(clientLocal, clientAggr pb.AirQualityMonitoringClient, m *metric.Metric) error {
	// TODO: think about the rtt, this time includes the processing time of the remote service
	_, recData, err := requestNewData(clientLocal)
	if err != nil {
		log.Printf("Error requesting new data: %v", err)
		return err
	}
	if len(recData) == 0 {
		log.Printf("No data received from local storage service")
		return nil
	}

	resBytes, err := json.Marshal(recData)
	if err != nil {
		log.Printf("Error marshalling processed data: %v", err)
	}
	log.Printf("Processed data: %s\n", string(resBytes))

	return nil
}
