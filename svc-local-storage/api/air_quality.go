package api

import (
	"time"

	api "github.com/etesami/air-quality-monitoring/api"
)

type DataType string

const (
	RequestPoints DataType = "points"
)

type DataRequest struct {
	StartTime   string   `json:"startTime,omitempty"`
	EndTime     string   `json:"endTime,omitempty"`
	LAT         float64  `json:"lat,omitempty"`
	LNG         float64  `json:"lng,omitempty"`
	RequestType DataType `json:"requestType,omitempty"`
}

type DataResponse struct {
	ID           int                `json:"id,omitempty"`
	Aqi          int                `json:"aqi,omitempty"`
	Idx          int                `json:"idx,omitempty"`
	Attributions []api.Attributions `json:"attributions,omitempty"`
	City         api.City           `json:"city,omitempty"`
	DominentPol  string             `json:"dominantpol,omitempty"`
	IAQI         api.IAQI           `json:"iaqi,omitempty"`
	Timestamp    time.Time          `json:"timestamp,omitempty"`
	Forecast     api.Forecast       `json:"forecast,omitempty"`
	Status       string             `json:"status,omitempty"`
}
