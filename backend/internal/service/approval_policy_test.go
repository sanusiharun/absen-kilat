package service

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/sanusi/absen-kilat/backend/internal/db"
	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// newPolicyEnv wires a real attendance service against a throwaway DB with
// real location + settings repos, plus the fixed test users (u1..u4) that the
// attendance rows reference (FK users.id).
func newPolicyEnv(t *testing.T) (*AttendanceService, *repo.LocationRepository, *repo.AttendanceRepository) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	pool, err := db.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	ar := repo.NewAttendanceRepository(pool)
	lr := repo.NewLocationRepository(pool)
	ur := repo.NewUserRepository(pool)
	ss := NewSettingsService(repo.NewSettingsRepository(pool))
	for i := 1; i <= 4; i++ {
		u := &domain.User{
			ID:       fmt.Sprintf("u%d", i),
			Email:    fmt.Sprintf("u%d@test.dev", i),
			Name:         fmt.Sprintf("User %d", i),
			PasswordHash: fmt.Sprintf("hash%d", i),
			IsActive:     true,
		}
		if err := ur.Create(context.Background(), u); err != nil {
			t.Fatalf("create user u%d: %v", i, err)
		}
	}
	svc := NewAttendanceService(ar, lr, ss, 9, OfficeConfig{})
	return svc, lr, ar
}

func addLocation(t *testing.T, lr *repo.LocationRepository, name, typ string, lat, lng float64, radiusM int) *domain.Location {
	t.Helper()
	l := &domain.Location{Name: name, Type: typ, Lat: lat, Lng: lng, RadiusM: radiusM, Active: true}
	if err := lr.Create(context.Background(), l); err != nil {
		t.Fatalf("create location %s: %v", name, err)
	}
	return l
}

func TestApprovalPolicy_OfficeInRadius(t *testing.T) {
	ctx := context.Background()
	svc, lr, _ := newPolicyEnv(t)
	// Office at (0,0), radius 1000m.
	addLocation(t, lr, "Kantor", "office", 0, 0, 1000)
	loc := time.FixedZone("WIB", 7*3600)
	now := time.Date(2026, 8, 15, 8, 0, 0, 0, loc)

	// Inside radius + photo → auto-approved.
	lat, lng := 0.0001, 0.0001
	att, err := svc.CheckIn(ctx, AttendanceInput{
		UserID: "u1", Type: domain.AttendanceCheckIn, Now: now, Location: loc,
		Latitude: &lat, Longitude: &lng, PhotoPath: "uploads/a.jpg",
	})
	if err != nil {
		t.Fatalf("CheckIn: %v", err)
	}
	if att.ApprovalStatus != domain.ApprovalApproved {
		t.Fatalf("approval = %q, want approved", att.ApprovalStatus)
	}
	if att.Source != domain.SourceApp {
		t.Fatalf("source = %q, want app", att.Source)
	}
	if att.LocationID == "" {
		t.Fatal("location_id should be matched to the nearest location")
	}
	if att.DistanceM == nil || *att.DistanceM > 20 {
		t.Fatalf("distance = %v, want ~0m", att.DistanceM)
	}

	// Inside radius WITHOUT photo → not rejected, but pending (no auto-approve).
	att2, err := svc.CheckIn(ctx, AttendanceInput{
		UserID: "u2", Type: domain.AttendanceCheckIn, Now: now, Location: loc,
		Latitude: &lat, Longitude: &lng,
	})
	if err != nil {
		t.Fatalf("CheckIn no photo: %v", err)
	}
	if att2.ApprovalStatus != domain.ApprovalPending {
		t.Fatalf("approval = %q, want pending", att2.ApprovalStatus)
	}
}

