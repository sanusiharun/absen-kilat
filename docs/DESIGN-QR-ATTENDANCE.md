# Design: Absensi via Scan QR Code Lokasi Kantor

> Issue #18. Status: **draft desain** — menunggu review sebelum implementasi.
> Benchmark: Jibble (QR check-in), Zoho People (QR attendance).

## 1. Tujuan

Memberi jalur absen **tanpa aplikasi mobile & tanpa login** untuk tamu/karyawan di lokasi: scan QR di kantor → isi form web singkat → absen tercatat → **divalidasi supervisor** (karena tidak ada bukti GPS/foto dari app).

## 2. Alur (end-to-end)

```
[Admin] buat QR utk lokasi tertentu (POST /admin/qr) 
   → QR berisi URL publik + token: https://absen.example.com/qr?t=<token>
        │
        ▼ (dicetak / ditampilkan di kantor)
[Karyawan/tamu] scan QR (kamera HP) → buka halaman web publik
   → form: Email ATAU Kode Karyawan + Nama + (opsional) selfie
        │
        ▼ POST /public/attendance/qr
[Backend] validasi token QR → cari user by email/employee_code
   → DEDUP: sudah ada absen (user + tanggal + tipe) → TOLAK (gagalkan proses terakhir)
   → simpan attendance: source='qr', approval_status='pending', lokasi = lokasi QR
        │
        ▼
[Supervisor] di admin: daftar pending → approve/reject (+ alasan)
   → final: approved / rejected
```

## 3. Detail Keputusan Desain

### 3.1 QR Code
- Isi QR: **URL + token** (`https://<host>/qr?t=<token>`), bukan data sensitif.
- Token: `uuid` acak, tersimpan di tabel `qr_tokens` (id, location_id, token, active, expires_at, created_by).
- Token **terikat lokasi** → absen tercatat di lokasi tersebut (bukan lokasi GPS).
- Opsional: token bisa expire (mis. harian) atau permanen per lokasi.

### 3.2 Identitas tanpa login
- Input: **Email** atau **Kode Karyawan** + Nama (untuk verifikasi cepat).
- Backend mencocokkan `users.email` (NOCASE) atau `users.employee_code`.
- Jika tidak ditemukan → tampilkan "Kode/email tidak dikenal, hubungi HR" (tidak bocorkan data).
- **Rate limit** per IP (mis. 5 req/menit) + minimal anti-spam; CAPTCHA opsional bila diserang.

### 3.3 Aturan Duplikasi (dedup)
- Kunci duplikasi: `(user_id, tanggal_lokal, type)` — seperti aturan check-in ganda yang sudah ada.
- **Jika duplikat → proses terakhir DITOLAK** ("Anda sudah absen hari ini") — persis permintaan: "jika ada data duplikasi saat absen akan gagalkan proses terakhir".
- Tambahan: 1 QR hanya boleh dipakai 1x per user per hari per type (dicek sama).

### 3.4 Status & Validasi
- Semua absen QR masuk `approval_status='pending'` (tidak ada bukti GPS).
- Supervisor melihat pending di halaman **Approval** (scoped ke tim).
- Approve/reject + alasan → `approval_log`.
- **Alur sama dengan RBAC design** (docs/DESIGN-RBAC.md) — QR adalah salah satu "source" yang selalu pending.

### 3.5 Keamanan
- Endpoint publik `POST /public/attendance/qr` **tanpa auth** tapi: token valid + rate limit + body kecil + tanpa data sensitif di respons.
- QR token tidak bisa ditebak (uuid v7), bisa di-revoke admin.
- Tidak ada endpoint publik lain; semua tetap di balik auth.

## 4. Perubahan Data Model (gabung dengan RBAC migration 006)

```sql
ALTER TABLE attendances ADD COLUMN source TEXT NOT NULL DEFAULT 'app';      -- app|qr|manual
ALTER TABLE attendances ADD COLUMN approval_status TEXT NOT NULL DEFAULT 'pending';

CREATE TABLE qr_tokens (
  id TEXT PRIMARY KEY,
  location_id TEXT NOT NULL REFERENCES locations(id),
  token TEXT NOT NULL UNIQUE,
  active INTEGER NOT NULL DEFAULT 1,
  expires_at TEXT,
  created_by TEXT NOT NULL REFERENCES users(id),
  created_at TEXT NOT NULL
);
```
(butuh tabel `locations` dari issue #9 — multi-lokasi.)

## 5. Backend & Frontend

**Backend:**
- `POST /admin/qr` (admin buat token utk lokasi) → `{token, url}`.
- `GET /admin/qr` / `DELETE /admin/qr/{id}` (list & revoke).
- `POST /public/attendance/qr` — publik: body `{token, email|employee_code, name?}` → validasi → dedup → create pending.
- Attendance handler: sertakan `source` + `approval_status` di respons.

**Frontend:**
- Admin: halaman **QR** (dalam menu Lokasi atau Device) — generate, tampilkan QR (bisa gambar SVG/PNG via library kecil `qrcode`), revoke.
- Halaman web publik (route `/qr` di admin app atau app kecil terpisah): form minimal + halaman sukses/gagal. **Tanpa login, mobile-friendly.**
- Admin: halaman **Approval** (issue #17) menampilkan pending dari source qr/manual/luar-radius.

## 6. Batasan & Catatan

- Absen QR TIDAK punya bukti GPS → bergantung pada validasi supervisor; itu disengaja (jalur cadangan).
- Selfie opsional di form QR → kalau ada, bukti tambahan utk supervisor.
- Integrasi notifikasi: saat pending dibuat, supervisor dapat notif (pakai fitur notifikasi yang sudah ada).
- Kapasitas: 200 karyawan × 2/hari = 400 request/hari — rate limit per IP tidak jadi masalah.

## 7. Open Question (perlu keputusan user)

1. QR token permanen per lokasi, atau di-generate ulang berkala (mis. harian/mingguan)?
2. Tamu/non-karyawan boleh pakai QR? (kalau ya, perlu tipe record 'guest' — di luar scope awal, saran: karyawan dulu)
3. Selfie di form QR: wajib atau opsional?
4. Halaman publik: pakai subdomain terpisah atau route di app admin yang sama?
