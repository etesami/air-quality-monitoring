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
	internal "github.com/etesami/air-quality-monitoring/svc-local-storage/internal"

	_ "github.com/mattn/go-sqlite3"
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
	svcTargetAddress := os.Getenv("SVC_TA_PROCESSOR_ADDR")
	svcTargetPort := os.Getenv("SVC_TA_PROCESSOR_PORT")
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

	svcAddress := os.Getenv("SVC_LO_STRG_ADDR")
	svcPort := os.Getenv("SVC_LO_STRG_PORT")
	thisSvc := &api.Service{
		Address: svcAddress,
		Port:    svcPort,
	}

	metricList := &metric.Metric{
		RttTimes:        make(map[string][]float64),
		ProcessingTimes: make(map[string][]float64),
	}

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
	pb.RegisterAirQualityMonitoringServer(grpcServer, &internal.Server{Db: db, Metric: metricList})

	go func() {
		log.Printf("gRPC server is running on port :%s\n", thisSvc.Port)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatal(err)
		}
	}()

	// First call to processTicker
	ctx := context.Background()
	if err := internal.ProcessTicker(ctx, &clientProcessor, db, "processor", metricList); err != nil {
		log.Printf("Error during processing: %v", err)
	}
	ctx = context.WithValue(ctx, "lastCallTime", time.Now())

	// Frequently send new data to the processor service
	updateFrequencyStr := os.Getenv("SVC_LO_STRG_UPDATE_FREQUENCY")
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
	}(metricList, &clientProcessor)

	metricAddr := os.Getenv("SVC_LO_STRG_METRIC_ADDR")
	metricPort := os.Getenv("SVC_LO_STRG_METRIC_PORT")
	log.Printf("Starting metric server on :%s\n", metricPort)
	http.HandleFunc("/metrics", metricList.IndexHandler())
	http.HandleFunc("/metrics/rtt", metricList.RttHandler())
	http.HandleFunc("/metrics/processing", metricList.ProcessingTimeHandler())
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%s", metricAddr, metricPort), nil))
}
