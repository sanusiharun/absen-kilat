package repo_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sanusi/absen-kilat/backend/internal/db"
	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
)

func newShiftRepo(t *testing.T) (*repo.ShiftRepository, *repo.UserRepository) {
	t.Helper()
	ctx := context.Background()
	pool, err := db.Open(ctx, filepath.Join(t.TempDir(), "shift.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return repo.NewShiftRepository(pool), repo.NewUserRepository(pool)
}

// CRUD shift: buat, baca, ubah, dan daftar.
func TestShiftRepository_CRUD(t *testing.T) {
	ctx := context.Background()
	sr, _ := newShiftRepo(t)

	s := &domain.Shift{Code: "PAGI", Name: "Shift Pagi", StartTime: "08:00", EndTime: "20:00", GraceMinutes: 15, Active: true}
	if err := sr.CreateShift(ctx, s); err != nil {
		t.Fatalf("CreateShift: %v", err)
	}
	got, err := sr.FindShiftByID(ctx, s.ID)
	if err != nil {
		t.Fatalf("FindShiftByID: %v", err)
	}
	if got.Code != "PAGI" || got.GraceMinutes != 15 {
		t.Fatalf("data shift tidak utuh: %+v", got)
	}

	got.Name = "Shift Pagi (revisi)"
	if err := sr.UpdateShift(ctx, got); err != nil {
		t.Fatalf("UpdateShift: %v", err)
	}
	list, err := sr.ListShifts(ctx)
	if err != nil {
		t.Fatalf("ListShifts: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Shift Pagi (revisi)" {
		t.Fatalf("list tidak sesuai: %+v", list)
	}
}

// Penugasan bertumpuk untuk pegawai yang sama harus ditolak.
func TestShiftRepository_AssignmentOverlapRejected(t *testing.T) {
	ctx := context.Background()
	sr, ur := newShiftRepo(t)

	u := &domain.User{Email: "shift@test.dev", Name: "U", PasswordHash: "x", Role: domain.RoleEmployee}
	if err := ur.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	s := &domain.Shift{Code: "MALAM", StartTime: "20:00", EndTime: "08:00", CrossesMidnight: true, Active: true}
	if err := sr.CreateShift(ctx, s); err != nil {
		t.Fatalf("CreateShift: %v", err)
	}

	first := &domain.ShiftAssignment{ShiftID: s.ID, UserID: u.ID, StartDate: "2026-12-01", EndDate: "2026-12-10"}
	if err := sr.CreateAssignment(ctx, first); err != nil {
		t.Fatalf("penugasan pertama harus berhasil: %v", err)
	}
	overlap := &domain.ShiftAssignment{ShiftID: s.ID, UserID: u.ID, StartDate: "2026-12-05", EndDate: "2026-12-15"}
	if err := sr.CreateAssignment(ctx, overlap); err != domain.ErrShiftOverlap {
		t.Fatalf("penugasan bertumpuk harus ditolak, dapat: %v", err)
	}
	// Rentang yang tidak bertumpuk tetap boleh.
	ok := &domain.ShiftAssignment{ShiftID: s.ID, UserID: u.ID, StartDate: "2026-12-11", EndDate: "2026-12-20"}
	if err := sr.CreateAssignment(ctx, ok); err != nil {
		t.Fatalf("penugasan bersambung harus berhasil: %v", err)
	}
	got, err := sr.AssignmentsForUser(ctx, u.ID, "2026-12-01", "2026-12-31")
	if err != nil {
		t.Fatalf("AssignmentsForUser: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("harus 2 penugasan, dapat %d", len(got))
	}
}
