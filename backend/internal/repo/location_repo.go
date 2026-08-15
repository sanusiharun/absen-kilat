package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// locationDAO maps a locations table row to/from a domain.Location.
type locationDAO struct {
	ID        string
	Name      string
	Type      string
	Lat       float64
	Lng       float64
	RadiusM   int
	Active    int
	CreatedAt string
}

const locationCols = `id, name, type, lat, lng, radius_m, active, created_at`

// LocationRepository is the SQLite implementation of domain.LocationRepository.
type LocationRepository struct {
	db *sql.DB
}

// NewLocationRepository creates a LocationRepository backed by the pool.
func NewLocationRepository(db *sql.DB) *LocationRepository {
	return &LocationRepository{db: db}
}

// ListActive returns all active locations, ordered by name.
func (r *LocationRepository) ListActive(ctx context.Context) ([]domain.Location, error) {
	return r.list(ctx, "active = 1")
}

// ListAll returns every location (active and inactive), ordered by name.
func (r *LocationRepository) ListAll(ctx context.Context) ([]domain.Location, error) {
	return r.list(ctx, "1=1")
}

func (r *LocationRepository) list(ctx context.Context, cond string) ([]domain.Location, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+locationCols+` FROM locations WHERE `+cond+` ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("repo.LocationRepository.list: %w", err)
	}
	defer rows.Close()

	var out []domain.Location
	for rows.Next() {
		var d locationDAO
		if err := rows.Scan(&d.ID, &d.Name, &d.Type, &d.Lat, &d.Lng,
			&d.RadiusM, &d.Active, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("repo.LocationRepository.list: scan: %w", err)
		}
		out = append(out, locationFromDAO(d))
	}
	return out, rows.Err()
}

// FindByID returns the location with the given id, or ErrNotFound.
func (r *LocationRepository) FindByID(ctx context.Context, id string) (*domain.Location, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+locationCols+` FROM locations WHERE id = ?`, id)
	var d locationDAO
	err := row.Scan(&d.ID, &d.Name, &d.Type, &d.Lat, &d.Lng,
		&d.RadiusM, &d.Active, &d.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.NotFound("location", id)
		}
		return nil, fmt.Errorf("repo.LocationRepository.FindByID: %w", err)
	}
	l := locationFromDAO(d)
	return &l, nil
}

// Create inserts a new location.
func (r *LocationRepository) Create(ctx context.Context, l *domain.Location) error {
	if l.ID == "" {
		l.ID = uuid.NewString()
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO locations (id, name, type, lat, lng, radius_m, active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.Name, l.Type, l.Lat, l.Lng, l.RadiusM, boolToInt(l.Active),
		formatTime(l.CreatedAt))
	if err != nil {
		return fmt.Errorf("repo.LocationRepository.Create: %w", err)
	}
	return nil
}

// Update persists changes to an existing location.
func (r *LocationRepository) Update(ctx context.Context, l *domain.Location) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE locations SET name = ?, type = ?, lat = ?, lng = ?, radius_m = ?, active = ?
		WHERE id = ?`,
		l.Name, l.Type, l.Lat, l.Lng, l.RadiusM, boolToInt(l.Active), l.ID)
	if err != nil {
		return fmt.Errorf("repo.LocationRepository.Update: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repo.LocationRepository.Update: rows: %w", err)
	}
	if n == 0 {
		return pkgerrors.NotFound("location", l.ID)
	}
	return nil
}

// SetActive soft-deletes (active=0) or re-activates a location.
func (r *LocationRepository) SetActive(ctx context.Context, id string, active bool) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE locations SET active = ? WHERE id = ?`, boolToInt(active), id)
	if err != nil {
		return fmt.Errorf("repo.LocationRepository.SetActive: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repo.LocationRepository.SetActive: rows: %w", err)
	}
	if n == 0 {
		return pkgerrors.NotFound("location", id)
	}
	return nil
}

// locationFromDAO maps a scanned DAO row to a domain.Location.
func locationFromDAO(d locationDAO) domain.Location {
	return domain.Location{
		ID:        d.ID,
		Name:      d.Name,
		Type:      d.Type,
		Lat:       d.Lat,
		Lng:       d.Lng,
		RadiusM:   d.RadiusM,
		Active:    d.Active == 1,
		CreatedAt: parseTime(d.CreatedAt),
	}
}
