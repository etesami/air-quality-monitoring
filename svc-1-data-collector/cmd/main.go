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
	utils "github.com/etesami/air-quality-monitoring/pkg/utils"
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
	token := os.Getenv("TOKEN")
	locationIdentifier := ""

	var locData *internal.LocationData

	// if the coordinates are not set, use a random city location,
	// we ensure at least 5 locations are returned
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		log.Printf("Error parsing coordinates. Using a random city location coordinate.")

		hostname, err := os.Hostname()
		if err != nil {
			fmt.Printf("Error getting hostname: %v\n", err)
			return
		}
		locationIdentifier = hostname

		for {
			coordinates := internal.GetRandomBoxCoordination(locationIdentifier)
			Lat1 = coordinates[0]
			Lng1 = coordinates[1]
			Lat2 = coordinates[2]
			Lng2 = coordinates[3]
			locData = &internal.LocationData{
				Lat1:  Lat1,
				Lng1:  Lng1,
				Lat2:  Lat2,
				Lng2:  Lng2,
				Token: token,
			}
			ids, err := locData.CollectLocationsIds()
			if err == nil && len(ids) < 5 {
				break
			}
			log.Printf("Error getting location IDs or not enough IDs: %v", err)
			time.Sleep(1 * time.Second)
		}
		log.Printf("Using random box coordinates: %f, %f, %f, %f\n", Lat1, Lng1, Lat2, Lng2)
	} else {
		locData = &internal.LocationData{
			Lat1:  Lat1,
			Lng1:  Lng1,
			Lat2:  Lat2,
			Lng2:  Lng2,
			Token: token,
		}
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

	sentDataBuckets := utils.ParseBuckets(os.Getenv("SENT_DATA_BUCEKTS"))
	procTimeBuckets := utils.ParseBuckets(os.Getenv("PROC_TIME_BUCKETS"))
	rttTimeBuckets := utils.ParseBuckets(os.Getenv("RTT_TIME_BUCKETS"))

	m := &metric.Metric{}
	m.RegisterMetrics(sentDataBuckets, procTimeBuckets, rttTimeBuckets)

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
	ticker := time.NewTicker(time.Duration(updateFrequency) * time.Second)
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
