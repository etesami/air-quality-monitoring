package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	// "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	api "github.com/etesami/air-quality-monitoring/api"
	metric "github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
)

type Server struct {
	pb.UnimplementedAirQualityMonitoringServer
	Client *pb.AirQualityMonitoringClient
	Metric *metric.Metric
}

func (s Server) SendDataToIngsetion(ctx context.Context, recData *pb.Data) (*pb.Ack, error) {
	receivedTime := time.Now()
	ReceivedTimestamp := receivedTime.UnixMilli()
	log.Printf("Received at [%s]: [%d]\n", receivedTime.Format("2006-01-02 15:04:05"), len(recData.Payload))

	go func(payload string) {
		// TODO: Here you can preprocess the data as needed
		start := time.Now()

		data := &api.AirQualityData{}
		if err := json.Unmarshal([]byte(payload), &data); err != nil {
			log.Printf("Error unmarshalling JSON: %v", err)
			s.Metric.Failure("ingestion")
			return
		}

		processingTime := time.Since(start).Milliseconds()
		s.Metric.Sucess("ingestion")
		s.Metric.AddProcessingTime("ingestion", float64(processingTime)/1000.0)

		// Sneding to the storage
		rtt, err := sendDataToStorage(*s.Client, data)
		if err != nil {
			log.Printf("Error sending data to storage: %v", err)
			s.Metric.Failure("toLocalStorage")
			return
		}
		log.Printf("Sent data to local storage. Len: [%d], RTT: [%d]ms\n", len(payload), rtt)
		s.Metric.Sucess("toLocalStorage")
		s.Metric.AddRttTime("toLocalStorage", float64(rtt)/1000.0)
	}(recData.Payload)

	ack := &pb.Ack{
		Status:                "ok",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     strconv.Itoa(int(ReceivedTimestamp)),
		AckSentTimestamp:      strconv.Itoa(int(time.Now().UnixMilli())),
	}

	return ack, nil
}

func sendDataToStorage(client pb.AirQualityMonitoringClient, d *api.AirQualityData) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Marhal the data to JSON
	byteData, err := json.Marshal(d)
	if err != nil {
		return -1, fmt.Errorf("error marshalling data to JSON: %v", err)
	}

	sentTimestamp := time.Now()
	ack, err := client.SendDataToStorage(ctx, &pb.Data{
		Payload:       string(byteData),
		SentTimestamp: strconv.Itoa(int(sentTimestamp.UnixMilli())),
	})
	if err != nil {
		return -1, fmt.Errorf("send data not successful: %v", err)
	}
	if ack.Status != "ok" {
		return -1, fmt.Errorf("ack status not expected: %s", ack.Status)
	}
	rtt := time.Since(sentTimestamp).Milliseconds()
	log.Printf("Sent [%d] bytes to local storage. Ack recevied. RTT: [%d]ms\n", len(byteData), rtt)
	return rtt, nil
}
