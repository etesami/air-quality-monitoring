package api

import (
	"encoding/json"
)

type Attributions struct {
	URL  string `json:"url,omitempty"`
	Name string `json:"name,omitempty"`
	Logo string `json:"logo,omitempty"`
}

type City struct {
	Geo      []float64 `json:"geo,omitempty"`
	Name     string    `json:"name,omitempty"`
	URL      string    `json:"url,omitempty"`
	Location string    `json:"location,omitempty"`
}

type IAQI struct {
	H    Measurement `json:"h,omitempty"`
	P    Measurement `json:"p,omitempty"`
	PM25 Measurement `json:"pm25,omitempty"`
	T    Measurement `json:"t,omitempty"`
	W    Measurement `json:"w,omitempty"`
	WG   Measurement `json:"wg,omitempty"`
}

type Measurement struct {
	V float64 `json:"v,omitempty"`
}

type Time struct {
	S   string `json:"s,omitempty"`
	TZ  string `json:"tz,omitempty"`
	V   int64  `json:"v,omitempty"`
	ISO string `json:"iso,omitempty"`
}

type ForecastDaily struct {
	Avg float64 `json:"avg,omitempty"`
	Day string  `json:"day,omitempty"`
	Max float64 `json:"max,omitempty"`
	Min float64 `json:"min,omitempty"`
}

type Forecast struct {
	Daily struct {
		O3   []ForecastDaily `json:"o3,omitempty"`
		PM10 []ForecastDaily `json:"pm10,omitempty"`
		PM25 []ForecastDaily `json:"pm25,omitempty"`
		UVI  []ForecastDaily `json:"uvi,omitempty"`
	} `json:"daily,omitempty"`
}

type Msg struct {
	Aqi          int            `json:"aqi,omitempty"`
	Idx          int            `json:"idx,omitempty"`
	Attributions []Attributions `json:"attributions,omitempty"`
	City         City           `json:"city,omitempty"`
	DominentPol  string         `json:"dominentpol,omitempty"`
	IAQI         IAQI           `json:"iaqi,omitempty"`
	Time         Time           `json:"time,omitempty"`
	Forecast     Forecast       `json:"forecast,omitempty"`
}

type Observation struct {
	Msg    Msg    `json:"msg,omitempty"`
	Status string `json:"status,omitempty"`
	Cached string `json:"cached,omitempty"`
}

type AirQualityData struct {
	Obs    []Observation `json:"obs"`
	Status string        `json:"status"`
	Ver    string        `json:"ver,omitempty"`
}

func (obs *Observation) ToMap() (map[string]any, error) {
	fields := make(map[string]any)

	marshaled := []struct {
		key   string
		value any
	}{
		{"aqi", obs.Msg.Aqi},
		{"idx", obs.Msg.Idx},
		{"attributions", obs.Msg.Attributions},
		{"city", obs.Msg.City},
		{"dominantpol", obs.Msg.DominentPol},
		{"forecast", obs.Msg.Forecast},
		{"iaqi", obs.Msg.IAQI},
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
