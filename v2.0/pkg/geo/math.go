package geo

import "math"

const EarthRadiusKm = 6371.0

// Haversine calculates the exact great-circle distance between two points on Earth in kilometers.
func Haversine(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := (lat2 - lat1) * (math.Pi / 180.0)
	dLon := (lon2 - lon1) * (math.Pi / 180.0)

	lat1Rad := lat1 * (math.Pi / 180.0)
	lat2Rad := lat2 * (math.Pi / 180.0)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(lat1Rad)*math.Cos(lat2Rad)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return EarthRadiusKm * c
}

// BoundingBox converts a center point and a radius into a 2D bounding square for QuadTree querying.
// 1 degree of latitude is roughly 111 km.
func BoundingBox(lat, lon, radiusKm float64) (x, y, width, height float64) {
	latDelta := radiusKm / 111.0
	lonDelta := radiusKm / (111.0 * math.Cos(lat*(math.Pi/180.0)))

	x = lat - latDelta
	y = lon - lonDelta
	width = latDelta * 2
	height = lonDelta * 2

	return x, y, width, height
}