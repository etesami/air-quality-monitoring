package internal

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	agapi "github.com/etesami/air-quality-monitoring/api/aggregated-storage"
	dpapi "github.com/etesami/air-quality-monitoring/api/data-processing"
	loapi "github.com/etesami/air-quality-monitoring/api/local-storage"

	"github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
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

func (s Server) ReceiveDataFromServer(ctx context.Context, req *pb.Data) (*pb.DataResponse, error) {
	recTime := time.Now()
	recTimestamp := recTime.UnixMilli()
	log.Printf("Received request for data: [%d]\n", len(req.Payload))

	var dataRequest loapi.DataRequest
	if err := json.Unmarshal([]byte(req.Payload), &dataRequest); err != nil {
		return nil, fmt.Errorf("error unmarshalling JSON: %v", err)
	}

	dataToBeSent, err := requestDataFromDb(s.Db, &dataRequest)
	if err != nil {
		return nil, fmt.Errorf("error requesting data: %v", err)
	}
	s.Metric.AddProcessingTime("processing", float64(time.Since(recTime).Milliseconds())/1000.0)

	if dataToBeSent == "" {
		log.Printf("No data to be sent")
		return &pb.DataResponse{
			Status:            "no_data_available",
			Payload:           "",
			ReceivedTimestamp: fmt.Sprintf("%d", int(recTimestamp)),
			SentTimestamp:     fmt.Sprintf("%d", int(time.Now().UnixMilli())),
		}, nil
	}

	res := &pb.DataResponse{
		Status:            "ok",
		Payload:           dataToBeSent,
		ReceivedTimestamp: fmt.Sprintf("%d", int(recTimestamp)),
		SentTimestamp:     fmt.Sprintf("%d", int(time.Now().UnixMilli())),
	}
	return res, nil
}

func (s Server) SendDataToServer(ctx context.Context, recData *pb.Data) (*pb.Ack, error) {
	recTime := time.Now()
	recTimestamp := recTime.UnixMilli()
	log.Printf("Received at [%s]: [%d]\n", recTime.Format("2006-01-02 15:04:05"), len(recData.Payload))

	go func(data string, db *sql.DB, m *metric.Metric, start time.Time) {
		aqData := []dpapi.EnhancedDataResponse{}
		if err := json.Unmarshal([]byte(recData.Payload), &aqData); err != nil {
			log.Printf("Error unmarshalling JSON: %v", err)
			return
		}

		// Insert data into the database
		if err := insertToDb(db, aqData); err != nil {
			log.Printf("Error inserting data into database: %v", err)
			return
		}
		m.AddProcessingTime("processing", float64(time.Since(start).Milliseconds())/1000.0)
	}(recData.Payload, s.Db, s.Metric, recTime)

	ack := &pb.Ack{
		Status:                "ok",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     fmt.Sprintf("%d", int(recTimestamp)),
		AckSentTimestamp:      fmt.Sprintf("%d", int(time.Now().UnixMilli())),
	}
	return ack, nil
}

