# Proposal: shift-247-s1s2

Change ID: `shift-247-s1s2` (EPIC #60, tiket #61 S1 + #62 S2). Status: **PROPOSED — G1/G2 pending**.
Base: `bf8c3d8f8842d9fdf57b891f47dc1442093786a6` (main setelah PR #77).
Dokumen sumber (discovery, bukan requirement): `docs/reengineering/features/shift-scheduling.md`,
`docs/reengineering/backlog/INTEGRATED-BACKLOG.md` baris 12.
Keputusan owner yang **mengikat**: §8 baseline (DM 2026-11-27) — shift per lokasi, grace default 15 menit,
`CUTOFF_HOUR` tetap fallback, hari libur = lembur penuh, rilis pertama penugasan manual (bukan pola).

## 1. Masalah (terverifikasi dari kode)

1. **Tidak ada model shift sama sekali.**
   `grep -riE 'shift|roster' backend/internal/db/migrations backend/internal --include=*.go --include=*.sql | grep -v _test` → **0 baris**.
2. **Jam kerja dipaku satu titik.** `AttendanceService.cutoffHour` dari env `CUTOFF_HOUR` (default 9)
   (`backend/internal/config/config.go:46`), dipakai `attendance_service.go:232`:
   `time.Date(..., s.cutoffHour, 0, 0, 0, loc)` — semua karyawan dianggap shift pagi 09:00.
3. **Setting `work_start`/`work_end` tidak mengikat perhitungan telat.** Tersimpan di tabel `settings`
   (`006_rbac_locations.sql:39-41`) dan bisa diubah lewat `PUT /admin/settings`, tetapi tidak dipakai
   fungsi telat. Jadi admin bisa "mengubah jam kerja" tanpa efek apa pun.
4. **Belum ada permission/menu shift.** Daftar menu tertutup di `internal/authz/registry.go` (`menus`);
   route admin tanpa rule ditolak (`middleware_permission_test.go:93 TestAuthorizeDeniesUnmappedAdminRoute`).
   Artinya CRUD shift hanya bisa hidup bila menu + route rule + grant ditambahkan bersamaan.
5. **Dokumen baseline memakai nomor migrasi yang sudah terpakai.** Baseline menyebut
   `012_shift.sql`, padahal `012_akses_separation.sql` sudah ada dan migrasi terakhir adalah
   `016_email_verifications.sql`. Nomor baru yang benar: **017** (diverifikasi dari `ls backend/internal/db/migrations`).
6. **Klaim baseline `(user_id, date, shift_id)` tidak cocok dengan skema nyata.** `attendances` tidak punya
   kolom `date`; kuncinya `occurred_at` + `type IN ('check_in','check_out')` + `status IN ('on_time','late')`
   (`001_init.sql:16-26`), tanpa unique constraint per hari. Konsekuensi: S1 **hanya** menambah kolom
   `shift_id` nullable + index; perubahan kunci/`status` adalah pekerjaan S3/S4 dan butuh change sendiri.

## 2. Scope perubahan (additive; S1 + S2 dari #61/#62)

| # | Perubahan | Berkas utama |
|---|---|---|
| S1a | Migrasi `017_shift.sql`: tabel `shifts`, `shift_assignments`, kolom `attendances.shift_id` nullable + index | `backend/internal/db/migrations/017_shift.sql` |
| S1b | Migrasi `018_seed_shift_permissions.sql`: grant idempoten `menu.shift.*` ke `super_admin` (preseden `013_seed_akses_permissions.sql`) | migrasi |
| S2a | Domain `Shift` + interface `ShiftRepository` | `backend/internal/domain/shift.go` |
| S2b | Repo SQLite: list/find/create/update(+set active) dengan validasi kode unik case-insensitive | `backend/internal/repo/shift_repo.go` |
| S2c | Service: validasi `HH:MM`, `start != end`, `crosses_midnight` diturunkan server, `grace_minutes` 0–120, `break_minutes` 0–480, `location_id` harus ada & aktif | `backend/internal/service/shift_service.go` |
| S2d | Handler + route: `GET/POST /admin/shifts`, `PATCH /admin/shifts/{id}` | `backend/internal/transport/shift_handler.go` |
| S2e | Registry menu `shift` (`view`, `create`, `update`, ScopeAll) + 3 route rule | `internal/authz/registry.go`, `routes.go` |
| S2f | Wiring: repo/service/handler di `backend/cmd/server/main.go` | main.go |

Tidak ada endpoint `DELETE`: non-aktifkan shift = `PATCH {active:false}` (soft delete, preseden
`LocationRepository.SetActive`). Ini menjaga permission tetap `view/create/update` seperti teks tiket #62.

## 3. Non-goals (eksplisit)

- **S3** (telat/early-leave dari shift, fallback `CUTOFF_HOUR`), **S4** (resolusi tanggal shift lintas
  tengah malam), **S5** (roster API + UI), **S6** (`/my/shift/today`), **S7** (lembur/checkpoint):
  tidak disentuh. `attendance_service.go` **tidak diubah** oleh change ini.
- `shift_assignments` dibuat di S1 tetapi **tidak** punya repo/service/endpoint di S2 — pengisian via
  roster (#65) atau seed manual. Alasannya: aturan bentrok penugasan (dua shift beririsan untuk satu
  orang) belum disetujui; menebaknya = mengarang aturan bisnis.
- Tidak ada UI admin shift di change ini (`/shift` belum ada). Halaman dikerjakan bersama S5.
- Tidak ada perubahan `status`/CHECK `attendances`, tidak ada backfill `shift_id`, tidak ada XLSX/payroll.
- `CUTOFF_HOUR` **tidak dihapus** (keputusan owner §8.3).

## 4. Kontrak API ringkas (detail di `design.md` §3)

```
GET  /admin/shifts?all=true|false          → 200 {shifts:[{id,code,name,start_time,end_time,
                                                 crosses_midnight,grace_minutes,break_minutes,
                                                 location_id,active}]}   (menu.shift.view)
POST /admin/shifts                         → 201 {shift:{...}}            (menu.shift.create)
PATCH /admin/shifts/{id}                   → 200 {shift:{...}}            (menu.shift.update)
```

Error seragam dengan handler admin lain: `400 VALIDATION` (format jam, rentang grace/break, kode
kosong/duplikat), `404 NOT_FOUND` (id/lokasi tidak ada), `403` (tanpa permission), `401` (tanpa token).

## 5. Definisi selesai

- Migrasi 017/018 jalan pada DB kosong **dan** pada salinan DB yang sudah ter-seed (`schema_migrations`
  berisi 001–016), `PRAGMA foreign_key_check` kosong, idempoten bila dijalankan ulang.
- Semua tes invarian V1–V9 di `verification.md` PASS dengan output nyata (bukan narasi).
- `gofmt -l .` kosong, `go vet ./...` exit 0, `go test ./internal/{authz,repo,service,transport} ./cmd/...`
  hijau pada head PR; CI backend/structure/frontend hijau.
- `python3 tools/validate_sdd.py` → `STRUCTURE PASS`.

## 6. Risiko

| Risiko | Mitigasi |
|---|---|
| `ALTER TABLE attendances ADD COLUMN ... REFERENCES` pada tabel panas | kolom nullable tanpa default, tidak ada rewrite data; migrasi berjalan dalam transaksi runner (`db.go:105-127`) |
| Grant permission baru tidak sampai ke instance yang sudah `access_seed` | migrasi 018 idempoten (pola `013`), bukan mengandalkan registry |
| Menu `shift` muncul di `/me/permissions` untuk role yang belum diberi grant | menu difilter `Has(menu.shift.view)` di `access_handler.go:66-70` ⇒ tersembunyi, terverifikasi V8 |
| Kode shift duplikat karena collation | unique index pada `lower(code)` |
| `shift_assignments` dibuat tanpa aturan bentrok | tabel kosong sampai S5; CHECK hanya `end_date >= start_date` dan pemilik (`user_id` atau `partner_id`) |
| Scope `ScopeAll` untuk shift terlalu lebar | shift adalah master data (preseden `lokasi` = ScopeAll); supervisor tidak mendapat grant default |

## 7. Gate

G1 (proposal) dan G2 (proposal+design+tasks+spec) **belum disetujui**. Tidak ada perubahan kode
(aplikasi, migrasi, wiring) sebelum keduanya disetujui lewat DM owner dengan format
`APPROVE shift-247-s1s2 G1|G2 <digest> bf8c3d8f...`. Digest dihitung ulang setelah berkas final
(`verification.md` §4).
