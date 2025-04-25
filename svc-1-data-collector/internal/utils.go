package internal

import "crypto/sha256"

var coordinates = [][]float64{
	{37.652250, -122.597672, 37.866291, -122.282451}, // San Francisco
	{33.840044, -118.686028, 34.263986, -117.956711}, // Los Angeles
	{40.469107, -74.422686, 40.991107, -73.427786},   // New York
	{41.522733, -88.356541, 42.144733, -87.107221},   // Chicago
	{29.289900, -96.002831, 30.357700, -93.890142},   // Houston
	{33.114045, -112.780331, 33.925040, -111.469234}, // Phoenix
	{39.465041, -105.486744, 40.063636, -104.223479}, // Denver
	{47.363529, -122.627724, 47.862528, -122.056405}, // Seattle
	{38.693596, -77.296183, 39.093596, -76.732969},   // Washington D.C.
	{32.434528, -97.276841, 33.144529, -96.309461},   // Dallas
	{42.140944, -71.370327, 42.483944, -70.806920},   // Boston
	{36.002964, -87.048347, 36.338965, -86.442097},   // Nashville
	{39.793162, -83.434794, 40.177162, -82.547681},   // Columbus
	{34.982026, -81.253988, 35.442026, -80.402803},   // Charlotte
	{39.599835, -86.583233, 39.976685, -85.706766},   // Indianapolis
	{29.963960, -98.331537, 30.565560, -97.123495},   // Austin
	{42.882486, -88.250080, 43.282486, -87.684598},   // Milwaukee
	{44.820865, -93.508506, 45.120865, -92.937207},   // Minneapolis
	{25.627459, -80.407812, 25.937459, -80.030695},   // Miami
	{45.355639, -122.936983, 45.659639, -122.345014}, // Portland
	{40.299757, -80.260356, 40.591757, -79.701992},   // Pittsburgh
	{38.042497, -86.127427, 38.443622, -85.335814},   // Louisville
	{-80.024308, 43.521753, -79.082231, 44.289536},   // Toronto
	{-74.608909, 45.047141, -72.439109, 46.110189},   // Montreal
	{43.700110, -79.416300, 44.000110, -78.500000},   // Ottawa
}

// getRandomBoxCoordination returns a random box coordinates based on the hash of the identifier
func GetRandomBoxCoordination(identifier string) []float64 {
	// Random box coordinates based on the hash of the identifier

	// get the hash of the identifier
	hash := sha256.Sum256([]byte(identifier))

	// Use the first byte of the hash to select a coordinate
	index := int(hash[0]) % len(coordinates)

	return coordinates[index]
}
