# Absen Kilat — HANDOFF

> File ini dibaca di AWAL tiap sesi kerja. Update di AKHIR sesi supaya sesi berikutnya lanjut dari posisi tepat.

## Posisi Terakhir
- **Track aktif:** Frontend (Sesi 2)
- **Status:** Backend B0 selesai — modul Go, struktur dir, `pkg/httpx` + `pkg/errors`, logger slog, server chi dengan `/healthz`. Lanjut Frontend F1 (setup Expo).

## Checklist Track
### Backend (B)
- [x] B0 Setup project & toolchain
- [ ] B1 Migrations & DB (SQLite)
- [ ] B2 Domain & repo (User, Attendance)
- [ ] B3 Auth service (bcrypt, JWT)
- [ ] B4 Auth transport + JWT middleware
- [ ] B5 Attendance service+repo
- [ ] B6 Attendance transport (multipart upload)
- [ ] B7 Profile transport+service
- [ ] B8 Upload file handling
- [ ] B9 Test & verifikasi
- [ ] B10 Run service (systemd, port 8090)

### Frontend (F)
- [ ] F1 Setup Expo
- [ ] F2 API client & secure storage
- [ ] F3 Auth screens
- [ ] F4 Home (absen sekarang)
- [ ] F5 Absen flow (kamera, GPS, konfirmasi)
- [ ] F6 Riwayat + filter tanggal
- [ ] F7 Detail absen
- [ ] F8 Profil
- [ ] F9 Polish & integrasi

## API Contract
Lihat `obsidian-vault/02-Projects/absen-kilat/plan/BUILD-PLAN.md` (bagian API Contract) — ini kontrak yang dipegang bersama FE & BE. Jangan ubah tanpa update kedua sisi.

## Konvensi Backend (WAJIB)
- **Baca `obsidian-vault/02-Projects/absen-kilat/docs/BACKEND-CONVENTIONS.md`** — ini sumber aturan backend (salinan dari golang-banking-platform).
- Layered: Transport → Service → Repo → DAO
- Router `chi` + stdlib, response pakai `pkg/httpx`
- Logging `slog` (bukan fmt.Println)
- Error wrapping `fmt.Errorf("pkg.Func: %w", err)` + domain error di `pkg/errors`
- Modul: `github.com/sanusi/absen-kilat/backend`
- Git identity: `sanusiharun <sanusiharun@gmail.com>`

## Catatan Sesi
### Sesi 2 (2026-08-14) — Backend: B0 Setup project & toolchain ✅
- **Dikerjakan:** `go mod init github.com/sanusi/absen-kilat/backend` (+ chi v5.3.1); struktur dir `cmd/api`, `internal/{config,domain,repo,service,transport}`, `pkg/{errors,httpx}`, `migrations/`.
- **File utama:** `cmd/api/main.go` (logger slog, config env, router chi, `/healthz`, graceful shutdown, NotFound/MethodNotAllowed JSON), `pkg/errors/errors.go` (domain errors + Is* helpers), `pkg/httpx/{errors,response}.go` (envelope kontrak `{"data":...}` / `{"error":{code,message}}`), `internal/config/config.go`, `internal/transport/middleware.go` (RequestLogger slog).
- **Catatan:** `httpx.WriteError(w, err)` sengaja tanpa argumen request — envelope absen-kilat (per BUILD-PLAN) tidak memuat `request_id`/`timestamp`, beda dari banking. Import order stdlib → external → internal sudah dijaga.
- **Verifikasi:** `go build ./...` OK, `go vet ./...` OK. Smoke test: `/healthz` → 200 `{"data":{"status":"ok"}}`; rute tak dikenal → 404 `{"error":{"code":"NOT_FOUND",...}}`; method salah → 405.
- **Kendala:** tidak ada. (HANDOFF sempat diubah sibling agent — menambah rujukan BACKEND-CONVENTIONS.md & git identity; tidak konflik.)
- **Berikutnya:** Frontend F1 — setup Expo (create-expo-app, expo-router, layout dasar).

