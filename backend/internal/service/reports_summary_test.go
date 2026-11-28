package service

import (
	"context"
	"testing"
	"time"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
)

// Invarian: total ringkasan == jumlah per hari, dan angka cocok dengan baris DB
// yang benar-benar dibuat (bukan dummy).
func TestSummaryReports_TotalsMatchRowsAndDays(t *testing.T) {
	ctx := context.Background()
	svc, ur, ar := newAdminTestEnv(t)
	loc := time.UTC

	u := addUser(t, ur, "sum@test.dev", domain.RoleEmployee)
	base := time.Now().UTC().Truncate(24 * time.Hour).Add(8 * time.Hour)
	rows := []struct {
		typ    domain.AttendanceType
		status domain.AttendanceStatus
		offset time.Duration
	}{
		{domain.AttendanceCheckIn, domain.StatusOnTime, 0},
		{domain.AttendanceCheckIn, domain.StatusLate, 30 * time.Minute},
		{domain.AttendanceCheckOut, domain.StatusOnTime, 9 * time.Hour},
	}
	for _, r := range rows {
		att := &domain.Attendance{UserID: u.ID, Type: r.typ, Status: r.status, OccurredAt: base.Add(r.offset)}
		if err := ar.Create(ctx, att); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	day := base.Format("2006-01-02")
	sum, err := svc.SummaryReports(ctx, loc, day, day)
	if err != nil {
		t.Fatalf("SummaryReports: %v", err)
	}
	if len(sum.ByDay) != 1 {
		t.Fatalf("harus 1 hari, dapat %d", len(sum.ByDay))
	}
	if sum.Totals.Hadir != 1 || sum.Totals.Telat != 1 || sum.Totals.Pulang != 1 {
		t.Fatalf("agregat salah: %+v", sum.Totals)
	}
	// Kontrak antar-data: total == jumlah per hari.
	var hadir, telat, pulang int
	for _, d := range sum.ByDay {
		hadir += d.Hadir
		telat += d.Telat
		pulang += d.Pulang
	}
	if hadir != sum.Totals.Hadir || telat != sum.Totals.Telat || pulang != sum.Totals.Pulang {
		t.Fatalf("total (%+v) harus sama dengan jumlah per hari", sum.Totals)
	}
}

// Validasi rentang ikut berlaku (ditolak sebelum query).
func TestSummaryReports_InvalidRangeRejected(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newAdminTestEnv(t)
	if _, err := svc.SummaryReports(ctx, time.UTC, "2026-11-10", "2026-11-01"); err == nil {
		t.Fatal("from > to harus ditolak")
	}
	if _, err := svc.SummaryReports(ctx, time.UTC, "", ""); err == nil {
		t.Fatal("rentang kosong harus ditolak")
	}
}
