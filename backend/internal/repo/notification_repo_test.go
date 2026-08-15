package repo

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// openTestGORM opens a throwaway GORM sqlite DB (pure-Go driver) and migrates
// the notifications table, mirroring what db.Open + the 002 migration do.
func openTestGORM(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	g, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open gorm: %v", err)
	}
	if err := g.AutoMigrate(&notificationRow{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return g
}

func TestNotificationRepository(t *testing.T) {
	ctx := context.Background()
	r := NewNotificationRepository(openTestGORM(t))

	// Create two notifications for user A and one for user B.
	a1 := &domain.Notification{UserID: "user-a", Title: "Absen masuk", Body: "08:00"}
	a2 := &domain.Notification{UserID: "user-a", Title: "Absen keluar", Body: "17:00"}
	b1 := &domain.Notification{UserID: "user-b", Title: "Salam", Body: "Hai"}
	for _, n := range []*domain.Notification{a1, a2, b1} {
		if err := r.Create(ctx, n); err != nil {
			t.Fatalf("Create(%s): %v", n.Title, err)
		}
		if n.ID == "" {
			t.Fatal("Create should set an id")
		}
	}

	// Unread counts are scoped per user.
	if c, _ := r.UnreadCount(ctx, "user-a"); c != 2 {
		t.Fatalf("user-a unread = %d, want 2", c)
	}
	if c, _ := r.UnreadCount(ctx, "user-b"); c != 1 {
		t.Fatalf("user-b unread = %d, want 1", c)
	}

	// List returns only that user's, newest first.
	list, err := r.ListByUser(ctx, "user-a")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len(list)=%d, want 2", len(list))
	}
	if list[0].Title != "Absen keluar" { // created second -> newest
		t.Fatalf("newest = %q, want %q", list[0].Title, "Absen keluar")
	}

	// Mark a single notification read.
	if err := r.MarkRead(ctx, "user-a", list[0].ID); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if c, _ := r.UnreadCount(ctx, "user-a"); c != 1 {
		t.Fatalf("user-a unread after single = %d, want 1", c)
	}

	// Marking someone else's id is not found.
	if err := r.MarkRead(ctx, "user-a", b1.ID); !pkgerrors.IsNotFound(err) {
		t.Fatalf("MarkRead other-user id: got %v, want not-found", err)
	}

	// Mark all read.
	if err := r.MarkRead(ctx, "user-a", ""); err != nil {
		t.Fatalf("MarkRead all: %v", err)
	}
	if c, _ := r.UnreadCount(ctx, "user-a"); c != 0 {
		t.Fatalf("user-a unread after all = %d, want 0", c)
	}
}
