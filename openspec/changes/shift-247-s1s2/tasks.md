# Tasks: shift-247-s1s2

Status: **G2 pending**. Semua task di bawah BLOCKED sampai G1 dan G2 disetujui owner
(format `APPROVE shift-247-s1s2 G1|G2 <digest> <base_sha>` di DM). Satu PR, satu worker,
branch `feat/shift-247-s1s2`, base `bf8c3d8f`.

## Task

- [ ] **T1** `backend/internal/db/migrations/017_shift.sql` — tabel `shifts`, `shift_assignments`,
      kolom `attendances.shift_id` + index seperti `design.md` §2. Additive, tanpa backfill.
      Bukti: V1.
- [ ] **T2** `018_seed_shift_permissions.sql` — grant idempoten `menu.shift.view/create/update` ke
      `super_admin` (pola `013_seed_akses_permissions.sql`). Bukti: V2.
- [ ] **T3** `domain/shift.go` — struct `Shift` + interface `ShiftRepository` (`List`, `FindByID`,
      `Create`, `Update`, `SetActive`). Tidak ada dependensi ke `domain.Attendance`.
- [ ] **T4** `repo/shift_repo.go` — implementasi SQLite di atas `*sql.DB`, kolom konstanta, scan DAO
      (pola `location_repo.go`), `ORDER BY code ASC`, `List(all bool)`.
- [ ] **T5** `service/shift_service.go` — validasi murni (`time.Parse("15:04")`, `start != end`,
      `crosses_midnight = end <= start`, rentang grace/break, whitelist kode) + cek lokasi via
      `domain.LocationRepository`; error `pkgerrors.ErrValidation`/`ErrNotFound`. Bukti: V4, V5, V6.
- [ ] **T6** `authz/registry.go` + `authz/routes.go` — menu `shift` (`view/create/update`, ScopeAll) dan
      3 route rule. Bukti: V7 (`go test ./internal/authz`).
- [ ] **T7** `transport/shift_handler.go` + registrasi di `cmd/server/main.go` — `GET/POST /admin/shifts`,
      `PATCH /admin/shifts/{id}`; respons proyeksi eksplisit. Bukti: V7, V8.
- [ ] **T8** Tes: `service/shift_service_test.go` (validasi + lokasi), `service/shift_repo_test.go` atau
      tes repo setara (kode duplikat case-insensitive, list filter aktif), tes handler otorisasi
      (401/403/sukses per peran). Bukti: V3–V8.
- [ ] **T9** `docs/reengineering/INDEX.md` — tautkan change ini (satu baris, status G1/G2 pending).
- [ ] **T10** Jalankan `verification.md` §3 seluruhnya dan tempel output nyata di body PR.

## Batasan eksekusi

- Hanya berkas yang disebut T1–T9; **tidak** menyentuh `attendance_service.go`, `admin_service.go`,
  `admin_handler.go`, frontend, mobile, atau `settings`.
- Tidak ada dependency Go baru, tidak ada migrasi lain, tidak ada perubahan `attendances.status`.
- Tes harus deterministik (tanpa `time.Now()` untuk aturan shift; hanya DB `:memory:`/temp file).
- Bila tes baseline merah, catat sebagai baseline issue dan jangan menutupinya dengan `t.Skip`.
- Gagal 2× berturut-turut atau telemetri/budget tidak jelas ⇒ berhenti, laporkan `BLOCKED`.
