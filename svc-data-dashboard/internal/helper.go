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
)

// requestNewData requests new data from the local storage service
func requestNewData(client pb.AirQualityMonitoringClient) (int64, string, error) {
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

	dataRes := []agapi.EnhancedResponse{}
	if err := json.Unmarshal([]byte(recData), &dataRes); err != nil {
		log.Printf("Error unmarshalling JSON: %v", err)
		return err
	}
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
