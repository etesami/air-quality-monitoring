package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	api "github.com/etesami/air-quality-monitoring/api"
	metric "github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
	internal "github.com/etesami/air-quality-monitoring/svc-data-processing/internal"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {

	m := &metric.Metric{}
	m.RegisterMetrics()

	// Aggregated storage service initialization
	svcTargetAggrAddress := os.Getenv("SVC_AGGR_STRG_ADDR")
	svcTargetAggrPort := os.Getenv("SVC_AGGR_STRG_PORT")
	targetSvc := &api.Service{
		Address: svcTargetAggrAddress,
		Port:    svcTargetAggrPort,
	}
	var conn *grpc.ClientConn
	var clientAggr pb.AirQualityMonitoringClient
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
				clientAggr = pb.NewAirQualityMonitoringClient(conn)
				log.Printf("Connected to target service: %s:%s\n", targetSvc.Address, targetSvc.Port)
				return
			} else {
				log.Printf("Target service is not reachable: %v", err)
				time.Sleep(5 * time.Second)
			}
		}
	}()
	defer conn.Close()

	svcAddress := os.Getenv("SVC_PROCESSOR_ADDR")
	svcPort := os.Getenv("SVC_PROCESSOR_PORT")
	thisSvc := &api.Service{
		Address: svcAddress,
		Port:    svcPort,
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", thisSvc.Port))
	if err != nil {
		log.Fatal(err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterAirQualityMonitoringServer(grpcServer, &internal.Server{Client: &clientAggr, Metric: m})

	go func() {
		log.Printf("gRPC server is running on port :%s\n", thisSvc.Port)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatal(err)
		}
	}()

	// First call to processTicker
	if err := internal.ProcessTicker(&clientAggr, "central-storage", m); err != nil {
		log.Printf("Error during processing: %v", err)
	}

	updateFrequencyStr := os.Getenv("UPDATE_FREQUENCY")
	updateFrequency, err := strconv.Atoi(updateFrequencyStr)
	if err != nil {
		log.Fatalf("Error parsing update frequency: %v", err)
	}
	ticker := time.NewTicker(time.Duration(updateFrequency) * time.Second)
	defer ticker.Stop()

	go func(m *metric.Metric, clientAggr *pb.AirQualityMonitoringClient) {
		// Target local storage service initialization
		for range ticker.C {
			if err := internal.ProcessTicker(clientAggr, "central-storage", m); err != nil {
				log.Printf("Error during processing: %v", err)
			}
		}
	}(m, &clientAggr)

	metricAddr := os.Getenv("METRIC_ADDR")
	metricPort := os.Getenv("METRIC_PORT")
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Starting server on :%s\n", metricPort)
	http.ListenAndServe(fmt.Sprintf("%s:%s", metricAddr, metricPort), nil)
}
