# Absen Kilat — HANDOFF

> File ini dibaca di AWAL tiap sesi kerja. Update di AKHIR sesi supaya sesi berikutnya lanjut dari posisi tepat.

## Posisi Terakhir
- **Track aktif:** — (SEMUA SELESAI)
- **Status:** Backend (B0–B10) ✅ + Frontend (F1–F9) ✅ — aplikasi Absen Kilat selesai dibangun & berjalan. Backend live di systemd port 8090, frontend Expo siap dijalankan via Expo Go / web.

## Checklist Track
### Backend (B)
- [x] B0 Setup project & toolchain
- [x] B1 Migrations & DB (SQLite)
- [x] B2 Domain & repo (User, Attendance)
- [x] B3 Auth service (bcrypt, JWT)
- [x] B4 Auth transport + JWT middleware
- [x] B5 Attendance service+repo
- [x] B6 Attendance transport (multipart upload)
- [x] B7 Profile transport+service
- [x] B8 Upload file handling
- [x] B9 Test & verifikasi
- [x] B10 Run service (systemd, port 8090)

### Frontend (F)
- [x] F1 Setup Expo
- [x] F2 API client & secure storage
- [x] F3 Auth screens
- [x] F4 Home (absen sekarang)
- [x] F5 Absen flow (kamera, GPS, konfirmasi)
- [x] F6 Riwayat + filter tanggal
- [x] F7 Detail absen
- [x] F8 Profil
- [x] F9 Polish & integrasi

## API Contract
Lihat `obsidian-vault/02-Projects/absen-kilat/plan/BUILD-PLAN.md` (bagian API Contract) — kontrak terimplementasi penuh di backend.

## Konvensi Backend (WAJIB)
- Layered: Transport → Service → Repo → DAO
- Router `chi` + stdlib, response pakai `pkg/httpx`
- Logging `slog` (bukan fmt.Println)
- Error wrapping `fmt.Errorf("pkg.Func: %w", err)` + domain error di `pkg/errors`
- Modul: `github.com/sanusi/absen-kilat/backend`
- Git identity: `sanusiharun <sanusiharun@gmail.com>`

## Cara Menjalankan

### Backend (sudah jalan via systemd)
```sh
sudo systemctl status absen-api        # cek status
curl localhost:8090/healthz            # health check
sudo systemctl restart absen-api       # restart
```
- Binary: `/usr/local/bin/absen-api`
- DB: `backend/data/absen-kilat.db`
- Uploads: `backend/uploads/`
- Unit: `/etc/systemd/system/absen-api.service`

### Frontend
```sh
cd ~/absen-kilat/mobile
npm run start        # Expo dev server → scan QR di Expo Go
```
- API default: `http://localhost:8090/api/v1` — override dengan `EXPO_PUBLIC_API_URL` di `.env` untuk test dari HP (pakai IP LAN VPS).
- Web: `npx expo start --web`

## Catatan Sesi
### Sesi 1 (2026-08-14) — Backend: B0 ✅ (setup project & toolchain)
### Sesi 2 (2026-08-14) — Backend: B0 verifikasi ✅ (smoke test lengkap)
### Sesi 3 (2026-08-14 21:40–21:50) — MARATHON: B1–B10 + F1–F9 ✅
- **B1** Migrations & DB (SQLite modernc, WAL, schema_migrations, users + attendances)
- **B2** Domain models + repo SQLite (User, Attendance DAO)
- **B3** Auth service (bcrypt, JWT HS256 access 15m / refresh 30d, verify/reset)
- **B4** Auth transport (register/login/refresh/forgot/reset/verify) + RequireAuth middleware
- **B5** Attendance service (check-in/out validasi urutan, on_time/late fix full-time cutoff)
- **B6** Attendance transport (multipart upload foto, today, list by date, detail + ownership check)
- **B7** Profile (GET/PATCH me, avatar upload, ganti password)
- **B8** Storage (validasi ekstensi jpg/png/webp, max 8MB, path YYYY/MM/uuid.ext)
- **B9** Unit test service layer — 9 test PASS; smoke test curl semua endpoint PASS
- **B10** systemd `absen-api.service` port 8090, auto-restart, JWT secret di-env
- **F1** Expo app (SDK 57, expo-router, tabs + auth stack, theme)
- **F2** API client (fetch wrapper, token secure-store, auto-refresh 401 → retry)
- **F3** Auth screens (login, register, forgot, reset, verify)
- **F4** Home (tombol absen dinamis dari /attendance/today, pull-to-refresh)
- **F5** Absen flow (kamera selfie, GPS, galeri fallback, konfirmasi, multipart submit)
- **F6** Riwayat (filter tanggal, badge tepat/lambat)
- **F7** Detail (foto, link Google Maps)
- **F8** Profil (edit nama, avatar, notif switch, ganti password, logout)
- **F9** Polish — tsc clean, web export OK (920 modules)

### Kendala & Solusi
- Konflik npm peer react-dom saat install @expo/vector-icons → `--legacy-peer-deps` versi 15.0.3
- `expo-linking` missing untuk web export → `npx expo install expo-linking`
- Bug status late: komparasi jam saja (9:30 == 9) → fix bandingkan full time terhadap cutoff hari itu
- `UID` bash readonly variable saat test → pakai `USER_ID`
- `.gitkeep` di internal dirs dihapus setelah diisi file nyata

### Berikutnya
- Test di device nyata (Expo Go) — kamera & GPS hanya bisa diuji penuh di HP
- Production hardening: ganti JWT secret default di systemd unit (sudah generate random), pertimbangkan HTTPS + reverse proxy