func insertToDb(db *sql.DB, data []dpapi.EnhancedDataResponse) error {
	count := 0
	for _, record := range data {
		tx, err := db.Begin()
		if err != nil {
			return err
		}

		_, err = tx.Exec("INSERT INTO city (idx, cityName, lat, lng) VALUES ($1, $2, $3, $4)",
			record.City.Idx,
			record.City.CityName,
			record.City.Lat,
			record.City.Lng,
		)
		if err != nil && err.Error() != "UNIQUE constraint failed: city.idx" {
			log.Printf("Error inserting city: %v\n", err)
			tx.Rollback()
			continue
		}

		hash, err := generateHash(map[string]any{
			"city_id":   record.City.Idx,
			"timestamp": record.AirQualityData.Timestamp,
		})
		if err != nil {
			fmt.Printf("Error generating hash: %v\n", err)
			tx.Rollback()
			continue
		}

		_, err = tx.Exec("INSERT INTO air_quality (hash, aqi, timestamp, dewPoint, humidity, pressure, temperature, windSpeed, windGust, pm25, city_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)",
			hash,
			record.AirQualityData.Aqi,
			record.AirQualityData.Timestamp,
			record.AirQualityData.DewPoint,
			record.AirQualityData.Humidity,
			record.AirQualityData.Pressure,
			record.AirQualityData.Temperature,
			record.AirQualityData.WindSpeed,
			record.AirQualityData.WindGust,
			record.AirQualityData.PM25,
			record.City.Idx,
		)
		if err != nil {
			if err.Error() != "UNIQUE constraint failed: air_quality.hash" {
				log.Printf("Error inserting air quality data: %v\n", err)
			}
			tx.Rollback()
			continue
		}

		if record.Alert != nil {

			log.Printf("Alert: %v\n", record.Alert)
			hash, err := generateHash(*record.Alert)
			if err != nil {
				tx.Rollback()
				continue
			}
			effective, err1 := time.Parse(time.RFC3339, record.Alert.AlertEffective)
			expires, err2 := time.Parse(time.RFC3339, record.Alert.AlertExpires)
			if err1 != nil || err2 != nil {
				fmt.Printf("error parsing timestamp: %v", fmt.Errorf("%v, %v", err1, err2))
				tx.Rollback()
				continue
			}

			_, err = tx.Exec("INSERT INTO alert (hash, alertDesc, alertEffective, alertExpires, alertStatus, alertCertainty, alertUrgency, alertSeverity, alertHeadline, alertDescription, alertEvent, city_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)",
				hash,
				record.Alert.AlertDesc,
				effective,
				expires,
				record.Alert.AlertStatus,
				record.Alert.AlertCertainty,
				record.Alert.AlertUrgency,
				record.Alert.AlertSeverity,
				record.Alert.AlertHeadline,
				record.Alert.AlertDescription,
				record.Alert.AlertEvent,
				record.City.Idx,
			)
			if err != nil && err.Error() != "UNIQUE constraint failed: alert.hash" {
				log.Printf("Error inserting alert data: %v\n", err)
				tx.Rollback()
				continue
			}
		}

		if err := tx.Commit(); err != nil {
			log.Printf("Error committing transaction: %v\n", err)
			return err
		}
		count++
	}

	log.Printf("Inserted [%d]/[%d] items into the database.", count, len(data))
	return nil
}

func generateHash(data any) (string, error) {
	byteAltert, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshalling JSON: %v", err)
		return "", err
	}
	hash := sha256.Sum256([]byte(byteAltert))
	return hex.EncodeToString(hash[:]), nil
}

