# qr-geofence
Status: DISCOVERY / unapproved. Baseline c85edc9a96c637037f16e838592a6016902670c9.
## Source inventory
- backend/internal/service/qr_service.go
- backend/internal/service/geofence.go
- backend/internal/transport/qr_handler.go
## Required discovery
Document actors, permissions, trigger, normal and alternative flows, validation, persistence, API request/response/error contracts, privacy, UI and test evidence. Verify actual behavior against source and tests; distinguish observed from desired. Business rules and acceptance scenarios remain TBD until reviewed. No implementation authorization.

---

## Discovery 2026-11-27 (P0, read-only, slice 2/4)

Sumber: main `c85edc9a96c637037f16e838592a6016902670c9`. Desain lama `docs/DESIGN-QR-ATTENDANCE.md`
(issue #18, commit `1bba95a`) dipakai sebagai pembanding, bukan requirement.

### API (observed)
- Admin: `POST /admin/qr` body `{location_id}` -> 201 `{token,url}`; `GET /admin/qr`; `DELETE /admin/qr/{id}`
  (`backend/internal/transport/qr_handler.go:39-42`), di balik `RequireRole(super_admin, hr)` (`cmd/api/main.go:157-160`).
- Publik: `POST /api/v1/public/attendance/qr`, multipart, tanpa auth, di luar grup `RequireAuth`
  (`qr_handler.go:47-48`, `main.go:139-142`). Field: `token`, `email` ATAU `employee_code`, `name?`, `photo?`.
- Halaman form publik = `admin/app/qr/page.tsx` (route `/qr?t=<token>`, tanpa login).

### Aturan bisnis (observed)
1. Token = 32 byte acak hex (64 char) (`qr_service.go:68`), **bukan** UUID v7 seperti desain §3.1.
2. `expires_at` **tidak pernah diisi** saat generate (`qr_service.go:56-81`); pengecekan expiry ada
   (`:104`) tetapi belum ada API/UI untuk menetapkan masa berlaku -> token efektif permanen sampai di-revoke.
3. Base URL QR hardcoded `http://43.157.243.110:3000/qr?t=` (`qr_service.go:19`) -> IP + HTTP + port dev,
   bukan host HTTPS produksi; wajib ubah source untuk deploy baru.
4. Identitas hanya dari email/kode karyawan tanpa verifikasi apa pun; `name` opsional dan informasional
   (`qr_service.go:118-127`). Siapa pun yang memegang token dan tahu email rekan bisa absen atas namanya (inferred).
5. Jalur QR **tidak mengevaluasi geofence**: lat/lng record diisi koordinat lokasi QR, bukan GPS pelapor
   (`qr_service.go:155-160`). Presensi QR terlihat seperti absen di dalam radius.
6. `photo_policy` tidak berlaku di jalur QR: foto hanya disimpan bila dikirim (`qr_handler.go:118-131`).
7. `status` selalu `on_time` (`qr_service.go:161`), tidak dihitung dari jam masuk -> absen QR pukul 22:00 tetap on_time.
8. `approval_status` selalu `pending`; dedup satu check-in per user per hari lokal memakai `time.Local`
   (`qr_service.go:145-158`, `repo/attendance_repo.go:89-91`) -> batas hari ikut TZ server.
9. Rate limit 10 req/menit per IP, in-memory (reset saat restart) (`qr_service.go:21-37,235-265`);
   kunci IP dari `RemoteAddr` saja, `X-Forwarded-For` diabaikan (`transport/attendance_handler.go:249-255`).
   Di balik reverse proxy, semua klien berbagi satu IP -> limit praktis global (inferred; lihat `infra/caddy/Caddyfile`).

### Geofence (observed)
- `checkGeofence` tidak pernah menolak absen: hanya mengisi `DistanceM`/`Outside`/lokasi terdekat
  (`backend/internal/service/geofence.go:37-104`). Jarak haversine, radius per lokasi `radius_m`
  (default 500, `migrations/006_rbac_locations.sql:28`).
- Fallback legacy `OfficeConfig` (env) hanya bila tabel `locations` kosong (`geofence.go:91-95`).
- `location_id` terpilih yang tidak aktif/tidak dikenal -> error validasi (`geofence.go:55-63`).
- Konsekuensi (sudah dicatat di `attendance.md`): di luar radius + foto = `pending`, bukan tolak.

### Tes yang ada (observed)
Tidak ada `qr_service_test.go` dan tidak ada tes handler QR. Geofence hanya `TestGeofence`
(`attendance_service_test.go:138`): dalam radius, luar radius, tanpa koordinat. Belum ada tes untuk
lokasi terdekat, fallback legacy, batas radius eksak, token revoke/expired, dedup QR, rate limiter, atau
`photo_policy` per-jalur.

### Gap (inferred, belum requirement)
| Gap | Dampak | Usulan change |
|---|---|---|
| Base URL & port hardcoded | deploy/host baru salah tautan | `qr-config` |
| Token permanen, tanpa masa berlaku | poster lama tetap valid selamanya | `qr-token-rotation` |
| Absen QR tanpa GPS/selfie diverifikasi | titip absen massal | `qr-integrity` |
| Status QR selalu on_time | data telat hilang -> payroll salah | bergantung A1–A3 (`work_start`) |
| Rate limit per RemoteAddr | limit lumpuh di balik proxy | `proxy-aware-ip` |
| Token penuh dikembalikan `GET /admin/qr` | token bocor ke layar admin mana pun | mask + regenerate |
| Tidak ada audit generate/revoke | sulit menelusuri token bocor | `identity-audit` |
| Tidak ada tes QR | regresi tak terdeteksi | `qr-tests` |

### Unknown
Apakah backend produksi memang di balik Caddy/nginx (Caddyfile hanya mem-proxy :3000 untuk `/admin`);
apakah absen QR perlu dikaitkan shift; apakah radius per lokasi cukup atau perlu override per proyek/kapal.

### Usulan irisan end-to-end terkecil (estimasi ideal dev-day, 1 backend familiar; inferred)
| # | Irisan | Scope | Acceptance test | Est. |
|---|---|---|---|---|
| S1 | `qr-tests` + `proxy-aware-ip` | suite `qr_service_test.go` + handler httptest; IP dari XFF hanya bila `RemoteAddr` di allowlist proxy | token revoke/expired -> 404; 11 req/menit -> 429; XFF dari non-proxy diabaikan | 1.5 d |
| S2 | `qr-config` | base URL dari env (`ABSEN_QR_BASE_URL`), fail-fast saat start bila kosong | generate -> URL memakai env; kosong -> proses gagal start | 0.5 d |
| S3 | `qr-integrity` | hentikan klaim GPS palsu (lat/lng NULL + `location_id` saja), terapkan `photo_policy`, status telat dari `work_start` | QR 08:30 -> on_time, 09:30 -> late; policy `always` tanpa foto -> 422; record tanpa lat/lng | 3 d (butuh A1–A3) |
| S4 | `qr-token-rotation` | expiry + regenerate + mask token di list | token 24 jam ditolak setelahnya; regenerate menonaktifkan lama; list termask | 2 d |

Urutan kritikal: A1–A3 (jam kerja mengikat, 1–2 d) -> S1+S2 (2 d) -> S3 (3 d) -> S4 (2 d) = ~8–9 dev-day
untuk QR/geofence P0, belum termasuk review/QA.

### Staffing vs scope
Risiko 8 minggu yang dilaporkan sebelumnya berasal dari **kapasitas**, bukan ukuran scope: 1 backend +
frontend part-time + 1 reviewer tanpa QA. Dengan tambahan 1 backend + QA paruh waktu, P0
(QR hardening + attendance + cuti + dashboard) realistis 3 minggu; tanpa tambahan, 5–6 minggu. Payroll/pajak
tetap di luar P0 pada kedua skenario.

### Catatan drift baseline
Discovery dipin ke main `c85edc9a`. PR #22 (`fix/ci-green`, head `2b74cdc35`) menurut review `7222576712`
hanya menyentuh lint/konfigurasi 15 berkas, tidak mengubah perilaku QR/geofence; setelah merge, discovery ini
perlu dipastikan ulang pada SHA main yang baru.
