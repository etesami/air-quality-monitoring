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
	utils "github.com/etesami/air-quality-monitoring/pkg/utils"
	internal "github.com/etesami/air-quality-monitoring/svc-data-ingestion/internal"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Target service initialization
	svcTargetAddress := os.Getenv("SVC_STRG_ADDR")
	svcTargetPort := os.Getenv("SVC_STRG_PORT")
	targetSvc := &api.Service{
		Address: svcTargetAddress,
		Port:    svcTargetPort,
	}

	var conn *grpc.ClientConn
	var clientStrg pb.AirQualityMonitoringClient
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
				clientStrg = pb.NewAirQualityMonitoringClient(conn)
				log.Printf("Connected to target service: %s:%s\n", targetSvc.Address, targetSvc.Port)
				return
			} else {
				log.Printf("Target service is not reachable: %v", err)
				time.Sleep(5 * time.Second)
			}
		}
	}()
	defer conn.Close()

	sentDataBuckets := utils.ParseBuckets(os.Getenv("SENT_DATA_BUCEKTS"))
	procTimeBuckets := utils.ParseBuckets(os.Getenv("PROC_TIME_BUCKETS"))
	rttTimeBuckets := utils.ParseBuckets(os.Getenv("RTT_TIME_BUCKETS"))

	m := &metric.Metric{}
	m.RegisterMetrics(sentDataBuckets, procTimeBuckets, rttTimeBuckets)

	// Local service initialization
	svcAddress := os.Getenv("SVC_INGST_ADDR")
	svcPort := os.Getenv("SVC_INGST_PORT")
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
		Client: &clientStrg,
		Metric: m,
	})

	go func() {
		log.Printf("starting gRPC server on port %s:%s\n", localSvc.Address, localSvc.Port)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// First call to processTicker
	if err := internal.ProcessTicker(&clientStrg, "local-storage", m); err != nil {
		log.Printf("Error during processing: %v", err)
	}

	// Set up a ticker to periodically call the gRPC server to measure the RTT
	updateFrequencyStr := os.Getenv("UPDATE_FREQUENCY")
	updateFrequency, err := strconv.Atoi(updateFrequencyStr)
	if err != nil {
		log.Fatalf("Error parsing update frequency: %v", err)
	}
	ticker := time.NewTicker(time.Duration(updateFrequency) * time.Minute)
	defer ticker.Stop()

	go func(m *metric.Metric, c *pb.AirQualityMonitoringClient) {
		for range ticker.C {
			if err := internal.ProcessTicker(c, "local-storage", m); err != nil {
				log.Printf("Error during processing: %v", err)
			}
		}
	}(m, &clientStrg)

	metricAddr := os.Getenv("METRIC_ADDR")
	metricPort := os.Getenv("METRIC_PORT")
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Starting server on :%s\n", metricPort)
	http.ListenAndServe(fmt.Sprintf("%s:%s", metricAddr, metricPort), nil)
}
