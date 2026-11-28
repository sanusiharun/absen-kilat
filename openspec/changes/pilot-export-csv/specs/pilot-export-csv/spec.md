# Spec delta: pilot-export-csv

Kemampuan: export CSV berentang dan ringkasan dashboard per proyek untuk menu Laporan.
Semua requirement **additive**; endpoint lama tidak berubah.

## R1 — Export CSV berentang

**R1.1** Pengguna dengan `menu.laporan.export` dapat mengunduh CSV untuk rentang `from`/`to` inklusif.

- GIVEN aktor `super_admin`, rentang valid 30 hari
- WHEN `GET /admin/reports/export?format=csv&type=daily&from=2026-11-01&to=2026-11-30`
- THEN 200, `Content-Type: text/csv; charset=utf-8`, `Content-Disposition: attachment; filename="absen-daily-2026-11-01_2026-11-30.csv"`, `Cache-Control: no-store`

**R1.2** Rentang tidak valid ditolak sebelum query dijalankan.

- GIVEN `from` bukan `YYYY-MM-DD`, atau `from > to`, atau rentang > 92 hari
- WHEN export dipanggil
- THEN 400 dengan pesan field; tidak ada body CSV dan tidak ada query data

**R1.3** `type=daily` menghasilkan satu baris per record `attendances` pada rentang, terurut `occurred_at, id`.

- GIVEN N record `attendances` dalam rentang
- WHEN `type=daily`
- THEN CSV berisi tepat N baris data dengan header tetap `tanggal,waktu,nip,nama,email,tipe,status,approval_status,terlambat_menit,source`

**R1.4** `type=users` menghasilkan satu baris per user dalam scope dengan agregat rentang.

- GIVEN rentang dan scope aktor
- WHEN `type=users`
- THEN header `nip,nama,email,departemen,hadir_hari,telat_hari,alpha_hari,check_in,check_out,total_menit`; `alpha_hari` memakai definisi `UserReports` yang sudah live

**R1.5** CSV selalu dapat di-parse ulang dan tidak mengeksekusi formula spreadsheet.

- GIVEN nama berisi koma/kutip/newline dan sel berawalan `=`, `+`, `-`, `@`
- WHEN CSV di-parse dengan pembaca RFC 4180
- THEN jumlah kolom tiap baris tetap; sel berbahaya diberi prefix `'`

## R2 — Ringkasan dashboard per proyek

**R2.1** Pengguna dengan `menu.laporan.view` mendapat agregat `totals` dan `by_day` untuk rentang.

- GIVEN aktor berhak, rentang valid
- WHEN `GET /admin/reports/summary?from=&to=`
- THEN 200 dengan `{from,to,project,totals:{hadir,telat,izin,sakit,alpha,lembur_menit},by_day:[...]}`

**R2.2** Filter proyek hanya menyertakan user yang punya `assignments` tumpang tindih rentang.

- GIVEN `project_id` valid
- WHEN summary dipanggil
- THEN `project` berisi `{id,code,name}` dan hanya user dengan assignment tumpang tindih yang dihitung; `project_id` tak dikenal → 404, format salah → 400

**R2.3** Angka `by_day` konsisten dengan `totals`.

- GIVEN rentang apa pun
- WHEN summary dikembalikan
- THEN `sum(by_day.telat) == totals.telat`, `sum(by_day.lembur_menit) == totals.lembur_menit`, `totals.alpha >= 0`, dan `hadir` per hari tidak melebihi jumlah user aktif dalam scope

## R3 — Scope tim (RBAC) untuk endpoint baru

**R3.1** Supervisor hanya menerima data direct report-nya.

- GIVEN aktor `supervisor` dengan permission terkait
- WHEN export/summary dipanggil (termasuk dengan `project_id` atau user di luar tim)
- THEN hasil hanya berisi user dengan `manager_id = actor.id` (+ dirinya bila Q2 disetujui); tidak pernah ada baris tim lain

**R3.2** Tanpa permission → 403; tanpa token → 401.

- GIVEN aktor tanpa `menu.laporan.export` (atau tanpa `view`)
- WHEN endpoint dipanggil
- THEN 403 (bukan 200 dengan menu tersembunyi); tanpa token → 401 dari middleware

**R3.3** Permission export dan view terpisah.

- GIVEN aktor punya `menu.laporan.view` tetapi tidak `menu.laporan.export`
- WHEN export dipanggil
- THEN 403, sementara summary tetap 200

## R4 — Non-regresi

**R4.1** Endpoint laporan lama tidak berubah.

- GIVEN data yang sama sebelum dan sesudah change
- WHEN `GET /admin/reports?days=14` dan `GET /admin/reports/users?days=14`
- THEN status dan body JSON identik dengan `bf8c3d8f`

**R4.2** Tidak ada perubahan skema dan dependency.

- GIVEN diff change
- WHEN diperiksa
- THEN tidak ada berkas migrasi baru, tidak ada `go.mod`/`package.json` dependency baru, dan tidak ada endpoint lain yang berubah permission

## R5 — Batas operasional

**R5.1** Export tidak menahan seluruh hasil di memori dan berhenti saat klien memutus koneksi.

- GIVEN rentang 92 hari
- WHEN export dijalankan/dibatalkan di tengah
- THEN respons dialirkan per batch 500 baris; pembatalan klien menghentikan query tanpa panic dan tanpa goroutine bocor
