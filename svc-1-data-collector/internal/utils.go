package internal

import "crypto/sha256"

var coordinates = [][]float64{
	{37.706749, -122.523489, 37.811792, -122.356634}, // San Francisco
	{33.941589, -118.521389, 34.161441, -118.121350}, // Los Angeles
	{40.578000, -74.150200, 40.882214, -73.700272},   // New York
	{41.644335, -87.940101, 42.023131, -87.523661},   // Chicago
	{29.537200, -95.823268, 30.110400, -95.069705},   // Houston
	{33.290260, -112.324287, 33.748825, -111.925278}, // Phoenix
	{39.614431, -105.109927, 39.914246, -104.600296}, // Denver
	{47.491912, -122.459696, 47.734145, -122.224433}, // Seattle
	{38.791645, -77.119759, 38.995547, -76.909393},   // Washington D.C.
	{32.617265, -96.999153, 32.961792, -96.587149},   // Dallas
	{42.227911, -71.191155, 42.396977, -70.986092},   // Boston
	{36.089245, -86.862065, 36.252684, -86.628379},   // Nashville
	{39.889084, -83.160423, 40.081240, -82.822052},   // Columbus
	{35.092842, -80.977859, 35.331210, -80.678932},   // Charlotte
	{39.687436, -86.317930, 39.889084, -85.972069},   // Indianapolis
	{30.097480, -97.936523, 30.432040, -97.518509},   // Austin
	{42.954292, -88.070908, 43.210680, -87.863770},   // Milwaukee
	{44.890289, -93.329334, 45.051440, -93.116379},   // Minneapolis
	{25.709043, -80.298775, 25.855874, -80.139732},   // Miami
	{45.432536, -122.762855, 45.582742, -122.520142}, // Portland
	{36.080338, -115.300125, 36.244355, -115.052185}, // Las Vegas
	{33.647808, -84.566345, 33.887737, -84.281548},   // Atlanta
	{35.026417, -90.123062, 35.234421, -89.844940},   // Memphis
	{27.821030, -82.588654, 28.015162, -82.370682},   // Tampa
	{39.030016, -84.593811, 39.172567, -84.368439},   // Cincinnati
	{36.659182, -119.883766, 36.838424, -119.656502}, // Fresno
	{40.371832, -80.095138, 40.519681, -79.867210},   // Pittsburgh
	{34.987618, -106.757278, 35.166277, -106.494217}, // Albuquerque
	{33.422268, -86.930084, 33.617015, -86.652985},   // Birmingham
	{38.140203, -85.861053, 38.345916, -85.602188},   // Louisville
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
