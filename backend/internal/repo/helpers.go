package repo

import (
	"database/sql"
	"strings"
	"time"
)

// timeFormat is the canonical RFC3339 UTC string stored in SQLite.
const timeFormat = time.RFC3339Nano

// formatTime renders t as the canonical storage string (UTC).
func formatTime(t time.Time) string {
	return t.UTC().Format(timeFormat)
}

// parseTime parses a stored time string, falling back to zero time.
func parseTime(s string) time.Time {
	t, err := time.Parse(timeFormat, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// boolToInt converts a bool to the SQLite integer representation.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// nullString converts an empty string to a NULL value (used for nullable
// TEXT columns such as manager_id / approved_by / location_id).
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// nullTime converts a *time.Time to a nullable RFC3339 string.
func nullTime(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: formatTime(*t), Valid: true}
}

// isUniqueViolation reports whether err is a SQLite UNIQUE constraint
// violation (SQLITE_CONSTRAINT_UNIQUE, extended code 2067).
func isUniqueViolation(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "constraint failed")
}
