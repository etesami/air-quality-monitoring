package aggregatedstorage

import (
	dpapi "github.com/etesami/air-quality-monitoring/api/data-processing"
)

type EnhancedResponse struct {
	City           dpapi.City             `json:"city,omitempty"`
	AirQualityData []dpapi.AirQualityData `json:"airQualityData,omitempty"`
	Alert          []dpapi.Alert          `json:"alert,omitempty"`
}
