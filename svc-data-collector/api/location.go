package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type LocationData struct {
	Lat1  float64 `json:"lat1"`
	Lng1  float64 `json:"lng1"`
	Lat2  float64 `json:"lat2"`
	Lng2  float64 `json:"lng2"`
	Token string  `json:"token"`
}

// fetchData fetches data from the API and returns it as a map
// containing the list of locations within the specified bounds
func (l *LocationData) CollectLocationsIds() (map[string]any, error) {
	url := fmt.Sprintf(
		"https://api.waqi.info/v2/map/bounds?latlng=%f,%f,%f,%f&token=%s",
		l.Lat1, l.Lng1, l.Lat2, l.Lng2, l.Token)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch data: %d", resp.StatusCode)
	}

	var res map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("error Decoding JSON: %v", err)
	}
	return res, nil
}