func requestDataFromDb(db *sql.DB, dataRequest *loapi.DataRequest) (string, error) {
	// If Request Type is set it will be used
	// - points: get all city data
	// - alerts: get all alert data
	// - airQuality: get all air quality data
	// - all: get all data

	// Else points and times are required
	// returns city data along with all air quality data and alerts in the given time range

	if dataRequest.RequestType == loapi.RequestPoints {
		rows, err := db.Query("SELECT * FROM city")
		if err != nil {
			return "", err
		}
		defer rows.Close()

		var cityData []dpapi.City
		for rows.Next() {
			var city dpapi.City
			if err := rows.Scan(&city.Idx, &city.CityName, &city.Lat, &city.Lng); err != nil {
				log.Printf("Error scanning row: %v", err)
				return "", err
			}
			cityData = append(cityData, city)
		}

		if err := rows.Err(); err != nil {
			log.Printf("Error iterating rows: %v", err)
			return "", err
		}

		// We should construct agapi.EnhancedResponse object
		resData := make([]agapi.EnhancedResponse, 0)
		for _, city := range cityData {
			response := agapi.EnhancedResponse{
				City: city,
			}
			resData = append(resData, response)
		}

		resDataByte, err := json.Marshal(resData)
		if err != nil {
			log.Printf("Error marshalling JSON: %v", err)
			return "", err
		}
		return string(resDataByte), nil
	}

	// If request type is not set, we need to check if start and end time are set
	if dataRequest.StartTime == "" || dataRequest.EndTime == "" || dataRequest.LAT == 0 || dataRequest.LNG == 0 {
		log.Printf("Error: Start and end time are required")
		return "", fmt.Errorf("start and end time and coordinates are required")
	}

	ttStart, err1 := time.Parse(time.RFC3339, dataRequest.StartTime)
	ttEnd, err2 := time.Parse(time.RFC3339, dataRequest.EndTime)
	if err1 != nil || err2 != nil {
		log.Printf("Error parsing timestamp: %v", fmt.Errorf("%v, %v", err1, err2))
		return "", fmt.Errorf("%v, %v", err1, err2)
	}
	rows, err := db.Query("SELECT idx, cityName FROM city WHERE lat = ? AND lng = ?", dataRequest.LAT, dataRequest.LNG)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var cityData []dpapi.City
	for rows.Next() {
		var city dpapi.City
		if err := rows.Scan(&city.Idx, &city.CityName); err != nil {
			log.Printf("Error scanning row: %v", err)
			return "", err
		}
		cityData = append(cityData, city)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Error iterating rows: %v", err)
		return "", err
	}
	if len(cityData) == 0 {
		log.Printf("No data found for the given coordinates")
		return "", fmt.Errorf("no data found for the given coordinates")
	}
	var cityIdx int64

	allResponses := make([]agapi.EnhancedResponse, 0)

	for _, city := range cityData {

		cityIdx = city.Idx
		rows, err := db.Query("SELECT * FROM air_quality WHERE timestamp > ? AND timestamp < ? AND city_id = ?", ttStart, ttEnd, cityIdx)
		if err != nil {
			return "", err
		}
		defer rows.Close()

		var msgList []dpapi.AirQualityData
		for rows.Next() {
			var msg dpapi.AirQualityData
			if err := rows.Scan(&msg.Aqi, &msg.Timestamp, &msg.DewPoint, &msg.Humidity, &msg.Pressure, &msg.Temperature, &msg.WindSpeed, &msg.WindGust, &msg.PM25); err != nil {
				log.Printf("Error scanning row: %v", err)
				continue
			}
			msgList = append(msgList, msg)
		}

		if err := rows.Err(); err != nil {
			log.Printf("Error iterating rows: %v", err)
			return "", err
		}

		// get alerts
		rows, err = db.Query("SELECT * FROM alert WHERE city_id = ? AND alertEffective > ? AND alertExpires < ?", cityIdx, ttStart, ttEnd)
		if err != nil {
			return "", err
		}
		defer rows.Close()

		var alertList []dpapi.Alert
		for rows.Next() {
			var alert dpapi.Alert
			if err := rows.Scan(&alert.AlertDesc, &alert.AlertEffective, &alert.AlertExpires, &alert.AlertStatus, &alert.AlertCertainty, &alert.AlertUrgency, &alert.AlertSeverity, &alert.AlertHeadline, &alert.AlertDescription, &alert.AlertEvent); err != nil {
				log.Printf("Error scanning row: %v", err)
				continue
			}
			alertList = append(alertList, alert)
		}

		if err := rows.Err(); err != nil {
			log.Printf("Error iterating rows: %v", err)
			return "", err
		}

		response := agapi.EnhancedResponse{
			City:           city,
			AirQualityData: msgList,
			Alert:          alertList,
		}
		allResponses = append(allResponses, response)
	}

	allDataJson, err := json.Marshal(allResponses)
	if err != nil {
		log.Printf("Error marshalling JSON: %v", err)
		return "", err
	}

	return string(allDataJson), nil
}
