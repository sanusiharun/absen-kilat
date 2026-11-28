# Tasks: pilot-export-csv

Dikerjakan hanya setelah **G1 + G2 disetujui**. Satu PR (≤ 300 baris kode aplikasi + tes), branch
`feat/pilot-export-csv`, base `bf8c3d8f`. Urutan: kontrak/types → repo → service → handler → route rule
→ frontend → dokumen → bukti.

| # | Tugas | Path | Verifikasi |
|---|---|---|---|
| T1 | Definisikan tipe `ReportRange` (parse + validasi `YYYY-MM-DD`, `from <= to`, ≤ 92 hari) dan `ReportScope` (semua vs `manager_id`) | `internal/service/report_service.go` (baru) | A2, A3, A4 |
| T2 | Query event keyset + agregat user berentang; agregat harian (hadir/telat/izin/sakit/alpha/lembur); filter `project_id` via `assignments` | `internal/repo/report_repo.go` (baru) | A1, A5, A8, A9 |
| T3 | `ExportDaily`/`ExportUsers` mengembalikan writer yang menulis CSV RFC 4180 + anti-injeksi; `Summary` mengembalikan struct JSON | `internal/service/report_service.go` | A1, A6, A9 |
| T4 | Handler `reportExport` (header, stream + flush per batch 500, error 400/403/404) dan `reportSummary`; aktor dari konteks | `internal/transport/admin_handler.go` | A2, A3, A7, A8 |
| T5 | Tambah 2 rule route: `export` → `menu.laporan.export`, `summary` → `menu.laporan.view` | `internal/authz/routes.go` | `TestRouteRulesAreConsistent` + A7 |
| T6 | UI `/laporan`: input `from`/`to`, tombol unduh (nonaktif bila invalid), 6 kartu ringkasan, state error 400/403 yang terbaca | `admin/app/laporan/page.tsx` (+ helper fetch yang ada) | A11 (manual/Playwright bila ada) |
| T7 | Tes otomatis A1–A10 (tabel di `verification.md`), termasuk kebocoran supervisor dan anti-injeksi CSV | `internal/service/report_service_test.go`, `internal/transport/report_handler_test.go` | A1–A10 |
| T8 | Dokumen skema kolom + arti `terlambat_menit`/`alpha_hari` | `docs/EXPORT-SCHEMA.md` (baru) | review dokumen |
| T9 | Bukti: `gofmt -l`, `go vet ./...`, `go test -count=1 ./...`, CI run id + SHA, pernyataan tidak ada migrasi/dependency baru | — | semua |

## Batasan eksekusi

- Tidak boleh menyentuh `admin_service.go`/`attendance_repo.go` yang melayani endpoint lama (A10 harus
  terbukti tidak berubah). Bila butuh helper bersama, salin pola query, jangan refactor endpoint live.
- Tidak boleh menambah migrasi, dependency, atau mengubah `attendances`/`users`.
- Tidak boleh memperluas permission yang sudah ada: hanya dua route baru yang memakai
  `menu.laporan.export` / `menu.laporan.view`.
- Bila A1 (jumlah baris CSV vs SQL) tidak sama, hentikan dan lapor — jangan melonggarkan asersi.
- Bila `go test ./...` gagal karena 4 kegagalan baseline di `KNOWN_BASELINE_ISSUES.md`, laporkan sebagai
  baseline + bandingkan dengan `bf8c3d8f`, bukan sebagai regresi change ini.
- Deviasi plan PR #69 (design §7) dan pertanyaan terbuka Q1–Q4 wajib dijawab owner sebelum T2/T3 final.

## Bukti yang dilampirkan di PR

1. Diff ringkas per file + alasan satu baris.
2. Output `go test -run 'Report|Export|Summary' -v ./internal/...` dengan nama A1–A10 terlihat.
3. Baris CSV contoh (data seed, bukan karangan) + jumlah baris CSV vs `SELECT COUNT(*)`.
4. Id run CI backend/structure/frontend pada head PR.
5. Pernyataan eksplisit: tanpa migrasi, tanpa dependency baru, endpoint lama tidak berubah.
