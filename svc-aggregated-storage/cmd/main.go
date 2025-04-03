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

	internal "github.com/etesami/air-quality-monitoring/svc-aggregated-storage/internal"

	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/grpc"
)

func createTables(db *sql.DB) error {
	// Create your tables here
	queries := []string{`
		CREATE TABLE IF NOT EXISTS air_quality (
				aqi INTEGER,
				timestamp DATETIME,
				dewPoint INTEGER,
				humidity INTEGER,
				pressure INTEGER,
				temperature INTEGER,
				windSpeed INTEGER,
				windGust INTEGER,
				pm25 INTEGER,
				city_id INTEGER,
				FOREIGN KEY (city_id) REFERENCES city(idx)
		);`,
		`CREATE TABLE IF NOT EXISTS city (
				idx INTEGER PRIMARY KEY,
				cityName TEXT,
				lat REAL,
				lng REAL
		);`,
		`CREATE TABLE IF NOT EXISTS alert (
				hash TEXT PRIMARY KEY UNIQUE,
				alertDesc TEXT,
				alertEffective DATETIME,
				alertExpires DATETIME,
				alertStatus TEXT,
				alertCertainty TEXT,
				alertUrgency TEXT,
				alertSeverity TEXT,
				alertHeadline TEXT,
				alertDescription TEXT,
				alertEvent TEXT,
				city_id INTEGER,
    		FOREIGN KEY (city_id) REFERENCES city(idx)
		);`,
	}
	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			log.Fatalf("Error executing query: %v", err)
			return err
		}
	}
	return nil
}

func main() {

	svcAddress := os.Getenv("SVC_LOCAL_ADD")
	svcPort := os.Getenv("SVC_LOCAL_PORT")
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

	metricPort := os.Getenv("METRIC_PORT")
	log.Printf("Starting metric server on :%s\n", metricPort)
	http.HandleFunc("/metrics", metricList.IndexHandler())
	http.HandleFunc("/metrics/rtt", metricList.RttHandler())
	http.HandleFunc("/metrics/processing", metricList.ProcessingTimeHandler())
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", metricPort), nil))
}
