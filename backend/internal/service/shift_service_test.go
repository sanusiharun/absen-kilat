package service

import (
	"context"
	"errors"
	"testing"

	"github.com/sanusi/absen-kilat/backend/internal/domain"
	pkgerrors "github.com/sanusi/absen-kilat/backend/pkg/errors"
)

// ---- fakes ---------------------------------------------------------------

// fakeShiftStore is an in-memory shiftStore. Duplicate codes are rejected the
// same way the SQLite repository rejects them (unique index on lower(code)).
type fakeShiftStore struct {
	items   []domain.Shift
	nextID  int
	failErr error
}

func (f *fakeShiftStore) ListShifts(context.Context) ([]domain.Shift, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}
	out := make([]domain.Shift, len(f.items))
	copy(out, f.items)
	return out, nil
}

func (f *fakeShiftStore) FindShiftByID(_ context.Context, id string) (*domain.Shift, error) {
	for i := range f.items {
		if f.items[i].ID == id {
			cp := f.items[i]
			return &cp, nil
		}
	}
	return nil, domain.ErrShiftNotFound
}

func (f *fakeShiftStore) CreateShift(_ context.Context, s *domain.Shift) error {
	if f.failErr != nil {
		return f.failErr
	}
	for i := range f.items {
		if equalFold(f.items[i].Code, s.Code) {
			return domain.ErrShiftCodeExists
		}
	}
	f.nextID++
	s.ID = "shift-" + string(rune('0'+f.nextID))
	f.items = append(f.items, *s)
	return nil
}

func (f *fakeShiftStore) UpdateShift(_ context.Context, s *domain.Shift) error {
	if f.failErr != nil {
		return f.failErr
	}
	for i := range f.items {
		if f.items[i].ID == s.ID {
			f.items[i] = *s
			return nil
		}
	}
	return domain.ErrShiftNotFound
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'a' <= ca && ca <= 'z' {
			ca -= 32
		}
		if 'a' <= cb && cb <= 'z' {
			cb -= 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// fakeLocationFinder resolves locations for the shift service.
type fakeLocationFinder struct {
	locations map[string]domain.Location
	err       error
}

func (f fakeLocationFinder) FindByID(_ context.Context, id string) (*domain.Location, error) {
	if f.err != nil {
		return nil, f.err
	}
	loc, ok := f.locations[id]
	if !ok {
		return nil, pkgerrors.NotFound("location", id)
	}
	return &loc, nil
}

func newShiftServiceForTest(store *fakeShiftStore) *ShiftService {
	return NewShiftService(store, fakeLocationFinder{locations: map[string]domain.Location{
		"loc-1": {ID: "loc-1", Name: "Kantor Pusat", Type: "office", Active: true},
		"loc-2": {ID: "loc-2", Name: "Gudang Lama", Type: "office", Active: false},
	}})
}

// ---- create --------------------------------------------------------------

// V1: nilai default owner (grace 15 menit) dipakai bila klien tidak mengirim.
func TestShiftService_CreateDefaultsGraceAndNormalisesCode(t *testing.T) {
	store := &fakeShiftStore{}
	svc := newShiftServiceForTest(store)

	sh, err := svc.CreateShift(context.Background(), ShiftInput{
		Code: "  pagi ", Name: "Shift Pagi", StartTime: "08:00", EndTime: "17:00",
	})
	if err != nil {
		t.Fatalf("CreateShift: %v", err)
	}
	if sh.GraceMinutes != DefaultGraceMinutes {
		t.Fatalf("grace = %d, want default %d", sh.GraceMinutes, DefaultGraceMinutes)
	}
	if sh.Code != "PAGI" {
		t.Fatalf("code = %q, want PAGI (dinormalisasi)", sh.Code)
	}
	if !sh.Active {
		t.Fatal("shift baru harus aktif")
	}
	if sh.CrossesMidnight {
		t.Fatal("08:00-17:00 bukan shift lintas tengah malam")
	}
}

// V2: crosses_midnight diturunkan server, bukan dari klien.
func TestShiftService_CreateDerivesCrossesMidnight(t *testing.T) {
	store := &fakeShiftStore{}
	svc := newShiftServiceForTest(store)

	night, err := svc.CreateShift(context.Background(), ShiftInput{
		Code: "MALAM", Name: "Shift Malam", StartTime: "22:00", EndTime: "06:00",
	})
	if err != nil {
		t.Fatalf("CreateShift: %v", err)
	}
	if !night.CrossesMidnight {
		t.Fatal("22:00-06:00 harus lintas tengah malam")
	}
}

// V3: semua aturan validasi proposal §4 menolak dengan satu error validasi
// multi-field, bukan error pertama saja.
func TestShiftService_ValidationRules(t *testing.T) {
	cases := []struct {
		name  string
		input ShiftInput
		field string
	}{
		{"kode kosong", ShiftInput{Code: "", Name: "x", StartTime: "08:00", EndTime: "17:00"}, "code"},
		{"kode huruf kecil/simbol", ShiftInput{Code: "pa gi!", Name: "x", StartTime: "08:00", EndTime: "17:00"}, "code"},
		{"nama kosong", ShiftInput{Code: "PAGI", Name: "  ", StartTime: "08:00", EndTime: "17:00"}, "name"},
		{"jam mulai bukan HH:MM", ShiftInput{Code: "PAGI", Name: "x", StartTime: "8:00", EndTime: "17:00"}, "start_time"},
		{"jam akhir di luar rentang", ShiftInput{Code: "PAGI", Name: "x", StartTime: "08:00", EndTime: "24:00"}, "end_time"},
		{"mulai sama dengan akhir", ShiftInput{Code: "PAGI", Name: "x", StartTime: "08:00", EndTime: "08:00"}, "end_time"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeShiftStore{}
			svc := newShiftServiceForTest(store)
			_, err := svc.CreateShift(context.Background(), tc.input)
			if !pkgerrors.IsValidation(err) {
				t.Fatalf("err = %v, want validation error", err)
			}
			v, ok := err.(*pkgerrors.ErrValidationMulti)
			if !ok {
				t.Fatalf("err type = %T, want *ErrValidationMulti", err)
			}
			if _, ok := v.Fields[tc.field]; !ok {
				t.Fatalf("field %q tidak dilaporkan: %v", tc.field, v.Fields)
			}
			if len(store.items) != 0 {
				t.Fatal("shift tidak boleh tersimpan bila validasi gagal")
			}
		})
	}
}

// V4: rentang grace/break mengikuti CHECK migrasi (0-120 / 0-480).
func TestShiftService_GraceAndBreakRanges(t *testing.T) {
	for _, tc := range []struct {
		name  string
		grace *int
		brk   *int
		field string
	}{
		{"grace negatif", ptrInt(-1), nil, "grace_minutes"},
		{"grace di atas 120", ptrInt(121), nil, "grace_minutes"},
		{"break di atas 480", nil, ptrInt(481), "break_minutes"},
		{"break negatif", nil, ptrInt(-5), "break_minutes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := newShiftServiceForTest(&fakeShiftStore{})
			in := ShiftInput{Code: "PAGI", Name: "x", StartTime: "08:00", EndTime: "17:00",
				GraceMinutes: tc.grace, BreakMinutes: tc.brk}
			_, err := svc.CreateShift(context.Background(), in)
			if !pkgerrors.IsValidation(err) {
				t.Fatalf("err = %v, want validation error", err)
			}
			v := err.(*pkgerrors.ErrValidationMulti)
			if _, ok := v.Fields[tc.field]; !ok {
				t.Fatalf("field %q tidak dilaporkan: %v", tc.field, v.Fields)
			}
		})
	}
}

