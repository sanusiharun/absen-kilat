# Design: pilot-export-csv

Change induk: `dashboard-redesign` (menu Laporan) + tiket EPIC #23 D1–D2 (versi terkecil).
Base `bf8c3d8f`; tidak ada kode aplikasi yang berubah sebelum G1/G2.

## 1. Titik sambung yang sudah ada (diverifikasi)

| Hal | Bukti |
|---|---|
| Route laporan lama | `internal/transport/admin_handler.go:66-67`; handler `:268`, `:294` |
| Service laporan lama | `internal/service/admin_service.go:225` (`Reports`), `:278` (`UserReports`) |
| Query per user | `internal/repo/attendance_repo.go:415` (`UserReports`, CTE `daily`) |
| Gerbang permission | `internal/authz/routes.go:45-46`, `middleware_permission.go` (unmapped `/admin/*` ditolak) |
| Permission export | `menu.laporan.export` sudah dideklarasikan di `registry.go:56`, belum dipetakan ke route |
| Preseden scope tim | `Approvals` menyaring `manager_id = actorID` untuk supervisor (`admin_service.go:619-624`) |
| Cutoff telat | `CUTOFF_HOUR` (default 9) → `AttendanceService.statusFor`, tanpa grace (`attendance_service.go:228-237`) |
| Zona waktu | Handler laporan memakai `time.Local`; dipertahankan agar konsisten |
| Cuti | `leaves` (status `approved`, `leave_type` dari `leave_types`: `annual|sick|emergency|maternity|paternity`) |
| Lembur | `overtimes.minutes`, `status='approved'`, `work_date` (YYYY-MM-DD lokal) |
| Proyek | `assignments(user_id, project_id, start_date, end_date)`; `end_date=''` = berjalan |

## 2. Rute dan middleware

```go
r.Get("/admin/reports/export",  h.reportExport)   // menu.laporan.export
r.Get("/admin/reports/summary", h.reportSummary)  // menu.laporan.view
```

Dua baris wajib ditambahkan ke `internal/authz/routes.go`, jika tidak `TestRouteRulesAreConsistent`
tidak menangkapnya tetapi middleware akan menolak route tak terpetakan (fail closed). Aktor diambil dari
konteks auth yang sudah ada (`actorID`, `actorRole`) — bukan dari query/body.

## 3. Kontrak

### 3.1 `GET /admin/reports/export`

Parameter: `format` (hanya `csv`; nilai lain 400), `type` (`daily|users`), `from`, `to` wajib,
format `YYYY-MM-DD`, `from <= to`, `to - from + 1 <= 92` hari.

Header respons:

```
Content-Type: text/csv; charset=utf-8
Content-Disposition: attachment; filename="absen-<type>-<from>_<to>.csv"
Cache-Control: no-store
```

**`type=daily` — satu baris per record `attendances`** (bukan per user-hari):

```
tanggal,waktu,nip,nama,email,tipe,status,approval_status,terlambat_menit,source
2026-11-01,07:52,AK-001,Budi, budi@example.com,check_in,on_time,approved,0,device
```

- `tanggal`/`waktu` = `occurred_at` dikonversi ke `time.Local` (`YYYY-MM-DD`, `HH:MM`).
- `terlambat_menit` = hanya untuk `check_in`: `max(0, round(local - cutoffHariItu))`; selain itu `0`.
  Tidak ada grace di kode saat ini, jadi tidak ada grace yang dikarang.
- Baris terurut `occurred_at ASC, id ASC` (deterministik untuk tes).
- Jumlah baris **sama dengan** jumlah record `attendances` pada rentang tersebut (A1).

**`type=users` — satu baris per user** (semantik sama dengan `/admin/reports/users`, tetapi berentang):

```
nip,nama,email,departemen,hadir_hari,telat_hari,alpha_hari,check_in,check_out,total_menit
```

- `alpha_hari = max(0, jumlah hari kalender rentang - hadir_hari)` — definisi yang sama dengan
  `UserReports` yang sudah live (`attendance_repo.go:461-467`), bukan aturan baru.
- Jumlah baris = jumlah user dalam scope (termasuk yang tidak punya record, seperti query lama).

**Escaping**: RFC 4180 (kutip ganda di-escape `""`, field berkoma/quote/newline dikutip). Sel yang
 dimulai `=`, `+`, `-`, `@`, TAB, atau CR diberi prefix `'` untuk mencegah injeksi spreadsheet (A6).

**Streaming**: tulis header + flush; ambil data per batch 500 dengan keyset (`occurred_at > ? OR
(occurred_at = ? AND id > ?)`) untuk `daily`, dan satu query agregat untuk `users`. Tidak menahan
seluruh hasil di memori. Client putus → `r.Context()` berhenti, tidak ada panic.

### 3.2 `GET /admin/reports/summary`

```json
{
  "from": "2026-11-01", "to": "2026-11-30",
  "project": {"id": "...", "code": "PRJ-1", "name": "Jack-up Alpha"},
  "totals": {"hadir": 12, "telat": 3, "izin": 2, "sakit": 1, "alpha": 4, "lembur_menit": 480},
  "by_day": [{"date": "2026-11-01", "hadir": 12, "telat": 1, "izin": 0, "sakit": 0, "alpha": 5, "lembur_menit": 60}]
}
```

- `hadir` = user distinct dengan ≥1 `check_in` pada hari itu; `telat` = user distinct dengan `check_in`
  berstatus `late` (subset `hadir`).
