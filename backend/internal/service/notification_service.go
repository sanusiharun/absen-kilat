// Package service holds the application use cases (business logic).
package service

import (
	"context"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
)

// NotificationService handles the Notifications feature.
type NotificationService struct {
	repo domain.NotificationRepository
}

// NewNotificationService creates a NotificationService.
func NewNotificationService(repo domain.NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo}
}

// List returns a user's notifications, newest first.
func (s *NotificationService) List(ctx context.Context, userID string) ([]domain.Notification, error) {
	return s.repo.ListByUser(ctx, userID)
}

// UnreadCount returns how many notifications a user has not read.
func (s *NotificationService) UnreadCount(ctx context.Context, userID string) (int, error) {
	return s.repo.UnreadCount(ctx, userID)
}

// MarkRead marks a single notification (id != "") or all of a user's
// notifications (id == "") as read.
func (s *NotificationService) MarkRead(ctx context.Context, userID, id string) error {
	return s.repo.MarkRead(ctx, userID, id)
}

// Create adds a notification for a user (e.g. on check-in). Returns the new
// notification with its generated id/timestamp.
func (s *NotificationService) Create(ctx context.Context, userID, title, body string) (*domain.Notification, error) {
	n := &domain.Notification{
		UserID: userID,
		Title:  title,
		Body:   body,
	}
	if err := s.repo.Create(ctx, n); err != nil {
		return nil, err
	}
	return n, nil
}