// V5: kode duplikat (tanpa memandang huruf besar/kecil) menjadi 400, bukan 500.
func TestShiftService_DuplicateCodeIsValidation(t *testing.T) {
	svc := newShiftServiceForTest(&fakeShiftStore{})
	ctx := context.Background()
	if _, err := svc.CreateShift(ctx, ShiftInput{Code: "PAGI", Name: "x", StartTime: "08:00", EndTime: "17:00"}); err != nil {
		t.Fatalf("CreateShift pertama: %v", err)
	}
	_, err := svc.CreateShift(ctx, ShiftInput{Code: "pagi", Name: "y", StartTime: "09:00", EndTime: "18:00"})
	if !pkgerrors.IsValidation(err) {
		t.Fatalf("err = %v, want validation error untuk kode duplikat", err)
	}
}

// V6: lokasi harus ada dan aktif (proposal §2 S2c).
func TestShiftService_LocationMustExistAndBeActive(t *testing.T) {
	svc := newShiftServiceForTest(&fakeShiftStore{})
	missing := "loc-404"
	_, err := svc.CreateShift(context.Background(), ShiftInput{
		Code: "PAGI", Name: "x", StartTime: "08:00", EndTime: "17:00", LocationID: &missing,
	})
	if !pkgerrors.IsNotFound(err) {
		t.Fatalf("err = %v, want not found untuk lokasi tidak ada", err)
	}

	inactive := "loc-2"
	_, err = svc.CreateShift(context.Background(), ShiftInput{
		Code: "PAGI", Name: "x", StartTime: "08:00", EndTime: "17:00", LocationID: &inactive,
	})
	if !pkgerrors.IsValidation(err) {
		t.Fatalf("err = %v, want validation untuk lokasi nonaktif", err)
	}

	ok := "loc-1"
	sh, err := svc.CreateShift(context.Background(), ShiftInput{
		Code: "PAGI", Name: "x", StartTime: "08:00", EndTime: "17:00", LocationID: &ok,
	})
	if err != nil {
		t.Fatalf("CreateShift lokasi aktif: %v", err)
	}
	if sh.LocationID != "loc-1" {
		t.Fatalf("location_id = %q, want loc-1", sh.LocationID)
	}
}

