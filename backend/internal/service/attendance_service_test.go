package service

import (
	"context"
	"testing"
	"time"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
)

// fakeAttendanceRepo is an in-memory domain.AttendanceRepository.
type fakeAttendanceRepo struct {
	records []domain.Attendance
	seq     int
}

func (f *fakeAttendanceRepo) Create(_ context.Context, a *domain.Attendance) error {
	f.seq++
	a.ID = "att-" + string(rune('0'+f.seq))
	f.records = append(f.records, *a)
	return nil
}

func (f *fakeAttendanceRepo) ListByUserDate(_ context.Context, userID string, date time.Time, loc *time.Location) ([]domain.Attendance, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc).UTC()
	end := start.Add(24 * time.Hour)
	var out []domain.Attendance
	for _, a := range f.records {
		if a.UserID != userID {
			continue
		}
		if !a.OccurredAt.Before(start) && a.OccurredAt.Before(end) {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeAttendanceRepo) FindByID(_ context.Context, id string) (*domain.Attendance, error) {
	for _, a := range f.records {
		if a.ID == id {
			return &a, nil
		}
	}
	return nil, &domainNotFound{}
}

func TestCheckInAndCheckOutFlow(t *testing.T) {
	ctx := context.Background()
	svc := NewAttendanceService(&fakeAttendanceRepo{}, 9, OfficeConfig{}) // cutoff 09:00
	loc := time.FixedZone("WIB", 7*3600)
	now := time.Date(2026, 8, 14, 8, 30, 0, 0, loc) // 08:30 → on time

	in := AttendanceInput{UserID: "u1", Type: domain.AttendanceCheckIn, Now: now, Location: loc}
	att, err := svc.CheckIn(ctx, in)
	if err != nil {
		t.Fatalf("CheckIn: %v", err)
	}
	if att.Status != domain.StatusOnTime {
		t.Fatalf("status = %q, want on_time", att.Status)
	}

	// Duplicate check-in must fail.
	if _, err := svc.CheckIn(ctx, in); err == nil {
		t.Fatal("duplicate check-in should fail")
	}

	// Check-out works after check-in.
	co, err := svc.CheckOut(ctx, AttendanceInput{UserID: "u1", Type: domain.AttendanceCheckOut, Now: now.Add(8 * time.Hour), Location: loc})
	if err != nil {
		t.Fatalf("CheckOut: %v", err)
	}
	if co.Type != domain.AttendanceCheckOut {
		t.Fatalf("type = %q", co.Type)
	}

	// Duplicate check-out must fail.
	if _, err := svc.CheckOut(ctx, AttendanceInput{UserID: "u1", Type: domain.AttendanceCheckOut, Now: now.Add(9 * time.Hour), Location: loc}); err == nil {
		t.Fatal("duplicate check-out should fail")
	}
}

func TestCheckOutWithoutCheckInFails(t *testing.T) {
	ctx := context.Background()
	svc := NewAttendanceService(&fakeAttendanceRepo{}, 9, OfficeConfig{})
	loc := time.FixedZone("WIB", 7*3600)
	now := time.Date(2026, 8, 14, 17, 0, 0, 0, loc)

	_, err := svc.CheckOut(ctx, AttendanceInput{UserID: "u2", Type: domain.AttendanceCheckOut, Now: now, Location: loc})
	if err == nil {
		t.Fatal("check-out without check-in should fail")
	}
}

func TestLateStatus(t *testing.T) {
	ctx := context.Background()
	svc := NewAttendanceService(&fakeAttendanceRepo{}, 9, OfficeConfig{}) // cutoff 09:00
	loc := time.FixedZone("WIB", 7*3600)

	late := time.Date(2026, 8, 14, 9, 30, 0, 0, loc) // 09:30 → late
	att, err := svc.CheckIn(ctx, AttendanceInput{UserID: "u3", Type: domain.AttendanceCheckIn, Now: late, Location: loc})
	if err != nil {
		t.Fatalf("CheckIn: %v", err)
	}
	if att.Status != domain.StatusLate {
		t.Fatalf("status = %q, want late", att.Status)
	}
}

func TestTodayState(t *testing.T) {
	ctx := context.Background()
	svc := NewAttendanceService(&fakeAttendanceRepo{}, 9, OfficeConfig{})
	loc := time.FixedZone("WIB", 7*3600)
	now := time.Date(2026, 8, 14, 8, 0, 0, 0, loc)

	st, err := svc.Today(ctx, "u1", now, loc)
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	if st.CheckedIn || st.CheckedOut {
		t.Fatal("fresh user should have no records")
	}

	_, _ = svc.CheckIn(ctx, AttendanceInput{UserID: "u1", Type: domain.AttendanceCheckIn, Now: now, Location: loc})
	st, err = svc.Today(ctx, "u1", now, loc)
	if err != nil {
		t.Fatalf("Today: %v", err)
	}
	if !st.CheckedIn {
		t.Fatal("should be checked in")
	}
	if st.CheckedOut {
		t.Fatal("should not be checked out yet")
	}
}

func TestGeofence(t *testing.T) {
	ctx := context.Background()
	// Office at (0,0) with 1000 m radius.
	svc := NewAttendanceService(&fakeAttendanceRepo{}, 9, OfficeConfig{Lat: 0, Lng: 0, RadiusM: 1000})
	loc := time.FixedZone("WIB", 7*3600)
	now := time.Date(2026, 8, 14, 8, 0, 0, 0, loc)

	// ~110 km away -> must be rejected.
	lat, lng := 1.0, 0.0
	_, err := svc.CheckIn(ctx, AttendanceInput{
		UserID: "u-g1", Type: domain.AttendanceCheckIn, Now: now, Location: loc,
		Latitude: &lat, Longitude: &lng,
	})
	if err == nil {
		t.Fatal("check-in far outside radius should be rejected")
	}

	// At the office -> accepted and distance recorded (~0 m).
	lat2, lng2 := 0.0001, 0.0001
	att, err := svc.CheckIn(ctx, AttendanceInput{
		UserID: "u-g2", Type: domain.AttendanceCheckIn, Now: now, Location: loc,
		Latitude: &lat2, Longitude: &lng2,
	})
	if err != nil {
		t.Fatalf("check-in inside radius should pass: %v", err)
	}
	if att.DistanceM == nil {
		t.Fatal("distance should be recorded")
	}

	// No coordinates -> geofence skipped, no error.
	att2, err := svc.CheckIn(ctx, AttendanceInput{
		UserID: "u-g3", Type: domain.AttendanceCheckIn, Now: now, Location: loc,
	})
	if err != nil {
		t.Fatalf("check-in without coords should pass: %v", err)
	}
	if att2.DistanceM != nil {
		t.Fatal("distance should be nil when no coordinates")
	}
}