func TestApprovalPolicy_OutsideRadius(t *testing.T) {
	ctx := context.Background()
	svc, lr, _ := newPolicyEnv(t)
	addLocation(t, lr, "Kantor", "office", 0, 0, 1000)
	loc := time.FixedZone("WIB", 7*3600)
	now := time.Date(2026, 8, 15, 8, 0, 0, 0, loc)

	// ~110 km away + photo → accepted, pending.
	lat, lng := 1.0, 0.0
	att, err := svc.CheckIn(ctx, AttendanceInput{
		UserID: "u1", Type: domain.AttendanceCheckIn, Now: now, Location: loc,
		Latitude: &lat, Longitude: &lng, PhotoPath: "uploads/b.jpg",
	})
	if err != nil {
		t.Fatalf("CheckIn outside with photo: %v", err)
	}
	if att.ApprovalStatus != domain.ApprovalPending {
		t.Fatalf("approval = %q, want pending", att.ApprovalStatus)
	}
	if att.DistanceM == nil || *att.DistanceM < 1000 {
		t.Fatalf("distance = %v, want > 1000m", att.DistanceM)
	}

	// Outside WITHOUT photo → photo wajib → validation error.
	lat2, lng2 := 1.0, 0.5
	_, err = svc.CheckIn(ctx, AttendanceInput{
		UserID: "u2", Type: domain.AttendanceCheckIn, Now: now, Location: loc,
		Latitude: &lat2, Longitude: &lng2,
	})
	if err == nil || !pkgerrors.IsValidation(err) {
		t.Fatalf("outside without photo: want validation, got %v", err)
	}
}

func TestApprovalPolicy_FieldLocation(t *testing.T) {
	ctx := context.Background()
	svc, lr, _ := newPolicyEnv(t)
	// A field location: photo always mandatory, approval always pending.
	field := addLocation(t, lr, "Lapangan", "field", -6.2, 106.8, 2000)
	loc := time.FixedZone("WIB", 7*3600)
	now := time.Date(2026, 8, 15, 8, 0, 0, 0, loc)

	// Inside field radius + photo → still pending (type=field).
	lat, lng := -6.2001, 106.8001
	att, err := svc.CheckIn(ctx, AttendanceInput{
		UserID: "u1", Type: domain.AttendanceCheckIn, Now: now, Location: loc,
		Latitude: &lat, Longitude: &lng, PhotoPath: "uploads/c.jpg",
	})
	if err != nil {
		t.Fatalf("CheckIn field with photo: %v", err)
	}
	if att.ApprovalStatus != domain.ApprovalPending {
		t.Fatalf("approval = %q, want pending", att.ApprovalStatus)
	}

	// Field WITHOUT photo → validation error.
	_, err = svc.CheckIn(ctx, AttendanceInput{
		UserID: "u2", Type: domain.AttendanceCheckIn, Now: now, Location: loc,
		Latitude: &lat, Longitude: &lng,
	})
	if err == nil || !pkgerrors.IsValidation(err) {
		t.Fatalf("field without photo: want validation, got %v", err)
	}

	// Field selected via location_id, no coords → photo still required.
	_, err = svc.CheckIn(ctx, AttendanceInput{
		UserID: "u3", Type: domain.AttendanceCheckIn, Now: now, Location: loc,
		LocationID: field.ID,
	})
	if err == nil || !pkgerrors.IsValidation(err) {
		t.Fatalf("field selected w/o photo: want validation, got %v", err)
	}
}

