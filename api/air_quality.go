package api

import (
	"encoding/json"
	"fmt"
)

type Attributions struct {
	URL  string `json:"url"`
	Name string `json:"name"`
	Logo string `json:"logo,omitempty"`
}

type City struct {
	Geo      []float64 `json:"geo"`
	Name     string    `json:"name"`
	URL      string    `json:"url"`
	Location string    `json:"location,omitempty"`
}

type IAQI struct {
	H    Measurement `json:"h"`
	P    Measurement `json:"p"`
	PM25 Measurement `json:"pm25"`
	T    Measurement `json:"t"`
	W    Measurement `json:"w"`
	WG   Measurement `json:"wg"`
}

type Measurement struct {
	V float64 `json:"v"`
}

type Time struct {
	S   string `json:"s"`
	TZ  string `json:"tz"`
	V   int64  `json:"v"`
	ISO string `json:"iso"`
}

type ForecastDaily struct {
	Avg float64 `json:"avg"`
	Day string  `json:"day"`
	Max float64 `json:"max"`
	Min float64 `json:"min"`
}

type Forecast struct {
	Daily struct {
		O3   []ForecastDaily `json:"o3,omitempty"`
		PM10 []ForecastDaily `json:"pm10,omitempty"`
		PM25 []ForecastDaily `json:"pm25,omitempty"`
		UVI  []ForecastDaily `json:"uvi,omitempty"`
	} `json:"daily"`
}

type Msg struct {
	Aqi          int            `json:"aqi"`
	Idx          int            `json:"idx"`
	Attributions []Attributions `json:"attributions"`
	City         City           `json:"city"`
	DominentPol  string         `json:"dominentpol"`
	IAQI         IAQI           `json:"iaqi"`
	Time         Time           `json:"time"`
	Forecast     Forecast       `json:"forecast"`
}

type Observation struct {
	Msg   Msg `json:"msg"`
	Debug struct {
		Sync string `json:"sync"`
	} `json:"debug"`
	Status string `json:"status"`
	Cached string `json:"cached"`
}

type AirQualityData struct {
	Obs    []Observation `json:"obs"`
	Status string        `json:"status"`
	Ver    string        `json:"ver"`
}

func (obs *Observation) ToMap() (map[string]any, error) {
	fields := make(map[string]any)

	marshaled := []struct {
		key   string
		value any
	}{
		{"aqi", obs.Msg.Aqi},
		{"attributions", obs.Msg.Attributions},
		{"city", obs.Msg.City},
		{"debug", obs.Debug},
		{"dominantpol", obs.Msg.DominentPol},
		{"forecast", obs.Msg.Forecast},
		{"iaqi", obs.Msg.IAQI},
		{"idx", fmt.Sprintf("%d", obs.Msg.Idx)},
		{"time", obs.Msg.Time},
		{"status", obs.Status},
	}

	for _, item := range marshaled {
		if jsonData, err := json.Marshal(item.value); err != nil {
			return nil, err
		} else {
			fields[item.key] = string(jsonData)
		}
	}

	return fields, nil
}
