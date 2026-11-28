package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
)

// Validasi rentang: semua penolakan harus terklasifikasi validation (HTTP 400).
func TestParseExportRange_Validation(t *testing.T) {
	loc := time.UTC
	cases := []struct {
		name     string
		from, to string
		ok       bool
	}{
		{"kosong", "", "", false},
		{"format salah", "2026-13-01", "2026-13-02", false},
		{"from > to", "2026-11-10", "2026-11-01", false},
		{"tepat 92 hari", "2026-01-01", "2026-04-02", true},
		{"93 hari ditolak", "2026-01-01", "2026-04-03", false},
		{"satu hari", "2026-11-01", "2026-11-01", true},
	}
	for _, c := range cases {
		_, _, err := ParseExportRange(c.from, c.to, loc)
		if c.ok && err != nil {
			t.Fatalf("%s: harus valid, dapat error %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Fatalf("%s: harus ditolak", c.name)
		}
	}
}

// CSV: header stabil dan escaping koma/kutip/newline benar.
func TestWriteAttendanceCSV_Escaping(t *testing.T) {
	rows := []ExportRow{{
		Date: "2026-11-01", NIP: "001", Name: `Budi, "Bud" Santoso`,
		Email: "budi@test.dev", Type: "in", Status: "late",
		ApprovalStatus: "auto", Source: "app", Device: "Android",
		IPAddress: "10.0.0.1", DistanceM: "12",
	}}
	var buf bytes.Buffer
	if err := WriteAttendanceCSV(&buf, rows); err != nil {
		t.Fatalf("WriteAttendanceCSV: %v", err)
	}
	recs, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("CSV tidak valid: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("harus 2 baris (header + data), dapat %d", len(recs))
	}
	if strings.Join(recs[0], ",") != strings.Join(AttendanceCSVHeader, ",") {
		t.Fatalf("header berubah: %v", recs[0])
	}
	if recs[1][2] != `Budi, "Bud" Santoso` {
		t.Fatalf("nama ber-koma/kutip harus utuh, dapat %q", recs[1][2])
	}
	if len(recs[1]) != len(AttendanceCSVHeader) {
		t.Fatalf("jumlah kolom baris data harus %d, dapat %d", len(AttendanceCSVHeader), len(recs[1]))
	}
}

// Invarian A1: jumlah baris CSV == jumlah baris yang benar-benar ada di DB untuk
// rentang yang sama (dihitung lewat jalur query yang sama sekali berbeda).
func TestExportAttendance_MatchesDatabaseRows(t *testing.T) {
	ctx := context.Background()
	svc, ur, ar := newAdminTestEnv(t)
	loc := time.UTC

	u := addUser(t, ur, "export@test.dev", domain.RoleEmployee)
	inRange := time.Now().UTC().Truncate(24 * time.Hour).Add(8 * time.Hour)
	for i := 0; i < 2; i++ {
		att := &domain.Attendance{UserID: u.ID, Type: domain.AttendanceCheckIn,
			Status: domain.StatusOnTime, OccurredAt: inRange.Add(time.Duration(i) * time.Minute)}
		if err := ar.Create(ctx, att); err != nil {
			t.Fatalf("create in-range: %v", err)
		}
	}
	outside := inRange.AddDate(0, 0, 10)
	if err := ar.Create(ctx, &domain.Attendance{UserID: u.ID, Type: domain.AttendanceCheckIn,
		Status: domain.StatusOnTime, OccurredAt: outside}); err != nil {
		t.Fatalf("create outside: %v", err)
	}

	day := inRange.Format("2006-01-02")
	rows, err := svc.ExportAttendance(ctx, loc, day, day)
	if err != nil {
		t.Fatalf("ExportAttendance: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("CSV harus memuat 2 baris pada rentang hari itu, dapat %d", len(rows))
	}

	// Pembanding: hitung langsung dari DB dengan rentang yang sama.
	start := inRange.Truncate(24 * time.Hour)
	end := start.Add(24 * time.Hour)
	_, dbTotal, err := ar.ListAdmin(ctx, repo.AdminAttendanceFilter{From: start, To: end, Limit: 1000})
	if err != nil {
		t.Fatalf("ListAdmin pembanding: %v", err)
	}
	if dbTotal != len(rows) {
		t.Fatalf("jumlah CSV (%d) harus sama dengan jumlah DB (%d)", len(rows), dbTotal)
	}
}
