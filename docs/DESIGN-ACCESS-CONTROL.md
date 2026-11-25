# Desain: Akses Menu (RBAC menu-level)

Dokumen ini menjelaskan bagaimana akses menu web admin ditentukan, ditegakkan, dan diubah.
Rencana asal: `docs/plans/2026-11-26-akses-menu-per-user.md` (EPIC #43).

## Ringkasan

- **Menu & permission didefinisikan di kode** (`backend/internal/authz`), **penugasan di database**
  (`role_permissions`, `user_permission_overrides`).
- Frontend **tidak menyimpan daftar menu sendiri**: nav dirender dari `GET /me/permissions`.
- Backend **selalu menolak** route yang permission-nya tidak dimiliki. Menyembunyikan menu di UI
  bukan kontrol keamanan.

## Tiga lapis

1. **Halaman** — `menu.<menu>.view`: menentukan menu tampil dan route halaman boleh dibuka.
2. **Aksi** — `menu.<menu>.<aksi>`: `create`, `update`, `delete`, `export`, `approve`, `override`.
   Bisa memberi `view` tanpa `export` (mis. laporan boleh dilihat, tidak boleh diunduh).
3. **Data (scope)** — `all` | `team` | `self`. Scope **diturunkan dari role**, tidak bisa dicentang
   bebas; pembatasan data nyata tetap dilakukan service (mis. supervisor hanya timnya).

## Cara izin dihitung

```
effective = (permission dari role template ∪ override allow) − override deny
```

- **Deny menang.** Aman memberi role lebar lalu mencabut menu tertentu per user.
- Permission yang tidak ada di registry **selalu ditolak** (tidak bisa "dikarang" di DB).
- Override kosong (`""`) berarti **ikut role** (inherit).

## Tabel data

| Tabel | Isi |
|---|---|
| `role_permissions` | template per role (`super_admin`, `hr`, `supervisor`, `employee`) |
| `user_permission_overrides` | pengecualian per user: `allow` atau `deny` |
| `permission_audit` | siapa mengubah apa: actor, target, permission, before, after |
| `access_seed` | penanda template sudah di-seed sekali (agar pencabutan tidak dibatalkan saat restart) |

Seed awal (idempoten, sekali): `super_admin` 26 izin · `hr` 24 · `supervisor` 9 · `employee` 2.

## Peta menu → permission → route

| Menu | Key | Route utama |
|---|---|---|
| Dashboard | `dashboard` | `GET /admin/stats`, `GET /admin/trends`, `GET /admin/locations/breakdown` |
| Pengguna | `pengguna` | `GET/POST /admin/users`, `PATCH /admin/users/{id}`, `GET/PATCH /admin/users/{id}/permissions` |
| Absensi | `absensi` | `GET /admin/attendance`, `PATCH /admin/attendance/{id}` |
| Laporan | `laporan` | `GET /admin/reports`, `GET /admin/reports/users` |
| Lokasi | `lokasi` | `/admin/locations*`, `/admin/qr*` |
| Device | `device` | `GET /admin/devices` |
| Cuti | `cuti` | `/admin/leave*` |
| Lembur | `lembur` | `/admin/overtime*` |
| Proyek & Mitra | `proyek` | `/admin/partners*`, `/admin/projects*`, `/admin/assignments*` |
| Approval | `approval` | `/admin/team`, `/admin/approvals*` |
| Pengaturan | `pengaturan` | `GET/PUT /admin/settings` |
| Profil | `akun-admin` | halaman profil (tidak punya endpoint admin) |

Tabel otoritatif ada di `backend/internal/authz/routes.go`.

## Menambah menu baru (5 langkah)

1. Tambah entry di `internal/authz/registry.go` (`Key`, `Label`, `Href`, `Actions`, `Scope`).
2. Tambah route-nya di `internal/authz/routes.go`.
3. Pasang `Authorize(...)` — sudah otomatis untuk grup admin; pastikan route didaftarkan di grup itu.
4. Seed: permission baru **tidak** otomatis masuk template lama. Berikan lewat UI Akses Menu, atau
   tambahkan ke `DefaultRoleTemplates()` (hanya berlaku untuk instalasi baru).
5. Verifikasi: `go test ./internal/authz/ ./internal/transport/` + buka `GET /me/permissions`.

## Guardrail

- **Owner tidak bisa mengunci diri sendiri**: perubahan ditolak bila menghasilkan tidak ada satu pun
  `super_admin` aktif yang masih punya `menu.pengguna.view` + `menu.pengguna.update`.
- Setiap perubahan override dicatat di `permission_audit`.
- **Deny by default**: route `/admin/*` yang belum punya mapping di `routes.go` ditolak dan
  dilaporkan sebagai warning di log — lupa mendaftarkan permission tidak boleh membuka endpoint.

## Aturan praktis untuk owner

- Pakai **template role** dulu; override hanya untuk pengecualian.
- Kalau ≥3 user butuh kombinasi menu yang sama, buat role baru (backlog #53), jangan override
  berulang — override yang menumpuk sulit diaudit.

## Batasan saat ini

- Scope data (`team`/`self`) belum bisa diatur per user; masih mengikuti service yang ada.
- Role kustom belum ada (backlog #53).
- Halaman `/qr` tidak muncul sebagai menu terpisah; endpoint-nya ikut kendali menu `lokasi`.
