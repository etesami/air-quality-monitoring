package main

import (
	"fmt"
	"log"
	"net"
	"os"

	api "github.com/etesami/air-quality-monitoring/api"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
	svcapi "github.com/etesami/air-quality-monitoring/svc-data-ingestion/api"

	"google.golang.org/grpc"
)

// // measureTime is a utility function to measure elapsed time
// // It returns the current time if start is nil, and the elapsed time if end is nil
// // If both are nil, it returns the current time and nil for duration
// func measureTime(start, end *time.Time) (time.Time, *time.Duration) {
// 	if start == nil {
// 		return time.Now(), nil
// 	}
// 	if end == nil {
// 		duration := time.Since(*start)
// 		return time.Time{}, &duration
// 	}
// 	// If both start and end are nil, return the current time
// 	return time.Now(), nil
// }

func main() {
	// Initialize the service
	svcAddress := os.Getenv("SVC_ADD")
	svcPort := os.Getenv("SVC_PORT")
	thisSvc := &api.Service{
		Address: svcAddress,
		Port:    svcPort,
	}

	// We listen on all interfaces
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", thisSvc.Port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterAirQualityMonitoringServer(grpcServer, &svcapi.Server{})

	log.Printf("gRPC server is running on port :%s\n", thisSvc.Port)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
