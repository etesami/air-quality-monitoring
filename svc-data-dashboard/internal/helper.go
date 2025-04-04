package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	agapi "github.com/etesami/air-quality-monitoring/api/aggregated-storage"
	loapi "github.com/etesami/air-quality-monitoring/api/local-storage"

	"github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
	utils "github.com/etesami/air-quality-monitoring/pkg/utils"
)

// requestNewData requests new data from the local storage service
func requestNewData(client pb.AirQualityMonitoringClient) (float64, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	reqBody := loapi.DataRequest{
		RequestType: loapi.RequestPoints,
	}
	reqByte, err := json.Marshal(reqBody)
	if err != nil {
		return -1, "", fmt.Errorf("error marshalling JSON: %v", err)
	}

	sentTimestamp := time.Now()
	res, err := client.ReceiveAggregatedData(ctx, &pb.Data{
		Payload:       string(reqByte),
		SentTimestamp: fmt.Sprintf("%d", int(sentTimestamp.UnixMilli())),
	})
	if err != nil {
		return -1, "", fmt.Errorf("error requesting data from local storage: %v", err)
	}

	rtt, err := utils.CalculateRtt(sentTimestamp, res.ReceivedTimestamp, time.Now(), res.SentTimestamp)
	if len(res.Payload) == 0 {
		log.Printf("No data received from local storage. RTT: [%f]ms\n", rtt)
		return rtt, "", nil
	}
	log.Printf("Response from local storage recevied. RTT: [%f]ms, len: [%d]\n", rtt, len(res.Payload))

	return rtt, res.Payload, nil
}

// processTicker processes the ticker event
// fetch data from the aggregated storage service in fixed intervals and show some statistics
func ProcessTicker(clientLocal, clientAggr pb.AirQualityMonitoringClient, m *metric.Metric) error {
	rtt, recData, err := requestNewData(clientLocal)
	if err != nil {
		log.Printf("Error requesting new data: %v", err)
		m.Failure("fromAggregatedStorage")
		return err
	}
	m.AddRttTime("fromAggregatedStorage", rtt)
	if len(recData) == 0 {
		log.Printf("No data received from local storage service")
		return nil
	}

	sProcssTime := time.Now()
	dataRes := []agapi.EnhancedResponse{}
	if err := json.Unmarshal([]byte(recData), &dataRes); err != nil {
		log.Printf("Error unmarshalling JSON: %v", err)
		m.Failure("processing")
		return err
	}
	m.Sucess("processing")
	m.AddProcessingTime("processing", time.Since(sProcssTime).Seconds())
	log.Printf("Received [%d] items.", len(dataRes))
	return nil

	// We can also display the data
	resBytes, err := json.Marshal(recData)
	if err != nil {
		log.Printf("Error marshalling processed data: %v", err)
	}
	log.Printf("Processed data: %s\n", string(resBytes))

	return nil
}
