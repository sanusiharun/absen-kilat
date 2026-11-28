# reporting-corrective-r1 — brief koreksi reporting (export CSV + summary)

Task: #59 · Base: `main` `c5c9e58560faf13aa2c4f7eae95188a8b7f34fc9` · Tanggal: 2026-11-28
Sumber instruksi: komentar review `7239083364` (re: `7225916731`, follow-up `7225946676`)
Status: **BUTUH APPROVAL OWNER** (security-sensitive: authz scope). Belum ada kode aplikasi diubah.

## Temuan terverifikasi (bukan asumsi)

| # | Temuan | Bukti |
|---|---|---|
| F1 | Export tanpa scope aktor/team | `backend/internal/service/reports_export.go:75-79` memanggil `ListAdmin` tanpa filter pemilik; `authz/registry.go:53` menyatakan `laporan` = **ScopeTeam** |
| F2 | **Truncation diam-diam di 500 baris**, bukan 100000 | `reports_export.go:78` minta `Limit: 100000`, tapi `repo/attendance_repo.go:324-330` clamp ke 500. Uji nyata: `rows_in_db=501 rows_exported=500 lost=1`, summary `hadir=500` (DB 501). Handler tetap balas **200** + CSV parsial |
| F3 | Formula injection belum dimitigasi | `reports_export.go:111-121` hanya `encoding/csv` (quoting), sel teks pengguna (`nama`, `email`, `perangkat`, `ip`) ditulis verbatim; sel berawalan `= + - @` tetap dieksekusi spreadsheet |
| F4 | `project_id` tidak dikenal endpoint summary | `transport/admin_handler.go:830-832` hanya mengoper `from`/`to`; `reports_summary.go` tidak punya parameter proyek, sementara `docs/plans/...-pilot-slice-1...` mencantumkannya |
| F5 | Kolom `nip` selalu kosong | `reports_export.go:92` `NIP: ""`; padahal `users.NIP` ada (`repo/user_repo.go:30`) |
| F6 | Zona waktu hardcode | `admin_handler.go:812,830` memakai `time.Local`, bukan zona lokasi seperti endpoint lain |
| F7 | Komparator tes memakai jalur yang sama | `reports_export_test.go:105` membandingkan dengan `ListAdmin` — bukan SQL independen, jadi truncation F2 tidak terdeteksi |
| F8 | Kebocoran bukan baru, tapi sistemik | `admin_service.go` `Reports()`/`UserReports()` juga tanpa scope; `ScopeTeam` di registry **tidak ditegakkan di mana pun** (`grep ScopeTeam` hanya di registry + tes) |

Catatan F1/F8: route guard hanya memeriksa izin `laporan.view`/`laporan.export`; metadata `ScopeTeam` tidak pernah ditegakkan. Dampak bergantung pada grant `role_permissions`/`user_permission_overrides` di DB produksi (tidak dibaca dari sini; tidak ada DB lokal).

## Usulan koreksi (satu jalur query ekspor, bukan tempelan di service)

1. `repo.ListForExport(ctx, ExportFilter{From,To,ManagerID *string,Limit,Offset})` — query khusus ekspor yang JOIN `users` (nip, manager_id) dan **mewajibkan** `manager_id` bila aktor supervisor.
2. `ExportAttendance(ctx, actor{ID,Role,Location}, from, to)` — supervisor ⇒ `ManagerID=actor.ID`; super_admin/hr ⇒ nil. Penegakan **di boundary query**, bukan filter pasca-query.
3. **Paging eksplisit**: baca per 500 baris sampai `total` terpenuhi; batas keras 20.000 baris ⇒ **400 eksplisit** (bukan CSV parsial). Tidak ada truncation diam-diam.
4. **Anti formula injection**: sel tekstual berawalan `= + - @ TAB CR` diberi prefiks `'`; `jarak_meter` tetap numerik. Aturan dicatat di `docs/EXPORT-SCHEMA.md`.
5. **NIP** diisi dari `users.nip`.
6. **Zona waktu**: ambil dari parameter lokasi yang sama dengan `/admin/stats` (bukan `time.Local`).
7. **`project_id`**: dikeluarkan dari kontrak r1 dan dibuka tiket backlog (attendances tidak punya `project_id`; butuh join `project_assignments` = keputusan model data, bukan bug fix).
8. **Tes komparator independen**: hitung dengan `SELECT COUNT(1)` mentah per status/hari, bukan `ListAdmin`.

## Acceptance test (wajib PASS sebelum usul merge)

- A1 501 baris ⇒ CSV 501 baris, dan tidak ada respons parsial.
- A2 > 20.000 baris ⇒ 400 dengan pesan batas, bukan 200.
- A3 supervisor: hanya baris `manager_id = dirinya`; `export` 403 bila tak punya `laporan.export`; tanpa token 401.
- A4 kebocoran lintas tim: supervisor minta rentang berisi tim lain ⇒ 0 baris tim lain (HTTP-level).
- A5 sel `=1+1`, `+A1`, `-2`, `@SUM` tertulis dengan prefiks `'`.
- A6 `jarak_meter` tetap numerik murni.
- A7 batas zona waktu 23:30 vs 00:30 memakai zona lokasi.
- A8 `sum(totals) == sum(by_day)` per field, termasuk `lainnya`.
- A9 regresi `/admin/reports` & `/admin/reports/users` tidak berubah (F8 dicatat terpisah, tidak diperbaiki di r1).
- A10 unduh nyata dari UI `/laporan` (tombol nonaktif saat rentang invalid).

## Risiko & rollback

- Risiko: mengubah jalur baca laporan yang sudah live; salah scope ⇒ laporan kosong bagi HR.
- Rollback: revert commit tunggal; tidak ada perubahan skema DB; tidak ada migrasi.
- **Tidak ada deploy** sampai approval + acceptance PASS.

## Yang diminta dari owner

Approval security-sensitive untuk change ini pada base `c5c9e585`, format DM:
`APPROVE reporting-corrective-r1 G1+G2 <digest> c5c9e58560faf13aa2c4f7eae95188a8b7f34fc9`
