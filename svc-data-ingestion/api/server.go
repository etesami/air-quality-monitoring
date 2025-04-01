package api

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"time"

	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
)

type Server struct {
	pb.UnimplementedAirQualityMonitoringServer
}

func (s Server) SendData(ctx context.Context, recData *pb.Data) (*pb.Ack, error) {
	receivedTime := time.Now()
	ReceivedTimestamp := receivedTime.UnixMilli()
	log.Printf("Received at [%s]: [%d]\n", receivedTime.Format("2006-01-02 15:04:05"), len(recData.Payload))

	go func() {
		// TODO: Here you can preprocess the data as needed
		// start preprocessing and parsing data
		// For this experiment, we skip additional preprocessing
		// and only consider unmarshalling the JSON data
		// fmt.Printf("%s\n\n", recData.Payload)
		data := &AirQuData{}
		if err := json.Unmarshal([]byte(recData.Payload), &data); err != nil {
			log.Printf("Error unmarshalling JSON: %v", err)
		}
		// Calculate elapsed time (processing time)
		// _, _ = measureTime(&receivedTime, nil)
	}()

	ack := &pb.Ack{
		Status:                "ok",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     strconv.Itoa(int(ReceivedTimestamp)),
		AckSentTimestamp:      strconv.Itoa(int(time.Now().UnixMilli())),
	}

	return ack, nil
}
