package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	api "github.com/etesami/air-quality-monitoring/api"
	"github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
	utils "github.com/etesami/air-quality-monitoring/pkg/utils"
	"google.golang.org/protobuf/proto"

	localapi "github.com/etesami/air-quality-monitoring/api/local-storage"
	_ "github.com/mattn/go-sqlite3"
)

type Server struct {
	pb.UnimplementedAirQualityMonitoringServer
	Metric *metric.Metric
	Db     *sql.DB
}

// CheckConnection is a simple ping-pong method to respond for the health check
func (s Server) CheckConnection(ctx context.Context, recData *pb.Data) (*pb.Ack, error) {
	t := time.Now()
	ack := &pb.Ack{
		Status:                "pong",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     fmt.Sprintf("%d", int(t.UnixMilli())),
		AckSentTimestamp:      fmt.Sprintf("%d", int(t.UnixMilli())),
	}
	return ack, nil
}

// SendDataToStorage receives data from the ingestion service and stores it in the database
func (s Server) SendDataToServer(ctx context.Context, recData *pb.Data) (*pb.Ack, error) {
	st := time.Now()
	recTimestamp := st.UnixMilli()
	log.Printf("Received at [%s]: [%d]\n", st.Format("2006-01-02 15:04:05"), len(recData.Payload))

	go func(data string, db *sql.DB, m *metric.Metric, start time.Time) {
		aqData := &api.AirQualityData{}
		if err := json.Unmarshal([]byte(recData.Payload), &aqData); err != nil {
			log.Printf("Error unmarshalling JSON: %v", err)
			return
		}
		// Insert data into the database
		if err := insertToAirQualityDb(db, *aqData); err != nil {
			log.Printf("Error inserting data into database: %v", err)
			return
		}
		m.AddProcessingTime("processing", float64(time.Since(start).Milliseconds())/1000.0)
	}(recData.Payload, s.Db, s.Metric, st)

	ack := &pb.Ack{
		Status:                "ok",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     fmt.Sprintf("%d", int(recTimestamp)),
		AckSentTimestamp:      fmt.Sprintf("%d", int(time.Now().UnixMilli())),
	}
	return ack, nil
}

// requestDataFromDb fetches data from the database after the given timestamp
func requestDataFromDb(db *sql.DB, t time.Time) ([]api.Msg, error) {
	msgList := make([]localapi.DataResponse, 0)

	log.Printf("Requesting data from the database after [%s]\n", t.Format("2006-01-02 15:04:05"))

	rows, err := db.Query("SELECT * FROM air_quality WHERE timestamp > $1", t)
	if err != nil {
		return nil, err
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

	return dataList, nil
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

	count := 0
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
			// log.Printf("Data is not newer than the latest record, skipping insertion")
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
		count++
		log.Printf("Inserted data into the database: [%s]", obs.Msg.Time.ISO)
	}

	if count > 0 {
		log.Printf("Inserted [%d]/[%d] items in total.", count, len(data.Obs))
	}
	return tx.Commit()
}

// processTicker processes the ticker event
func ProcessTicker(ctx context.Context, client *pb.AirQualityMonitoringClient, db *sql.DB, serverName string, metricList *metric.Metric) error {
	if *client == nil {
		log.Printf("Client is not ready yet")
		return nil
	}
	go func(m *metric.Metric) {
		ping := &pb.Data{
			Payload:       "ping",
			SentTimestamp: fmt.Sprintf("%d", int(time.Now().UnixMilli())),
		}
		pong, err := (*client).CheckConnection(context.Background(), ping)
		if err != nil {
			log.Printf("Error checking connection: %v", err)
			return
		}
		rtt, err := utils.CalculateRtt(ping.SentTimestamp, pong.ReceivedTimestamp, pong.AckSentTimestamp, time.Now())
		if err != nil {
			log.Printf("Error calculating RTT: %v", err)
			return
		}
		m.AddRttTime(serverName, float64(rtt)/1000.0)
		log.Printf("RTT to [%s] service: [%.2f] ms\n", serverName, float64(rtt)/1000.0)
	}(metricList)

	// Check if there is any data to be sent
	// If there is no lastCallTime, we fetch all data for the last 24 hours
	st := time.Now()
	var lastCallTime time.Time
	if t := ctx.Value("lastCallTime"); t != nil {
		if v, ok := t.(time.Time); ok {
			lastCallTime = v
		} else {
			return fmt.Errorf("error converting lastCallTime to time.Time")
		}
	} else {
		lastCallTime = time.Now().Add(-24 * time.Hour)
	}

	dataToBeSent, err := requestDataFromDb(db, lastCallTime)
	if err != nil {
		log.Printf("Error requesting new data after [%s]: %v", lastCallTime.String(), err)
		return err
	}

	dataToBeSentByte, err := json.Marshal(dataToBeSent)
	if err != nil {
		return fmt.Errorf("Error marshalling data: %v", err)
	}

	if len(dataToBeSent) > 0 {
		res := &pb.Data{
			Payload:       string(dataToBeSentByte),
			SentTimestamp: fmt.Sprintf("%d", int(time.Now().UnixMilli())),
		}
		metricList.AddProcessingTime("processing", float64(time.Since(st).Milliseconds())/1000.0)

		sentBytes := proto.Size(res)
		_, err := (*client).SendDataToServer(context.Background(), res)
		if err != nil {
			return fmt.Errorf("Error sending data to server: %v", err)
		} else {
			metricList.AddSentDataBytes("processor", float64(sentBytes))
		}
	}

	return nil
}
