package main

import (
	"fmt"
	"log"
	"net"
	"os"

	api "github.com/etesami/air-quality-monitoring/api"
	metric "github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
	svcapi "github.com/etesami/air-quality-monitoring/svc-data-ingestion/api"

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
	conn, err := grpc.NewClient(
		targetSvc.Address+":"+targetSvc.Port,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect to target service: %v", err)
	}
	defer conn.Close()
	log.Printf("Connected to target service: %s:%s\n", targetSvc.Address, targetSvc.Port)

	metricList := &metric.Metric{
		RttTimes:        make([]float64, 0),
		ProcessingTimes: make([]float64, 0),
		SuccessCount:    0,
		FailureCount:    0,
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
	pb.RegisterAirQualityMonitoringServer(grpcServer, &svcapi.Server{
		Client: &client,
		Metric: metricList,
	})

	log.Printf("starting gRPC server on port %s:%s\n", localSvc.Address, localSvc.Port)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
