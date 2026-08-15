package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// GeocodeResult is a human-readable address resolved from coordinates.
type GeocodeResult struct {
	Address     string // full display name, e.g. "Jl. Sudirman, Jakarta"
	ShortAddress string // road/city-ish short form
}

// GeocodeService resolves lat/lng to an address using OSM Nominatim (free, no
// API key). Results are cached per coordinate pair to stay well under
// Nominatim's usage policy (max ~1 req/sec).
type GeocodeService struct {
	client *http.Client
	mu     sync.Mutex
	cache  map[string]geocodeCacheEntry
}

type geocodeCacheEntry struct {
	res     GeocodeResult
	expires time.Time
}

const geocodeCacheTTL = 24 * time.Hour

// NewGeocodeService creates a GeocodeService.
func NewGeocodeService() *GeocodeService {
	return &GeocodeService{
		client: &http.Client{Timeout: 6 * time.Second},
		cache:  map[string]geocodeCacheEntry{},
	}
}

// Reverse resolves coordinates to an address. Cache hits avoid repeat calls.
func (s *GeocodeService) Reverse(ctx context.Context, lat, lng float64) (*GeocodeResult, error) {
	key := fmt.Sprintf("%.5f,%.5f", lat, lng) // ~1.1 m grid, good cache granularity

	s.mu.Lock()
	if e, ok := s.cache[key]; ok && time.Now().Before(e.expires) {
		s.mu.Unlock()
		return &e.res, nil
	}
	s.mu.Unlock()

	u := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?format=jsonv2&lat=%f&lon=%f", lat, lng)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("service.GeocodeService.Reverse: new request: %w", err)
	}
	req.Header.Set("User-Agent", "absen-kilat/1.0 (admin@absenkilat.dev)")
	req.Header.Set("Accept-Language", "id,en")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("service.GeocodeService.Reverse: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("service.GeocodeService.Reverse: status %d", resp.StatusCode)
	}

	var body struct {
		DisplayName string `json:"display_name"`
		Address     struct {
			Road     string `json:"road"`
			Suburb   string `json:"suburb"`
			City     string `json:"city"`
			State    string `json:"state"`
			Country  string `json:"country"`
			Village  string `json:"village"`
			Town     string `json:"town"`
		} `json:"address"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("service.GeocodeService.Reverse: decode: %w", err)
	}
	if body.DisplayName == "" {
		return nil, fmt.Errorf("service.GeocodeService.Reverse: empty address for %.5f,%.5f", lat, lng)
	}

	res := GeocodeResult{
		Address: body.DisplayName,
		ShortAddress: firstNonEmpty(body.Address.Road, body.Address.Suburb,
			body.Address.Town, body.Address.City, body.Address.Village, body.Address.State),
	}
	s.mu.Lock()
	s.cache[key] = geocodeCacheEntry{res: res, expires: time.Now().Add(geocodeCacheTTL)}
	s.mu.Unlock()
	return &res, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
