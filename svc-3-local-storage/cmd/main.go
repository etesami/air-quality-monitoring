package main

import (
	"context"
	"database/sql"
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
	internal "github.com/etesami/air-quality-monitoring/svc-local-storage/internal"

	_ "github.com/mattn/go-sqlite3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func createTables(db *sql.DB) error {
	// Create your tables here
	query := `
			CREATE TABLE IF NOT EXISTS air_quality (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					aqi INTEGER,
  				idx INTEGER,
  				timestamp DATETIME,
  				attributions TEXT,
  				city TEXT,
  				dominentpol TEXT,
  				forecast TEXT,
  				iaqi TEXT,
					status TEXT
			);`
	_, err := db.Exec(query)
	return err
}

func main() {

	// Target service initialization
	svcTargetAddress := os.Getenv("SVC_PROCESSOR_ADDR")
	svcTargetPort := os.Getenv("SVC_PROCESSOR_PORT")
	targetSvc := &api.Service{
		Address: svcTargetAddress,
		Port:    svcTargetPort,
	}

	var conn *grpc.ClientConn
	var clientProcessor pb.AirQualityMonitoringClient
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
				clientProcessor = pb.NewAirQualityMonitoringClient(conn)
				log.Printf("Connected to target service: %s:%s\n", targetSvc.Address, targetSvc.Port)
				return
			} else {
				log.Printf("Target service is not reachable: %v", err)
				time.Sleep(5 * time.Second)
			}
		}
	}()
	defer conn.Close()

	svcAddress := os.Getenv("SVC_STRG_ADDR")
	svcPort := os.Getenv("SVC_STRG_PORT")
	thisSvc := &api.Service{
		Address: svcAddress,
		Port:    svcPort,
	}

	sentDataBuckets := utils.ParseBuckets(os.Getenv("SENT_DATA_BUCKETS"))
	procTimeBuckets := utils.ParseBuckets(os.Getenv("PROC_TIME_BUCKETS"))
	rttTimeBuckets := utils.ParseBuckets(os.Getenv("RTT_TIME_BUCKETS"))

	m := &metric.Metric{}
	m.RegisterMetrics(sentDataBuckets, procTimeBuckets, rttTimeBuckets)

	db, err := sql.Open("sqlite3", "./data.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = createTables(db)
	if err != nil {
		log.Fatal(err)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", thisSvc.Port))
	if err != nil {
		log.Fatal(err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterAirQualityMonitoringServer(grpcServer, &internal.Server{Db: db, Metric: m})

	go func() {
		log.Printf("gRPC server is running on port :%s\n", thisSvc.Port)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatal(err)
		}
	}()

	// First call to processTicker
	ctx := context.Background()
	if err := internal.ProcessTicker(ctx, &clientProcessor, db, "processor", m); err != nil {
		log.Printf("Error during processing: %v", err)
	}
	ctx = context.WithValue(ctx, "lastCallTime", time.Now())

	// Frequently send new data to the processor service
	updateFrequencyStr := os.Getenv("UPDATE_FREQUENCY")
	updateFrequency, err := strconv.Atoi(updateFrequencyStr)
	if err != nil {
		log.Fatalf("Error parsing update frequency: %v", err)
	}
	ticker := time.NewTicker(time.Duration(updateFrequency) * time.Minute)
	defer ticker.Stop()

	go func(m *metric.Metric, c *pb.AirQualityMonitoringClient) {
		for range ticker.C {
			if err := internal.ProcessTicker(ctx, c, db, "processor", m); err != nil {
				log.Printf("Error during processing: %v", err)
			}
			ctx = context.WithValue(ctx, "lastCallTime", time.Now())
		}
	}(m, &clientProcessor)

	metricAddr := os.Getenv("METRIC_ADDR")
	metricPort := os.Getenv("METRIC_PORT")
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Starting server on :%s\n", metricPort)
	http.ListenAndServe(fmt.Sprintf("%s:%s", metricAddr, metricPort), nil)
}
