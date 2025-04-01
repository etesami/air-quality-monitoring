package main

import (
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
	token := os.Getenv("TOKEN")
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		log.Fatalf("Error parsing environment variables: %v, %v, %v, %v", err1, err2, err3, err4)
	}

	locData := &svcapi.LocationData{
		Lat1:  Lat1,
		Lng1:  Lng1,
		Lat2:  Lat2,
		Lng2:  Lng2,
		Token: token,
	}

	svcAddress := os.Getenv("SVC_ADD")
	svcPort := os.Getenv("SVC_PORT")
	svc := &api.Service{
		Address: svcAddress,
		Port:    svcPort,
	}
	if err := svc.ServiceReachable(); err != nil {
		log.Fatalf("Error checking environment variables: %v", err)
	}

	for {
		if err := svc.ServiceReachable(); err == nil {
			break
		} else {
			log.Printf("Service is not reachable: %v", err)
			time.Sleep(1 * time.Second)
		}
	}

	conn, err := grpc.NewClient(svc.Address+":"+svc.Port, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	metricList := &metric.Metric{
		RttTimes:        make([]float64, 0),
		ProcessingTimes: make([]float64, 0),
		SuccessCount:    0,
		FailureCount:    0,
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

	http.HandleFunc("/metrics/rtt", metricList.RttHandler())
	http.HandleFunc("/metrics/processing", metricList.ProcessingTimeHandler())
	log.Println("Starting server on :8089")
	log.Fatal(http.ListenAndServe(":8089", nil))
}
