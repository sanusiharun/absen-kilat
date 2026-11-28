# Change: identity-rate-limit-atomic

Status: **DRAFT — paket G1 (corrective)**. Base `bf8c3d8f8842d9fdf57b891f47dc1442093786a6` (main setelah
PR #77). Head yang dikoreksi: PR #78 `feat/identity-slice2b2` @ `7a217d4fca682abac923af4cf11e64e170918a83`.
Sumber: review `7234621195` (BLOCK) dan `7236820149` (butir overflow #79 tertutup; admission #78 tetap blocker).
Change induk: `identity-hardening` R4 (spec), I10/I17/I18/I22 (verification).

> Change ini menyentuh **auth** ⇒ kategori *security-sensitive*. Sesuai AUTONOMY_POLICY §2.3, implementasi
> hanya boleh dimulai setelah owner menyetujui G1 **dan** G2 lewat DM Telegram terverifikasi. Dokumen ini
> usulan; tidak ada kode aplikasi yang ditulis sebelum approval.

## Why

Reviewer independen menolak merge #78 karena **admission** rate limit tidak atomik: pemeriksaan batas dan
pencatatan percobaan adalah dua operasi terpisah, sehingga N request paralel dapat lolos semua sebelum
penghitung naik. Ini bukan dugaan — terbaca dari head #78:

| Lokasi | Perilaku sekarang | Konsekuensi |
|---|---|---|
| `auth_handler.go:133` → `rateBlocked` (`:249`) | `SELECT hits` per scope, tanpa reservasi | Cek-lalu-aksi (TOCTOU): 50 request paralel semuanya membaca `hits=9` |
| `auth_handler.go:140` | `recordAttempt` baru dipanggil **setelah** login gagal | Jendela race selebar durasi verifikasi password |
| `auth_handler.go:183/206/230` | `recordAttempt` sebelum operasi, tapi tanpa batas atomik | Burst paralel melewati batas per IP maupun per akun |
| `auth_handler.go:269-272` | `_ = h.limiter.Record(...)` | Error tulis diabaikan ⇒ percobaan tidak terhitung tanpa jejak |
| `auth_ratelimit.go:67-88` | `Allowed` dan `Record` terpisah, dua round-trip | Tidak ada titik serialisasi antara cek dan tambah |
| `auth_rate_limit_repo.go:40` | `Increment` atomik per baris, tetapi tidak dipakai sebagai gerbang | Upsert atomik saja tidak menegakkan admission |

Akibat yang bisa dieksploitasi: brute force password / enumerasi akun dengan paralelisme. Batas 10/menit
yang disetujui (I10) **tidak** berlaku efektif di bawah burst, sehingga klaim keamanan #78 belum benar.

## What changes (ringkas — detail di `design.md`)

1. **Satu operasi reservasi atomik** untuk semua scope yang terlibat (IP + akun) di dalam satu transaksi
   `BEGIN IMMEDIATE`, dieksekusi **sebelum** operasi yang dilindungi. All-or-nothing: bila salah satu scope
   sudah mentok, kenaikan scope lain di-rollback (tidak membakar kuota IP untuk permintaan yang ditolak).
2. **Gerbang berbasis `RowsAffected`** dari satu statement upsert bersyarat
   (`ON CONFLICT ... DO UPDATE SET hits = hits + 1 WHERE hits < ?`): 1 baris = diterima, 0 baris = ditolak,
   tanpa `SELECT` terpisah dan tanpa `RETURNING`.
3. **Fail closed**: error store saat reservasi ⇒ permintaan ditolak dengan 429 generik yang identik (I18),
   operasi terlindungi **tidak** dijalankan, error dicatat tanpa materi kunci (tanpa IP/email mentah).
4. **Refund hanya untuk login sukses** (semantik hitung yang sudah disetujui: hanya kegagalan yang
   menghabiskan kuota, I10). Refund memakai handle reservasi sehingga mengenai jendela yang sama persis.
5. **Tes**: konkurensi deterministik lintas dua pool DB, injeksi kegagalan store, all-or-nothing dua scope,
   refund, dan pemeriksaan bahwa batas/trusted-proxy/respons generik tidak berubah.

## Non-goals

- Tidak mengubah angka batas (10/5/10/10), lebar jendela 1 menit, `TRUSTED_PROXIES`, derivasi IP, atau
  hash kunci (`sha256(scope + 0x00 + key)`).
- Tidak menambah migrasi: skema `auth_rate_limits` (017) tetap; hanya semantik pemakaian yang berubah.
- Tidak menambah dependensi (tanpa Redis, tanpa library rate limit) dan tanpa biaya tambahan.
- Tidak menyentuh refresh/rotation (#75, #79, #80) atau kontrak API mobile.
- Tidak menutup sisa temuan #80 (provenance legacy) — itu change terpisah.

## Risiko dan mitigasi

| Risiko | Mitigasi |
|---|---|
| Burst login sah (NAT kantor) bisa kena 429 transien karena reservasi mendahului operasi | Refund saat sukses; batas tetap 10/menit; diuji A4/A6 |
| Transaksi tulis per percobaan menambah latensi | 1 transaksi `BEGIN IMMEDIATE` + ≤1 refund; `busy_timeout` 5000 ms sudah ada; diukur di A6 |
| SQLite writer serialization ⇒ 429 pada `SQLITE_BUSY` | Fail closed (429 generik) + log terstruktur; perilaku ini eksplisit, bukan silent |
| Perubahan antarmuka `AuthRateLimitStore` memutus tes lama | Tes lama diperbarui di tugas T4; `Allowed`/`Record` dihapus setelah semua pemanggil pindah |

## Definition of done

- Semua skenario `verification.md` (A1–A6) hijau, `go vet ./...` bersih, `gofmt -l` kosong.
- CI `backend` + `structure` + `frontend` hijau pada head PR baru.
- Tidak ada perubahan perilaku di luar butir *What changes* (dibuktikan A5).
- Bukti (perintah, revisi, exit code, id run CI) dilampirkan di PR sebelum minta G4.

## Approval yang diminta

G1 atas dokumen ini (digest `sha256` dari `tools/validate_sdd.py --digest identity-rate-limit-atomic G1`),
lalu G2 atas `design.md` + `tasks.md` + `specs/identity-rate-limit-atomic/spec.md`. Format DM:
`APPROVE identity-rate-limit-atomic G1 <digest> <base_sha>`.
