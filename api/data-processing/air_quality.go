package dataprocessing

type EnhancedDataResponse struct {
	City           `json:"city,omitempty"`
	AirQualityData `json:"airQualityData,omitempty"`
	Alert          `json:"alert,omitempty"`
}

type City struct {
	Idx      int64   `json:"idx,omitempty"`
	CityName string  `json:"cityName,omitempty"`
	Lat      float64 `json:"lat,omitempty"`
	Lng      float64 `json:"lng,omitempty"`
}

type Alert struct {
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
