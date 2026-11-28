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
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// seedAttendance membuat n baris kehadiran pada hari yang sama (detik ke-i),
// semuanya di dalam satu hari sehingga cocok dengan ekspor satu hari.
func seedAttendance(t *testing.T, ar *repo.AttendanceRepository, userID string, day time.Time, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		att := &domain.Attendance{
			UserID:     userID,
			Type:       domain.AttendanceCheckIn,
			Status:     domain.StatusOnTime,
			OccurredAt: day.Add(time.Duration(i) * time.Second),
		}
		if err := ar.Create(ctx, att); err != nil {
			t.Fatalf("seedAttendance %d: %v", i, err)
		}
	}
}

// E1: ekspor melewati batas 500 baris tanpa pemotongan diam-diam.
// Sebelum perbaikan, klien menerima 200 + CSV dengan tepat 500 baris.
func TestExportAttendance_PaginatesBeyond500Rows(t *testing.T) {
	ctx := context.Background()
	svc, ur, ar := newAdminTestEnv(t)
	loc := time.UTC

	u := addUser(t, ur, "paginate@test.dev", domain.RoleEmployee)
	day := time.Date(2026, 5, 4, 8, 0, 0, 0, time.UTC)
	const want = 620
	seedAttendance(t, ar, u.ID, day, want)

	from := day.Format("2006-01-02")
	rows, err := svc.ExportAttendance(ctx, loc, from, from)
	if err != nil {
		t.Fatalf("ExportAttendance: %v", err)
	}
	if len(rows) != want {
		t.Fatalf("baris CSV = %d, want %d (paginasi gagal / terpotong)", len(rows), want)
	}

	// Pembanding independen: hitung langsung dari DB dengan rentang yang sama.
	_, total, err := ar.ListAdmin(ctx, repo.AdminAttendanceFilter{
		From:  day.Truncate(24 * time.Hour),
		To:    day.Truncate(24 * time.Hour).Add(24 * time.Hour),
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("ListAdmin pembanding: %v", err)
	}
	if total != want {
		t.Fatalf("jumlah DB = %d, want %d", total, want)
	}

	// Tidak boleh ada baris ganda: setiap tanggal+NIP unik di sini, jadi hitung
	// keunikan memakai kombinasi tanggal+status sebagai pemeriksaan kasar isi.
	seen := map[string]bool{}
	for _, r := range rows {
		seen[r.Date+"|"+r.Email+"|"+r.Status] = true
	}
	if len(seen) != 1 {
		t.Fatalf("data uji seharusnya identik per pengguna, dapat %d varian", len(seen))
	}
}

// E2: di atas plafon keamanan, permintaan DITOLAK eksplisit (bukan CSV parsial).
func TestExportAttendance_RejectsAboveRowCeiling(t *testing.T) {
	ctx := context.Background()
	svc, ur, ar := newAdminTestEnv(t)
	loc := time.UTC

	u := addUser(t, ur, "ceiling@test.dev", domain.RoleEmployee)
	day := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
	seedAttendance(t, ar, u.ID, day, 12)

	prev := MaxExportRows
	MaxExportRows = 10
	t.Cleanup(func() { MaxExportRows = prev })

	from := day.Format("2006-01-02")
	rows, err := svc.ExportAttendance(ctx, loc, from, from)
	if rows != nil {
		t.Fatalf("tidak boleh ada baris dikembalikan saat melewati plafon, dapat %d", len(rows))
	}
	if !pkgerrors.IsValidation(err) {
		t.Fatalf("err = %v, want validation error", err)
	}
	if !strings.Contains(err.Error(), "12") {
		t.Fatalf("pesan harus menyebut jumlah baris sebenarnya: %v", err)
	}
}

// E3: CSV formula injection dinetralkan; nilai normal tidak berubah.
func TestWriteAttendanceCSV_NeutralisesFormulaInjection(t *testing.T) {
	rows := []ExportRow{{
		Date: "2026-11-01", Name: `=HYPERLINK("http://evil","klik")`,
		Email: "plus@test.dev", Type: "+cmd", Status: "-2+3",
		ApprovalStatus: "@SUM(A1:A9)", Source: "app",
		Device: "Android", IPAddress: "10.0.0.1", DistanceM: "12",
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
		t.Fatalf("harus 2 baris, dapat %d", len(recs))
	}
	got := recs[1]
	for i, want := range map[int]string{
		2: "'=HYPERLINK(\"http://evil\",\"klik\")", // nama
		4: "'+cmd",                                 // tipe
		5: "'-2+3",                                 // status
		6: "'@SUM(A1:A9)",                          // approval_status
	} {
		if got[i] != want {
			t.Fatalf("kolom %d = %q, want %q", i, got[i], want)
		}
	}
	// Nilai tanpa karakter pemicu tidak boleh diubah.
	if got[3] != "plus@test.dev" || got[7] != "app" || got[10] != "12" {
		t.Fatalf("nilai normal berubah: %v", got)
	}
}
