package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/sanusi/absen-kilat/backend/internal/db"
	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// newAdminTestEnv wires real repos against a throwaway DB (migrations applied).
func newAdminTestEnv(t *testing.T) (*AdminService, *repo.UserRepository, *repo.AttendanceRepository) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	pool, err := db.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	ur := repo.NewUserRepository(pool)
	ar := repo.NewAttendanceRepository(pool)
	svc := NewAdminService(ur, ar, "admin@test.dev")
	return svc, ur, ar
}

func addUser(t *testing.T, r *repo.UserRepository, email string, role domain.UserRole) *domain.User {
	t.Helper()
	u := &domain.User{Email: email, Name: "U", PasswordHash: "x", Role: role}
	if err := r.Create(context.Background(), u); err != nil {
		t.Fatalf("create %s: %v", email, err)
	}
	return u
}

func TestAdminService_SeedAdmin(t *testing.T) {
	ctx := context.Background()
	svc, ur, _ := newAdminTestEnv(t)

	// First seed creates the admin.
	if err := svc.SeedAdmin(ctx, "secret123"); err != nil {
		t.Fatalf("SeedAdmin: %v", err)
	}
	a, err := ur.FindByEmail(ctx, "admin@test.dev")
	if err != nil {
		t.Fatalf("admin not found: %v", err)
	}
	if a.Role != domain.RoleAdmin {
		t.Fatalf("role = %q, want admin", a.Role)
	}
	if !a.IsActive {
		t.Fatal("admin should be active")
	}

	// Second seed is a no-op (no duplicate/error).
	if err := svc.SeedAdmin(ctx, "other123"); err != nil {
		t.Fatalf("SeedAdmin second: %v", err)
	}
	if n, _ := ur.CountActive(ctx); n != 1 {
		t.Fatalf("active count = %d, want 1", n)
	}
}

func TestAdminService_TodayStats(t *testing.T) {
	ctx := context.Background()
	svc, ur, ar := newAdminTestEnv(t)

	// 3 active users; 2 check in today (1 on_time, 1 late).
	u1 := addUser(t, ur, "a@test.dev", domain.RoleEmployee)
	addUser(t, ur, "b@test.dev", domain.RoleEmployee)
	addUser(t, ur, "c@test.dev", domain.RoleEmployee)

	loc := time.Local
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, loc)

	onTime := &domain.Attendance{UserID: u1.ID, Type: domain.AttendanceCheckIn, Status: domain.StatusOnTime, OccurredAt: today}
	if err := ar.Create(ctx, onTime); err != nil {
		t.Fatalf("create on_time: %v", err)
	}
	late := &domain.Attendance{UserID: u1.ID, Type: domain.AttendanceCheckIn, Status: domain.StatusLate, OccurredAt: today}
	if err := ar.Create(ctx, late); err != nil {
		t.Fatalf("create late: %v", err)
	}

	st, err := svc.TodayStats(ctx, loc)
	if err != nil {
		t.Fatalf("TodayStats: %v", err)
	}
	if st.TotalUsers != 3 {
		t.Fatalf("total = %d, want 3", st.TotalUsers)
	}
	if st.Present != 1 {
		t.Fatalf("present = %d, want 1", st.Present)
	}
	if st.Late != 1 {
		t.Fatalf("late = %d, want 1", st.Late)
	}
	if st.Absent != 2 { // total 3 - present 1 (u1 counted once, c/b absent)
		t.Fatalf("absent = %d, want 2", st.Absent)
	}
}

