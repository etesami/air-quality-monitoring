package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	api "github.com/etesami/air-quality-monitoring/api"
	"github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
	_ "github.com/mattn/go-sqlite3"
)

type Server struct {
	pb.UnimplementedAirQualityMonitoringServer
	Metric *metric.Metric
	Db     *sql.DB
}

// func (s Server) ReceiveDataFromLocalStorage(ctx context.Context, req *pb.Data) (*pb.DataResponse, error) {
// 	receivedTime := time.Now()
// 	ReceivedTimestamp := receivedTime.UnixMilli()
// 	log.Printf("Received request for data: [%d]\n", len(req.Payload))

// 	// should get the data from database, compare the timestamp and send any
// 	// new data to the client
// 	// TODO: Complete this part

// 	res := &pb.DataResponse{
// 		Status:                "ok",
// 		Payload: ,
// 		ReceivedTimestamp:     strconv.Itoa(int(ReceivedTimestamp)),
// 		SentTimestamp:      strconv.Itoa(int(time.Now().UnixMilli())),
// 	}
// 	return res, nil
// }

// timestampIsNewer compares the given timestamp with the latest one in the database
// and returns true if the given timestamp is newer
func timestampIsNewer(db *sql.DB, sId string, timestamp time.Time) (bool, error) {
	var maxTime *time.Time
	row := db.QueryRow("SELECT MAX(timestamp) FROM air_quality WHERE idx = $1", sId)
	err := row.Scan(&maxTime)
	if err != nil && err != sql.ErrNoRows {
		return false, err
	}
	if maxTime == nil {
		return true, nil
	}
	if timestamp.After(*maxTime) {
		return true, nil
	}

	return false, nil
}

// SendDataToStorage receives data from the ingestion service and stores it in the database
func (s Server) SendDataToStorage(ctx context.Context, recData *pb.Data) (*pb.Ack, error) {
	receivedTime := time.Now()
	ReceivedTimestamp := receivedTime.UnixMilli()
	log.Printf("Received at [%s]: [%d]\n", receivedTime.Format("2006-01-02 15:04:05"), len(recData.Payload))

	go func(data string, db *sql.DB, m *metric.Metric, start time.Time) {
		aqData := &api.AirQualityData{}
		if err := json.Unmarshal([]byte(recData.Payload), &aqData); err != nil {
			log.Printf("Error unmarshalling JSON: %v", err)
			m.Failure("localStorage")
			return
		}
		// Insert data into the database
		if err := insertToAirQualityDb(db, *aqData); err != nil {
			log.Printf("Error inserting data into database: %v", err)
			m.Failure("localStorage")
			return
		}
		m.Sucess("localStorage")
		m.AddProcessingTime("localStorage", float64(time.Since(start).Milliseconds())/1000.0)
	}(recData.Payload, s.Db, s.Metric, receivedTime)

	ack := &pb.Ack{
		Status:                "ok",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     strconv.Itoa(int(ReceivedTimestamp)),
		AckSentTimestamp:      strconv.Itoa(int(time.Now().UnixMilli())),
	}
	return ack, nil
}

// stringToTime converts a string to time.Time
func stringToTime(dateStr string) (time.Time, error) {
	layout := "2006-01-02 15:04:05"
	return time.Parse(layout, dateStr)
}

func IsoToTime(dateStr string) (time.Time, error) {
	return time.Parse(time.RFC3339, dateStr)
}

func millisToTime(millis int64) time.Time {
	return time.Unix(0, millis*int64(time.Millisecond))
}

func insertToAirQualityDb(db *sql.DB, data api.AirQualityData) error {
	// Use a transaction for safety
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	for _, obs := range data.Obs {
		// If any error occurs, we return error and do not continue
		// with the rest of the observation
		if obs.Status != "ok" {
			log.Printf("Received status is not ok: %s", obs.Status)
			return fmt.Errorf("received status is not ok: %s", obs.Status)
		}

		tt, err := IsoToTime(obs.Msg.Time.ISO)
		if err != nil {
			log.Printf("Error parsing timestamp: %v", err)
			continue
		}

		newer, err := timestampIsNewer(db, fmt.Sprintf("%d", obs.Msg.Idx), tt)
		if err != nil {
			log.Printf("Error comparing timestamp: %v", err)
			return err
		}
		if !newer {
			log.Printf("Data is not newer than the latest record, skipping insertion")
			continue
		}

		// convert to JSON string
		fields, err := obs.ToMap()
		if err != nil {
			log.Printf("Error marshalling data: %v", err)
			return err
		}
		_, err = tx.Exec("INSERT INTO air_quality (aqi, idx, timestamp, attributions, city, dominentpol, forecast, iaqi, status) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
			fields["aqi"],
			fields["idx"],
			tt,
			fields["attributions"],
			fields["city"],
			fields["dominantpol"],
			fields["forecast"],
			fields["iaqi"],
			fields["status"],
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	log.Printf("Inserted [%d] items into the database.", len(data.Obs))
	return tx.Commit()
}
