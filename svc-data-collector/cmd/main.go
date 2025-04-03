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
	svcapi "github.com/etesami/air-quality-monitoring/svc-data-collector/api"
	internal "github.com/etesami/air-quality-monitoring/svc-data-collector/internal"

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
	locData := &svcapi.LocationData{
		Lat1:  Lat1,
		Lng1:  Lng1,
		Lat2:  Lat2,
		Lng2:  Lng2,
		Token: token,
	}

	svcAddress := os.Getenv("SVC_TARGET_COLLECTOR_ADDR")
	svcPort := os.Getenv("SVC_TARGET_COLLECTOR_PORT")
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

	metricList := &metric.Metric{
		RttTimes:        make(map[string][]float64),
		ProcessingTimes: make(map[string][]float64),
		FailureCount:    make(map[string]int),
		SuccessCount:    make(map[string]int),
	}
	client := pb.NewAirQualityMonitoringClient(conn)

	// Call api evey 60 seconds
	// TODO: Adjust the time
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			if err := internal.ProcessTicker(client, locData, metricList); err != nil {
				log.Printf("Error during processing: %v", err)
			}
			// TODO: Remove the break
			break
		}
	}()

	metricPort := os.Getenv("METRIC_PORT")
	http.HandleFunc("/metrics", metricList.IndexHandler())
	http.HandleFunc("/metrics/rtt", metricList.RttHandler())
	http.HandleFunc("/metrics/processing", metricList.ProcessingTimeHandler())
	log.Printf("Starting server on :%s\n", metricPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", metricPort), nil))
}
