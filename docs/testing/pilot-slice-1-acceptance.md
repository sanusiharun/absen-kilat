# Pilot slice 1 — acceptance tests (export CSV berentang + ringkasan dashboard)

Branch: `docs/pilot-slice-1-export` · Base: `0413c9cc` · Status: **rencana tes, bukan kode**.
Tidak ada implementasi sebelum change disetujui; dokumen ini yang menjadi kriteria terima.

## 0. Prinsip

- **Tanpa data dummy.** Semua tes memakai data yang benar-benar ada di database (seed migrasi +
  data uji yang dibuat lewat API resmi dan dibersihkan di akhir). Tidak ada angka karangan di laporan.
- **Kontrak API diverifikasi dari kode yang ada**, bukan dari ingatan: `GET /admin/reports`
  (`admin_handler.go:66`), `GET /admin/reports/users` (`:67`), `GET /admin/stats` (`:54`),
  `TodayStats(ctx, loc)` (`admin_service.go:74`), `UserReports(ctx, loc, days)` (`:274`).
- **Zona waktu**: batas hari mengikuti parameter lokasi pemanggil, konsisten dengan `TodayStats`.

## 1. Kontrak yang diuji (harus cocok dengan implementasi)

```
GET /admin/reports/export?format=csv&type=daily|users&from=YYYY-MM-DD&to=YYYY-MM-DD
  headers: Content-Type: text/csv; charset=utf-8
           Content-Disposition: attachment; filename="absen-<type>-<from>_<to>.csv"
  200 -> CSV, header kolom stabil
  400 -> from/to tidak valid, from > to, atau rentang > 92 hari
  403 -> aktor tidak punya permission laporan.export

GET /admin/reports/summary?from=&to=&project_id=
  200 -> {from,to,project:{id,code,name},totals:{...},by_day:[{date,hadir,telat,izin,sakit,alpha,lembur_menit}]}
  400 -> parameter tidak valid
  403 -> aktor tidak punya permission laporan.lihat
```

## 2. Cakupan otorisasi (wajib, bukan opsional)

| Aktor | export | summary | Catatan |
|---|---|---|---|
| `super_admin` | 200 | 200 | baseline |
| HR (`hr`) | 200 | 200 | hanya bila permission ada di template |
| Supervisor | **403** | 200 (tim sendiri) | batas tim harus dihormati, bukan hanya menu disembunyikan |
| Staff/employee | 403 | 403 | tidak boleh ada kebocoran data |
| Tanpa token / token kedaluwarsa | 401 | 401 | |

Supervisor **tidak** boleh menerima baris di luar timnya walau memanggil ID user lain secara eksplisit
— ini tes kebocoran, bukan tes menu.

## 3. Acceptance tests (A1–A10)

| ID | Skenario | Kriteria lulus |
|---|---|---|
| A1 | Export `type=daily`, rentang 30 hari, aktor super_admin | 200, header CSV persis seperti skema, jumlah baris == jumlah baris tabel `attendances` pada rentang itu (dihitung terpisah lewat SQL) |
| A2 | `from > to` | 400 dengan pesan validasi |
| A3 | rentang 93 hari | 400 (batas 92 hari) |
| A4 | `from == to` | 200 dan hanya 1 tanggal muncul |
| A5 | Batas zona waktu | record pukul 23:30 lokal pada tanggal `to` ikut; pukul 00:30 pada `to+1` tidak |
| A6 | Escaping CSV | nama berisi koma, tanda kutip, dan newline tetap menghasilkan CSV valid (di-parse ulang oleh pembaca CSV) |
| A7 | Otorisasi | matriks §2 semuanya sesuai (403/401 bukan 200) |
| A8 | Kebocoran supervisor | supervisor meminta `project_id` di luar timnya → 403 atau hasil kosong, **tidak pernah** berisi baris tim lain |
| A9 | Ringkasan konsisten | `sum(totals.*) == sum(by_day.*)` untuk setiap hari pada rentang |
| A10 | Regresi | `GET /admin/reports` dan `/admin/reports/users` tetap 200 dengan body identik seperti sebelum perubahan (dibandingkan pada data yang sama) |

## 4. Bukti yang harus dilampirkan saat implementasi

1. Perintah `go test ./... -run 'Export|Summary'` dengan hasil per tes.
2. Skrip `scripts/acceptance/pilot-slice-1.sh` yang menjalankan A1–A10 terhadap instance lokal,
   mencetak status per tes (tanpa menyimpan token atau PII ke repo).
3. Perbandingan jumlah baris A1: nilai dari CSV vs nilai dari query SQL pembanding (angka nyata).
4. Run CI (id + status) dan konfirmasi tidak ada perubahan skema DB.

## 5. Yang **tidak** boleh dilakukan pada slice ini

- Menambah tabel atau kolom.
- Menyentuh payroll, absensi tulis, atau shift (di luar field tampilan).
- Merge atau deploy tanpa approval owner.
