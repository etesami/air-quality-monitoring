package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	api "github.com/etesami/air-quality-monitoring/api"
	metric "github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
)

type Server struct {
	pb.UnimplementedAirQualityMonitoringServer
	Client *pb.AirQualityMonitoringClient
	Metric *metric.Metric
}

// updateMetrics updates the metrics with the received RTT
func updateMetrics(metricList *metric.Metric, t string, rtt int64) {
	metricList.Lock()
	defer metricList.Unlock()
	if t == "rtt" {
		metricList.RttTimes = append(metricList.RttTimes, float64(rtt))
	} else if t == "processing" {
		metricList.ProcessingTimes = append(metricList.ProcessingTimes, float64(rtt))
	}
	metricList.SuccessCount++
}

func (s Server) SendDataToIngsetion(ctx context.Context, recData *pb.Data) (*pb.Ack, error) {
	receivedTime := time.Now()
	ReceivedTimestamp := receivedTime.UnixMilli()
	log.Printf("Received at [%s]: [%d]\n", receivedTime.Format("2006-01-02 15:04:05"), len(recData.Payload))

	go func() {
		// TODO: Here you can preprocess the data as needed
		start_processing := time.Now()
		data := &api.AirQualityData{}
		if err := json.Unmarshal([]byte(recData.Payload), &data); err != nil {
			log.Printf("Error unmarshalling JSON: %v", err)
		}
		end_processing := time.Now()
		processingTime := end_processing.UnixMilli() - start_processing.UnixMilli()
		log.Printf("Processing time: [%d]ms\n", processingTime)
		updateMetrics(s.Metric, "processing", processingTime)

		// Sneding to the storage
		rtt, err := sendDataToStorage(*s.Client, data)
		if err != nil {
			log.Printf("Error sending data to storage: %v", err)
			return
		}
		if rtt >= 0 {
			log.Printf("RTT: [%d]ms\n", rtt)
			updateMetrics(s.Metric, "rtt", rtt)
		}
	}()

	ack := &pb.Ack{
		Status:                "ok",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     strconv.Itoa(int(ReceivedTimestamp)),
		AckSentTimestamp:      strconv.Itoa(int(time.Now().UnixMilli())),
	}

	return ack, nil
}

func sendDataToStorage(client pb.AirQualityMonitoringClient, d *api.AirQualityData) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Marhal the data to JSON
	jsonString, err := json.Marshal(d)
	if err != nil {
		return -1, fmt.Errorf("error marshalling data to JSON: %v", err)
	}

	sentTimestamp := time.Now().UnixMilli()
	ack, err := client.SendDataToStorage(ctx, &pb.Data{
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