func TestApprovalPolicy_LocationSelection(t *testing.T) {
	ctx := context.Background()
	svc, lr, _ := newPolicyEnv(t)
	office := addLocation(t, lr, "Kantor", "office", 0, 0, 1000)
	client := addLocation(t, lr, "Klien", "client", 1.0, 1.0, 500) // ~157km from office
	loc := time.FixedZone("WIB", 7*3600)
	now := time.Date(2026, 8, 15, 8, 0, 0, 0, loc)

	// Coords at the office, but user selects the client location:
	// distance is measured from the SELECTED location → outside → pending.
	lat, lng := 0.0001, 0.0001
	att, err := svc.CheckIn(ctx, AttendanceInput{
		UserID: "u1", Type: domain.AttendanceCheckIn, Now: now, Location: loc,
		Latitude: &lat, Longitude: &lng, PhotoPath: "uploads/d.jpg",
		LocationID: client.ID,
	})
	if err != nil {
		t.Fatalf("CheckIn selected client: %v", err)
	}
	if att.ApprovalStatus != domain.ApprovalPending {
		t.Fatalf("approval = %q, want pending (selected far location)", att.ApprovalStatus)
	}
	if att.LocationID != client.ID {
		t.Fatalf("location_id = %q, want %q", att.LocationID, client.ID)
	}

	// Selecting the office location with matching coords → approved.
	att2, err := svc.CheckIn(ctx, AttendanceInput{
		UserID: "u2", Type: domain.AttendanceCheckIn, Now: now, Location: loc,
		Latitude: &lat, Longitude: &lng, PhotoPath: "uploads/e.jpg",
		LocationID: office.ID,
	})
	if err != nil {
		t.Fatalf("CheckIn selected office: %v", err)
	}
	if att2.ApprovalStatus != domain.ApprovalApproved {
		t.Fatalf("approval = %q, want approved", att2.ApprovalStatus)
	}

	// Unknown/inactive location_id → validation error.
	_, err = svc.CheckIn(ctx, AttendanceInput{
		UserID: "u3", Type: domain.AttendanceCheckIn, Now: now, Location: loc,
		Latitude: &lat, Longitude: &lng, PhotoPath: "uploads/f.jpg",
		LocationID: "bogus",
	})
	if err == nil || !pkgerrors.IsValidation(err) {
		t.Fatalf("bogus location_id: want validation, got %v", err)
	}

	// Inactive location cannot be selected either.
	if err := lr.SetActive(ctx, client.ID, false); err != nil {
		t.Fatalf("deactivate client: %v", err)
	}
	_, err = svc.CheckIn(ctx, AttendanceInput{
		UserID: "u4", Type: domain.AttendanceCheckIn, Now: now, Location: loc,
		Latitude: &lat, Longitude: &lng, PhotoPath: "uploads/g.jpg",
		LocationID: client.ID,
	})
	if err == nil || !pkgerrors.IsValidation(err) {
		t.Fatalf("inactive location_id: want validation, got %v", err)
	}
}

func TestApprovalPolicy_CheckOutSharesPolicy(t *testing.T) {
	ctx := context.Background()
	svc, lr, ar := newPolicyEnv(t)
	addLocation(t, lr, "Kantor", "office", 0, 0, 1000)
	loc := time.FixedZone("WIB", 7*3600)
	now := time.Date(2026, 8, 15, 8, 0, 0, 0, loc)

	// Check in first (inside radius, no photo → pending).
	lat, lng := 0.0001, 0.0001
	if _, err := svc.CheckIn(ctx, AttendanceInput{
		UserID: "u1", Type: domain.AttendanceCheckIn, Now: now, Location: loc,
		Latitude: &lat, Longitude: &lng,
	}); err != nil {
		t.Fatalf("CheckIn: %v", err)
	}

	// Check-out outside radius without photo → photo wajib → validation.
	lat2, lng2 := 1.0, 0.0
	if _, err := svc.CheckOut(ctx, AttendanceInput{
		UserID: "u1", Type: domain.AttendanceCheckOut, Now: now.Add(8 * time.Hour), Location: loc,
		Latitude: &lat2, Longitude: &lng2,
	}); err == nil || !pkgerrors.IsValidation(err) {
		t.Fatalf("checkout outside w/o photo: want validation, got %v", err)
	}

	// Check-out inside radius + photo → approved.
	att, err := svc.CheckOut(ctx, AttendanceInput{
		UserID: "u1", Type: domain.AttendanceCheckOut, Now: now.Add(8 * time.Hour), Location: loc,
		Latitude: &lat, Longitude: &lng, PhotoPath: "uploads/h.jpg",
	})
	if err != nil {
		t.Fatalf("CheckOut: %v", err)
	}
	if att.ApprovalStatus != domain.ApprovalApproved {
		t.Fatalf("approval = %q, want approved", att.ApprovalStatus)
	}
	if att.DistanceM == nil {
		t.Fatal("distance should be recorded")
	}

	// Both records persisted in the real repo.
	list, err := ar.ListByUserDate(ctx, "u1", now.In(loc), loc)
	if err != nil {
		t.Fatalf("ListByUserDate: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("records = %d, want 2", len(list))
	}
}
