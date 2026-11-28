# Design: identity-rate-limit-atomic

Change induk `identity-hardening` R4. Base `bf8c3d8f`; koreksi atas head PR #78 `7a217d4fc`.

## 1. Masalah: cek dan tambah bukan satu operasi

Alur #78 untuk login (`auth_handler.go:125-150`):

```
Allowed(IP)  -> SELECT hits       (t1)
Allowed(akun) -> SELECT hits      (t2)
svc.Login(...)                    (t3, ~bcrypt)
Record(IP) / Record(akun)         (t4, hanya bila gagal)
```

Dua cacat struktural:

1. **TOCTOU.** Antara t1 dan t4 tidak ada serialisasi. k request paralel membaca nilai yang sama; semuanya
   lanjut ke t3. Batas hanya berlaku pada percobaan yang *berurutan*.
2. **Error tulis ditelan.** `recordAttempt` (`:265-273`) membuang error (`_ =`), sehingga kegagalan tulis
   berarti percobaan gratis-tanpa-catatan.

`Increment` (`auth_rate_limit_repo.go:40`) sudah atomik **per baris**, tetapi dipanggil sesudah operasi,
sehingga tidak bisa menjadi gerbang admission. Yang kurang bukan atomisitas tambah, melainkan **titik
tunggal tempat keputusan terima/tolak diambil bersama kenaikan penghitung**.

## 2. Primitif: satu statement bersyarat, gerbang dari `RowsAffected`

```sql
INSERT INTO auth_rate_limits (scope, key_hash, window_start, hits)
VALUES (?, ?, ?, 1)
ON CONFLICT(scope, key_hash, window_start)
DO UPDATE SET hits = hits + 1 WHERE auth_rate_limits.hits < ?
```

| Hasil | `RowsAffected` | Arti |
|---|---|---|
| baris baru | 1 | diterima (hits=1) |
| konflik, `hits < limit` | 1 | diterima (hits+1) |
| konflik, `hits >= limit` | 0 | **ditolak**, penghitung tidak berubah |

Kenapa bentuk ini, bukan `SELECT ... FOR UPDATE` atau `RETURNING`:

- SQLite tidak punya `SELECT FOR UPDATE`; serialisasi didapat dari transaksi tulis (`BEGIN IMMEDIATE`,
  sudah aktif lewat DSN `_txlock=immediate` di `db.go:28`, plus `busy_timeout(5000)` dan
  `SetMaxOpenConns(1)` per pool).
- `RowsAffected` portabel di driver `glebarez/go-sqlite` yang dipakai (tidak bergantung versi SQLite yang
  mengembalikan `RETURNING`), dan tidak butuh query baca kedua.
- Upsert bersyarat adalah **satu** unit kerja: tidak ada celah antara baca, keputusan, dan tulis.

## 3. Dua scope, satu keputusan (all-or-nothing)

Satu request login menyentuh dua scope (IP dan akun). Bila masing-masing direservasi sendiri-sendiri,
permintaan yang ditolak oleh scope kedua tetap membakar kuota scope pertama — penyerang cukup membanjiri
satu akun untuk mematikan login seluruh IP (amplifikasi DoS).

```
BeginTx(immediate)
  reserve(IP)    -> 0 rows? => Rollback, DITOLAK
  reserve(akun)  -> 0 rows? => Rollback, DITOLAK   (kenaikan IP ikut batal)
  prune(window_start < now-2m)
Commit                                          => DITERIMA
```

Urutan IP lalu akun dipertahankan dari #78. Scope yang `limit == 0` atau kuncinya kosong dilewati
(tidak mengubah perilaku nil/disabled limiter).

## 4. Refund: hanya login yang sukses

Semantik yang sudah disetujui (I10, design induk §4) menghitung **kegagalan**, bukan setiap percobaan.
Karena reservasi kini mendahului operasi, login yang berhasil harus mengembalikan slotnya:

```sql
UPDATE auth_rate_limits SET hits = hits - 1
 WHERE scope = ? AND key_hash = ? AND window_start = ? AND hits > 0
```

- Handle reservasi (`{scopes, windowStart}`) dipakai saat refund supaya kenaikan dan pengurangan terjadi di
  **jendela yang sama** meski request melewati batas menit.
- Refund **tidak** boleh menggagalkan request: login yang sudah sukses tetap 200. Error refund dicatat
  sebagai warning; arah salahnya aman (maksimal over-count 1, tidak pernah under-count).
- Forgot / reset / verify-email tetap menghitung setiap permintaan (tanpa refund) — sama seperti #78.

## 5. Fail closed dan bentuk error

| Kondisi | Respons | Operasi terlindungi |
|---|---|---|
| reservasi ditolak (batas) | 429 generik (`httpx.ErrTooManyRequests`) | tidak dijalankan |
| store error / `SQLITE_BUSY` / ctx timeout | 429 generik, **identik** | tidak dijalankan |
| refund error setelah sukses | 200 | sudah dijalankan |

Error store dicatat `slog.Warn` dengan field `scope` dan `err` saja — tidak ada IP, email, atau hash kunci
(dipakai bersama field `request_id` yang sudah ada). 429 generik wajib identik dengan jalur batas (I18),
supaya kegagalan internal tidak bisa dipakai membedakan ada/tidaknya akun.

## 6. Antarmuka yang berubah

```go
// service
 type ReserveHandle struct { Scope, KeyHash string; WindowStart int64; Limit int }
 type AuthRateLimitStore interface {
     Reserve(ctx context.Context, r []ReserveRequest, now int64, pruneBefore int64) (bool, error)
     Release(ctx context.Context, h []ReserveHandle) error
 }
```

`Allowed` dan `Record` dihapus setelah semua pemanggil pindah (T4). `AuthRateLimiter.Reserve(...)` tetap
melakukan hashing kunci, jadi IP/email mentah tidak pernah mencapai repo. `LimitFor` tidak berubah.

Handler berubah bentuk menjadi:

```go
h, ok, err := h.reserve(ctx, ip, email, service.ScopeLoginIP, service.ScopeLoginAccount)
if err != nil || !ok { httpx.WriteHTTPError(w, httpx.ErrTooManyRequests); return }
user, pair, err := h.svc.Login(ctx, req.Email, req.Password)
if err == nil { h.release(ctx, h) }   // gagal => tetap terhitung
```

## 7. Kinerja dan anggaran

- Tambahan: 1 transaksi tulis (≤3 statement) + ≤1 `UPDATE` refund per percobaan login. Tanpa biaya
  layanan, tanpa dependensi baru.
- Dengan `busy_timeout` 5000 ms dan satu writer per pool, antrean tulis tercermin sebagai latensi, bukan
  sebagai lolosnya admission.
- Tidak ada perubahan skema; migrasi 017 tetap. Rollback aplikasi = `git revert`, data tetap kompatibel.

## 8. Alternatif yang ditolak

| Alternatif | Alasan ditolak |
|---|---|
| Token bucket in-memory per proses | Batas efektif N× pada N instance; bertentangan dengan I22 |
| Redis / layanan eksternal | Biaya + dependensi baru ⇒ butuh approval biaya; tidak perlu untuk satu instance |
| Serialisasi via mutex proses | Hanya benar untuk satu proses; tes dua pool akan gagal |
| Hitung semua percobaan (tanpa refund) | Mengubah perilaku I10 (login sukses memakan kuota) ⇒ perubahan aturan bisnis tanpa persetujuan |
| `SELECT` di dalam transaksi lalu `UPDATE` | Benar tapi dua statement + rawan salah saat scope kedua ditolak; upsert bersyarat lebih sedikit celah |