// ---- update --------------------------------------------------------------

// V7: PATCH mengubah hanya field yang dikirim dan menurunkan ulang crosses_midnight.
func TestShiftService_UpdateAppliesOnlyProvidedFields(t *testing.T) {
	store := &fakeShiftStore{}
	svc := newShiftServiceForTest(store)
	ctx := context.Background()
	created, err := svc.CreateShift(ctx, ShiftInput{Code: "PAGI", Name: "Pagi", StartTime: "08:00", EndTime: "17:00"})
	if err != nil {
		t.Fatalf("CreateShift: %v", err)
	}

	nightStart, nightEnd := "22:00", "06:00"
	updated, err := svc.UpdateShift(ctx, created.ID, ShiftPatch{StartTime: &nightStart, EndTime: &nightEnd})
	if err != nil {
		t.Fatalf("UpdateShift: %v", err)
	}
	if updated.Name != "Pagi" {
		t.Fatalf("name berubah padahal tidak dikirim: %q", updated.Name)
	}
	if !updated.CrossesMidnight {
		t.Fatal("crosses_midnight harus dihitung ulang setelah jam diubah")
	}
	if updated.GraceMinutes != DefaultGraceMinutes {
		t.Fatalf("grace berubah padahal tidak dikirim: %d", updated.GraceMinutes)
	}
}

// V8: id yang tidak ada menjadi 404, bukan 500.
func TestShiftService_UpdateNotFound(t *testing.T) {
	svc := newShiftServiceForTest(&fakeShiftStore{})
	name := "Baru"
	_, err := svc.UpdateShift(context.Background(), "shift-404", ShiftPatch{Name: &name})
	if !pkgerrors.IsNotFound(err) {
		t.Fatalf("err = %v, want not found", err)
	}
}

// V8b: soft delete lewat PATCH {active:false} tetap harus melewati validasi.
func TestShiftService_DeactivateKeepsShiftValid(t *testing.T) {
	store := &fakeShiftStore{}
	svc := newShiftServiceForTest(store)
	ctx := context.Background()
	created, err := svc.CreateShift(ctx, ShiftInput{Code: "PAGI", Name: "Pagi", StartTime: "08:00", EndTime: "17:00"})
	if err != nil {
		t.Fatalf("CreateShift: %v", err)
	}
	inactive := false
	updated, err := svc.UpdateShift(ctx, created.ID, ShiftPatch{Active: &inactive})
	if err != nil {
		t.Fatalf("UpdateShift: %v", err)
	}
	if updated.Active {
		t.Fatal("shift harus nonaktif setelah PATCH {active:false}")
	}
}

// ---- list ----------------------------------------------------------------

// V9: ?all=false (default) hanya mengembalikan shift aktif.
func TestShiftService_ListFiltersInactiveByDefault(t *testing.T) {
	svc := newShiftServiceForTest(&fakeShiftStore{})
	ctx := context.Background()
	if _, err := svc.CreateShift(ctx, ShiftInput{Code: "PAGI", Name: "Pagi", StartTime: "08:00", EndTime: "17:00"}); err != nil {
		t.Fatalf("CreateShift pagi: %v", err)
	}
	off := false
	night, err := svc.CreateShift(ctx, ShiftInput{Code: "MALAM", Name: "Malam", StartTime: "22:00", EndTime: "06:00"})
	if err != nil {
		t.Fatalf("CreateShift malam: %v", err)
	}
	if _, err := svc.UpdateShift(ctx, night.ID, ShiftPatch{Active: &off}); err != nil {
		t.Fatalf("UpdateShift nonaktif: %v", err)
	}

	active, err := svc.ListShifts(ctx, false)
	if err != nil {
		t.Fatalf("ListShifts: %v", err)
	}
	if len(active) != 1 || active[0].Code != "PAGI" {
		t.Fatalf("shift aktif = %+v, want hanya PAGI", active)
	}

	all, err := svc.ListShifts(ctx, true)
	if err != nil {
		t.Fatalf("ListShifts all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("jumlah shift = %d, want 2", len(all))
	}
}

// V10: error repository yang bukan domain error diteruskan apa adanya.
func TestShiftService_PropagatesUnexpectedStoreError(t *testing.T) {
	boom := errors.New("disk penuh")
	svc := newShiftServiceForTest(&fakeShiftStore{failErr: boom})
	if _, err := svc.ListShifts(context.Background(), true); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}

func ptrInt(v int) *int { return &v }
