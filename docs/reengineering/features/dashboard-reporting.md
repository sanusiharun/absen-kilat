# dashboard-reporting
Status: DISCOVERY / unapproved. Baseline c85edc9a96c637037f16e838592a6016902670c9.
## Source inventory
- backend/internal/service/admin_service.go
- backend/internal/transport/admin_handler.go
- admin/app/dashboard/page.tsx
- admin/app/laporan/page.tsx
## Required discovery
Document actors, permissions, trigger, normal and alternative flows, validation, persistence, API request/response/error contracts, privacy, UI and test evidence. Verify actual behavior against source and tests; distinguish observed from desired. Business rules and acceptance scenarios remain TBD until reviewed. No implementation authorization.

---

## Discovery 2026-11-27 (P0, read-only, slice 3/4)

Sumber: `main` setelah PR #22 di-merge (`d082b134`), backend tidak berubah dari `c85edc9a`.

### Permukaan API (observed)
- `GET /admin/stats` → `AdminService.TodayStats(ctx, loc)` — `admin_handler.go:54`, `admin_service.go:74`.
- `GET /admin/reports` → laporan harian (`admin_handler.go:66`).
- `GET /admin/reports/users` → `AdminService.UserReports(ctx, loc, days)` — `admin_handler.go:67`, `admin_service.go:274`.
- Semua menerima `loc *time.Location` → batas hari mengikuti zona waktu pemanggil, bukan UTC.

### Karakteristik teknis
1. **Tidak ada export berkas.** Semua respons JSON; CSV/XLSX belum ada (EPIC #23 tiket D1–D4).
2. **Tidak ada agregasi biaya tenaga kerja** (per proyek/kapal/mitra) — hanya kehadiran.
3. `days` pada `UserReports` berupa angka hari ke belakang, bukan rentang `from`/`to` → tidak bisa
   menarik periode payroll (mis. 26–25) tanpa perubahan API.
4. Belum ada laporan cuti/lembur gabungan; lembur punya `MonthlyMinutes` tetapi tidak terekspos
   di reporting admin.
5. Tidak ada indeks/paginasi eksplisit yang terdokumentasi untuk dataset besar (belum diverifikasi
   runtime pada volume tinggi).

### Gap dan usulan
| Gap | Dampak | Usulan |
|---|---|---|
| Tanpa export CSV/XLSX | integrasi sistem eksternal/manual rekap | tiket D1–D4 (EPIC #23) |
| Tanpa rentang `from`/`to` | tidak cocok untuk periode payroll | `report-range-api` |
| Tanpa laporan biaya per proyek | nilai jual ke pemilik kapal lemah | `project-cost-report` (butuh payroll) |
| Tanpa laporan cuti/lembur gabungan | rekap manual | `consolidated-report` |
| Tidak ada otorisasi per-jenis laporan | HR bisa lihat data lintas tim | dikaitkan RBAC `laporan.export` |

## Discovery 2026-11-27 (P0) — cuti & izin (slice 4/4)

Sumber: `backend/internal/service/leave_service.go`, `migrations/008_leave.sql`.

### Aturan yang terbukti dari kode
1. **Kuota hanya ditegakkan untuk `annual`**: `if in.TypeCode == "annual"` → `used := CountApprovedDays(...)`,
   sisa = `u.LeaveQuota - used` (`leave_service.go:95-104`). Tipe lain tidak memotong kuota.
2. **Batas `DefaultDays` per tipe** ditegakkan (`:81`), **kecuali** `paternity` yang boleh
   `DefaultDays + ExtraDays` (`:75`).
3. Hari dihitung dengan `countWorkdays(start, end, workdays)` (`:202`) → menghormati setting
   `workdays`, **tetapi tidak mengenal hari libur nasional** (tidak ada tabel holiday).
4. Alur: `Apply` → `Approve`/`Reject` (dengan alasan) → `Cancel` oleh pemilik (`:139/:158/:178`).
5. Jenis cuti berasal dari tabel master (`domain/models.go:22`): annual, sick, emergency,
   maternity, paternity — dapat ditambah tanpa perubahan kode.

### Gap
| Gap | Dampak | Usulan |
|---|---|---|
| Tanpa kalender libur nasional | hari libur terhitung sebagai cuti | `holiday-calendar` (P1) |
| Tanpa carry-over/accrual | kuota tahunan statis | `leave-accrual` (P2) |
| Tanpa lampiran wajib untuk `sick` | potensi penyalahgunaan | `leave-attachment-policy` |
| Tanpa izin khusus (mis. post-dive rest) | tidak cocok operasi offshore | `leave-type-config` |
| Tanpa rekap kuota di reporting | HR hitung manual | `leave-balance-report` |
| Tidak ada tes untuk batas kuota/carry | regresi tidak tertangkap | tambah tes invarian |

### Unknown
Apakah `LeaveQuota` per pengguna di-reset tahunan oleh job tertentu (belum ada job terlihat),
dan bagaimana perilaku bila `workdays` diubah setelah cuti disetujui.
