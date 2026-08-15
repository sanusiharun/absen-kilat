package service

import (
	"context"
	"testing"
	"time"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/repo"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

func ptr[T any](v T) *T { return &v }

func TestSettingsService_GetDefaults(t *testing.T) {
	ctx := context.Background()
	_, _, _, pool := newAdminTestEnvFull(t)
	sr := repo.NewSettingsRepository(pool)
	svc := NewSettingsService(sr)

	st, err := svc.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if st.WorkStart != "09:00" || st.WorkEnd != "17:00" {
		t.Fatalf("hours = %s-%s, want 09:00-17:00", st.WorkStart, st.WorkEnd)
	}
	if st.AbsenMode != "both" {
		t.Fatalf("absen_mode = %q, want both", st.AbsenMode)
	}
	if st.PhotoPolicy != "radius" {
		t.Fatalf("photo_policy = %q, want radius", st.PhotoPolicy)
	}
	if len(st.Workdays) != 5 || st.Workdays[0] != "mon" || st.Workdays[4] != "fri" {
		t.Fatalf("workdays = %v, want mon..fri", st.Workdays)
	}
}

func TestSettingsService_Update(t *testing.T) {
	ctx := context.Background()
	_, _, _, pool := newAdminTestEnvFull(t)
	sr := repo.NewSettingsRepository(pool)
	svc := NewSettingsService(sr)

	days := []string{"mon", "wed", "sun"}
	st, err := svc.Update(ctx, SettingsUpdate{
		WorkStart:   ptr("08:30"),
		AbsenMode:   ptr("checkin_only"),
		Workdays:    &days,
		PhotoPolicy: ptr("always"),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if st.WorkStart != "08:30" || st.AbsenMode != "checkin_only" || st.PhotoPolicy != "always" {
		t.Fatalf("updated = %+v", st)
	}
	if len(st.Workdays) != 3 || st.Workdays[2] != "sun" {
		t.Fatalf("workdays = %v, want [mon wed sun]", st.Workdays)
	}

	// Persisted across a fresh read.
	st2, err := svc.Get(ctx)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if st2.WorkStart != "08:30" || st2.AbsenMode != "checkin_only" || st2.PhotoPolicy != "always" {
		t.Fatalf("persisted = %+v", st2)
	}

	// Invalid values → validation.
	if _, err := svc.Update(ctx, SettingsUpdate{AbsenMode: ptr("xyz")}); !pkgerrors.IsValidation(err) {
		t.Fatalf("bad absen_mode: want validation, got %v", err)
	}
	if _, err := svc.Update(ctx, SettingsUpdate{Workdays: &[]string{"munday"}}); !pkgerrors.IsValidation(err) {
		t.Fatalf("bad workday: want validation, got %v", err)
	}
	if _, err := svc.Update(ctx, SettingsUpdate{WorkStart: ptr("9am")}); !pkgerrors.IsValidation(err) {
		t.Fatalf("bad work_start: want validation, got %v", err)
	}
	if _, err := svc.Update(ctx, SettingsUpdate{PhotoPolicy: ptr("sometimes")}); !pkgerrors.IsValidation(err) {
		t.Fatalf("bad photo_policy: want validation, got %v", err)
	}
}

func TestAdminService_LocationCRUD(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newAdminTestEnv(t)

	// Create.
	l, err := svc.CreateLocation(ctx, LocationInput{Name: "Kantor Pusat", Type: "office", Lat: -6.2088, Lng: 106.8456, RadiusM: 500, Active: true})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}
	if l.ID == "" || !l.Active {
		t.Fatalf("created = %+v, want id + active", l)
	}
	if l.RadiusM != 500 {
		t.Fatalf("radius = %d, want default 500", l.RadiusM)
	}

	// Invalid input → validation.
	if _, err := svc.CreateLocation(ctx, LocationInput{Name: "", Type: "bunker", Lat: 999, Lng: 0, RadiusM: -1}); !pkgerrors.IsValidation(err) {
		t.Fatalf("bad input: want validation, got %v", err)
	}

	// List active.
	locs, err := svc.ListLocations(ctx, false)
	if err != nil {
		t.Fatalf("ListLocations: %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("active count = %d, want 1", len(locs))
	}

	// Update.
	upd, err := svc.UpdateLocation(ctx, l.ID, LocationUpdate{Name: ptr("Kantor Baru"), RadiusM: ptr(1000)})
	if err != nil {
		t.Fatalf("UpdateLocation: %v", err)
	}
	if upd.Name != "Kantor Baru" || upd.RadiusM != 1000 {
		t.Fatalf("updated = %+v", upd)
	}

	// Soft delete.
	if err := svc.DeleteLocation(ctx, l.ID); err != nil {
		t.Fatalf("DeleteLocation: %v", err)
	}
	locs, _ = svc.ListLocations(ctx, false)
	if len(locs) != 0 {
		t.Fatalf("active after delete = %d, want 0", len(locs))
	}
	locs, _ = svc.ListLocations(ctx, true)
	if len(locs) != 1 || locs[0].Active {
		t.Fatalf("all after delete = %+v, want 1 inactive", locs)
	}

	// Unknown id → not found.
	if _, err := svc.UpdateLocation(ctx, "nope", LocationUpdate{Name: ptr("x")}); !pkgerrors.IsNotFound(err) {
		t.Fatalf("update unknown: want not found, got %v", err)
	}
	if err := svc.DeleteLocation(ctx, "nope"); !pkgerrors.IsNotFound(err) {
		t.Fatalf("delete unknown: want not found, got %v", err)
	}

	// Re-activate via update.
	back, err := svc.UpdateLocation(ctx, l.ID, LocationUpdate{Active: ptr(true)})
	if err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	if !back.Active {
		t.Fatal("location should be active again")
	}
}

func TestAdminService_OverrideAttendance(t *testing.T) {
	ctx := context.Background()
	svc, ur, ar, pool := newAdminTestEnvFull(t)
	admin := addUser(t, ur, "admin@test.dev", domain.RoleSuperAdmin)
	emp := addUser(t, ur, "emp@test.dev", domain.RoleEmployee)

	att := &domain.Attendance{
		UserID: emp.ID, Type: domain.AttendanceCheckIn, Status: domain.StatusOnTime,
		Source: domain.SourceApp, ApprovalStatus: domain.ApprovalApproved,
		OccurredAt: time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC),
	}
	if err := ar.Create(ctx, att); err != nil {
		t.Fatalf("create attendance: %v", err)
	}

	// Reason is mandatory.
	if _, err := svc.OverrideAttendance(ctx, att.ID, admin.ID, OverrideInput{}); !pkgerrors.IsValidation(err) {
		t.Fatalf("no reason: want validation, got %v", err)
	}

	// No-op body → validation.
	if _, err := svc.OverrideAttendance(ctx, att.ID, admin.ID, OverrideInput{Reason: "tes"}); !pkgerrors.IsValidation(err) {
		t.Fatalf("empty change: want validation, got %v", err)
	}

	// Invalid values → validation.
	if _, err := svc.OverrideAttendance(ctx, att.ID, admin.ID, OverrideInput{Reason: "x", Status: ptr(domain.AttendanceStatus("banana"))}); !pkgerrors.IsValidation(err) {
		t.Fatalf("bad status: want validation, got %v", err)
	}

	// Override type + approval → approved.
	newTime := time.Date(2026, 8, 15, 9, 30, 0, 0, time.UTC)
	updated, err := svc.OverrideAttendance(ctx, att.ID, admin.ID, OverrideInput{
		Type:           ptr(domain.AttendanceCheckOut),
		Status:         ptr(domain.StatusLate),
		ApprovalStatus: ptr(domain.ApprovalApproved),
		OccurredAt:     &newTime,
		Reason:         "karyawan lupa absen keluar",
	})
	if err != nil {
		t.Fatalf("OverrideAttendance: %v", err)
	}
	if updated.Type != domain.AttendanceCheckOut || updated.Status != domain.StatusLate {
		t.Fatalf("updated = %+v", updated)
	}
	if updated.ApprovalStatus != domain.ApprovalApproved || updated.ApprovedBy != admin.ID {
		t.Fatalf("approval = %q by %q, want approved by admin", updated.ApprovalStatus, updated.ApprovedBy)
	}
	if !updated.OccurredAt.Equal(newTime) {
		t.Fatalf("occurred_at = %v, want %v", updated.OccurredAt, newTime)
	}

	// Audit trail written.
	logs, err := ar.ListApprovalLogs(ctx, att.ID)
	if err != nil {
		t.Fatalf("ListApprovalLogs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("log count = %d, want 1", len(logs))
	}
	if logs[0].Action != "override" || logs[0].Reason != "karyawan lupa absen keluar" || logs[0].ActorID != admin.ID {
		t.Fatalf("log = %+v", logs[0])
	}

	// Reject: sets rejection_reason and stamps actor.
	rejected, err := svc.OverrideAttendance(ctx, att.ID, admin.ID, OverrideInput{
		ApprovalStatus: ptr(domain.ApprovalRejected),
		Reason:         "foto tidak jelas",
	})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rejected.ApprovalStatus != domain.ApprovalRejected || rejected.RejectionReason != "foto tidak jelas" {
		t.Fatalf("rejected = %+v", rejected)
	}
	if rejected.ApprovedBy != admin.ID || rejected.ApprovedAt == nil {
		t.Fatalf("reject stamp missing: %+v", rejected)
	}

	// Unknown attendance → not found.
	if _, err := svc.OverrideAttendance(ctx, "nope", admin.ID, OverrideInput{Reason: "x", Status: ptr(domain.StatusLate)}); !pkgerrors.IsNotFound(err) {
		t.Fatalf("unknown attendance: want not found, got %v", err)
	}

	// Raw DB check: approval_log rows exist for the record.
	var n int
	if err := pool.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM approval_log WHERE attendance_id = ?`, att.ID).Scan(&n); err != nil {
		t.Fatalf("count logs: %v", err)
	}
	if n != 2 {
		t.Fatalf("raw log count = %d, want 2", n)
	}
}
