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

// qrTokenDAO maps a qr_tokens table row to/from a domain.QrToken.
type qrTokenDAO struct {
	ID         string
	LocationID string
	Token      string
	Active     int
	ExpiresAt  sql.NullString
	CreatedBy  string
	CreatedAt  string
}

const qrTokenCols = `id, location_id, token, active, expires_at, created_by, created_at`

// QrTokenRepository is the SQLite implementation of domain.QrTokenRepository.
type QrTokenRepository struct {
	db *sql.DB
}

// NewQrTokenRepository creates a QrTokenRepository backed by the pool.
func NewQrTokenRepository(db *sql.DB) *QrTokenRepository {
	return &QrTokenRepository{db: db}
}

func (r *QrTokenRepository) Create(ctx context.Context, t *domain.QrToken) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO qr_tokens (id, location_id, token, active, expires_at, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.LocationID, t.Token, boolToInt(t.Active),
		nullTime(t.ExpiresAt), t.CreatedBy, formatTime(t.CreatedAt))
	if err != nil {
		if isUniqueViolation(err) {
			return pkgerrors.Conflict("qr_token", "token", t.Token)
		}
		return fmt.Errorf("repo.QrTokenRepository.Create: %w", err)
	}
	return nil
}

func (r *QrTokenRepository) FindByToken(ctx context.Context, token string) (*domain.QrToken, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+qrTokenCols+` FROM qr_tokens WHERE token = ?`, token)
	var d qrTokenDAO
	err := row.Scan(&d.ID, &d.LocationID, &d.Token, &d.Active, &d.ExpiresAt,
		&d.CreatedBy, &d.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.NotFound("qr_token", "")
		}
		return nil, fmt.Errorf("repo.QrTokenRepository.FindByToken: %w", err)
	}
	return qrTokenFromDAO(d), nil
}

// ListAll returns every token joined with its location name, newest first.
func (r *QrTokenRepository) ListAll(ctx context.Context) ([]domain.QrTokenView, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.id, t.location_id, l.name, t.token, t.active, t.expires_at,
			t.created_by, t.created_at
		FROM qr_tokens t
		JOIN locations l ON l.id = t.location_id
		ORDER BY t.created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("repo.QrTokenRepository.ListAll: %w", err)
	}
	defer rows.Close()

	var out []domain.QrTokenView
	for rows.Next() {
		var v domain.QrTokenView
		var active int
		var expiresAt, createdAt sql.NullString
		if err := rows.Scan(&v.ID, &v.LocationID, &v.LocationName, &v.Token, &active,
			&expiresAt, &v.CreatedBy, &createdAt); err != nil {
			return nil, fmt.Errorf("repo.QrTokenRepository.ListAll: scan: %w", err)
		}
		v.Active = active == 1
		if expiresAt.Valid {
			t := parseTime(expiresAt.String)
			v.ExpiresAt = &t
		}
		v.CreatedAt = parseTime(createdAt.String)
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repo.QrTokenRepository.ListAll: rows: %w", err)
	}
	return out, nil
}

// SetActive revokes (active=false) or re-activates a token.
func (r *QrTokenRepository) SetActive(ctx context.Context, id string, active bool) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE qr_tokens SET active = ? WHERE id = ?`, boolToInt(active), id)
	if err != nil {
		return fmt.Errorf("repo.QrTokenRepository.SetActive: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repo.QrTokenRepository.SetActive: rows: %w", err)
	}
	if n == 0 {
		return pkgerrors.NotFound("qr_token", id)
	}
	return nil
}

// qrTokenFromDAO maps a scanned DAO row to a domain.QrToken.
func qrTokenFromDAO(d qrTokenDAO) *domain.QrToken {
	t := &domain.QrToken{
		ID:         d.ID,
		LocationID: d.LocationID,
		Token:      d.Token,
		Active:     d.Active == 1,
		CreatedBy:  d.CreatedBy,
		CreatedAt:  parseTime(d.CreatedAt),
	}
	if d.ExpiresAt.Valid {
		exp := parseTime(d.ExpiresAt.String)
		t.ExpiresAt = &exp
	}
	return t
}
