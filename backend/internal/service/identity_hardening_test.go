package service

import (
	"context"
	"testing"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// Invarian identitas/otorisasi (change identity-hardening, tes I1-I5 dan I16).
// Setiap tes di berkas ini harus GAGAL pada base 456eb1ad dan HIJAU setelah guard
// diterapkan. Tidak ada tes yang membaca isi source; semuanya memeriksa perilaku.

func rolePtr(r domain.UserRole) *domain.UserRole { return &r }

func boolPtr(b bool) *bool { return &b }

// I1: HR tidak boleh menaikkan role dirinya sendiri menjadi super_admin.
func TestIdentity_HRCannotPromoteSelfToSuperAdmin(t *testing.T) {
	ctx := context.Background()
	svc, ur, _ := newAdminTestEnv(t)
	hr := addUser(t, ur, "hr-self@test.dev", domain.RoleHR)

	_, err := svc.UpdateUser(ctx, hr.ID, hr.ID, UserUpdate{Role: rolePtr(domain.RoleSuperAdmin)})
	if !pkgerrors.IsForbidden(err) {
		t.Fatalf("HR menaikkan role sendiri harus 403, dapat: %v", err)
	}

	stored, findErr := ur.FindByID(ctx, hr.ID)
	if findErr != nil {
		t.Fatalf("FindByID: %v", findErr)
	}
	if stored.Role != domain.RoleHR {
		t.Fatalf("role tersimpan harus tetap hr, dapat %q", stored.Role)
	}
}

// I6 (varian): perubahan role untuk diri sendiri ditolak untuk semua arah.
func TestIdentity_SelfRoleChangeAlwaysForbidden(t *testing.T) {
	ctx := context.Background()
	svc, ur, _ := newAdminTestEnv(t)
	sup := addUser(t, ur, "sup-self@test.dev", domain.RoleSupervisor)

	_, err := svc.UpdateUser(ctx, sup.ID, sup.ID, UserUpdate{Role: rolePtr(domain.RoleEmployee)})
	if !pkgerrors.IsForbidden(err) {
		t.Fatalf("menurunkan role sendiri harus 403, dapat: %v", err)
	}
}

// I2: HR tidak boleh menaikkan pengguna lain menjadi super_admin.
func TestIdentity_HRCannotPromoteOtherToSuperAdmin(t *testing.T) {
	ctx := context.Background()
	svc, ur, _ := newAdminTestEnv(t)
	hr := addUser(t, ur, "hr-actor@test.dev", domain.RoleHR)
	target := addUser(t, ur, "emp-target@test.dev", domain.RoleEmployee)

	_, err := svc.UpdateUser(ctx, target.ID, hr.ID, UserUpdate{Role: rolePtr(domain.RoleSuperAdmin)})
	if !pkgerrors.IsForbidden(err) {
		t.Fatalf("HR menaikkan orang lain jadi super_admin harus 403, dapat: %v", err)
	}

	stored, _ := ur.FindByID(ctx, target.ID)
	if stored.Role != domain.RoleEmployee {
		t.Fatalf("role target harus tetap employee, dapat %q", stored.Role)
	}
}

// I2b: HR juga tidak boleh mengubah akun admin (hr/super_admin) menjadi role lain.
func TestIdentity_HRCannotDemoteAdminAccount(t *testing.T) {
	ctx := context.Background()
	svc, ur, _ := newAdminTestEnv(t)
	hr := addUser(t, ur, "hr-actor2@test.dev", domain.RoleHR)
	otherHR := addUser(t, ur, "hr-target@test.dev", domain.RoleHR)

	_, err := svc.UpdateUser(ctx, otherHR.ID, hr.ID, UserUpdate{Role: rolePtr(domain.RoleEmployee)})
	if !pkgerrors.IsForbidden(err) {
		t.Fatalf("HR menurunkan akun HR lain harus 403, dapat: %v", err)
	}
}

// I3: perubahan role non-admin oleh HR tetap boleh (perilaku lama tidak rusak).
func TestIdentity_HRCanPromoteEmployeeToSupervisor(t *testing.T) {
	ctx := context.Background()
	svc, ur, _ := newAdminTestEnv(t)
	hr := addUser(t, ur, "hr-ok@test.dev", domain.RoleHR)
	target := addUser(t, ur, "emp-ok@test.dev", domain.RoleEmployee)

	if _, err := svc.UpdateUser(ctx, target.ID, hr.ID, UserUpdate{Role: rolePtr(domain.RoleSupervisor)}); err != nil {
		t.Fatalf("HR menaikkan employee ke supervisor harus berhasil, dapat: %v", err)
	}
	stored, _ := ur.FindByID(ctx, target.ID)
	if stored.Role != domain.RoleSupervisor {
		t.Fatalf("role target harus supervisor, dapat %q", stored.Role)
	}
}

// I4: super_admin tetap bisa mengubah role siapa pun (selain dirinya sendiri).
func TestIdentity_SuperAdminCanChangeRoles(t *testing.T) {
	ctx := context.Background()
	svc, ur, _ := newAdminTestEnv(t)
	owner := addUser(t, ur, "owner@test.dev", domain.RoleSuperAdmin)
	target := addUser(t, ur, "emp-target2@test.dev", domain.RoleEmployee)

	if _, err := svc.UpdateUser(ctx, target.ID, owner.ID, UserUpdate{Role: rolePtr(domain.RoleSupervisor)}); err != nil {
		t.Fatalf("super admin harus boleh mengubah role, dapat: %v", err)
	}
	stored, _ := ur.FindByID(ctx, target.ID)
	if stored.Role != domain.RoleSupervisor {
		t.Fatalf("role target harus supervisor, dapat %q", stored.Role)
	}
}

// I5: super admin aktif terakhir tidak boleh diturunkan atau dinonaktifkan.
func TestIdentity_LastActiveSuperAdminCannotBeDemotedOrDeactivated(t *testing.T) {
	ctx := context.Background()
	svc, ur, _ := newAdminTestEnv(t)
	owner := addUser(t, ur, "only-owner@test.dev", domain.RoleSuperAdmin)
	actor := addUser(t, ur, "hr-actor3@test.dev", domain.RoleHR)

	if _, err := svc.UpdateUser(ctx, owner.ID, actor.ID, UserUpdate{Role: rolePtr(domain.RoleHR)}); err == nil {
		t.Fatal("menurunkan super admin aktif terakhir harus ditolak")
	}
	if _, err := svc.UpdateUser(ctx, owner.ID, actor.ID, UserUpdate{IsActive: boolPtr(false)}); err == nil {
		t.Fatal("menonaktifkan super admin aktif terakhir harus ditolak")
	}

	stored, _ := ur.FindByID(ctx, owner.ID)
	if stored.Role != domain.RoleSuperAdmin || !stored.IsActive {
		t.Fatalf("super admin terakhir harus tetap aktif dan super_admin, dapat role=%q active=%v", stored.Role, stored.IsActive)
	}
}

// I16: HR tidak boleh membuat akun baru dengan role super_admin.
func TestIdentity_HRCannotCreateSuperAdminAccount(t *testing.T) {
	ctx := context.Background()
	svc, ur, _ := newAdminTestEnv(t)
	hr := addUser(t, ur, "hr-creator@test.dev", domain.RoleHR)

	_, err := svc.CreateUser(ctx, CreateUserInput{
		Email:     "new-super@test.dev",
		Password:  "rahasia-panjang-123",
		Name:      "New Super",
		Role:      domain.RoleSuperAdmin,
		ActorRole: domain.RoleHR,
		ActorID:   hr.ID,
	})
	if !pkgerrors.IsForbidden(err) {
		t.Fatalf("HR membuat akun super_admin harus 403, dapat: %v", err)
	}
}

// I16b: super_admin boleh membuat akun super_admin baru.
func TestIdentity_SuperAdminCanCreateSuperAdminAccount(t *testing.T) {
	ctx := context.Background()
	svc, ur, _ := newAdminTestEnv(t)
	owner := addUser(t, ur, "owner-create@test.dev", domain.RoleSuperAdmin)

	if _, err := svc.CreateUser(ctx, CreateUserInput{
		Email:     "second-super@test.dev",
		Password:  "rahasia-panjang-123",
		Name:      "Second Super",
		Role:      domain.RoleSuperAdmin,
		ActorRole: domain.RoleSuperAdmin,
		ActorID:   owner.ID,
	}); err != nil {
		t.Fatalf("super admin harus boleh membuat super admin baru, dapat: %v", err)
	}
}
