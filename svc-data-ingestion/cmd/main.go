package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	api "github.com/etesami/air-quality-monitoring/api"
	metric "github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
	internal "github.com/etesami/air-quality-monitoring/svc-data-ingestion/internal"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Target service initialization
	svcTargetAddress := os.Getenv("SVC_TARGET_LOCAL_STRG_ADDR")
	svcTargetPort := os.Getenv("SVC_TARGET_LOCAL_STRG_PORT")
	targetSvc := &api.Service{
		Address: svcTargetAddress,
		Port:    svcTargetPort,
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

	metricList := &metric.Metric{
		RttTimes:        make(map[string][]float64),
		ProcessingTimes: make(map[string][]float64),
		FailureCount:    make(map[string]int),
		SuccessCount:    make(map[string]int),
	}

	// Local service initialization
	svcAddress := os.Getenv("SVC_LOCAL_INGESTION_ADDR")
	svcPort := os.Getenv("SVC_LOCAL_INGESTION_PORT")
	localSvc := &api.Service{
		Address: svcAddress,
		Port:    svcPort,
	}

	// We listen on all interfaces
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", localSvc.Port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterAirQualityMonitoringServer(grpcServer, &internal.Server{
		Client: &client,
		Metric: metricList,
	})

	go func() {
		log.Printf("starting gRPC server on port %s:%s\n", localSvc.Address, localSvc.Port)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	metricPort := os.Getenv("METRIC_PORT")
	http.HandleFunc("/metrics", metricList.IndexHandler())
	http.HandleFunc("/metrics/rtt", metricList.RttHandler())
	http.HandleFunc("/metrics/processing", metricList.ProcessingTimeHandler())
	log.Printf("Starting server on :%s\n", metricPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", metricPort), nil))
}
