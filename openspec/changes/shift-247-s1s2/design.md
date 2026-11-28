# Design: shift-247-s1s2

Status: **PROPOSED — G2 pending**. Base `bf8c3d8f8842d9fdf57b891f47dc1442093786a6`.
Desain ini hanya mencakup S1 (skema) dan S2 (domain/repo/service/CRUD + permission). S3–S7 di luar cakupan.

## 1. Preseden yang diikuti (bukan desain baru)

| Kebutuhan | Preseden di repo |
|---|---|
| CRUD master data admin | `AdminService.ListLocations/CreateLocation/UpdateLocation` (`admin_service.go:724-810`) + `LocationRepository` (`repo/location_repo.go`) |
| Soft delete | `LocationRepository.SetActive` (`location_repo.go:126`), `DELETE /admin/locations/{id}` → `active=0` |
| Permission menu | `internal/authz/registry.go` (`menus`, `Permission(menu, action)`) + `routes.go` |
| Grant untuk instance yang sudah ter-seed | `013_seed_akses_permissions.sql` (INSERT ... WHERE NOT EXISTS, idempoten) |
| Migrasi | runner embedded berurutan + `schema_migrations` (`db.go:60-130`), tiap berkas dalam satu transaksi |
| Validasi HTTP | `pkgerrors.ErrValidation`/`ErrNotFound` di service, `httpx.NewHTTPError(422, "VALIDATION_ERROR", …)` di handler |

## 2. Skema — `backend/internal/db/migrations/017_shift.sql`

```sql
-- 017_shift.sql — S1 EPIC #60 (tiket #61). Additive: tidak ada data lama yang diubah.
CREATE TABLE IF NOT EXISTS shifts (
    id               TEXT PRIMARY KEY,
    code             TEXT NOT NULL,                 -- "PAGI", "MALAM", "JAGA-A"
    name             TEXT NOT NULL,
    start_time       TEXT NOT NULL,                 -- "HH:MM" waktu lokal
    end_time         TEXT NOT NULL,                 -- "HH:MM" waktu lokal
    crosses_midnight INTEGER NOT NULL DEFAULT 0,    -- diturunkan server: end_time <= start_time
    grace_minutes    INTEGER NOT NULL DEFAULT 15,   -- keputusan owner 2026-11-27
    break_minutes    INTEGER NOT NULL DEFAULT 0,
    location_id      TEXT REFERENCES locations(id), -- NULL = berlaku semua lokasi
    active           INTEGER NOT NULL DEFAULT 1,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_shifts_code_lower ON shifts (lower(code));
CREATE INDEX IF NOT EXISTS idx_shifts_location ON shifts (location_id, active);

CREATE TABLE IF NOT EXISTS shift_assignments (
    id         TEXT PRIMARY KEY,
    shift_id   TEXT NOT NULL REFERENCES shifts(id),
    user_id    TEXT REFERENCES users(id),
    partner_id TEXT REFERENCES partners(id),
    start_date TEXT NOT NULL,                 -- YYYY-MM-DD, inklusif
    end_date   TEXT NOT NULL,                 -- YYYY-MM-DD, inklusif
    source     TEXT NOT NULL DEFAULT 'manual',
    created_by TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    CHECK (user_id IS NOT NULL OR partner_id IS NOT NULL),
    CHECK (end_date >= start_date)
);

CREATE INDEX IF NOT EXISTS idx_shift_assignments_user
    ON shift_assignments (user_id, start_date, end_date);
CREATE INDEX IF NOT EXISTS idx_shift_assignments_partner
    ON shift_assignments (partner_id, start_date, end_date);
CREATE INDEX IF NOT EXISTS idx_shift_assignments_shift
    ON shift_assignments (shift_id, start_date);

ALTER TABLE attendances ADD COLUMN shift_id TEXT REFERENCES shifts(id);
CREATE INDEX IF NOT EXISTS idx_attendances_shift
    ON attendances (shift_id) WHERE shift_id IS NOT NULL;
```

Catatan yang mengikat implementasi:

1. `attendances` **tetap** tanpa kolom `date` dan tanpa unique constraint baru. Kunci per hari baru
   masuk di S3/S4 (butuh rebuild tabel karena `status` CHECK harus ditambah `early_leave`).
2. `crosses_midnight` disimpan (bukan dihitung saat query) supaya laporan dan roster tidak menebak;
   nilainya **selalu** diturunkan server dari `end_time <= start_time`, input klien diabaikan.
3. `locations` punya FK; `partners`/`users` juga. Runner menjalankan migrasi dengan
   `PRAGMA foreign_keys=OFF` (`db.go:104-112`), jadi pelanggaran FK **tidak** tertangkap saat migrasi —
   validasi `location_id` wajib di service (V6).
4. Tidak ada backfill `shift_id`; baris lama tetap `NULL` = perilaku lama (`CUTOFF_HOUR`).

## 3. Kontrak HTTP

### 3.1 `GET /admin/shifts?all=false|true`

- `all` default `false` → hanya `active=1`. Nilai selain `true`/`false` → `422 VALIDATION_ERROR`
  (preseden `status` di `listUsers`).
- `200 {"shifts":[…]}` urut `code ASC`; `location_id` sebagai string `""` bila NULL (pola
  `assignmentDAO`/`toAdminUserResponse` yang memakai string kosong, bukan `null`).

### 3.2 `POST /admin/shifts`

Body: `{code, name, start_time, end_time, grace_minutes?, break_minutes?, location_id?, active?}`

