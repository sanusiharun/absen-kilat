package service

import (
	"context"
	"math"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
)

// OfficeConfig describes a legacy single office point used as a geofence
// fallback when no locations are configured. RadiusM == 0 disables it.
type OfficeConfig struct {
	Lat     float64
	Lng     float64
	RadiusM int
}

// geoResult is the outcome of a geofence check. Attendances are never
// rejected for being outside anymore: the result only informs the photo
// requirement and the auto-approval decision.
type geoResult struct {
	// DistanceM is the distance (meters, rounded) to the selected or
	// nearest active location. Nil when no coordinates were provided.
	DistanceM *int
	// Outside is true when coordinates exist and no active location is
	// within its allowed radius (or the selected location is outside).
	Outside bool
	// LocationID is the matched (selected or nearest) location id.
	LocationID string
	// LocationType is the type of the matched location ("" when none).
	LocationType string
}

// checkGeofence computes the attendance location outcome against the active
// locations table (falling back to the legacy OfficeConfig when no locations
// are configured). It never rejects: see geoResult for the semantics.
func (s *AttendanceService) checkGeofence(ctx context.Context, in AttendanceInput) (*geoResult, error) {
	res := &geoResult{}

	var locations []domain.Location
	if s.locations != nil {
		var err error
		locations, err = s.locations.ListActive(ctx)
		if err != nil {
			return nil, err
		}
	}

	// A selected location ("absen dimana") pins the distance/type even
	// without coordinates; unknown or inactive ids are invalid input.
	var selected *domain.Location
	if in.LocationID != "" {
		for i := range locations {
			if locations[i].ID == in.LocationID {
				selected = &locations[i]
				break
			}
		}
		if selected == nil {
			return nil, validationLocationID
		}
		res.LocationID = selected.ID
		res.LocationType = selected.Type
	}

	if in.Latitude == nil || in.Longitude == nil {
		return res, nil
	}
	lat, lng := *in.Latitude, *in.Longitude

	switch {
	case selected != nil:
		dm := int(math.Round(haversineMeters(selected.Lat, selected.Lng, lat, lng)))
		res.DistanceM = &dm
		res.Outside = dm > selected.RadiusM
	case len(locations) > 0:
		best := locations[0]
		bestDist := haversineMeters(best.Lat, best.Lng, lat, lng)
		for i := 1; i < len(locations); i++ {
			d := haversineMeters(locations[i].Lat, locations[i].Lng, lat, lng)
			if d < bestDist {
				bestDist = d
				best = locations[i]
			}
		}
		dm := int(math.Round(bestDist))
		res.DistanceM = &dm
		res.Outside = dm > best.RadiusM
		res.LocationID = best.ID
		res.LocationType = best.Type
	case s.office.RadiusM > 0:
		// Legacy fallback: single office point from env config.
		dm := int(math.Round(haversineMeters(s.office.Lat, s.office.Lng, lat, lng)))
		res.DistanceM = &dm
		res.Outside = dm > s.office.RadiusM
	}
	return res, nil
}

// photoRequired decides whether a photo is mandatory for the given geo
// outcome under the configured photo policy:
//   - always: photo required for every attendance;
//   - off: photo never required;
//   - radius (default): required when outside the radius or at a 'field'
//     location.
func (s *AttendanceService) photoRequired(ctx context.Context, g *geoResult) bool {
	policy := "radius"
	if s.settings != nil {
		if st, err := s.settings.Get(ctx); err == nil && st.PhotoPolicy != "" {
			policy = st.PhotoPolicy
		}
	}
	switch policy {
	case "always":
		return true
	case "off":
		return false
	default: // "radius"
		return g.Outside || g.LocationType == "field"
	}
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
