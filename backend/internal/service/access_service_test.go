package service

import (
	"context"
	"strings"
	"testing"

	"github.com/sanusi/absen-kilat/backend/internal/authz"
	"github.com/sanusi/absen-kilat/backend/internal/domain"
)

// ---- fakes ---------------------------------------------------------------

type fakeAccessStore struct {
	rolePerms  map[string][]string
	allow      []string
	deny       []string
	setCalls   [][3]string // userID, permission, effect
	clearCalls [][2]string // userID, permission
}

func (f *fakeAccessStore) RolePermissions(_ context.Context, role string) ([]string, error) {
	return f.rolePerms[role], nil
}

func (f *fakeAccessStore) UserOverrides(_ context.Context, _ string) ([]string, []string, error) {
	return f.allow, f.deny, nil
}

func (f *fakeAccessStore) SetUserOverride(_ context.Context, userID, permission, effect, _ string) error {
	f.setCalls = append(f.setCalls, [3]string{userID, permission, effect})
	return nil
}

func (f *fakeAccessStore) ClearUserOverride(_ context.Context, userID, permission, _ string) error {
	f.clearCalls = append(f.clearCalls, [2]string{userID, permission})
	return nil
}

type fakeUsers struct{ users []domain.User }

func (f *fakeUsers) ListAll(context.Context) ([]domain.User, error) { return f.users, nil }

func (f *fakeUsers) FindByID(_ context.Context, id string) (*domain.User, error) {
	for i := range f.users {
		if f.users[i].ID == id {
			return &f.users[i], nil
		}
	}
	return nil, errNotFound
}

func newAccessSvc(store *fakeAccessStore, users []domain.User) *AccessService {
	return NewAccessService(store, &fakeUsers{users: users}, authz.NewResolver(store))
}

// ---- tests ---------------------------------------------------------------

func TestForUserExplainsPermissionSource(t *testing.T) {
	store := &fakeAccessStore{
		rolePerms: map[string][]string{"hr": {
			authz.Permission("pengguna", authz.ActionView),
			authz.Permission("laporan", authz.ActionView),
		}},
		deny: []string{authz.Permission("laporan", authz.ActionView)},
		allow: []string{
			authz.Permission("laporan", authz.ActionExport),
		},
	}
	users := []domain.User{{ID: "u1", Name: "Budi", Email: "b@x", Role: domain.RoleHR, IsActive: true}}
	svc := newAccessSvc(store, users)

	got, err := svc.ForUser(context.Background(), "u1")
	if err != nil {
		t.Fatal(err)
	}

	byPerm := map[string]PermissionItem{}
	for _, it := range got.Items {
		byPerm[it.Permission] = it
	}

	if len(got.Items) != len(authz.AllPermissions()) {
		t.Fatalf("every registry permission must be listed, got %d", len(got.Items))
	}
	view := byPerm[authz.Permission("pengguna", authz.ActionView)]
	if !view.RoleGranted || view.Override != "" || !view.Effective {
		t.Fatalf("role-granted permission must be effective without override: %+v", view)
	}

	denied := byPerm[authz.Permission("laporan", authz.ActionView)]
	if !denied.RoleGranted || denied.Override != "deny" || denied.Effective {
		t.Fatalf("deny override must win and be explained: %+v", denied)
	}

	allowed := byPerm[authz.Permission("laporan", authz.ActionExport)]
	if allowed.RoleGranted || allowed.Override != "allow" || !allowed.Effective {
		t.Fatalf("allow override must be explained: %+v", allowed)
	}
}

func TestUpdateRejectsUnknownPermission(t *testing.T) {
	store := &fakeAccessStore{rolePerms: map[string][]string{}}
	svc := newAccessSvc(store, []domain.User{{ID: "u1", Role: domain.RoleHR, IsActive: true}})

	err := svc.Update(context.Background(), "actor", "u1", []PermissionChange{
		{Permission: "menu.ghost.view", Effect: "allow"},
	})
	if err == nil || !strings.Contains(err.Error(), "tidak dikenal") {
		t.Fatalf("expected unknown-permission error, got %v", err)
	}
	if len(store.setCalls) != 0 {
		t.Fatal("nothing may be written when validation fails")
	}
}

func TestUpdateRejectsInvalidEffect(t *testing.T) {
	store := &fakeAccessStore{rolePerms: map[string][]string{}}
	svc := newAccessSvc(store, []domain.User{{ID: "u1", Role: domain.RoleHR, IsActive: true}})

	err := svc.Update(context.Background(), "actor", "u1", []PermissionChange{
		{Permission: authz.Permission("laporan", authz.ActionView), Effect: "maybe"},
	})
	if err == nil {
		t.Fatal("expected invalid-effect error")
	}
	if len(store.setCalls) != 0 {
		t.Fatal("nothing may be written when validation fails")
	}
}

func TestUpdateCannotRemoveLastAdminAccess(t *testing.T) {
	store := &fakeAccessStore{rolePerms: map[string][]string{
		"super_admin": authz.AllPermissions(),
	}}
	users := []domain.User{{ID: "owner", Role: domain.RoleSuperAdmin, IsActive: true}}
	svc := newAccessSvc(store, users)

	err := svc.Update(context.Background(), "owner", "owner", []PermissionChange{
		{Permission: authz.Permission("pengguna", authz.ActionUpdate), Effect: "deny"},
	})
	if err == nil || !strings.Contains(err.Error(), "super admin") {
		t.Fatalf("expected lock-out guard, got %v", err)
	}
	if len(store.setCalls) != 0 {
		t.Fatal("guard must reject before writing")
	}
}

func TestUpdateAllowedWhenAnotherAdminRemains(t *testing.T) {
	store := &fakeAccessStore{rolePerms: map[string][]string{
		"super_admin": authz.AllPermissions(),
	}}
	users := []domain.User{
		{ID: "owner", Role: domain.RoleSuperAdmin, IsActive: true},
		{ID: "owner2", Role: domain.RoleSuperAdmin, IsActive: true},
	}
	svc := newAccessSvc(store, users)

	err := svc.Update(context.Background(), "owner", "owner", []PermissionChange{
		{Permission: authz.Permission("pengguna", authz.ActionUpdate), Effect: "deny"},
	})
	if err != nil {
		t.Fatalf("expected the change to be allowed, got %v", err)
	}
	if len(store.setCalls) != 1 {
		t.Fatalf("expected one override write, got %v", store.setCalls)
	}
}

func TestUpdateClearsOverrideOnEmptyEffect(t *testing.T) {
	store := &fakeAccessStore{
		rolePerms: map[string][]string{"hr": {authz.Permission("laporan", authz.ActionView)}},
		deny:      []string{authz.Permission("laporan", authz.ActionView)},
	}
	svc := newAccessSvc(store, []domain.User{{ID: "u1", Role: domain.RoleHR, IsActive: true}})

	if err := svc.Update(context.Background(), "actor", "u1", []PermissionChange{
		{Permission: authz.Permission("laporan", authz.ActionView), Effect: ""},
	}); err != nil {
		t.Fatal(err)
	}
	if len(store.clearCalls) != 1 || store.clearCalls[0][1] != authz.Permission("laporan", authz.ActionView) {
		t.Fatalf("expected the override to be cleared, got %v", store.clearCalls)
	}
}

func TestForUserRejectsUnknownUser(t *testing.T) {
	svc := newAccessSvc(&fakeAccessStore{rolePerms: map[string][]string{}}, nil)
	if _, err := svc.ForUser(context.Background(), "ghost"); err == nil {
		t.Fatal("expected not-found error")
	}
}