- `izin` = user distinct dengan cuti `approved` bertipe `annual|emergency|maternity|paternity` yang
  rentangnya menutupi hari itu; `sakit` = tipe `sick`.
- `alpha` = `max(0, user aktif dalam scope - user distinct yang hadir atau cuti approved)`; satu user
  tidak dihitung dua kali.
- `lembur_menit` = `SUM(overtimes.minutes)` dengan `status='approved'` dan `work_date` = hari itu.
- `project` = `null` bila `project_id` tidak diberikan; bila diberikan, scope user dibatasi ke
  `assignments` yang tumpang tindih rentang (`start_date <= to AND (end_date = '' OR end_date >= from)`).
  `project_id` tak dikenal → 404; format bukan id → 400.
- Jumlah hari maksimum sama (92). `totals` = jumlah `by_day` (A9), kecuali `hadir`/`alpha` yang
  memang per-hari dan tidak dijumlah lintas hari di `totals` (didokumentasikan di `docs/EXPORT-SCHEMA.md`).

> Catatan: `totals` sengaja **tidak** menjumlahkan `by_day` untuk `hadir`/`alpha` karena satu user bisa
> hadir di banyak hari; A9 menguji `sum(by_day.telat) == totals.telat` dan `sum(by_day.lembur_menit) ==
> totals.lembur_menit`, serta `hadir <= jumlah user`.

## 4. Scope (RBAC) untuk endpoint baru

| Role | export | summary | Data |
|---|---|---|---|
| `super_admin`, `hr` | izin sesuai permission | izin sesuai permission | semua user |
| `supervisor` | hanya jika `menu.laporan.export` | hanya jika `menu.laporan.view` | `manager_id = actor.id` |
| role lain | 403 (permission tidak ada di template) | 403 | — |

Implementasi: satu helper `scopeUserIDs(ctx, actorRole, actorID) (filter string, args []any, err error)`
 yang mengembalikan `AND u.manager_id = ?` untuk supervisor; seluruh query export/summary memakai helper
ini. Bukan `ScopeTeam` generik — sengaja mengikuti preseden `Approvals` agar tidak ada mesin scope baru.

## 5. Perubahan berkas (perkiraan ≤ 300 baris kode + tes)

| Berkas | Perubahan |
|---|---|
| `internal/authz/routes.go` | 2 rule baru (`export` → `menu.laporan.export`, `summary` → `menu.laporan.view`) |
| `internal/transport/admin_handler.go` | parse/validasi rentang, dua handler, header CSV, stream + flush |
| `internal/service/report_service.go` (baru) | `ExportDaily`, `ExportUsers`, `Summary`; validasi 92 hari; helper scope |
| `internal/repo/report_repo.go` (baru) | query keyset event, agregat user, agregat harian (cuti/lembur/assignment) |
| `admin/app/laporan/page.tsx` | input `from`/`to`, tombol unduh, 6 kartu; tombol nonaktif saat rentang invalid |
| `admin/lib/api.ts` (atau pola fetch yang ada) | helper unduh blob + error 400/403 |
| `docs/EXPORT-SCHEMA.md` (baru) | header, tipe, arti kolom, contoh |

Tidak menyentuh `attendance_repo.go` yang sudah dipakai endpoint lama kecuali menambah method baru di
file repo baru (menghindari risiko regresi A10).

## 6. Alternatif yang ditolak

1. **Menambah `from`/`to` ke endpoint lama** — mengubah kontrak live dan berisiko regresi; ditolak.
2. **Export XLSX sekarang** — butuh dependency baru; di luar slice terkecil.
3. **Menghitung `shift_masuk`/`shift_keluar` dari cutoff** — menambah kolom yang terlihat seperti data
   shift padahal bukan; menyesatkan laporan pilot. Ditunda sampai #60–#67.
4. **Membuat mesin scope generik** — terlalu besar; preseden `manager_id` cukup untuk slice ini.

## 7. Deviasi dari dokumen PR #69 (perlu keputusan owner di G1)

| Butir plan | Deviasi | Alasan |
|---|---|---|
| Header `daily` memuat `proyek,shift_masuk,shift_keluar` | Diganti `waktu,tipe,approval_status,terlambat_menit` tanpa proyek/shift | Penautan user-hari dan shift belum punya aturan disetujui; A1 justru menghitung baris = record attendances |
| A1: baris CSV == baris tabel `attendances` | Dipertahankan apa adanya | Konsisten dengan keputusan baris-per-record di atas |
| Scope tim | Hanya endpoint baru | Mengubah endpoint lama = perubahan RBAC di luar slice; dicatat sebagai gap terpisah |

## 8. Pertanyaan terbuka (jawaban owner menentukan implementasi)

- Q1: Apakah `alpha` harus mengecualikan hari libur/weekend? Saat ini dihitung semua hari kalender,
  sama seperti `Reports` yang live (reuse semantik, bukan aturan baru).
- Q2: Apakah supervisor boleh mengunduh data dirinya sendiri bila bukan bagian dari timnya?
  Usulan: ya (self + direct reports).
- Q3: Batas 92 hari — cukup atau perlu 31 hari untuk VPS pilot?
- Q4: `type=users` untuk staff (scope self) ditolak 403 atau diizinkan hanya baris sendiri?
  Usulan: 403, karena menu `laporan` bukan untuk staff.
