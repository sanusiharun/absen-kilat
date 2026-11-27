# Rencana Rilis 0.6 — Akses Menu per User (RBAC menu-level)

> **Epic:** #43 · Milestone: *Rilis 0.6 — Akses Menu per User*
> Melengkapi bagian "pembatasan menu" dari #17 (approval bertingkat tetap di #17).
>
> Tiket:
- #43 — [EPIC] Rilis 0.6 — Akses Menu per User (checkbox per menu, template per role)
- #44 — [M1][backend] Registry menu & permission (single source of truth)
- #45 — [M2][backend] Migrasi tabel akses + seed template role (behaviour-preserving)
- #46 — [M3][backend] Resolver izin + middleware RequirePermission
- #47 — [M4] GET /me/permissions + nav admin dinamis
- #48 — [M5][frontend] Edit user: checkbox tiga status per menu
- #49 — [M6] Matriks aksi per menu (view ≠ delete ≠ export)
- #50 — [M7] "Lihat akses efektif" + asal izinnya
- #51 — [M8][backend] Guardrail: jangan sampai owner terkunci sendiri
- #52 — [M9][docs] DESIGN-ACCESS-CONTROL.md: peta menu → permission → route
- #53 — [M10][backlog] Role kustom + scope data yang bisa diatur (P2)

---

# Akses Menu per User (RBAC menu-level) — Rencana Rilis 0.6

> **Untuk Hermes:** eksekusi per task dengan TDD. P0 bersifat *behaviour-preserving* —
> setelah P0 rilis, tampilan & akses tiap role harus sama persis dengan hari ini.

**Goal:** owner/super admin bisa mencentang menu mana yang boleh dibuka tiap user di web admin,
tanpa mengubah aplikasi pegawai (mobile tetap seragam).

**Architecture:** Registry menu+aksi **di kode** (`internal/authz`), penugasan **di DB**
(template per role + override per user). Backend menegakkan lewat middleware per-route;
nav admin dirender dari `GET /me/permissions`, bukan hardcoded lagi.

**Tech stack:** Go 1.26 + chi + SQLite, Next.js 16, tanpa dependency baru.

---

## 1. Kondisi hari ini (fakta)

- Nav admin **hardcoded** di `admin/components/admin-shell.tsx`: satu daftar untuk `super_admin`/`hr`,
  daftar lain (lebih pendek) untuk role lain. Tidak ada cara memberi 1 user akses tambahan.
- Role hanya 4: `super_admin`, `hr`, `supervisor`, `employee` — dan sifatnya kasar:
  `RequireRole(...)` menjaga grup route, bukan per menu.
- Belum ada tabel permission apa pun; `users.role` satu kolom teks.
- Tiket terkait yang sudah ada: **#17** (RBAC mature + approval bertingkat) — rencana ini
  mengambil bagian "pembatasan menu", sedangkan approval bertingkat tetap di #17.

## 2. Yang diambil dari referensi (dan kenapa)

| Referensi | Yang diikuti |
|---|---|
| **OWASP** (via ringkasan saasframe) | Jangan pernah memakai sembunyinya menu sebagai batas keamanan; **deny by default**, cek otorisasi di setiap request, least privilege, tanpa mengandalkan pengecekan client. |
| **Dynamics 365 Business Central — Effective Permissions** | Halaman "akses efektif" yang menjelaskan **apa yang bisa dilakukan user dan dari mana asalnya**. Ini yang mencegah salah konfigurasi. |
| **Google Workspace — prebuilt + custom admin roles** | Template role siap pakai lebih dulu; role kustom belakangan. Jangan mulai dari "semua manual". |
| **Zoho People — Roles & Profiles** | Pemisahan role (siapa) dan profil (boleh apa) supaya penugasan tidak melebar. |
| **Tiga lapis permission** (page / action / data) | Menu ≠ aksi ≠ cakupan data. Mencentang menu tidak otomatis memberi hak hapus. |
| **Filament Shield / spatie permission** | Penamaan permission `resource.action`, matriks role × permission, user boleh punya pengecualian langsung. |

## 3. Model akses (3 lapis)

1. **Lapis halaman (menu)** — `menu.<key>.view`. Menentukan nav tampil dan route halaman boleh dibuka.
2. **Lapis aksi** — `menu.<key>.<action>`: `create`, `update`, `delete`, `export`, `approve`, `override`.
3. **Lapis data (scope)** — `all` | `team` | `self`. Dipakai untuk supervisor (hanya timnya) dan
   employee (hanya dirinya). Scope tidak dicentang bebas: nilainya diturunkan dari role, bukan override.

**Resolusi izin:** `effective = (permission dari role template ∪ override allow) − override deny`,
dengan **deny menang**. Permission yang tidak ada di registry → dianggap tidak ada (deny).

## 4. Skema data

Tabel baru (migrasi `014_access_control.sql`):
- `role_permissions(role TEXT, permission_key TEXT, PRIMARY KEY(role, permission_key))`
- `user_permission_overrides(user_id TEXT, permission_key TEXT, effect TEXT CHECK(effect IN ('allow','deny')), created_by TEXT, created_at TEXT, PRIMARY KEY(user_id, permission_key))`
- `permission_audit(id, actor_id, target_user_id, permission_key, before, after, created_at)`

`users.role` **tetap dipakai** sebagai template role (tidak ada tabel `roles` di P0 — role kustom = P2).

## 5. Registry di kode (single source of truth)

