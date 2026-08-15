package transport

import (
	"context"
	"mime/multipart"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/service"
	"github.com/sanusi/absen-kilat/backend/pkg/httpx"
)

// attendanceService is the subset of service.AttendanceService used here.
type attendanceService interface {
	CheckIn(ctx context.Context, in service.AttendanceInput) (*domain.Attendance, error)
	CheckOut(ctx context.Context, in service.AttendanceInput) (*domain.Attendance, error)
	Today(ctx context.Context, userID string, now time.Time, loc *time.Location) (*service.TodayState, error)
	ListByDate(ctx context.Context, userID string, date time.Time, loc *time.Location) ([]domain.Attendance, error)
	Get(ctx context.Context, id string) (*domain.Attendance, error)
}

// photoStorer saves an uploaded photo and returns a relative path.
type photoStorer interface {
	Store(file multipart.File, header *multipart.FileHeader) (string, error)
}

// AttendanceHandler wires attendance endpoints onto a router.
type AttendanceHandler struct {
	svc   attendanceService
	store photoStorer
}

// NewAttendanceHandler creates an AttendanceHandler.
func NewAttendanceHandler(svc attendanceService, store photoStorer) *AttendanceHandler {
	return &AttendanceHandler{svc: svc, store: store}
}

// RegisterRoutes mounts attendance routes under an authenticated router.
func (h *AttendanceHandler) RegisterRoutes(r chi.Router) {
	r.Post("/attendance", h.create)
	r.Get("/attendance", h.list)
	r.Get("/attendance/today", h.today)
	r.Get("/attendance/{id}", h.detail)
}

type attendanceResponse struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Status     string   `json:"status"`
	PhotoURL   string   `json:"photo_url"`
	Latitude   *float64 `json:"latitude,omitempty"`
	Longitude  *float64 `json:"longitude,omitempty"`
	Address    string   `json:"address,omitempty"`
	Device         string   `json:"device,omitempty"`
	IPAddress      string   `json:"ip_address,omitempty"`
	DistanceM      *int     `json:"distance_m,omitempty"`
	Source         string   `json:"source"`
	ApprovalStatus string   `json:"approval_status"`
	LocationID     string   `json:"location_id,omitempty"`
	OccurredAt     string   `json:"occurred_at"`
}

type todayResponse struct {
	CheckedIn  bool                 `json:"checked_in"`
	CheckedOut bool                 `json:"checked_out"`
	Records    []attendanceResponse `json:"records"`
}

func (h *AttendanceHandler) create(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		httpx.WriteHTTPError(w, httpx.ErrUnauthorized)
		return
	}

	if err := r.ParseMultipartForm(16 << 20); err != nil { // 16 MB
		httpx.WriteHTTPError(w, httpx.ErrBadRequest)
		return
	}

	atype := domain.AttendanceType(r.FormValue("type"))
	if atype != domain.AttendanceCheckIn && atype != domain.AttendanceCheckOut {
		httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "type must be check_in or check_out"))
		return
	}

	photoPath := ""
	if file, header, err := r.FormFile("photo"); err == nil {
		defer file.Close()
		p, serr := h.store.Store(file, header)
		if serr != nil {
			httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", serr.Error()))
			return
		}
		photoPath = p
	}

	var lat, lng *float64
	if v := r.FormValue("lat"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err == nil {
			lat = &f
		}
	}
	if v := r.FormValue("lng"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err == nil {
			lng = &f
		}
	}

	in := service.AttendanceInput{
		UserID:     userID,
		Type:       atype,
		PhotoPath:  photoPath,
		Latitude:   lat,
		Longitude:  lng,
		Address:    r.FormValue("address"),
		Notes:      r.FormValue("notes"),
		Device:     r.FormValue("device"),
		IPAddress:  clientIP(r.RemoteAddr),
		LocationID: r.FormValue("location_id"),
		Location:   time.Local,
	}

	var (
		att *domain.Attendance
		err error
	)
	if atype == domain.AttendanceCheckIn {
		att, err = h.svc.CheckIn(r.Context(), in)
	} else {
		att, err = h.svc.CheckOut(r.Context(), in)
	}
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteCreated(w, h.toResponse(att))
}

func (h *AttendanceHandler) today(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		httpx.WriteHTTPError(w, httpx.ErrUnauthorized)
		return
	}

	st, err := h.svc.Today(r.Context(), userID, time.Now(), time.Local)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	records := make([]attendanceResponse, 0, len(st.Records))
	for _, a := range st.Records {
		records = append(records, h.toResponse(&a))
	}
	httpx.WriteOK(w, todayResponse{
		CheckedIn:  st.CheckedIn,
		CheckedOut: st.CheckedOut,
		Records:    records,
	})
}

func (h *AttendanceHandler) list(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		httpx.WriteHTTPError(w, httpx.ErrUnauthorized)
		return
	}

	dateStr := r.URL.Query().Get("date")
	date := time.Now()
	if dateStr != "" {
		parsed, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			httpx.WriteHTTPError(w, httpx.NewHTTPError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "date must be YYYY-MM-DD"))
			return
		}
		date = parsed
	}

	list, err := h.svc.ListByDate(r.Context(), userID, date, time.Local)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	records := make([]attendanceResponse, 0, len(list))
	for _, a := range list {
		records = append(records, h.toResponse(&a))
	}
	httpx.WriteOK(w, map[string]any{"records": records, "date": date.Format("2006-01-02")})
}

func (h *AttendanceHandler) detail(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		httpx.WriteHTTPError(w, httpx.ErrUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	att, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if att.UserID != userID {
		httpx.WriteHTTPError(w, httpx.ErrForbidden)
		return
	}
	httpx.WriteOK(w, h.toResponse(att))
}

// toResponse converts a domain.Attendance to the API response shape.
func (h *AttendanceHandler) toResponse(a *domain.Attendance) attendanceResponse {
	resp := attendanceResponse{
		ID:             a.ID,
		Type:           string(a.Type),
		Status:         string(a.Status),
		PhotoURL:       a.PhotoPath,
		Address:        a.Address,
		Device:         a.Device,
		IPAddress:      a.IPAddress,
		DistanceM:      a.DistanceM,
		Source:         string(a.Source),
		ApprovalStatus: string(a.ApprovalStatus),
		LocationID:     a.LocationID,
		OccurredAt:     a.OccurredAt.Format(time.RFC3339),
	}
	if a.Latitude != nil {
		resp.Latitude = a.Latitude
	}
	if a.Longitude != nil {
		resp.Longitude = a.Longitude
	}
	return resp
}

// clientIP extracts the IP from a RemoteAddr, stripping the port and handling
// IPv6 brackets (e.g. "[::1]:8080" → "::1"). Falls back to the raw value.
func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