| Aturan | Error |
|---|---|
| `code` wajib, 2–20 karakter `A–Z0–9-` (dinormalisasi `upper`, trim) | 422 |
| `code` unik case-insensitive (index `lower(code)`; tabrakan → `ErrValidation`) | 422 |
| `name` wajib, ≤80 karakter | 422 |
| `start_time`/`end_time` wajib `^([01]\d|2[0-3]):[0-5]\d$` | 422 |
| `start_time != end_time` (shift 24 jam tidak didukung; shift 24/7 = penugasan bergilir) | 422 |
| `grace_minutes` 0–120 (default 15), `break_minutes` 0–480 (default 0) | 422 |
| `location_id` kosong → NULL; terisi → harus ada di `locations` | 404 |

`201 {"shift":{…}}`.

### 3.3 `PATCH /admin/shifts/{id}`

- Hanya field yang dikirim yang berubah (pointer/`map[string]any` + whitelist field) — pola yang sama
  dengan update lokasi (field opsional) tanpa menimpa `code` secara tidak sengaja.
- `active:false` = non-aktifkan (soft delete). Shift yang sudah dirujuk `attendances.shift_id` atau
  `shift_assignments` **boleh** dinonaktifkan, tetapi tidak dihapus — riwayat tetap utuh.
- `200 {"shift":{…}}`; id tidak ada → `404`.

### 3.4 Tidak ada di change ini

`DELETE`, roster (`/admin/roster`), `/my/shift/today`, perhitungan telat. Semua butuh G1/G2 terpisah
(S5/S6/S3).

## 4. RBAC wiring (harus mendarat bersamaan)

1. `internal/authz/registry.go` → tambah menu:
   `{Key: "shift", Label: "Shift", Href: "/shift", Actions: []Action{ActionView, ActionCreate, ActionUpdate}, Scope: ScopeAll}`.
   Tanpa `ActionDelete` karena tidak ada route hapus (sesuai teks tiket #62).
2. `internal/authz/routes.go` → tambah 3 rule:
   `GET /admin/shifts` → `menu.shift.view`; `POST /admin/shifts` → `menu.shift.create`;
   `PATCH /admin/shifts/{id}` → `menu.shift.update`.
3. `018_seed_shift_permissions.sql` → grant ketiga permission ke `super_admin` (idempoten, pola `013`).
   **Tidak** memberi grant ke `hr`/`supervisor`: siapa yang boleh mengelola master shift adalah
   keputusan owner (Q1) dan tidak boleh ditebak agent.
4. Konsekuensi terverifikasi: `access_handler.go:66-70` menyembunyikan menu tanpa `menu.shift.view`,
   jadi menambah menu tidak membocorkan navigasi ke role lain.

Urutan aman: registry+rules+grant → handler; kalau handler mendarat tanpa rule, middleware menolak
(`TestAuthorizeDeniesUnmappedAdminRoute`), bukan diam-diam membuka akses.

## 5. Detail implementasi

| Berkas | Isi |
|---|---|
| `domain/shift.go` | `Shift{ID,Code,Name,StartTime,EndTime,CrossesMidnight,GraceMinutes,BreakMinutes,LocationID,Active,CreatedAt,UpdatedAt}`; `ShiftRepository{List(ctx, all bool); FindByID; Create; Update; SetActive}` |
| `repo/shift_repo.go` | DAO + kolom konstan (pola `locationDAO`), `Create` memakai `uuid.NewString()` seperti repo lain, `Update` menulis `updated_at` |
| `service/shift_service.go` | `Validate()` murni (dapat dites tanpa DB) + cek `location_id` lewat `domain.LocationRepository`; error `pkgerrors.ErrValidation`/`NotFound` |
| `transport/shift_handler.go` | `RegisterRoutes(r chi.Router)` dengan `GET/POST /admin/shifts`, `PATCH /admin/shifts/{id}`; proyeksi respons eksplisit (bukan struct domain langsung) |
| `cmd/server/main.go` | `shiftRepo := repo.NewShiftRepository(pool)`, `shiftSvc := service.NewShiftService(shiftRepo, locationRepo)`, daftarkan handler |

Service **tidak** menyentuh `AttendanceService` — perubahan perilaku absensi adalah S3.

## 6. Deviasi dari dokumen sumber (dilaporkan, bukan disembunyikan)

| Dokumen | Klaim | Kenyataan |
|---|---|---|
| `features/shift-scheduling.md` §6 | migrasi `012_shift.sql` | 012 sudah terpakai → `017_shift.sql` |
| baseline §3.5 | constraint unik `(user_id, date, shift_id)` | tidak ada kolom `date`/unique constraint; ditunda ke S4 |
| baseline §3.2 | `grace_minutes` per shift | dipertahankan; default 15 (owner) |
| baseline §4 | permission `shift.lihat/tambah/ubah` | dipetakan ke konvensi repo: `menu.shift.view/create/update` |
| baseline §3.3 | status `early_leave` | butuh rebuild CHECK `attendances.status` → S3, bukan S1 |

## 7. Pertanyaan terbuka untuk owner (tidak memblokir S1/S2 kecuali Q1)

- **Q1** Role selain `super_admin` yang boleh `menu.shift.create/update` (hr? supervisor?) — default
  usulan: hanya `super_admin` sampai ada keputusan.
- **Q2** `code` shift: bebas diisi admin atau pakai enum tetap (`PAGI/SIANG/MALAM`)? Usulan: bebas,
  unik, agar kapal bisa punya `JAGA-A`.
- **Q3** Shift dengan `location_id` NULL (berlaku semua lokasi) diizinkan? Usulan: ya, karena
  penugasan manual per orang tetap menentukan.
- **Q4** Batas `break_minutes` 480 menit masuk akal? Usulan: ya, cukup longgar untuk jaga laut.
