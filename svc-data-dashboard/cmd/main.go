package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
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
		FailureCount:    make(map[string]int),
		SuccessCount:    make(map[string]int),
	}

	// Aggregated storage service initialization
	svcTargetAggrAddress := os.Getenv("SVC_TARGET_AGGR_STORAGE_ADD")
	svcTargetAggrPort := os.Getenv("SVC_TARGET_AGGR_STORAGE_PORT")
	targetAggrSvc := &api.Service{
		Address: svcTargetAggrAddress,
		Port:    svcTargetAggrPort,
	}
	// Wait until the target service is reachable
	for {
		if err := targetAggrSvc.ServiceReachable(); err == nil {
			break
		} else {
			log.Printf("Service [%s:%s] is not reachable: %v", targetAggrSvc.Address, targetAggrSvc.Port, err)
			time.Sleep(3 * time.Second)
		}
	}

	connAggr, err := grpc.NewClient(
		targetAggrSvc.Address+":"+targetAggrSvc.Port,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect to target [aggregated storage] service: %v", err)
	}
	defer connAggr.Close()
	log.Printf("Connected to target [aggregated storage] service: %s:%s\n", targetAggrSvc.Address, targetAggrSvc.Port)
	clientAggr := pb.NewAirQualityMonitoringClient(connAggr)

	go func(m *metric.Metric, clientAggr pb.AirQualityMonitoringClient) {
		// Target local storage service initialization
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if err := internal.ProcessTicker(clientAggr, clientAggr, m); err != nil {
				log.Printf("Error during processing: %v", err)
			}

			// TODO: Remove the break
			break
		}

	}(m, clientAggr)

	metricPort := os.Getenv("METRIC_PORT")
	log.Printf("Starting metric server on :%s\n", metricPort)
	http.HandleFunc("/metrics", m.IndexHandler())
	http.HandleFunc("/metrics/rtt", m.RttHandler())
	http.HandleFunc("/metrics/processing", m.ProcessingTimeHandler())
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", metricPort), nil))

}
