package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	api "github.com/etesami/air-quality-monitoring/api"
	metric "github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
	internal "github.com/etesami/air-quality-monitoring/svc-data-dashboard/internal"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {

	m := &metric.Metric{
		RttTimes:        make(map[string][]float64),
		ProcessingTimes: make(map[string][]float64),
	}

	// Aggregated storage service initialization
	svcTargetAggrAddress := os.Getenv("SVC_AGGR_STRG_ADDR")
	svcTargetAggrPort := os.Getenv("SVC_AGGR_STRG_PORT")
	targetSvc := &api.Service{
		Address: svcTargetAggrAddress,
		Port:    svcTargetAggrPort,
	}

	var conn *grpc.ClientConn
	var client pb.AirQualityMonitoringClient
	go func() {
		for {
			if err := targetSvc.ServiceReachable(); err == nil {
				var err error
				conn, err = grpc.NewClient(
					targetSvc.Address+":"+targetSvc.Port,
					grpc.WithTransportCredentials(insecure.NewCredentials()))
				if err != nil {
					log.Printf("Failed to connect to target service: %v", err)
					return
				}
				client = pb.NewAirQualityMonitoringClient(conn)
				log.Printf("Connected to target service: %s:%s\n", targetSvc.Address, targetSvc.Port)
				return
			} else {
				log.Printf("Target service is not reachable: %v", err)
				time.Sleep(5 * time.Second)
			}
		}
	}()
	defer conn.Close()

	// First call to processTicker
	if err := internal.ProcessTicker(&client, m); err != nil {
		log.Printf("Error during processing: %v", err)
	}

	updateFrequencyStr := os.Getenv("UPDATE_FREQUENCY")
	updateFrequency, err := strconv.Atoi(updateFrequencyStr)
	if err != nil {
		log.Fatalf("Error parsing update frequency: %v", err)
	}

	go func(m *metric.Metric, c *pb.AirQualityMonitoringClient, u int) {
		// Target local storage service initialization
		ticker := time.NewTicker(time.Duration(u) * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if err := internal.ProcessTicker(c, m); err != nil {
				log.Printf("Error during processing: %v", err)
			}
		}

	}(m, &client, updateFrequency)

	metricAddr := os.Getenv("METRIC_ADDR")
	metricPort := os.Getenv("METRIC_PORT")
	log.Printf("Starting metric server on :%s\n", metricPort)
	http.HandleFunc("/metrics", m.IndexHandler())
	http.HandleFunc("/metrics/rtt", m.RttHandler())
	http.HandleFunc("/metrics/processing", m.ProcessingTimeHandler())
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%s", metricAddr, metricPort), nil))
}