`backend/internal/authz/registry.go`:
```go
type Menu struct {
    Key      string   // "pengguna"
    Label    string   // "Pengguna"
    Parent   string   // "" untuk root
    Actions  []string // create/update/delete/export/approve/override
    Scope    string   // all|team|self
}
```
Registry memuat daftar menu hari ini: dashboard, pengguna, absensi, laporan, lokasi, device, cuti,
lembur, proyek, approval, pengaturan, akun-admin (+ nanti `tim`, `checkpoint`, `maintenance`).
**Menu baru tidak bisa muncul tanpa entry registry** — ini yang mencegah "menu ada tapi tak terjaga".

## 6. Task

### Fase P0 — Fondasi tanpa mengubah perilaku

**M1. Registry menu & permission**
- Files: `backend/internal/authz/registry.go` (baru), `registry_test.go`.
- Tes: `TestRegistryUniqueKeys`, `TestRegistryActionsValid`, `TestRegistryCoversAdminRoutes`
  (setiap route di grup `/admin` punya mapping permission — gagal kalau ada yang lupa).

**M2. Migrasi + seed template role**
- Files: `backend/internal/db/migrations/014_access_control.sql`,
  `backend/internal/db/seed_access.go` (idempoten, dijalankan saat start).
- Seed meniru akses hari ini: `super_admin` semua; `hr` semua kecuali `maintenance`;
  `supervisor` dashboard/approval/cuti/lembur/absensi/laporan(team)/tim; `employee` (tanpa web admin).
- Tes: `TestSeedIdempotent`, `TestSeedMatchesCurrentNav`.

**M3. Resolver + middleware**
- Files: `backend/internal/authz/resolver.go`, `backend/internal/transport/middleware_permission.go`
  (baru), `admin_handler.go` & handler lain (pasang `RequirePermission`), `cmd/api/main.go`.
- Tes: `TestEffectivePermissionsDenyWins`, `TestPermissionDeniedReturns403`,
  `TestUnknownPermissionDenied`, `TestEveryAdminRouteGuarded`.

**M4. `GET /me/permissions` + nav dinamis**
- Files: `backend/internal/transport/profile_handler.go` (atau `auth`), `admin/lib/api.ts`,
  `admin/components/admin-shell.tsx` (hapus daftar hardcoded, render dari server).
- Tes: role `hr` vs `supervisor` menghasilkan nav berbeda; user tanpa permission tidak melihat menu.
- Acceptance: setelah P0, nav tiap role **identik** dengan hari ini (regresi visual dicek manual).

### Fase P1 — Override per user (inti permintaan)

**M5. UI edit user: checkbox tiga status**
- Files: `admin/app/pengguna/page.tsx` (modal edit), komponen baru `admin/components/menu-access.tsx`.
- Tiap menu: `[ ] / [✓]` + indikator **"dari role"** vs **"override"**, plus aksi "kembalikan ke role".
- Aksi per menu bisa dibuka (accordion) — minimal `view` dulu, aksi lain menyusul di M6.
- Tes: komponen diuji lewat build + skenario manual; API: `PATCH /admin/users/{id}/permissions`.

**M6. Matriks aksi per menu (create/update/delete/export/approve/override)**
- Files: `admin/components/menu-access.tsx`, `backend/internal/transport/access_handler.go` (baru).
- Acceptance: mencentang `view` saja tidak memberi tombol hapus; `export` terpisah dari `view`.

**M7. "Lihat akses efektif"**
- Files: `backend/internal/transport/access_handler.go` (`GET /admin/users/{id}/effective-permissions`),
  modal read-only di `admin/app/pengguna/page.tsx`.
- Isi: daftar permission + **asalnya** (`role: hr` / `override allow` / `ditolak oleh override`).
- Ini meniru *Effective Permissions* Dynamics 365; tujuannya supaya owner tidak menebak-nebak.

**M8. Guardrail anti salah-kunci**
- Super admin tidak bisa mencabut `menu.maintenance.*`/`akses` miliknya sendiri (tes).
- Perubahan permission butuh konfirmasi dua langkah di UI untuk aksi sensitif (`export`, `delete`, `override`).
- Tes: `TestCannotRevokeOwnAdminAccess`, `TestAtLeastOneSuperAdminRemains`.

### Fase P2 — Setelah POC (jangan sekarang)

**M9. Role kustom** (`roles` table + `user_roles`), hanya kalau jumlah user bersamaan >5 butuh kombinasi sama.
**M10. Scope data yang bisa diatur** (mis. supervisor lintas proyek), setelah modul shift ada.
**M11. Audit perubahan akses → tampil di menu Maintenance** (menyambung ke rencana menu maintenance,
jangan bikin dua sistem log).

## 7. Dokumentasi
- `docs/DESIGN-ACCESS-CONTROL.md`: peta menu → permission → route, cara menambah menu baru
  (wajib: registry + middleware + seed), dan arti tiga lapis (page/action/data).

## 8. Risiko
- **Salah konfigurasi mengunci owner.** Mitigasi: M8 + seed idempoten + test "minimal satu super admin".
- **Menu baru lupa dijaga.** Mitigasi: `TestRegistryCoversAdminRoutes` yang membaca router dan gagal
  kalau ada route `/admin` tanpa permission.
- **Override menumpuk jadi sulit diaudit.** Mitigasi: halaman daftar user-yang-punya-override (M6),
  dan rekomendasi resmi "≥3 user dengan kombinasi sama → jadikan role" (P2).
- **Akses lewat URL langsung.** Mitigasi: semua route di-guard server-side (bukan hanya nav disembunyikan).

## 9. Verifikasi tiap fase
```bash
cd ~/absen-kilat/backend && go build ./... && go vet ./... && go test ./...
cd ~/absen-kilat/admin && npm run lint -- --max-warnings=0 && npm run build
```
Plus uji manual matriks: 4 role × 12 menu, dan 1 user dengan 2 override, dicatat di PR.
