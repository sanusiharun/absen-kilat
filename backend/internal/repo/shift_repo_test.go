package repo_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/sanusi/absen-kilat/backend/internal/db"
	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
)

func newShiftRepo(t *testing.T) (*repo.ShiftRepository, *repo.UserRepository, *sql.DB) {
	t.Helper()
	ctx := context.Background()
	pool, err := db.Open(ctx, filepath.Join(t.TempDir(), "shift.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return repo.NewShiftRepository(pool), repo.NewUserRepository(pool), pool
}

// CRUD shift: buat, baca, ubah, dan daftar.
func TestShiftRepository_CRUD(t *testing.T) {
	ctx := context.Background()
	sr, _, _ := newShiftRepo(t)

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

// Kode shift unik tanpa memandang huruf besar/kecil (proposal §6).
func TestShiftRepository_CodeUniqueCaseInsensitive(t *testing.T) {
	ctx := context.Background()
	sr, _, _ := newShiftRepo(t)

	if err := sr.CreateShift(ctx, &domain.Shift{Code: "PAGI", StartTime: "08:00", EndTime: "16:00", Active: true}); err != nil {
		t.Fatalf("CreateShift pertama: %v", err)
	}
	err := sr.CreateShift(ctx, &domain.Shift{Code: "pagi", StartTime: "08:00", EndTime: "16:00", Active: true})
	if !errors.Is(err, domain.ErrShiftCodeExists) {
		t.Fatalf("kode duplikat harus ditolak dengan ErrShiftCodeExists, dapat: %v", err)
	}

	second := &domain.Shift{Code: "SIANG", StartTime: "12:00", EndTime: "20:00", Active: true}
	if err := sr.CreateShift(ctx, second); err != nil {
		t.Fatalf("CreateShift kedua: %v", err)
	}
	second.Code = "pagi"
	if err := sr.UpdateShift(ctx, second); !errors.Is(err, domain.ErrShiftCodeExists) {
		t.Fatalf("rename ke kode yang sudah ada harus ditolak, dapat: %v", err)
	}
}

// UpdateShift pada id tak dikenal mengembalikan ErrShiftNotFound.
func TestShiftRepository_UpdateUnknownNotFound(t *testing.T) {
	ctx := context.Background()
	sr, _, _ := newShiftRepo(t)
	err := sr.UpdateShift(ctx, &domain.Shift{ID: "tidak-ada", Code: "X", StartTime: "08:00", EndTime: "09:00"})
	if !errors.Is(err, domain.ErrShiftNotFound) {
		t.Fatalf("harus ErrShiftNotFound, dapat: %v", err)
	}
}

// Penugasan disimpan apa adanya tanpa aturan bentrok (aturan belum disetujui).
func TestShiftRepository_AssignmentPersisted(t *testing.T) {
	ctx := context.Background()
	sr, ur, _ := newShiftRepo(t)

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
		t.Fatalf("CreateAssignment: %v", err)
	}
	got, err := sr.AssignmentsForUser(ctx, u.ID, "2026-12-01", "2026-12-31")
	if err != nil {
		t.Fatalf("AssignmentsForUser: %v", err)
	}
	if len(got) != 1 || got[0].StartDate != "2026-12-01" || got[0].EndDate != "2026-12-10" {
		t.Fatalf("penugasan tidak sesuai: %+v", got)
	}
	// Rentang di luar jendela tidak ikut terbaca.
	if none, err := sr.AssignmentsForUser(ctx, u.ID, "2027-01-01", "2027-01-31"); err != nil || len(none) != 0 {
		t.Fatalf("jendela kosong harus 0 baris, dapat %d (err=%v)", len(none), err)
	}
}

// CHECK(end_date >= start_date) menolak rentang terbalik di level DB.
func TestShiftRepository_AssignmentInvalidRangeRejected(t *testing.T) {
	ctx := context.Background()
	sr, ur, _ := newShiftRepo(t)

	u := &domain.User{Email: "range@test.dev", Name: "U", PasswordHash: "x", Role: domain.RoleEmployee}
	if err := ur.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	s := &domain.Shift{Code: "SORE", StartTime: "14:00", EndTime: "22:00", Active: true}
	if err := sr.CreateShift(ctx, s); err != nil {
		t.Fatalf("CreateShift: %v", err)
	}
	a := &domain.ShiftAssignment{ShiftID: s.ID, UserID: u.ID, StartDate: "2026-12-10", EndDate: "2026-12-01"}
	if err := sr.CreateAssignment(ctx, a); err == nil {
		t.Fatal("rentang terbalik harus ditolak CHECK constraint")
	}
}

// Migrasi 018 menambah attendances.shift_id + index-nya.
func TestMigration018_AttendanceShiftColumn(t *testing.T) {
	ctx := context.Background()
	_, _, pool := newShiftRepo(t)

	rows, err := pool.QueryContext(ctx, `SELECT name FROM pragma_table_info('attendances') WHERE name = 'shift_id'`)
	if err != nil {
		t.Fatalf("pragma_table_info: %v", err)
	}
	hasShiftID := rows.Next()
	if err := rows.Err(); err != nil {
		t.Fatalf("pragma_table_info: %v", err)
	}
	rows.Close()
	if !hasShiftID {
		t.Fatal("kolom attendances.shift_id tidak ada")
	}

	var name string
	if err := pool.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_attendances_shift'`).Scan(&name); err != nil {
		t.Fatalf("index idx_attendances_shift tidak ada: %v", err)
	}
}
