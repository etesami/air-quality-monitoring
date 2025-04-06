package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	api "github.com/etesami/air-quality-monitoring/api"
	metric "github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
	internal "github.com/etesami/air-quality-monitoring/svc-data-collector/internal"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {

	Lat1, err1 := strconv.ParseFloat(os.Getenv("LAT1"), 64)
	Lng1, err2 := strconv.ParseFloat(os.Getenv("LNG1"), 64)
	Lat2, err3 := strconv.ParseFloat(os.Getenv("LAT2"), 64)
	Lng2, err4 := strconv.ParseFloat(os.Getenv("LNG2"), 64)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		log.Fatalf("Error parsing environment variables: %v, %v, %v, %v", err1, err2, err3, err4)
	}

	token := os.Getenv("TOKEN")
	locData := &internal.LocationData{
		Lat1:  Lat1,
		Lng1:  Lng1,
		Lat2:  Lat2,
		Lng2:  Lng2,
		Token: token,
	}

	svcAddress := os.Getenv("SVC_INGESTION_ADDR")
	svcPort := os.Getenv("SVC_INGESTION_PORT")
	svc := &api.Service{
		Address: svcAddress,
		Port:    svcPort,
	}

	for {
		if err := svc.ServiceReachable(); err == nil {
			break
		} else {
			log.Printf("Service [%s:%s] is not reachable: %v", svc.Address, svc.Port, err)
			time.Sleep(3 * time.Second)
		}
	}

	conn, err := grpc.NewClient(svc.Address+":"+svc.Port, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect to [%s:%s]: %v", svc.Address, svc.Port, err)
	}
	defer conn.Close()
	log.Printf("Connected to target service: [%s:%s]\n", svc.Address, svc.Port)

	m := &metric.Metric{}
	m.RegisterMetrics()

	client := pb.NewAirQualityMonitoringClient(conn)

	// First call to processTicker
	if err := internal.ProcessTicker(&client, "ingestor", locData, m); err != nil {
		log.Printf("Error during processing: %v", err)
	}

	updateFrequencyStr := os.Getenv("UPDATE_FREQUENCY")
	updateFrequency, err := strconv.Atoi(updateFrequencyStr)
	if err != nil {
		log.Fatalf("Error parsing update frequency: %v", err)
	}
	ticker := time.NewTicker(time.Duration(updateFrequency) * time.Minute)
	defer ticker.Stop()

	go func(c *pb.AirQualityMonitoringClient, metricList *metric.Metric) {
		for range ticker.C {
			if err := internal.ProcessTicker(c, "ingestor", locData, metricList); err != nil {
				log.Printf("Error during processing: %v", err)
			}
		}
	}(&client, m)

	metricAddr := os.Getenv("METRIC_ADDR")
	metricPort := os.Getenv("METRIC_PORT")
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Starting server on :%s\n", metricPort)
	http.ListenAndServe(fmt.Sprintf("%s:%s", metricAddr, metricPort), nil)
}
