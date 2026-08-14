# Absen Kilat — HANDOFF

> File ini dibaca di AWAL tiap sesi kerja. Update di AKHIR sesi supaya sesi berikutnya lanjut dari posisi tepat.

## Posisi Terakhir
- **Track aktif:** Backend (Sesi 1)
- **Status:** Mulai dari awal — workspace & Go 1.26.6 sudah disiapkan.

## Checklist Track
### Backend (B)
- [ ] B0 Setup project & toolchain
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
_<diisi tiap sesi>_
