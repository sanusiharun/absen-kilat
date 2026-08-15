package service

import (
	"context"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
)

// PublicSettings is the lightweight settings subset exposed to the mobile
// app (no workdays — those are an admin concern).
type PublicSettings struct {
	WorkStart   string `json:"work_start"`
	WorkEnd     string `json:"work_end"`
	AbsenMode   string `json:"absen_mode"`
	PhotoPolicy string `json:"photo_policy"`
}

// AppService serves the authenticated mobile endpoints: public settings and
// the active location list ("absen dimana" picker).
type AppService struct {
	settings  *SettingsService
	locations *repo.LocationRepository
}

// NewAppService creates an AppService.
func NewAppService(settings *SettingsService, locations *repo.LocationRepository) *AppService {
	return &AppService{settings: settings, locations: locations}
}

// PublicSettings returns the lightweight settings subset for mobile.
func (s *AppService) PublicSettings(ctx context.Context) (*PublicSettings, error) {
	st, err := s.settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	return &PublicSettings{
		WorkStart:   st.WorkStart,
		WorkEnd:     st.WorkEnd,
		AbsenMode:   st.AbsenMode,
		PhotoPolicy: st.PhotoPolicy,
	}, nil
}

// ActiveLocations returns only the active locations (id, name, type) for the
// mobile "absen dimana" picker.
func (s *AppService) ActiveLocations(ctx context.Context) ([]domain.Location, error) {
	return s.locations.ListActive(ctx)
}
