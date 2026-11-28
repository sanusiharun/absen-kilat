# Verification: pilot-export-csv

Status gate: **G1 PENDING, G2 PENDING**. G3 belum dijalankan. G4 (merge) akan diminta terpisah setelah
CI hijau + review.

## 1. Acceptance tests (harus PASS sebelum G4)

| ID | Skenario | Harapan | Cara bukti |
|---|---|---|---|
| A1 | `GET /admin/reports/export?type=daily&from&to` 30 hari, `super_admin` | 200, header CSV persis, jumlah baris CSV == `SELECT COUNT(*) FROM attendances WHERE occurred_at >= ? AND occurred_at < ?` pada data yang sama | tes integrasi + angka nyata di PR |
| A2 | `from > to`, format tanggal salah, `from` kosong | 400 dengan field yang jelas, tanpa body CSV | tes handler |
| A3 | Rentang 92 hari → 200; 93 hari → 400 | batas inklusif benar | tes batas (±1 hari) |
| A4 | `from == to` | 200, hanya 1 tanggal di CSV | tes |
| A5 | Batas zona waktu: record 23:30 lokal pada `to` ikut; 00:30 pada `to+1` tidak | masuk/tidak masuk sesuai | tes dengan `time.Local` tetap |
| A6 | Nama berisi koma, kutip, newline; nama berawalan `=`/`+`/`-`/`@` | CSV valid saat di-parse ulang; tidak ada formula dieksekusi | tes parse `encoding/csv` + cek prefix `'` |
| A7 | Matriks otorisasi: super_admin/HR/supervisor/staff/tanpa token untuk export & summary | 200/403/401 sesuai `design.md` §4 | tes per peran + tanpa token |
| A8 | Supervisor meminta data user di luar tim / `project_id` di luar tim | 403 atau hasil kosong; **tidak pernah** ada baris tim lain; tidak ada kebocoran lewat `type=users`/`summary` | tes kebocoran dua arah |
| A9 | Konsistensi summary: `sum(by_day.telat) == totals.telat`, `sum(by_day.lembur_menit) == totals.lembur_menit`, `sum(by_day.hadir) >= totals.hadir`, `totals.alpha >= 0` | semua benar pada data nyata | tes properti |
| A10 | Regresi: `GET /admin/reports` dan `/admin/reports/users` | status + body identik dengan `bf8c3d8f` pada data yang sama | tes snapshot terkontrol / diff JSON |
| A11 | UI `/laporan`: rentang invalid | tombol unduh nonaktif, pesan error terbaca, tidak ada request | cek manual + screenshot di PR |

## 2. Invarian yang tidak boleh dilanggar

1. Tidak ada PII/token di log atau nama file unduhan (hanya `<type>-<from>_<to>`).
2. Tidak ada perubahan skema/tabel/kolom, tidak ada dependency baru.
3. Endpoint lama tidak berubah perilaku maupun permission-nya.
4. `menu.laporan.export` hanya membuka export; `view` tidak otomatis memberi export.
5. Query selalu dibatasi scope aktor sebelum agregasi (bukan filter setelah kirim).

## 3. Perintah bukti

```bash
cd backend
gofmt -l .                         # harus kosong
go vet ./...                       # exit 0
go test -count=1 ./...             # catat kegagalan baseline (jika ada) vs bf8c3d8f
go test -run 'Report|Export|Summary' -v ./internal/service ./internal/transport ./internal/authz
cd .. && python3 tools/validate_sdd.py   # STRUCTURE PASS
```

## 4. Digest (dihitung dari isi berkas, bukan approval)

```
G1 sha256:ef2f88eb3a54ceb67cbe443320a78183d0e4b0f0e4fa8bf1137a5caaa2baebfc
G2 sha256:fc2b87688064f94ad762790818e68c1c3b47f38c7f1ae39e44674a02c0c4ba2e
```

Dihitung ulang setelah tidak ada perubahan berkas:
`python3 tools/validate_sdd.py --digest pilot-export-csv G1|G2`.
Digest ini hanya untuk mengikat review owner ke revisi; bukan bukti identitas.
