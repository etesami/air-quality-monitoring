package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	api "github.com/etesami/air-quality-monitoring/api"
	localapi "github.com/etesami/air-quality-monitoring/api/local-storage"
	"github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
	_ "github.com/mattn/go-sqlite3"
)

type Server struct {
	pb.UnimplementedAirQualityMonitoringServer
	Metric *metric.Metric
	Db     *sql.DB
}

// ReceiveDataFromLocalStorage receive data request from the processing service
// and send the data that are newer than the timestamp in the request.
// If there is no timestamp in the request, it will send all available data
func (s Server) ReceiveDataFromLocalStorage(ctx context.Context, req *pb.Data) (*pb.DataResponse, error) {
	receivedTime := time.Now()
	ReceivedTimestamp := receivedTime.UnixMilli()
	log.Printf("Received request for data: [%d]\n", len(req.Payload))

	var dataRequest localapi.DataRequest
	if err := json.Unmarshal([]byte(req.Payload), &dataRequest); err != nil {
		log.Printf("Error unmarshalling JSON: %v", err)
		s.Metric.Failure("localStorage")
		return nil, err
	}

	dataToBeSent, err := requestDataFromDb(s.Db, &dataRequest)
	if err != nil {
		log.Printf("Error requesting data: %v", err)
		s.Metric.Failure("localStorage")
		return nil, err
	}
	if dataToBeSent == "" {
		log.Printf("No data to be sent")
		return &pb.DataResponse{
			Status:            "no_data",
			Payload:           "",
			ReceivedTimestamp: fmt.Sprintf("%d", int(ReceivedTimestamp)),
			SentTimestamp:     fmt.Sprintf("%d", int(time.Now().UnixMilli())),
		}, nil
	}
	s.Metric.Sucess("localStorage")
	s.Metric.AddProcessingTime("localStorage", float64(time.Since(receivedTime).Milliseconds())/1000.0)

	res := &pb.DataResponse{
		Status:            "ok",
		Payload:           dataToBeSent,
		ReceivedTimestamp: fmt.Sprintf("%d", int(ReceivedTimestamp)),
		SentTimestamp:     fmt.Sprintf("%d", int(time.Now().UnixMilli())),
	}
	return res, nil
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
		ReceivedTimestamp:     fmt.Sprintf("%d", int(ReceivedTimestamp)),
		AckSentTimestamp:      fmt.Sprintf("%d", int(time.Now().UnixMilli())),
	}
	return ack, nil
}

// requestDataFromDb queries the database for data after the given timestamp
func requestDataFromDb(db *sql.DB, dataRequest *localapi.DataRequest) (string, error) {

	msgList := make([]localapi.DataResponse, 0)

	if dataRequest.StartTime != "" && dataRequest.EndTime != "" {

		ttStart, err1 := time.Parse(time.RFC3339, dataRequest.StartTime)
		ttEnd, err2 := time.Parse(time.RFC3339, dataRequest.EndTime)
		if err1 != nil || err2 != nil {
			log.Printf("Error parsing timestamp: %v", fmt.Errorf("%v, %v", err1, err2))
			return "", fmt.Errorf("%v, %v", err1, err2)
		}

		rows, err := db.Query("SELECT * FROM air_quality WHERE timestamp > $1 AND timestamp < $2", ttStart, ttEnd)
		if err != nil {
			return "", err
		}
		defer rows.Close()

		for rows.Next() {
			var msg localapi.DataResponse
			var atr string
			var city string
			var forecast string
			var iaqi string
			if err := rows.Scan(
				&msg.ID, &msg.Aqi, &msg.Idx, &msg.Timestamp,
				&atr, &city, &msg.DominentPol,
				&forecast, &iaqi, &msg.Status); err != nil {
				log.Printf("Error scanning row: %v", err)
				continue
			}
			if err := json.Unmarshal([]byte(atr), &msg.Attributions); err != nil {
				log.Printf("Error unmarshalling attributions: %v", err)
				continue
			}
			if err := json.Unmarshal([]byte(city), &msg.City); err != nil {
				log.Printf("Error unmarshalling city: %v", err)
				continue
			}
			if err := json.Unmarshal([]byte(forecast), &msg.Forecast); err != nil {
				log.Printf("Error unmarshalling forecast: %v", err)
				continue
			}
			if err := json.Unmarshal([]byte(iaqi), &msg.IAQI); err != nil {
				log.Printf("Error unmarshalling iaqi: %v", err)
				continue
			}
			msgList = append(msgList, msg)
		}

		dataList := make([]api.Msg, 0)
		for _, msg := range msgList {
			dataList = append(dataList, api.Msg{
				Aqi:          msg.Aqi,
				Idx:          msg.Idx,
				Attributions: msg.Attributions,
				City:         msg.City,
				DominentPol:  msg.DominentPol,
				IAQI:         msg.IAQI,
				Time:         api.Time{ISO: msg.Timestamp.Format(time.RFC3339)},
				Forecast:     msg.Forecast,
			})
		}
		log.Printf("Found [%d] items in the database for req.\n", len(dataList))

		jsonResponse, err := json.Marshal(dataList)
		if err != nil {
			log.Printf("Error marshalling data: %v", err)
			return "", err
		}
		return string(jsonResponse), nil
	}

	if dataRequest.RequestType != "" {
		// TODO: handle based on request type
	}
	return "", nil
}

// timestampIsNewer compares the given timestamp with the latest one in the database
// and returns true if the given timestamp is newer
func timestampIsNewer(db *sql.DB, sId string, timestamp time.Time) (bool, error) {
	var maxTimeStr sql.NullString

	row := db.QueryRow("SELECT MAX(timestamp) FROM air_quality WHERE idx = $1", sId)
	err := row.Scan(&maxTimeStr)
	if err != nil && err != sql.ErrNoRows {
		return false, err
	}
	if !maxTimeStr.Valid {
		return true, nil
	}

	maxTime, err := time.Parse("2006-01-02 15:04:05-07:00", maxTimeStr.String) // Adjust format if necessary
	if err != nil {
		return false, fmt.Errorf("error parsing max timestamp: %v", err)
	}

	if timestamp.After(maxTime) {
		return true, nil
	}

	return false, nil
}

func printObject(obj api.Observation) {
	// Convert the object to JSON
	objByte, err := json.Marshal(obj)
	if err != nil {
		log.Printf("Error marshalling object for printing: %v", err)
		return
	}
	log.Printf("Object: %s\n", string(objByte))
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
			printObject(obs)
			continue
		}

		tt, err := time.Parse(time.RFC3339, obs.Msg.Time.ISO)
		if err != nil {
			log.Printf("Error parsing timestamp: %v", err)
			printObject(obs)
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
		log.Printf("Inserted data into the database: [%s]", obs.Msg.Time.ISO)
	}

	log.Printf("Inserted [%d] items in total.", len(data.Obs))
	return tx.Commit()
}
