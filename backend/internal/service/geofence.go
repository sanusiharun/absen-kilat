package service

import (
	"fmt"
	"math"

	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// OfficeConfig describes the office location and the geofence radius used to
// accept attendance check-ins. RadiusM == 0 disables geofencing.
type OfficeConfig struct {
	Lat     float64
	Lng     float64
	RadiusM int
}

// checkGeofence validates a reported location against the office point using
// the haversine distance. It returns the distance in meters (rounded) and an
// error when the location is outside the allowed radius. When the office has
// no radius configured (disabled) or coordinates are missing, it returns 0, nil.
func (c OfficeConfig) checkGeofence(lat, lng *float64) (int, error) {
	if c.RadiusM <= 0 || lat == nil || lng == nil {
		return 0, nil
	}
	dist := haversineMeters(c.Lat, c.Lng, *lat, *lng)
	dm := int(math.Round(dist))
	if dist > float64(c.RadiusM) {
		return dm, pkgerrors.Validation("location",
			fmt.Sprintf("di luar area kantor (jarak %d m, maks %d m)", dm, c.RadiusM))
	}
	return dm, nil
}

// haversineMeters returns the great-circle distance between two coordinates in
// meters (earth radius 6371 km).
func haversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusM = 6371000.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusM * c
}
