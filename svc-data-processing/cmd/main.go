package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	api "github.com/etesami/air-quality-monitoring/api"
	metric "github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
	internal "github.com/etesami/air-quality-monitoring/svc-data-processing/internal"

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

	// Local storage service initialization
	// We need both target services to be reachable, otherwise we cannot process
	svcTargetLocalAddress := os.Getenv("SVC_TA_LO_STRG_ADDR")
	svcTargetLocalPort := os.Getenv("SVC_TA_LO_STRG_PORT")
	targetLocalStrgSvc := &api.Service{
		Address: svcTargetLocalAddress,
		Port:    svcTargetLocalPort,
	}
	// Wait until the target service is reachable
	for {
		if err := targetLocalStrgSvc.ServiceReachable(); err == nil {
			break
		} else {
			log.Printf("Service [%s:%s] is not reachable: %v", targetLocalStrgSvc.Address, targetLocalStrgSvc.Port, err)
			time.Sleep(3 * time.Second)
		}
	}

	// Aggregated storage service initialization
	svcTargetAggrAddress := os.Getenv("SVC_TA_AGGR_STRG_ADDR")
	svcTargetAggrPort := os.Getenv("SVC_TA_AGGR_STRG_PORT")
	targetAggrStrgSvc := &api.Service{
		Address: svcTargetAggrAddress,
		Port:    svcTargetAggrPort,
	}
	// Wait until the target service is reachable
	for {
		if err := targetAggrStrgSvc.ServiceReachable(); err == nil {
			break
		} else {
			log.Printf("Service [%s:%s] is not reachable: %v", targetAggrStrgSvc.Address, targetAggrStrgSvc.Port, err)
			time.Sleep(3 * time.Second)
		}
	}

	connLocal, err := grpc.NewClient(
		targetLocalStrgSvc.Address+":"+targetLocalStrgSvc.Port,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect to target [local storage] service: %v", err)
	}
	defer connLocal.Close()
	log.Printf("Connected to target [local storage] service: %s:%s\n", targetLocalStrgSvc.Address, targetLocalStrgSvc.Port)
	clientLocal := pb.NewAirQualityMonitoringClient(connLocal)

	connAggr, err := grpc.NewClient(
		targetAggrStrgSvc.Address+":"+targetAggrStrgSvc.Port,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect to target [aggregated storage] service: %v", err)
	}
	defer connAggr.Close()
	log.Printf("Connected to target [aggregated storage] service: %s:%s\n", targetAggrStrgSvc.Address, targetAggrStrgSvc.Port)
	clientAggr := pb.NewAirQualityMonitoringClient(connAggr)
	// var clientAggr pb.AirQualityMonitoringClient

	go func(m *metric.Metric, clientLocal pb.AirQualityMonitoringClient, clientAggr pb.AirQualityMonitoringClient) {
		// Target local storage service initialization
		ctx := context.Background()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if err := internal.ProcessTicker(ctx, clientLocal, clientAggr, m); err != nil {
				log.Printf("Error during processing: %v", err)
			}
			// We check for up to one hour in advance, to cover for time inconsistencies
			// between the the external data sources
			ctx = context.WithValue(ctx, "lastCall", time.Now().Add(1*time.Hour))

		}

	}(m, clientLocal, clientAggr)

	metricAddr := os.Getenv("SVC_LO_DP_METRIC_ADDR")
	metricPort := os.Getenv("SVC_LO_DP_METRIC_PORT")
	log.Printf("Starting metric server on :%s\n", metricPort)
	http.HandleFunc("/metrics", m.IndexHandler())
	http.HandleFunc("/metrics/rtt", m.RttHandler())
	http.HandleFunc("/metrics/processing", m.ProcessingTimeHandler())
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%s", metricAddr, metricPort), nil))

}
