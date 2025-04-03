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

	"github.com/etesami/air-quality-monitoring/pkg/metric"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
	_ "github.com/mattn/go-sqlite3"
)

type Server struct {
	pb.UnimplementedAirQualityMonitoringServer
	Metric *metric.Metric
	Db     *sql.DB
}

type AlertRaw struct {
	AreaDesc    string `json:"areaDesc,omitempty"`
	Sent        string `json:"sent,omitempty"`
	Effective   string `json:"effective,omitempty"`
	Expires     string `json:"expires,omitempty"`
	Ends        string `json:"ends,omitempty"`
	Status      string `json:"status,omitempty"`
	Certainty   string `json:"certainty,omitempty"`
	Urgency     string `json:"urgency,omitempty"`
	Event       string `json:"event,omitempty"`
	Headline    string `json:"headline,omitempty"`
	Description string `json:"description,omitempty"`
	Instruction string `json:"instruction,omitempty"`
	Severity    string `json:"severity,omitempty"`
}

type Record struct {
	City           City           `json:"city,omitempty"`
	AirQualityData AirQualityData `json:"airQualityData,omitempty"`
	Alert          *Alert1        `json:"alert,omitempty"`
}

type City struct {
	Idx      int64   `json:"idx,omitempty"`
	CityName string  `json:"cityName,omitempty"`
	Lat      float64 `json:"lat,omitempty"`
	Lng      float64 `json:"lng,omitempty"`
}

type Alert1 struct {
	AlertDesc        string `json:"alertDesc,omitempty"`
	AlertEffective   string `json:"alertEffective,omitempty"`
	AlertExpires     string `json:"alertExpires,omitempty"`
	AlertStatus      string `json:"alertStatus,omitempty"`
	AlertCertainty   string `json:"alertCertainty,omitempty"`
	AlertUrgency     string `json:"alertUrgency,omitempty"`
	AlertSeverity    string `json:"alertSeverity,omitempty"`
	AlertHeadline    string `json:"alertHeadline,omitempty"`
	AlertDescription string `json:"alertDescription,omitempty"`
	AlertEvent       string `json:"alertEvent,omitempty"`
}

// This is a modifed version of the original msg struct
// with fewer fields
type AirQualityData struct {
	Timestamp   string `json:"timestamp,omitempty"`
	Aqi         int64  `json:"aqi,omitempty"`
	DewPoint    int64  `json:"dewPoint,omitempty"`
	Humidity    int64  `json:"humidity,omitempty"`
	Pressure    int64  `json:"pressure,omitempty"`
	Temperature int64  `json:"temperature,omitempty"`
	WindSpeed   int64  `json:"windSpeed,omitempty"`
	WindGust    int64  `json:"windGust,omitempty"`
	PM25        int64  `json:"pm25,omitempty"`
	PM10        int64  `json:"pm10,omitempty"`
}

func insertToDb(db *sql.DB, data []Record) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	for _, record := range data {

		_, err = tx.Exec("INSERT INTO city (idx, cityName, lat, lng) VALUES ($1, $2, $3, $4)",
			record.City.Idx,
			record.City.CityName,
			record.City.Lat,
			record.City.Lng,
		)

		_, err = tx.Exec("INSERT INTO air_quality (aqi, timestamp, dewPoint, humidity, pressure, temperature, windSpeed, windGust, pm25, city_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)",
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

		if record.Alert != nil {
			hash, err := generateHash(*record.Alert)
			if err != nil {
				fmt.Printf("Error generating hash: %v\n", err)
				tx.Rollback()
			}
			_, err = tx.Exec("INSERT INTO alert (hash, alertDesc, alertEffective, alertExpires, alertStatus, alertCertainty, alertUrgency, alertSeverity, alertHeadline, alertDescription, alertEvent, city_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)",
				hash,
				record.Alert.AlertDesc,
				record.Alert.AlertEffective,
				record.Alert.AlertExpires,
				record.Alert.AlertStatus,
				record.Alert.AlertCertainty,
				record.Alert.AlertUrgency,
				record.Alert.AlertSeverity,
				record.Alert.AlertHeadline,
				record.Alert.AlertDescription,
				record.Alert.AlertEvent,
				record.City.Idx,
			)
		}

		if err != nil {
			tx.Rollback()
			return err
		}
	}

	log.Printf("Inserted [%d] items into the database.", len(data))
	return tx.Commit()
}

func generateHash(data Alert1) (string, error) {
	byteAltert, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshalling JSON: %v", err)
		return "", err
	}
	hash := sha256.Sum256([]byte(byteAltert))
	return hex.EncodeToString(hash[:]), nil
}

func (s Server) SendToAggregatedStorage(ctx context.Context, recData *pb.Data) (*pb.Ack, error) {
	receivedTime := time.Now()
	ReceivedTimestamp := receivedTime.UnixMilli()
	log.Printf("Received at [%s]: [%d]\n", receivedTime.Format("2006-01-02 15:04:05"), len(recData.Payload))

	go func(data string, db *sql.DB, m *metric.Metric, start time.Time) {
		aqData := []Record{}
		if err := json.Unmarshal([]byte(recData.Payload), &aqData); err != nil {
			log.Printf("Error unmarshalling JSON: %v", err)
			m.Failure("localStorage")
			return
		}

		// Insert data into the database
		if err := insertToDb(db, aqData); err != nil {
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
