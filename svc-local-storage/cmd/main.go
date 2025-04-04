package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	api "github.com/etesami/air-quality-monitoring/api"
	metric "github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
	internal "github.com/etesami/air-quality-monitoring/svc-local-storage/internal"

	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/grpc"
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

	svcAddress := os.Getenv("SVC_LO_STRG_ADDR")
	svcPort := os.Getenv("SVC_LO_STRG_PORT")
	thisSvc := &api.Service{
		Address: svcAddress,
		Port:    svcPort,
	}

	metricList := &metric.Metric{
		RttTimes:        make(map[string][]float64),
		ProcessingTimes: make(map[string][]float64),
		FailureCount:    make(map[string]int),
		SuccessCount:    make(map[string]int),
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

	metricAddr := os.Getenv("SVC_LO_STRG_METRIC_ADDR")
	metricPort := os.Getenv("SVC_LO_STRG_METRIC_PORT")
	log.Printf("Starting metric server on :%s\n", metricPort)
	http.HandleFunc("/metrics", metricList.IndexHandler())
	http.HandleFunc("/metrics/rtt", metricList.RttHandler())
	http.HandleFunc("/metrics/processing", metricList.ProcessingTimeHandler())
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%s", metricAddr, metricPort), nil))
}
