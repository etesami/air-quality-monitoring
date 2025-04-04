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
	utils "github.com/etesami/air-quality-monitoring/pkg/utils"
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
		if *s.Client == nil {
			log.Printf("Client is not ready yet")
			s.Metric.Failure("toLocalStorage")
			return
		}

		// Make sure there is no empty data (with enpty city name)
		preprocessedData := &api.AirQualityData{
			Status: data.Status,
			Ver:    data.Ver,
		}
		for _, obs := range data.Obs {
			if obs.Msg.City.Name == "" {
				log.Printf("City name is empty, skipping observation")
				continue
			}
			preprocessedData.Obs = append(preprocessedData.Obs, obs)
		}
		if len(preprocessedData.Obs) == 0 {
			log.Printf("No valid observations found, skipping data")
			return
		}

		rtt, err := sendDataToStorage(*s.Client, preprocessedData)
		if err != nil {
			log.Printf("Error sending data to storage: %v", err)
			s.Metric.Failure("toLocalStorage")
			return
		}

		s.Metric.Sucess("toLocalStorage")
		s.Metric.AddRttTime("toLocalStorage", rtt)

	}(recData.Payload)

	ack := &pb.Ack{
		Status:                "ok",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     strconv.Itoa(int(ReceivedTimestamp)),
		AckSentTimestamp:      strconv.Itoa(int(time.Now().UnixMilli())),
	}

	return ack, nil
}

func sendDataToStorage(client pb.AirQualityMonitoringClient, d *api.AirQualityData) (float64, error) {
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
	log.Printf("Sent [%d] bytes. Ack recevied.\n", len(byteData))
	rtt, err := utils.CalculateRtt(sentTimestamp, time.Now(), *ack)
	if err != nil {
		return -1, fmt.Errorf("error calculating RTT: %v", err)
	}
	return rtt, nil
}
