package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"strconv"
	"time"

	api "github.com/etesami/air-quality-monitoring/api"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
	_ "github.com/mattn/go-sqlite3"
)

type Server struct {
	pb.UnimplementedAirQualityMonitoringServer
	Db *sql.DB
}

func (s Server) SendDataToStorage(ctx context.Context, recData *pb.Data) (*pb.Ack, error) {
	receivedTime := time.Now()
	ReceivedTimestamp := receivedTime.UnixMilli()
	log.Printf("Received at [%s]: [%d]\n", receivedTime.Format("2006-01-02 15:04:05"), len(recData.Payload))

	go func() {
		aqData := &api.AirQualityData{}
		if err := json.Unmarshal([]byte(recData.Payload), &aqData); err != nil {
			log.Printf("Error unmarshalling JSON: %v", err)
		}
		// Insert data into the database
		if err := insertAirQuData(s.Db, *aqData); err != nil {
			log.Printf("Error inserting data into database: %v", err)
		}
		log.Printf("Data inserted into database successfully\n")
	}()

	ack := &pb.Ack{
		Status:                "ok",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     strconv.Itoa(int(ReceivedTimestamp)),
		AckSentTimestamp:      strconv.Itoa(int(time.Now().UnixMilli())),
	}
	return ack, nil
}

func insertAirQuData(db *sql.DB, data api.AirQualityData) error {
	// Use a transaction for safety
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	for _, obs := range data.Obs {
		if obs.Status != "ok" {
			log.Printf("Received status is not ok: %s", obs.Status)
			continue
		}

		_, err := tx.Exec("INSERT INTO air_quality (aqi, attributions, city, debug, dominentpol, forecast, iaqi, idx, time, status) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)",
			obs.Msg.Aqi,
			obs.Msg.Attributions,
			obs.Msg.City,
			obs.Debug.Sync,
			obs.Msg.DominentPol,
			obs.Msg.Forecast,
			obs.Msg.IAQI,
			obs.Msg.Idx,
			obs.Msg.Time,
			obs.Status,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}
