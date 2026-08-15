package transport

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/pkg/httpx"
)

// notificationService is the subset of service.NotificationService used here.
type notificationService interface {
	List(ctx context.Context, userID string) ([]domain.Notification, error)
	UnreadCount(ctx context.Context, userID string) (int, error)
	MarkRead(ctx context.Context, userID, id string) error
}

// NotificationHandler wires notification endpoints onto an authenticated router.
type NotificationHandler struct {
	svc notificationService
}

// NewNotificationHandler creates a NotificationHandler.
func NewNotificationHandler(svc notificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

// RegisterRoutes mounts notification routes under an authenticated router.
func (h *NotificationHandler) RegisterRoutes(r chi.Router) {
	r.Get("/notifications", h.list)
	r.Get("/notifications/unread-count", h.unreadCount)
	r.Post("/notifications/{id}/read", h.markRead)
	r.Post("/notifications/read-all", h.markReadAll)
}

func (h *NotificationHandler) list(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	list, err := h.svc.List(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, notificationListResponse(list))
}

func (h *NotificationHandler) unreadCount(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	count, err := h.svc.UnreadCount(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]int{"unread": count})
}

func (h *NotificationHandler) markRead(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.svc.MarkRead(r.Context(), userID, id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]bool{"ok": true})
}

func (h *NotificationHandler) markReadAll(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if err := h.svc.MarkRead(r.Context(), userID, ""); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteOK(w, map[string]bool{"ok": true})
}

// notificationResponse is the wire representation of a notification.
type notificationResponse struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	IsRead    bool   `json:"is_read"`
	CreatedAt string `json:"created_at"`
}

func notificationListResponse(list []domain.Notification) map[string]any {
	out := make([]notificationResponse, 0, len(list))
	for _, n := range list {
		out = append(out, notificationResponse{
			ID:        n.ID,
			Title:     n.Title,
			Body:      n.Body,
			IsRead:    n.IsRead,
			CreatedAt: n.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	return map[string]any{"notifications": out}
}
