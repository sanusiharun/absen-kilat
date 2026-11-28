package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	"github.com/sanusi/absen-kilat/backend/internal/service"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// ---- fake service --------------------------------------------------------

type fakeShiftService struct {
	shifts []domain.Shift
	err    error

	lastAll  bool
	lastIn   service.ShiftInput
	lastID   string
	lastPtch service.ShiftPatch
}

func (f *fakeShiftService) ListShifts(_ context.Context, all bool) ([]domain.Shift, error) {
	f.lastAll = all
	if f.err != nil {
		return nil, f.err
	}
	return f.shifts, nil
}

func (f *fakeShiftService) CreateShift(_ context.Context, in service.ShiftInput) (*domain.Shift, error) {
	f.lastIn = in
	if f.err != nil {
		return nil, f.err
	}
	return &domain.Shift{ID: "s1", Code: "PAGI", Name: in.Name, StartTime: in.StartTime,
		EndTime: in.EndTime, GraceMinutes: service.DefaultGraceMinutes, Active: true}, nil
}

func (f *fakeShiftService) UpdateShift(_ context.Context, id string, p service.ShiftPatch) (*domain.Shift, error) {
	f.lastID = id
	f.lastPtch = p
	if f.err != nil {
		return nil, f.err
	}
	return &domain.Shift{ID: id, Code: "PAGI", Name: "Pagi", StartTime: "08:00",
		EndTime: "17:00", GraceMinutes: service.DefaultGraceMinutes, Active: false}, nil
}

func shiftRouter(svc shiftService) http.Handler {
	r := chi.NewRouter()
	NewShiftHandler(svc).RegisterRoutes(r)
	return r
}

func doShift(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd *strings.Reader = strings.NewReader(body)
	req := httptest.NewRequest(method, target, rd)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// ---- list ----------------------------------------------------------------

// H1: GET /admin/shifts default = hanya shift aktif.
func TestShiftHandler_ListDefaultsToActiveOnly(t *testing.T) {
	svc := &fakeShiftService{shifts: []domain.Shift{{ID: "s1", Code: "PAGI", Active: true}}}
	rec := doShift(t, shiftRouter(svc), http.MethodGet, "/admin/shifts", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if svc.lastAll {
		t.Fatal("all harus false bila parameter tidak dikirim")
	}
	var out struct {
		Data struct {
			Shifts []struct {
				Code            string `json:"code"`
				CrossesMidnight bool   `json:"crosses_midnight"`
			} `json:"shifts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Data.Shifts) != 1 || out.Data.Shifts[0].Code != "PAGI" {
		t.Fatalf("payload tak terduga: %s", rec.Body.String())
	}
}

// H2: ?all=true diteruskan ke service; nilai lain ditolak 422.
func TestShiftHandler_ListAllFlagValidation(t *testing.T) {
	svc := &fakeShiftService{}
	rec := doShift(t, shiftRouter(svc), http.MethodGet, "/admin/shifts?all=true", "")
	if rec.Code != http.StatusOK || !svc.lastAll {
		t.Fatalf("all=true: status %d, lastAll %v", rec.Code, svc.lastAll)
	}

	rec = doShift(t, shiftRouter(&fakeShiftService{}), http.MethodGet, "/admin/shifts?all=yes", "")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("all=yes: status = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "VALIDATION_ERROR") {
		t.Fatalf("kode error tak terduga: %s", rec.Body.String())
	}
}

// ---- create --------------------------------------------------------------

// H3: POST membuat shift dan mengembalikan 201 dengan proyeksi eksplisit.
func TestShiftHandler_CreateReturns201Projection(t *testing.T) {
	svc := &fakeShiftService{}
	body := `{"code":"PAGI","name":"Shift Pagi","start_time":"08:00","end_time":"17:00","grace_minutes":10}`
	rec := doShift(t, shiftRouter(svc), http.MethodPost, "/admin/shifts", body)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (%s)", rec.Code, rec.Body.String())
	}
	if svc.lastIn.Code != "PAGI" || svc.lastIn.GraceMinutes == nil || *svc.lastIn.GraceMinutes != 10 {
		t.Fatalf("input tidak ter-parse: %+v", svc.lastIn)
	}
	if !strings.Contains(rec.Body.String(), `"grace_minutes":15`) {
		t.Fatalf("proyeksi response tak terduga: %s", rec.Body.String())
	}
}

// H4: body rusak = 400, bukan 500.
func TestShiftHandler_CreateRejectsMalformedJSON(t *testing.T) {
	rec := doShift(t, shiftRouter(&fakeShiftService{}), http.MethodPost, "/admin/shifts", `{"code":`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// H5: error validasi dari service dipetakan ke 422 VALIDATION_ERROR.
func TestShiftHandler_CreateMapsValidationError(t *testing.T) {
	svc := &fakeShiftService{err: pkgerrors.ValidationMulti(map[string]string{"start_time": "must be HH:MM (00:00-23:59)"})}
	rec := doShift(t, shiftRouter(svc), http.MethodPost, "/admin/shifts", `{"code":"PAGI"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (%s)", rec.Code, rec.Body.String())
	}
}

// ---- update --------------------------------------------------------------

// H6: PATCH memakai id dari path dan hanya meneruskan field yang dikirim.
func TestShiftHandler_UpdateUsesPathID(t *testing.T) {
	svc := &fakeShiftService{}
	rec := doShift(t, shiftRouter(svc), http.MethodPatch, "/admin/shifts/shift-9", `{"active":false}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if svc.lastID != "shift-9" {
		t.Fatalf("id = %q, want shift-9", svc.lastID)
	}
	if svc.lastPtch.Active == nil || *svc.lastPtch.Active {
		t.Fatalf("active tidak ter-parse: %+v", svc.lastPtch)
	}
	if svc.lastPtch.Name != nil {
		t.Fatal("field yang tidak dikirim harus nil (bukan string kosong)")
	}
}

// H7: shift tidak ada = 404, bukan 500.
func TestShiftHandler_UpdateMapsNotFound(t *testing.T) {
	svc := &fakeShiftService{err: pkgerrors.NotFound("shift", "shift-9")}
	rec := doShift(t, shiftRouter(svc), http.MethodPatch, "/admin/shifts/shift-9", `{"name":"x"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (%s)", rec.Code, rec.Body.String())
	}
}
