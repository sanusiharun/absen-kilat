# Pilot slice 1 — export CSV berentang + ringkasan dashboard per proyek

Status: **implementation-ready draft** (belum ada coding). Menunggu gate sesuai AUTONOMY_POLICY:
perubahan aplikasi butuh G1/G2 + approval owner; dokumen ini menyiapkan bahan supaya implementasi
bisa jalan tanpa desain ulang.

Konteks owner (#59): prioritaskan **pilot dashboard** dan **slice end-to-end terkecil** untuk scope
yang sudah disetujui. Pilot: 10–20 orang, 1 proyek/kapal, 2–4 minggu.

## 1. Slice terkecil yang bernilai (dan kenapa ini yang dipilih)

| Kandidat | Nilai | Biaya | Dipilih? |
|---|---|---|---|
| Export CSV berentang `from`/`to` | rekap manual → otomatis; prasyarat integrasi eksternal dan payroll | kecil: 1 handler + 1 query + tes | **ya (slice 1)** |
| Ringkasan dashboard per proyek | bahan demo pilot | sedang: 1 query agregat + 1 kartu UI | **ya (slice 1b)** |
| XLSX export | kenyamanan | sedang: dependency `excelize` | tidak (slice 2) |
| Laporan biaya tenaga kerja | butuh payroll | besar | tidak (setelah payroll) |
| Checkpoint/regularisasi | nilai tinggi | sedang | paralel (EPIC #23 B1–B6) |

## 2. Kontrak API (usulan, additive)

```
GET /admin/reports/export?format=csv&type=daily|users&from=YYYY-MM-DD&to=YYYY-MM-DD
  -> 200 text/csv; charset=utf-8
     Content-Disposition: attachment; filename="absen-daily-2026-11-01_2026-11-30.csv"
  -> 400 bila from/to tidak valid atau rentang > 92 hari
  -> 403 bila actor tidak punya permission laporan.export

GET /admin/reports/summary?from=&to=&project_id=
  -> 200 {from,to,project:{id,code,name},totals:{hadir,telat,izin,sakit,alpha,lembur_menit},by_day:[...]}
```

Aturan:
- `from`/`to` inklusif, zona waktu dari parameter lokasi pemanggil (konsisten dengan `TodayStats(ctx, loc)`).
- Batas 92 hari mencegah unduhan raksasa di VPS 2 vCPU.
- Streaming (`http.Flusher`), bukan buffer penuh — pola sama dengan rencana D1–D4 di EPIC #23.

## 3. Rencana perubahan kode (belum dieksekusi)

| Berkas | Perubahan |
|---|---|
| `backend/internal/transport/admin_handler.go` | route `GET /admin/reports/export` + `GET /admin/reports/summary` |
| `backend/internal/service/admin_service.go` | `ExportReports(ctx, loc, filter) (io.Reader, string, error)` + `ReportSummary(ctx, loc, filter)` |
| `backend/internal/repo/attendance_repo.go` | query baru: rentang tanggal + filter `project_id` (join `assignments`) |
| `backend/internal/authz/registry.go` | permission `laporan.export`, `laporan.lihat` + pemetaan route |
| `admin/app/laporan/page.tsx` | pemilih rentang tanggal + tombol unduh CSV + 6 kartu ringkasan |

## 4. Tes yang harus ada (dibuat bersamaan, bukan sesudah)

1. **Invarian rentang**: `from > to` → 400; rentang 93 hari → 400; `from == to` → 200 dan tepat 1 hari.
2. **Zona waktu**: data 23:30 lokal pada `to` ikut terhitung; data 00:30 pada `to+1` tidak.
3. **CSV**: header stabil (`tanggal,nip,nama,proyek,shift_masuk,shift_keluar,telat_menit,status`), escaping koma/quote, UTF-8 BOM opsional.
4. **Otorisasi**: aktor tanpa `laporan.export` → 403 (bukan menu tersembunyi).
5. **Summary**: `totals.hadir` == jumlah baris status hadir pada `by_day` (kontrak antar-data, bukan snapshot).
6. **Tanpa regresi**: `/admin/reports` dan `/admin/reports/users` tetap berperilaku sama.

## 5. Definisi selesai (slice 1)

- Endpoint + tes hijau, CI hijau, `go vet` bersih.
- Halaman `/laporan` bisa unduh CSV untuk rentang pilihan; tombol nonaktif bila rentang tidak valid.
- Dokumentasi `docs/EXPORT-SCHEMA.md` memuat header, tipe, dan arti setiap kolom.
- Tidak ada perubahan skema DB (slice ini read-only terhadap data yang ada).

## 6. Risiko dan mitigasi

| Risiko | Mitigasi |
|---|---|
| Unduhan besar membebani VPS | batas 92 hari + streaming + `LIMIT` per batch |
| Perbedaan zona waktu klien | zona eksplisit dari parameter lokasi, diuji invarian |
| Ambigu arti `telat_menit` sebelum shift ada | definisikan sebagai `max(0, check_in - (start+grace))` dengan fallback `cutoffHour`; dokumentasikan |
| Ketergantungan pada payroll | slice ini tidak menyentuh payroll |

## 7. Hubungan dengan tiket yang sudah ada

- EPIC #23 tiket **D1–D4** (export) → slice 1 ini adalah versi paling kecil dari D1+D2 (CSV + rentang).
- **C1–C2** (tampilan manager) tetap paralel, tidak diblokir slice ini.
- Shift (#60–#67) tetap prasyarat untuk `shift_masuk/shift_keluar` yang akurat; sementara fallback `cutoffHour` dipakai dan ditandai di dokumen.
