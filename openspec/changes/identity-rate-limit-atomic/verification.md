# Verification: identity-rate-limit-atomic

Semua tes dijalankan pada head PR change ini. "Deterministik" berarti asersi tidak bergantung pada
penjadwalan goroutine: yang dihitung adalah **jumlah keputusan terima/tolak**, bukan urutan pemenang.

| ID | Skenario | Harapan |
|---|---|---|
| A1 | 20 goroutine `Reserve` paralel untuk kunci yang sama, limit 10, dua pool DB ke file yang sama, start bersamaan lewat barrier | tepat **10** diterima, 10 ditolak, `hits = 10` di DB, tidak pernah > 10 |
| A2 | Store mengembalikan error (dan `SQLITE_BUSY` disimulasikan) saat reservasi | 429 generik; `svc.Login` **tidak** dipanggil; penghitung tidak berubah (transaksi di-rollback) |
| A3 | Scope akun sudah mentok (`hits = limit`), scope IP masih 0 | permintaan ditolak; `hits` IP tetap 0 (all-or-nothing) |
| A4 | 10 login **sukses** berturut-turut dari IP yang sama, lalu 10 kegagalan | kesepuluh login sukses 200; percobaan gagal ke-10 masih diproses (401), ke-11 (total) 429 |
| A5 | Non-regresi: batas per scope (10/5/10/10), jendela 1 menit, XFF dari klien tak-terpercaya, respons 429 identik untuk email ada/tidak ada, kunci tersimpan hanya hash | semua tes I10/I17/I18/I22 di `identity-hardening` tetap hijau tanpa perubahan asersi |
| A6 | Pengukuran latensi: 100 login berurutan dengan limiter aktif vs `limiter == nil` | selisih p95 ≤ 25 ms pada mesin dev; bila lebih, laporkan angka apa adanya (bukan disembunyikan) |
| A7 | `go test ./... -race -count=1` pada `service`, `repo`, `transport` | 0 data race; 4 kegagalan baseline (bila masih ada) dicatat terpisah dengan pembanding `bf8c3d8f` |

## Bukti spike desain (pre-G1, epistemik: belum kode change)

Mekanisme inti sudah **dijalankan**, bukan hanya dirancang, memakai driver dan pembukaan pool yang sama
dengan produksi (`db.Open`: `journal_mode=WAL`, `busy_timeout(5000)`, `_txlock=immediate`,
`SetMaxOpenConns(1)`). Dua probe sekali-pakai dijalankan di worktree terpisah lalu **dihapus, tidak
di-commit** (tidak ada kode aplikasi berubah sebelum approval):

| Probe | Hasil nyata |
|---|---|
| Upsert bersyarat 15× dengan limit 10, satu pool | `admitted=10 hits=10`, `sqlite_version=3.53.3` — deny mengembalikan `RowsAffected=0` **tanpa** menaikkan `hits` |
| 20 goroutine × reservasi dua scope dalam satu transaksi, tersebar di **dua pool** ke file yang sama | `admitted=10 denied=10 errs=0 ipHits=10 acctHits=10` (dengan `-race`, 0 data race, tanpa `SQLITE_BUSY`) |

Perintah (untuk reproduksi): `cd backend && go test -race -run 'ProbeConditionalUpsert\|ProbeConcurrentReserveTwoPools' ./internal/repo -v`.
Keterbatasan: probe memakai tabel minimal (migrasi 017 hanya ada di branch #78), bukan handler HTTP, dan
belum menguji injeksi error store — itu tugas A2/A3 pada implementasi sebenarnya.

## Invarian yang harus tetap benar (ditegakkan A5)

1. Respons 429 tidak membedakan akun ada/tidak ada (status, body, panjang).
2. `X-Forwarded-For` hanya dipercaya dari `TRUSTED_PROXIES`; selain itu `RemoteAddr`.
3. Tidak ada IP/email mentah di tabel `auth_rate_limits` maupun di log.
4. Batas tetap agregat lintas proses (dua pool berbagi penghitung).
5. Hanya kegagalan login yang menghabiskan kuota (I10); forgot/reset/verify menghitung setiap permintaan.

## Perintah bukti

```bash
cd backend
gofmt -l . | tee /tmp/gofmt.txt            # harus kosong
go vet ./...                               # exit 0
go test -race -count=1 ./internal/repo ./internal/service ./internal/transport -run 'RateLimit|Atomic' -v
go test -race -count=1 ./internal/...      # suite penuh, bandingkan baseline
go test -count=1 ./...                     # baseline: 4 kegagalan KNOWN_BASELINE_ISSUES, catat apa adanya
```

## Bukti yang dilampirkan

Output perintah di atas (nama tes A1–A7 terlihat), id run CI per workflow pada head PR, `git rev-parse HEAD`,
serta daftar file yang berubah dibanding `bf8c3d8f`.

## Yang belum diverifikasi oleh change ini (harus dinyatakan di PR)

- Perilaku di bawah beban produksi nyata dan di belakang reverse proxy asli.
- Efektivitas terhadap penyerang terdistribusi (banyak IP) — mensyaratkan pembatasan di edge/WAF, di luar scope.
- Perilaku multi-instance nyata (dua proses server, bukan dua pool dalam satu proses) — I22 adalah proksi terdekat.
