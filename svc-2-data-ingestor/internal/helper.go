package internal

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
	utils "github.com/etesami/air-quality-monitoring/pkg/utils"
)

type Server struct {
	pb.UnimplementedAirQualityMonitoringServer
	Client *pb.AirQualityMonitoringClient
	Metric *metric.Metric
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

	go func(payload string, st time.Time) {

		// TODO: Here you can preprocess the data as needed
		data := &api.AirQualityData{}
		if err := json.Unmarshal([]byte(payload), &data); err != nil {
			log.Printf("Error unmarshalling JSON: %v", err)
			return
		}

		pTime := time.Since(st).Milliseconds()

		// Sneding to the storage
		if *s.Client == nil {
			log.Printf("Client is not ready yet")
			s.Metric.AddProcessingTime("processing", float64(pTime)/1000.0)
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
		pTime = time.Since(st).Milliseconds()
		s.Metric.AddProcessingTime("processing", float64(pTime)/1000.0)

		if err := sendDataToStorage(*s.Client, preprocessedData); err != nil {
			log.Printf("Error sending data to storage: %v", err)
			return
		}

	}(recData.Payload, st)

	ack := &pb.Ack{
		Status:                "ok",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     strconv.Itoa(int(recTimestamp)),
		AckSentTimestamp:      strconv.Itoa(int(time.Now().UnixMilli())),
	}

	return ack, nil
}

func sendDataToStorage(client pb.AirQualityMonitoringClient, d *api.AirQualityData) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Marhal the data to JSON
	byteData, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("error marshalling data to JSON: %v", err)
	}

	sentTimestamp := time.Now()
	ack, err := client.SendDataToServer(ctx, &pb.Data{
		Payload:       string(byteData),
		SentTimestamp: strconv.Itoa(int(sentTimestamp.UnixMilli())),
	})
	if err != nil {
		return fmt.Errorf("send data not successful: %v", err)
	}
	if ack.Status != "ok" {
		return fmt.Errorf("ack status not expected: %s", ack.Status)
	}
	log.Printf("Sent [%d] bytes. Ack recevied, status: [%s]]\n", len(byteData), ack.Status)
	return nil
}

// processTicker processes the ticker event
func ProcessTicker(client *pb.AirQualityMonitoringClient, serverName string, metricList *metric.Metric) error {
	if *client == nil {
		log.Printf("Client is not ready yet")
		return nil
	}
	go func(m *metric.Metric) {
		pong, err := (*client).CheckConnection(context.Background(), &pb.Data{
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

	return nil
}
