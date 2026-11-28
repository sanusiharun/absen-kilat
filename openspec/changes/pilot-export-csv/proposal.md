# Proposal: pilot-export-csv

Change ID: `pilot-export-csv`. Status: **PROPOSED — G1/G2 pending**. Base: `bf8c3d8f8842d9fdf57b891f47dc1442093786a6` (main setelah PR #77).
Dokumen sumber (sudah merge, bukan approval): `docs/plans/2026-11-27-pilot-slice-1-export-dashboard.md`,
`docs/testing/pilot-slice-1-acceptance.md` (PR #69), `docs/reengineering/features/dashboard-reporting.md`.

## 1. Masalah (terverifikasi dari kode, bukan asumsi)

1. **Tidak ada export berkas.** `GET /admin/reports` dan `/admin/reports/users` hanya JSON
   (`admin_handler.go:66-67`, `admin_service.go:225,278`). Rekap pilot masih manual.
2. **Tidak ada rentang `from`/`to`.** Keduanya memakai `?days=N` relatif terhadap hari ini, jadi periode
   payroll (mis. 26–25) tidak bisa ditarik.
3. **Scope tim belum ditegakkan.** `authz.registry` menyatakan menu `laporan` ber-`ScopeTeam`, tetapi
   `Rules()` hanya memetakan `menu.laporan.view` (`internal/authz/routes.go:45-46`) dan service laporan
   tidak menerima identitas aktor (`Reports`/`UserReports` hanya `loc, days`). Supervisor yang punya
   `menu.laporan.view` melihat data seluruh perusahaan. Ini gap yang sudah tercatat di
   `docs/reengineering/features/dashboard-reporting.md` ("tidak ada otorisasi per-jenis laporan").
4. Permission `menu.laporan.export` **sudah ada** di registry (`registry.go:56`) tetapi tidak dipakai
   route mana pun — export belum punya gerbang sendiri.

## 2. Scope perubahan (additive, tanpa migrasi)

| # | Perubahan |
|---|---|
| S1 | `GET /admin/reports/export?format=csv&type=daily|users&from=&to=` → stream CSV; gerbang `menu.laporan.export` |
| S2 | `GET /admin/reports/summary?from=&to=&project_id=` → JSON agregat + `by_day`; gerbang `menu.laporan.view` |
| S3 | Penegakan scope `ScopeTeam` **pada dua endpoint baru** memakai preseden yang sudah ada: supervisor hanya user dengan `manager_id = actor.id` (`admin_service.go:619-624`), super_admin/hr seluruh data |
| S4 | Aturan validasi rentang: `from`/`to` wajib `YYYY-MM-DD`, `from <= to`, rentang ≤ 92 hari inklusif |
| S5 | Halaman `/laporan`: pemilih rentang + tombol unduh CSV + kartu ringkasan (memakai endpoint S1/S2) |

## 3. Non-goals (eksplisit)

- **Tidak** mengubah `/admin/reports`, `/admin/reports/users`, `/admin/stats` (perilaku tetap; lihat A10).
- **Tidak** ada tabel/kolom/migrasi baru; slice ini read-only terhadap data yang ada.
- **Tidak** ada XLSX, payroll, biaya tenaga kerja, atau penjadwalan.
- **Tidak** memasukkan kolom shift (`shift_masuk`/`shift_keluar`) atau penautan check-in↔check-out:
  shift baru didefinisikan di #60–#67, dan penautan butuh aturan bisnis yang belum disetujui.
  Karena itu `type=daily` mengekspor **satu baris per record `attendances`** (bukan per user-hari).
- **Tidak** mengubah scope endpoint laporan lama; gap §1.3 untuk endpoint lama dicatat sebagai
  perubahan terpisah agar slice ini tidak memperluas RBAC tanpa keputusan.

## 4. Kontrak API (ringkas; detail di `design.md`)

```
GET /admin/reports/export?format=csv&type=daily|users&from=YYYY-MM-DD&to=YYYY-MM-DD
  200 text/csv; charset=utf-8 + Content-Disposition: attachment; filename="absen-daily-<from>_<to>.csv"
  400 format/type/from/to tidak valid, from > to, atau rentang > 92 hari
  401 tanpa token / token kedaluwarsa (middleware yang ada)
  403 tanpa menu.laporan.export

GET /admin/reports/summary?from=&to=&project_id=
  200 {from,to,project:{id,code,name}|null,totals:{...},by_day:[{date,...}]}
  400 parameter tidak valid; 403 tanpa menu.laporan.view
```

## 5. Definisi selesai

- S1–S5 selesai, tes A1–A10 di `verification.md` PASS dengan angka nyata (bukan snapshot karangan).
- `go vet ./...` bersih, CI backend/structure/frontend hijau pada head PR.
- `docs/EXPORT-SCHEMA.md` memuat header, tipe kolom, dan arti `terlambat_menit`.
- Tidak ada perubahan skema, dependensi baru, atau biaya tambahan.

## 6. Risiko

| Risiko | Mitigasi |
|---|---|
| Unduhan besar membebani VPS 2 vCPU | batas 92 hari, stream + flush per batch 500 baris, query keyset (`occurred_at,id`) |
| CSV injection di spreadsheet | sel yang diawali `= + - @` diberi prefix `'`; escaping RFC 4180 |
| Perbedaan zona waktu | `time.Local` seperti handler laporan yang ada; dikunci di tes invarian batas hari |
| Scope tim salah | tes kebocoran A8 (supervisor minta user/project di luar tim) wajib ada |
| Deviasi dari plan PR #69 | dicatat di §3 + `design.md` §7; butuh persetujuan owner di G1 |

## 7. Gate

G1 (proposal) dan G2 (proposal+design+tasks+spec) **belum disetujui**. Tidak ada kode aplikasi yang
boleh berubah sebelum keduanya disetujui lewat DM owner. Digest di `verification.md` (bukan di file ini,
agar tidak self-referential).