func TestAdminService_UpdateUserCannotDeactivateSelf(t *testing.T) {
	ctx := context.Background()
	svc, ur, _ := newAdminTestEnv(t)
	admin := addUser(t, ur, "admin@test.dev", domain.RoleAdmin)

	falseVal := false
	// Admin must not be able to deactivate their own account.
	_, err := svc.UpdateUser(ctx, admin.ID, admin.ID, UserUpdate{IsActive: &falseVal})
	if err == nil {
		t.Fatal("deactivating self should be forbidden")
	}
	if !pkgerrors.IsForbidden(err) {
		t.Fatalf("expected forbidden, got %v", err)
	}

	// Admin must not be able to demote their own role away from admin.
	emp := domain.RoleEmployee
	_, err = svc.UpdateUser(ctx, admin.ID, admin.ID, UserUpdate{Role: &emp})
	if err == nil {
		t.Fatal("demoting self should be forbidden")
	}
	if !pkgerrors.IsForbidden(err) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestAdminService_UpdateUserByOtherAdmin(t *testing.T) {
	ctx := context.Background()
	svc, ur, _ := newAdminTestEnv(t)
	admin := addUser(t, ur, "admin@test.dev", domain.RoleAdmin)
	target := addUser(t, ur, "emp@test.dev", domain.RoleEmployee)

	// Admin can deactivate another employee.
	falseVal := false
	updated, err := svc.UpdateUser(ctx, target.ID, admin.ID, UserUpdate{IsActive: &falseVal})
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if updated.IsActive {
		t.Fatal("target should be inactive")
	}
	if updated.Role != domain.RoleEmployee {
		t.Fatalf("role = %q, want employee", updated.Role)
	}
}

func TestAdminService_Locations(t *testing.T) {
	ctx := context.Background()
	svc, ur, ar := newAdminTestEnv(t)
	u := addUser(t, ur, "emp@test.dev", domain.RoleEmployee)

	// No attendances yet → empty.
	locs, err := svc.Locations(ctx, 5)
	if err != nil {
		t.Fatalf("Locations empty: %v", err)
	}
	if len(locs) != 0 {
		t.Fatalf("want 0 locations, got %d", len(locs))
	}

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		att := &domain.Attendance{UserID: u.ID, Type: domain.AttendanceCheckIn,
			Status: domain.StatusOnTime, Address: "Kantor", OccurredAt: now}
		if err := ar.Create(ctx, att); err != nil {
			t.Fatalf("create kantor: %v", err)
		}
	}
	att := &domain.Attendance{UserID: u.ID, Type: domain.AttendanceCheckIn,
		Status: domain.StatusLate, Address: "Lapangan", OccurredAt: now}
	if err := ar.Create(ctx, att); err != nil {
		t.Fatalf("create lapangan: %v", err)
	}

	locs, err = svc.Locations(ctx, 5)
	if err != nil {
		t.Fatalf("Locations: %v", err)
	}
	if len(locs) != 2 {
		t.Fatalf("want 2 locations, got %d", len(locs))
	}
	if locs[0].Location != "Kantor" || locs[0].Count != 3 {
		t.Fatalf("top = %+v, want Kantor x3", locs[0])
	}
	if locs[1].Location != "Lapangan" || locs[1].Count != 1 {
		t.Fatalf("second = %+v, want Lapangan x1", locs[1])
	}
}

func TestAdminService_Trends(t *testing.T) {
	ctx := context.Background()
	svc, ur, ar := newAdminTestEnv(t)
	u1 := addUser(t, ur, "a@test.dev", domain.RoleEmployee)
	addUser(t, ur, "b@test.dev", domain.RoleEmployee)
	addUser(t, ur, "c@test.dev", domain.RoleEmployee)

	loc := time.Local
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, loc)

	// 2 active users check in today: one on-time, one late.
	onTime := &domain.Attendance{UserID: u1.ID, Type: domain.AttendanceCheckIn, Status: domain.StatusOnTime, OccurredAt: today}
	if err := ar.Create(ctx, onTime); err != nil {
		t.Fatalf("create on_time: %v", err)
	}
	late := &domain.Attendance{UserID: u1.ID, Type: domain.AttendanceCheckIn, Status: domain.StatusLate, OccurredAt: today}
	if err := ar.Create(ctx, late); err != nil {
		t.Fatalf("create late: %v", err)
	}

	trends, err := svc.Trends(ctx, loc, 7)
	if err != nil {
		t.Fatalf("Trends: %v", err)
	}
	if len(trends) != 7 {
		t.Fatalf("want 7 trends, got %d", len(trends))
	}
	// Today's row is the last entry, oldest first.
	last := trends[len(trends)-1]
	if last.Date != today.Format("2006-01-02") {
		t.Fatalf("last date = %q, want %q", last.Date, today.Format("2006-01-02"))
	}
	if last.Present != 1 || last.Late != 1 {
		t.Fatalf("today present/late = %d/%d, want 1/1", last.Present, last.Late)
	}
	if last.Absent != 2 { // total 3 - present 1
		t.Fatalf("today absent = %d, want 2", last.Absent)
	}
	// An earlier day with no check-ins: present=0, absent=total.
	first := trends[0]
	if first.Present != 0 || first.Late != 0 {
		t.Fatalf("first present/late = %d/%d, want 0/0", first.Present, first.Late)
	}
	if first.Absent != 3 {
		t.Fatalf("first absent = %d, want 3", first.Absent)
	}
}

