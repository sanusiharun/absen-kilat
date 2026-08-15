# Design: RBAC + Approval Bertingkat — Absen Kilat

> Issue #17. Status: **draft desain** — menunggu review sebelum implementasi.
> Benchmark: Mekari Talenta, SAP SuccessFactors, Zoho People, OrangeHRM (open source), Jibble.

## 1. Prinsip

1. **Role berbasis hierarki** — akses menurun ke bawah, tidak ada "semua bisa semua".
2. **Pembatasan menu & aksi per role** — frontend (sidebar) dan backend (middleware) enforce bersama; backend adalah sumber kebenaran.
3. **Approval bertingkat** — rekaman absen punya status hidup: `pending → approved / rejected`, dengan jejak audit.
4. **Data scoping** — supervisor hanya melihat timnya; HR/admin melihat semua; karyawan hanya dirinya.
5. **Konfigurabel** — alur approval bisa disederhanakan (1 tingkat) atau diperdalam (2 tingkat) sesuai kebijakan perusahaan.

## 2. Benchmark ringkas (pola umum sistem mature)

| Sistem | Role | Approval | Catatan |
|---|---|---|---|
| Mekari Talenta | Admin, HR, Manager, Employee | Cuti/absensi: employee → manager approve | Geofence + foto |
| SAP SuccessFactors | System Admin, HR, Manager, Employee | Approval workflow + escalation, delegated admin | Enterprise-grade |
| Zoho People | Admin, Manager, Employee (custom roles) | Multi-level approval, role-based menu | QR attendance ada |
| OrangeHRM | Admin, ESS, Supervisor | Attendance dapat di-edit admin; approval utk leave | Open source |
| Jibble | Owner, Admin, Manager, Member | Geo + **QR check-in**, foto opsional/wajib | Paling mirip kebutuhan kita |

**Pola yang diadopsi:** hierarki role + permission matrix + approval workflow + audit trail + konfigurasi wajib-foto.

## 3. Model Role (usulan)

| Role | Deskripsi | Menu utama |
|---|---|---|
| `super_admin` | Pemilik sistem, akses penuh + pengaturan | Semua + Pengaturan + Akun Admin |
| `hr` | Kelola karyawan & data absensi | Dashboard, Pengguna, Absensi, Laporan, Lokasi, Device |
| `supervisor` | Tim kecil, validasi absensi | Dashboard, **Approval Absen**, Absensi (tim), Laporan (tim) |
| `employee` | Absen & lihat milik sendiri | Mobile: absen, riwayat, profil |

**Relasi:** `users.manager_id → users.id` (supervisor = manager dari sekelompok employee).

## 4. Permission Matrix (menu × role)

| Menu/Aksi | super_admin | hr | supervisor | employee |
|---|---|---|---|---|
| Dashboard | ✅ | ✅ | ✅ (tim) | — |
| Pengguna (CRUD) | ✅ | ✅ | ❌ | ❌ |
| Absensi (semua) | ✅ | ✅ | ✅ (tim) | ❌ |
| Approval Absen | ✅ | ✅ | ✅ (tim) | ❌ |
| Laporan | ✅ | ✅ | ✅ (tim) | ❌ |
| Lokasi / Device | ✅ | ✅ | ❌ | ❌ |
| Pengaturan Sistem | ✅ | ❌ | ❌ | ❌ |
| Ubah status absen (override) | ✅ | ✅ | ⚠️ (tim, dgn catatan) | ❌ |

Enforcement: middleware `RequireRole("super_admin","hr")` di backend; sidebar admin memfilter menu sesuai role; scoping query `WHERE manager_id = ?` untuk supervisor.

## 5. Alur Approval Absen

```
[Karyawan absen (app/QR)] ──► record status = pending (source: app/qr/manual)
        │
        ▼
[Supervisor lihat daftar pending tim] ──► approve / reject (+ alasan)
        │
        ▼
[Opsional: HR konfirmasi] ──► final: approved / rejected
```

- **Auto-approve (KEPUTUSAN 2026-08-15):** absen dari app dalam radius & foto ada → langsung `approved`. Di luar radius / QR / manual / tanpa foto (jika wajib) → `pending`.
- **Escalation:** pending > 24 jam → notifikasi ke HR.
- **Audit trail:** tabel `approval_log` (attendance_id, actor_id, action, reason, created_at).
- **Overrides** (issue #10) selalu tercatat + butuh alasan.

## 6. Perubahan Data Model

```sql
-- migration 006
ALTER TABLE users ADD COLUMN manager_id TEXT REFERENCES users(id);
ALTER TABLE users ADD COLUMN employee_code TEXT UNIQUE;   -- utk QR/identitas

ALTER TABLE attendances ADD COLUMN source TEXT NOT NULL DEFAULT 'app'  -- app|qr|manual
ALTER TABLE attendances ADD COLUMN approval_status TEXT NOT NULL DEFAULT 'pending' -- pending|approved|rejected
ALTER TABLE attendances ADD COLUMN approved_by TEXT;
ALTER TABLE attendances ADD COLUMN approved_at TEXT;
ALTER TABLE attendances ADD COLUMN rejection_reason TEXT;

CREATE TABLE approval_log (
  id TEXT PRIMARY KEY, attendance_id TEXT NOT NULL REFERENCES attendances(id),
  actor_id TEXT NOT NULL REFERENCES users(id),
  action TEXT NOT NULL, reason TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
```

> Catatan: `status` (on_time/late) tetap dipisah dari `approval_status`. Status waktu vs status persetujuan adalah dua hal berbeda.

## 7. Backend Endpoints Baru

- `GET /api/v1/admin/team` — daftar tim (untuk supervisor).
- `GET /api/v1/admin/approvals?status=pending` — daftar butuh persetujuan (scoped).
- `POST /api/v1/admin/approvals/{id}` — body `{action: approve|reject, reason?}`.
- `PATCH /api/v1/admin/attendance/{id}` (issue #10) — override, butuh reason, log otomatis.
- Frontend admin: halaman **Approval** baru + filter menu per role.

## 8. Catatan Implementasi

- Backend: ganti `RequireAdmin` → `RequireRole(roles...)`; `RequireAuth` tetap.
- Frontend admin: `lib/auth.ts` (role dari `/me`) → filter sidebar + guard route.
- Mobile: tidak berubah signifikan (karyawan), hanya status pending muncul di riwayat.
- Migrasi data: user existing → role `employee`, supervisor ditunjuk manual oleh HR.

## 9. Open Question (perlu keputusan user)

1. ~~Auto-approve untuk absen normal (dalam radius + foto)?~~ → **DIPUTUSKAN: ya, auto-approve** (2026-08-15).
2. Butuh role `owner` terpisah dari `super_admin`? (saran: cukup super_admin)
3. Approval 2 tingkat (supervisor → HR) default atau opsional?
4. Employee code: wajib utk semua karyawan? (saran: ya, utk QR & identifikasi cepat)
