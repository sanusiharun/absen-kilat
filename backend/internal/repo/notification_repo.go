package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// notificationRow is the GORM model for the notifications table. Times are
// stored as RFC3339 text (same convention as the raw DAO repos).
type notificationRow struct {
	ID        string `gorm:"primaryKey;column:id"`
	UserID    string `gorm:"column:user_id"`
	Title     string `gorm:"column:title"`
	Body      string `gorm:"column:body"`
	IsRead    int    `gorm:"column:is_read"`
	CreatedAt string `gorm:"column:created_at"`
}

func (notificationRow) TableName() string { return "notifications" }

// NotificationRepository is the GORM-backed implementation of
// domain.NotificationRepository.
type NotificationRepository struct {
	db *gorm.DB
}

// NewNotificationRepository creates a GORM-backed NotificationRepository.
func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Create(ctx context.Context, n *domain.Notification) error {
	if n.ID == "" {
		n.ID = uuid.NewString()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	row := notificationRow{
		ID:        n.ID,
		UserID:    n.UserID,
		Title:     n.Title,
		Body:      n.Body,
		IsRead:    boolToInt(n.IsRead),
		CreatedAt: formatTime(n.CreatedAt),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return pkgerrors.Internal(err)
	}
	return nil
}

func (r *NotificationRepository) ListByUser(ctx context.Context, userID string) ([]domain.Notification, error) {
	var rows []notificationRow
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC, id DESC").
		Find(&rows).Error; err != nil {
		return nil, pkgerrors.Internal(err)
	}
	out := make([]domain.Notification, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Notification{
			ID:        row.ID,
			UserID:    row.UserID,
			Title:     row.Title,
			Body:      row.Body,
			IsRead:    row.IsRead == 1,
			CreatedAt: parseTime(row.CreatedAt),
		})
	}
	return out, nil
}

func (r *NotificationRepository) MarkRead(ctx context.Context, userID, id string) error {
	q := r.db.WithContext(ctx).Model(&notificationRow{}).Where("user_id = ?", userID)
	if id != "" {
		q = q.Where("id = ?", id)
	}
	res := q.Update("is_read", 1)
	if res.Error != nil {
		return pkgerrors.Internal(res.Error)
	}
	if id != "" && res.RowsAffected == 0 {
		return pkgerrors.NotFound("notification", id)
	}
	return nil
}

func (r *NotificationRepository) UnreadCount(ctx context.Context, userID string) (int, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&notificationRow{}).
		Where("user_id = ? AND is_read = 0", userID).
		Count(&count).Error; err != nil {
		return 0, pkgerrors.Internal(err)
	}
	return int(count), nil
}