func TestAdminService_ListAttendances(t *testing.T) {
	ctx := context.Background()
	svc, ur, ar := newAdminTestEnv(t)
	u1 := addUser(t, ur, "alice@test.dev", domain.RoleEmployee)
	u2 := addUser(t, ur, "bob@test.dev", domain.RoleEmployee)

	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC) // fixed noon: never near midnight
	yesterday := now.AddDate(0, 0, -1)
	atts := []*domain.Attendance{
		{UserID: u1.ID, Type: domain.AttendanceCheckIn, Status: domain.StatusOnTime, Device: "Android", OccurredAt: now},
		{UserID: u1.ID, Type: domain.AttendanceCheckOut, Status: domain.StatusOnTime, Device: "Android", OccurredAt: now.Add(time.Hour)},
		{UserID: u2.ID, Type: domain.AttendanceCheckIn, Status: domain.StatusLate, Device: "iPhone", OccurredAt: now.Add(-30 * time.Minute)},
		{UserID: u2.ID, Type: domain.AttendanceCheckIn, Status: domain.StatusOnTime, Device: "iPhone", OccurredAt: yesterday},
	}
	for _, a := range atts {
		if err := ar.Create(ctx, a); err != nil {
			t.Fatalf("create attendance: %v", err)
		}
	}

	// Unfiltered: all 4, newest first (yesterday last).
	list, total, err := svc.ListAttendances(ctx, AttendanceFilter{Limit: 100})
	if err != nil {
		t.Fatalf("ListAttendances: %v", err)
	}
	if total != 4 || len(list) != 4 {
		t.Fatalf("total/len = %d/%d, want 4/4", total, len(list))
	}
	if list[3].OccurredAt.Format("2006-01-02") != yesterday.Format("2006-01-02") {
		t.Fatalf("oldest row = %v, want yesterday", list[3].OccurredAt)
	}
	if list[0].UserName != "U" || list[0].UserEmail != "alice@test.dev" {
		t.Fatalf("join failed: %+v", list[0])
	}

	// Filter by status=late.
	list, total, err = svc.ListAttendances(ctx, AttendanceFilter{Status: string(domain.StatusLate)})
	if err != nil {
		t.Fatalf("ListAttendances late: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].UserEmail != "bob@test.dev" {
		t.Fatalf("late: total/len = %d/%d, want 1/1 bob", total, len(list))
	}

	// Filter by type=check_out.
	list, total, err = svc.ListAttendances(ctx, AttendanceFilter{Type: string(domain.AttendanceCheckOut)})
	if err != nil {
		t.Fatalf("ListAttendances check_out: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("check_out: total/len = %d/%d, want 1/1", total, len(list))
	}

	// Search by name/email substring, case-insensitive.
	list, total, err = svc.ListAttendances(ctx, AttendanceFilter{Search: "ALICE"})
	if err != nil {
		t.Fatalf("ListAttendances search: %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("search alice: total/len = %d/%d, want 2/2", total, len(list))
	}

	// Date range: only today's records (yesterday excluded).
	today := now.In(time.Local).Format("2006-01-02")
	list, total, err = svc.ListAttendances(ctx, AttendanceFilter{From: today, To: today})
	if err != nil {
		t.Fatalf("ListAttendances range: %v", err)
	}
	if total != 3 || len(list) != 3 {
		t.Fatalf("range today: total/len = %d/%d, want 3/3", total, len(list))
	}

	// Pagination: limit 2 offset 0 keeps the total at 4.
	list, total, err = svc.ListAttendances(ctx, AttendanceFilter{Limit: 2})
	if err != nil {
		t.Fatalf("ListAttendances page: %v", err)
	}
	if total != 4 || len(list) != 2 {
		t.Fatalf("page: total/len = %d/%d, want 4/2", total, len(list))
	}

	// Invalid date → validation error.
	if _, _, err = svc.ListAttendances(ctx, AttendanceFilter{From: "not-a-date"}); !pkgerrors.IsValidation(err) {
		t.Fatalf("bad from date: want validation, got %v", err)
	}
}

func TestAdminService_CreateUser(t *testing.T) {
	ctx := context.Background()
	svc, ur, _ := newAdminTestEnv(t)

	u, err := svc.CreateUser(ctx, "new@test.dev", "secret123", "New User", domain.RoleEmployee)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.Role != domain.RoleEmployee || !u.IsActive || !u.IsEmailVerified {
		t.Fatalf("created user = %+v, want employee/active/verified", u)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("secret123")); err != nil {
		t.Fatalf("password not hashed with bcrypt: %v", err)
	}
	if _, err := ur.FindByEmail(ctx, "new@test.dev"); err != nil {
		t.Fatalf("user not persisted: %v", err)
	}

	// Duplicate email → conflict.
	if _, err := svc.CreateUser(ctx, "new@test.dev", "secret123", "Again", domain.RoleEmployee); !pkgerrors.IsConflict(err) {
		t.Fatalf("duplicate email: want conflict, got %v", err)
	}

	// Invalid inputs → validation.
	if _, err := svc.CreateUser(ctx, "bad", "short", "", domain.RoleEmployee); !pkgerrors.IsValidation(err) {
		t.Fatalf("bad input: want validation, got %v", err)
	}

	// Empty role defaults to employee.
	u2, err := svc.CreateUser(ctx, "emp@test.dev", "secret123", "Emp", "")
	if err != nil {
		t.Fatalf("CreateUser default role: %v", err)
	}
	if u2.Role != domain.RoleEmployee {
		t.Fatalf("default role = %q, want employee", u2.Role)
	}

	// Admin role is allowed.
	u3, err := svc.CreateUser(ctx, "boss@test.dev", "secret123", "Boss", domain.RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser admin: %v", err)
	}
	if u3.Role != domain.RoleAdmin {
		t.Fatalf("role = %q, want admin", u3.Role)
	}
}

func TestAdminService_Devices(t *testing.T) {
	ctx := context.Background()
	svc, ur, ar := newAdminTestEnv(t)
	u := addUser(t, ur, "emp@test.dev", domain.RoleEmployee)
	now := time.Now().UTC()

	// Empty device values are excluded from the aggregate.
	for _, dev := range []string{"", "Android", "Android", "iPhone"} {
		att := &domain.Attendance{UserID: u.ID, Type: domain.AttendanceCheckIn,
			Status: domain.StatusOnTime, Device: dev, OccurredAt: now}
		if err := ar.Create(ctx, att); err != nil {
			t.Fatalf("create attendance: %v", err)
		}
	}

	devs, err := svc.Devices(ctx)
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	if len(devs) != 2 {
		t.Fatalf("want 2 devices, got %d", len(devs))
	}
	if devs[0].Device != "Android" || devs[0].Count != 2 {
		t.Fatalf("top = %+v, want Android x2", devs[0])
	}
	if devs[1].Device != "iPhone" || devs[1].Count != 1 {
		t.Fatalf("second = %+v, want iPhone x1", devs[1])
	}
}
