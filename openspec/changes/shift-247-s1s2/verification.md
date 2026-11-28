# Verification: shift-247-s1s2

Status gate: **G1 PENDING, G2 PENDING**. G3/G4 belum dijalankan. Tidak ada bukti implementasi karena
belum ada kode yang ditulis — change ini docs-only sampai approval.

## 1. Yang sudah diverifikasi pada base `bf8c3d8f`

| # | Klaim | Perintah | Hasil |
|---|---|---|---|
| B1 | Tidak ada kode/tabel shift | `grep -riE 'shift\|roster' backend/internal/db/migrations backend/internal --include=*.go --include=*.sql \| grep -v _test \| wc -l` | `0` |
| B2 | Jam kerja dari `CUTOFF_HOUR` | `grep -rn 'cutoffHour\|CUTOFF_HOUR' backend/internal --include=*.go \| grep -v _test` | `config.go:46`, `attendance_service.go:56,63,68,232` |
| B3 | Nomor migrasi berikutnya | `ls backend/internal/db/migrations \| tail -3` | `014_…`, `015_…`, `016_email_verifications.sql` ⇒ berikutnya `017` |
| B4 | Bentuk `attendances` | `sed -n 16,33p 001_init.sql` | `type IN ('check_in','check_out')`, `status IN ('on_time','late')`, tanpa kolom `date`, tanpa unique per hari |
| B5 | Route admin tak terpetakan ditolak | `go test -run TestAuthorizeDeniesUnmappedAdminRoute ./internal/transport` | PASS |
| B6 | Menu difilter oleh `view` | `access_handler.go:66-70` | `if !set.Has(Permission(m.Key, ActionView)) { continue }` |
| B7 | `build` + tes authz/repo hijau pada base | `go build ./... && go test ./internal/authz ./internal/repo` | `BUILD_OK`, `ok authz`, `ok repo` |
| B8 | Struktur SDD | `python3 tools/validate_sdd.py` | `STRUCTURE PASS` (setelah `verification.md` ada) |

## 2. Acceptance tests (harus PASS sebelum G4)

| ID | Skenario | Harapan | Cara bukti |
|---|---|---|---|
| V1 | Migrasi 017 pada DB kosong | exit 0, `PRAGMA table_info(shifts)` = 12 kolom, `table_info(attendances)` memuat `shift_id`, `PRAGMA foreign_key_check` kosong | log migrasi + output pragma di PR |
| V2 | Migrasi 018 pada DB yang sudah ter-seed (001–016 + `access_seed`) | 3 baris baru `role_permissions` untuk `super_admin`; dijalankan ulang → tetap 3 (idempoten) | query `SELECT count(*) …` dua kali |
| V3 | Kode duplikat case-insensitive | `PAGI` lalu `pagi` → 422, `SELECT count(*)` tetap 1 | tes repo + tes handler |
| V4 | Validasi waktu | `8:0`, `24:00`, `09:60`, `start==end` → 422; `22:00`–`06:00` → 201 dengan `crosses_midnight=1` | tes service table-driven |
| V5 | Rentang grace/break | `-1`, `121` grace dan `-1`, `481` break → 422; `0` dan `120` → 201 | tes service |
| V6 | Lokasi tidak ada | `location_id="tidak-ada"` → 404, tidak ada row; `location_id` kosong → `""` | tes service + tes repo |
| V7 | Otorisasi | super_admin 200/201; hr/supervisor tanpa grant → 403; tanpa token → 401; semua pada tiga endpoint | tes handler per peran |
| V8 | `/me/permissions` tanpa grant shift | `shift` tidak ada di daftar menu; setelah grant `menu.shift.view` → muncul dengan `href="/shift"` | tes handler access |
| V9 | Regresi absensi | `go test ./internal/service -run 'CheckIn\|CheckOut\|Attendance'` hasil identik dengan base; `attendance_service.go` tidak berubah (`git diff --stat` kosong untuk berkas itu) | diff + tes |

## 3. Perintah bukti (dijalankan worker, output ditempel di PR)

```bash
cd backend
gofmt -l .                          # harus kosong
go vet ./...                        # exit 0
go build ./...                      # exit 0
go test -count=1 ./internal/authz ./internal/repo ./internal/service ./internal/transport
go test -count=1 ./cmd/...
cd .. && python3 tools/validate_sdd.py   # STRUCTURE PASS
```

Untuk V1/V2, jalankan binary dengan `DB_PATH` sementara (DB kosong + salinan DB ber-`schema_migrations`
001–016), lalu periksa dengan `sqlite3`:

```bash
sqlite3 "$TMPDB" "PRAGMA table_info(shifts);" "PRAGMA table_info(attendances);" "PRAGMA foreign_key_check;"
sqlite3 "$TMPDB" "SELECT role, permission_key FROM role_permissions WHERE permission_key LIKE 'menu.shift.%';"
```

## 4. Invarian yang tidak boleh dilanggar

1. Tidak ada perubahan pada `attendance_service.go`, `admin_service.go`, `admin_handler.go`.
2. Tidak ada backfill `attendances.shift_id`; semua baris lama tetap `NULL`.
3. Tidak ada dependency Go baru, tidak ada migrasi selain 017/018.
4. Setiap route shift punya rule authz; tidak ada endpoint yang hanya disembunyikan di UI.
5. `CUTOFF_HOUR` tetap berfungsi sebagai satu-satunya sumber telat sampai S3 disetujui.

## 5. Digest (dihitung dari isi berkas, bukan approval)

```
G1 sha256:d8eac8f9b5767374896cb7f8281233c4794113b7c4680c818b9f2b140bd6c1ff
G2 sha256:e330273ac08337ed70c16d0493e606f5cfe454747d916dda61813a36f16f95d7
```

Reproduksi: `python3 tools/validate_sdd.py --digest shift-247-s1s2 G1|G2`.
Digest mengikat review owner ke revisi berkas ini; bukan bukti identitas dan bukan approval.
