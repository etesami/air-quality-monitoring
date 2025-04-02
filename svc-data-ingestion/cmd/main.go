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
	svcTargetAddress := os.Getenv("SVC_TARGET_ADD")
	svcTargetPort := os.Getenv("SVC_TARGET_PORT")
	targetSvc := &api.Service{
		Address: svcTargetAddress,
		Port:    svcTargetPort,
	}
	// Wait until the target service is reachable
	for {
		if err := targetSvc.ServiceReachable(); err == nil {
			break
		} else {
			log.Printf("Service is not reachable: %v", err)
			time.Sleep(3 * time.Second)
		}
	}

	conn, err := grpc.NewClient(
		targetSvc.Address+":"+targetSvc.Port,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect to target service: %v", err)
	}
	defer conn.Close()
	log.Printf("Connected to target service: %s:%s\n", targetSvc.Address, targetSvc.Port)

	metricList := &metric.Metric{
		RttTimes:        make(map[string][]float64),
		ProcessingTimes: make(map[string][]float64),
		FailureCount:    make(map[string]int),
		SuccessCount:    make(map[string]int),
	}

	client := pb.NewAirQualityMonitoringClient(conn)

	// Local service initialization
	svcAddress := os.Getenv("SVC_LOCAL_ADD")
	svcPort := os.Getenv("SVC_LOCAL_PORT")
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
