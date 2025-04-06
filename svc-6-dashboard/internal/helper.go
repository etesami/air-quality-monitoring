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
	"github.com/etesami/air-quality-monitoring/pkg/utils"
)

// requestNewData requests new data from the central storage service
func requestNewData(client pb.AirQualityMonitoringClient) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	reqBody := loapi.DataRequest{
		RequestType: loapi.RequestPoints,
	}
	reqByte, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshalling JSON: %v", err)
	}

	sentTimestamp := time.Now()
	res, err := client.ReceiveDataFromServer(ctx, &pb.Data{
		Payload:       string(reqByte),
		SentTimestamp: fmt.Sprintf("%d", int(sentTimestamp.UnixMilli())),
	})
	if err != nil {
		return "", fmt.Errorf("error requesting data from local storage: %v", err)
	}
	if len(res.Payload) == 0 {
		log.Printf("No data received from local storage.\n")
		return "", nil
	}
	log.Printf("Response from storage recevied, len: [%d]\n", len(res.Payload))

	return res.Payload, nil
}

// processTicker processes the ticker event
// fetch data from the aggregated storage service in fixed intervals and show some statistics
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

	recData, err := requestNewData(*client)
	if err != nil {
		return fmt.Errorf("error requesting new data: %v", err)
	}
	if len(recData) == 0 {
		return fmt.Errorf("no data received from local storage service")
	}

	sProcssTime := time.Now()
	dataRes := []agapi.EnhancedResponse{}
	if err := json.Unmarshal([]byte(recData), &dataRes); err != nil {
		return fmt.Errorf("error unmarshalling JSON: %v", err)
	}
	m.AddProcessingTime("processing", time.Since(sProcssTime).Seconds())
	log.Printf("Received [%d] items.", len(dataRes))
	return nil

	// We can also display the data
	// resBytes, err := json.Marshal(recData)
	// if err != nil {
	// 	log.Printf("Error marshalling processed data: %v", err)
	// }
	// log.Printf("Processed data: %s\n", string(resBytes))

	return nil
}
