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
	utils "github.com/etesami/air-quality-monitoring/pkg/utils"

	internal "github.com/etesami/air-quality-monitoring/svc-aggregated-storage/internal"

	_ "github.com/mattn/go-sqlite3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

func createTables(db *sql.DB) error {
	// Create your tables here
	queries := []string{`
		CREATE TABLE IF NOT EXISTS air_quality (
				hash TEXT PRIMARY KEY UNIQUE,
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
				idx INTEGER PRIMARY KEY UNIQUE,
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

	svcAddress := os.Getenv("SVC_AGGR_STRG_ADDR")
	svcPort := os.Getenv("SVC_AGGR_STRG_PORT")
	thisSvc := &api.Service{
		Address: svcAddress,
		Port:    svcPort,
	}

	sentDataBuckets := utils.ParseBuckets(os.Getenv("SENT_DATA_BUCEKTS"))
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

	metricAddr := os.Getenv("METRIC_ADDR")
	metricPort := os.Getenv("METRIC_PORT")
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Starting server on :%s\n", metricPort)
	http.ListenAndServe(fmt.Sprintf("%s:%s", metricAddr, metricPort), nil)
}
