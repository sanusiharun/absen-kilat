package service

import (
	"context"
	"encoding/json"
	"regexp"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// Default settings applied when the settings table is missing a key.
const (
	DefaultWorkStart   = "09:00"
	DefaultWorkEnd     = "17:00"
	DefaultAbsenMode   = "both"
	DefaultPhotoPolicy = "radius"
)

var defaultWorkdays = []string{"mon", "tue", "wed", "thu", "fri"}

var (
	validAbsenModes  = map[string]bool{"both": true, "checkin_only": true}
	validPhotoPolicy = map[string]bool{"radius": true, "always": true, "off": true}
	validWorkday     = map[string]bool{
		"mon": true, "tue": true, "wed": true, "thu": true,
		"fri": true, "sat": true, "sun": true,
	}
	timeRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
)

// SettingsService manages the system settings (work hours, mode, photo
// policy) stored in the settings key/value table.
type SettingsService struct {
	repo *repo.SettingsRepository
}

// NewSettingsService creates a SettingsService.
func NewSettingsService(repo *repo.SettingsRepository) *SettingsService {
	return &SettingsService{repo: repo}
}

// SettingsUpdate carries optional settings changes; nil fields are left
// unchanged.
type SettingsUpdate struct {
	WorkStart   *string
	WorkEnd     *string
	AbsenMode   *string
	Workdays    *[]string
	PhotoPolicy *string
}

// Get returns the full settings, applying defaults for missing keys.
func (s *SettingsService) Get(ctx context.Context) (*domain.Settings, error) {
	raw, err := s.repo.All(ctx)
	if err != nil {
		return nil, err
	}
	return settingsFromRaw(raw), nil
}

// Update applies the provided changes after validation and returns the
// resulting full settings.
func (s *SettingsService) Update(ctx context.Context, up SettingsUpdate) (*domain.Settings, error) {
	fields := map[string]string{}
	if up.WorkStart != nil {
		if !timeRe.MatchString(*up.WorkStart) {
			fields["work_start"] = "must be HH:MM"
		}
	}
	if up.WorkEnd != nil {
		if !timeRe.MatchString(*up.WorkEnd) {
			fields["work_end"] = "must be HH:MM"
		}
	}
	if up.AbsenMode != nil && !validAbsenModes[*up.AbsenMode] {
		fields["absen_mode"] = "must be both or checkin_only"
	}
	if up.PhotoPolicy != nil && !validPhotoPolicy[*up.PhotoPolicy] {
		fields["photo_policy"] = "must be radius, always, or off"
	}
	if up.Workdays != nil {
		for _, d := range *up.Workdays {
			if !validWorkday[d] {
				fields["workdays"] = "must be an array of mon..sun"
				break
			}
		}
	}
	if len(fields) > 0 {
		return nil, pkgerrors.ValidationMulti(fields)
	}

	if up.WorkStart != nil {
		if err := s.repo.Set(ctx, "work_start", *up.WorkStart); err != nil {
			return nil, err
		}
	}
	if up.WorkEnd != nil {
		if err := s.repo.Set(ctx, "work_end", *up.WorkEnd); err != nil {
			return nil, err
		}
	}
	if up.AbsenMode != nil {
		if err := s.repo.Set(ctx, "absen_mode", *up.AbsenMode); err != nil {
			return nil, err
		}
	}
	if up.PhotoPolicy != nil {
		if err := s.repo.Set(ctx, "photo_policy", *up.PhotoPolicy); err != nil {
			return nil, err
		}
	}
	if up.Workdays != nil {
		encoded, err := json.Marshal(dedupeWorkdays(*up.Workdays))
		if err != nil {
			return nil, err
		}
		if err := s.repo.Set(ctx, "workdays", string(encoded)); err != nil {
			return nil, err
		}
	}
	return s.Get(ctx)
}

// settingsFromRaw builds a Settings from the raw key/value map, applying
// defaults and parsing the JSON workdays value.
func settingsFromRaw(raw map[string]string) *domain.Settings {
	st := &domain.Settings{
		WorkStart:   DefaultWorkStart,
		WorkEnd:     DefaultWorkEnd,
		AbsenMode:   DefaultAbsenMode,
		Workdays:    append([]string(nil), defaultWorkdays...),
		PhotoPolicy: DefaultPhotoPolicy,
	}
	if v := raw["work_start"]; v != "" {
		st.WorkStart = v
	}
	if v := raw["work_end"]; v != "" {
		st.WorkEnd = v
	}
	if v := raw["absen_mode"]; v != "" {
		st.AbsenMode = v
	}
	if v := raw["photo_policy"]; v != "" {
		st.PhotoPolicy = v
	}
	if v := raw["workdays"]; v != "" {
		var days []string
		if err := json.Unmarshal([]byte(v), &days); err == nil && len(days) > 0 {
			st.Workdays = days
		}
	}
	return st
}

// dedupeWorkdays removes duplicates while preserving order.
func dedupeWorkdays(days []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(days))
	for _, d := range days {
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	return out
}
