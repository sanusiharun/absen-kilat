# Audit read-only — PR #71 identity-hardening slice 2a

- **Task ID:** `7230320388` (issue #57, review PR #71 rev `0d0889c78`)
- **PR:** #71 `feat/identity-hardening-slice2` @ `0d0889c78fee85d57ad56fde5ee690c601c3de3c`, base `main` `6785ec30de20406536eff73b71610434ce202598`
- **Sifat:** audit read-only. Tidak ada perubahan kode aplikasi, tidak ada merge, deploy, worker dispatch, atau biaya.
- **Metode:** `git archive` head PR ke `/tmp/probe-ident` (repo tidak disentuh), lalu `go build ./...`, `go test ./internal/service -run TestRotation`, dan probe test tambahan di salinan sementara.

## Ringkasan

Review `7230320388` merekomendasikan HOLD. Audit ini memverifikasi klaim tersebut dan menemukan **empat penyimpangan perilaku nyata** terhadap spec `identity-hardening` R3/R8, plus dua kelemahan bukti/ketahanan. Semua ditemukan di head `0d0889c` dan direproduksi dengan eksekusi.

| # | Temuan | Spec | Bukti |
|---|---|---|---|
| F1 | Tidak ada jendela grace 10 detik untuk reuse token yang baru dikonsumsi; reuse langsung dianggap replay dan mencabut seluruh family | R3 | probe `TestProbe_ReuseGraceWindow` gagal: token anak hasil rotasi tidak bisa dipakai |
| F2 | Skenario "concurrent refresh" spec gagal: token pemenang ikut dicabut oleh request yang kalah | R3 | probe `TestProbe_ConcurrentWinnerTokenUsable` gagal |
| F3 | Legacy JWT tidak sekali-pakai: tidak ada tabel marker `legacy_refresh_consumed`, tidak ada cut-off `LEGACY_REFRESH_ACCEPT_UNTIL`, tidak ada audit replay, family pertama tidak dicabut | R8, I19/I19c/I19d, tasks T5 | probe `TestProbe_LegacyReplaySequential`: JWT legacy yang sama diterima dua kali (200 + family baru); family pertama tetap hidup |
| F4 | Kegagalan `RevokeAllForUser` setelah ganti/reset password hanya di-`slog.Warn`, password tetap berubah dan sesi lama tetap valid | R3 (pencabutan sesi) | probe `TestProbe_RevocationFailureSwallowed`: `ChangePassword` mengembalikan nil, refresh token lama masih diterima |
| F5 | `expires_at` yang tidak bisa diparse diperlakukan sebagai belum kedaluwarsa (fail-open) | prinsip fail-closed review | probe `TestProbe_MalformedExpiryFailOpen`: outcome `RotateOK` |
| F6 | Klaim "`BEGIN IMMEDIATE`" di komentar repo tidak ada di kode (`BeginTx(ctx, nil)` = BEGIN deferred); lintas koneksi/proses kalah-race menghasilkan error 500, bukan 401 | review "prove actual SQLite transaction locking across connections" | probe `TestProbe_CrossPoolRace` (dua pool): pemenang `ok`, yang kalah `database is locked (517)` pada `refresh.Rotate: consume` |

Catatan F6: produksi memakai `SetMaxOpenConns(1)` (`backend/internal/db/db.go`), sehingga race antar-goroutine di satu proses terserialisasi dan CI hijau. Perlindungan lintas proses/instance tidak ada; komentar kode menyesatkan. Ini risiko saat replika API >1 atau pool dinaikkan.

## Detail bukti

### Baseline (head `0d0889c`)

- `go build ./...` → OK
- `go test ./internal/service -run TestRotation -count=1` → `ok` (semua 6 tes rotasi lulus)
- CI PR #71 @ `0d0889c`: `backend`, `structure`, `frontend` semuanya success

Artinya: tes yang ada lulus, tetapi tes itu tidak menguji perilaku yang diwajibkan spec (lihat F1/F2/F3).

### F1 + F2 — R3 grace dan concurrent refresh

Spec R3: reuse dalam grace window (default 10 detik) → 401 **tanpa** mencabut family; skenario concurrent: pemenang 200, yang kalah 401, **family tetap aktif dan token baru masih bisa dipakai**.

Kode `refresh_token_repo.go` `Rotate`: setiap `consumed_at` terisi langsung `revokeFamilyTx(..., "replay")` tanpa memeriksa selisih waktu. Tidak ada parameter grace di `AuthService`.

Probe (reuse segera setelah rotasi, jauh di dalam 10 detik):

```
FAIL R3: family revoked by benign reuse inside grace window: child unusable: invalid or expired refresh token
FAIL R3 concurrency: winner token unusable (family revoked by loser): invalid or expired refresh token
```

Dampak: retry klien yang normal (timeout jaringan, double-submit mobile) memaksa logout seluruh family.

### F3 — Sesi legacy (R8)

Spec R8 mewajibkan at-most-once **berbasis state**: marker row per SHA-256 JWT, insert + pembuatan family dalam satu transaksi, cut-off `LEGACY_REFRESH_ACCEPT_UNTIL` (rilis + maks 7 hari), audit `legacy_refresh`/`legacy_refresh_replay`, replay 401 + cabut family pertama, tanpa grace.

Kenyataan di `auth_service.go` `refreshLegacyUpgrade`: parse JWT → cek user aktif → `issueTokens`. Tidak ada marker, tidak ada cut-off, tidak ada audit (hanya `slog.Info`), tidak ada pencabutan family.

`git grep -n "LEGACY_REFRESH_ACCEPT_UNTIL\|legacy_refresh_consumed"` pada head PR → tidak ada hasil.

Probe: JWT legacy yang sama dipresentasikan dua kali berurutan → keduanya 200, dua family berbeda, family pertama masih bisa dipakai. Ini melanggar I19, I19d, dan tasks T5.

### F4 — Pencabutan sesi setelah ganti password

`ResetPassword` dan `ChangePassword` (`auth_service.go`): `users.Update` lalu `RevokeAllForUser` terpisah; error pencabutan hanya `slog.Warn`. Probe dengan repository pembungkus yang selalu gagal mencabut: `ChangePassword` mengembalikan `nil` dan refresh token lama tetap bisa dipakai. Tidak ada jalur kompensasi/rollback.

### F5 — Ekspirasi malformed fail-open

`refresh_token_repo.go`: `if exp, perr := time.Parse(...); perr == nil && now.After(exp)` — bila parse gagal, kondisi false dan token diperlakukan valid, lalu dikonsumsi. Probe menyisipkan `expires_at = 'not-a-date'`: `Rotate` mengembalikan `ok`. Untuk data yang ditulis aplikasi sendiri ini defense-in-depth, tetapi review meminta fail-closed eksplisit.

### F6 — Locking lintas koneksi

Komentar `Rotate` mengklaim "BEGIN IMMEDIATE ensures only one concurrent request can win", tetapi kode memakai `db.BeginTx(ctx, nil)` (deferred). Probe dua pool terpisah ke file SQLite yang sama: pemenang `ok`; yang kalah gagal di `UPDATE ... consumed_at` dengan `database is locked (517)`. Di `Refresh` error repo diteruskan mentah, sehingga klien menerima 500 alih-alih 401 dan replay tidak tercatat. Produksi saat ini tertutup oleh `SetMaxOpenConns(1)`, jadi ini bukan kegagalan CI, melainkan bukti bahwa klaim atomisitas lintas koneksi belum benar.

## Kesimpulan audit

- Rekomendasi HOLD pada review `7230320388` **terbukti benar**; empat perilaku wajib spec tidak terpenuhi (F1–F4) dan satu fail-open (F5).
- CI hijau tidak membuktikan kepatuhan spec: tes konkurensi hanya menghitung sukses/gagal, tidak memeriksa status 401, keaktifan family, atau replay legacy. Tidak ada tes I19c/I19d, grace window, cut-off, atau kegagalan pencabutan.
- Perbaikan menyentuh kode runtime dan semantik keamanan. Sesuai policy, koreksi menunggu approval owner via DM Telegram (binding change+gate+base/head+digest+plan hash). Audit ini tidak mengubah kode.

## Lampiran — probe test (tidak dikomit ke backend)

Probe dijalankan di salinan `/tmp/probe-ident` (hasil `git archive` head PR). Hasil ringkas:

```
TestProbe_ReuseGraceWindow            FAIL (child unusable)
TestProbe_ConcurrentWinnerTokenUsable FAIL (winner unusable)
TestProbe_LegacyReplaySequential      FAIL (replay accepted twice, first family alive)
TestProbe_RevocationFailureSwallowed  FAIL (ChangePassword nil, old session valid)
TestProbe_MalformedExpiryFailOpen     FAIL (outcome=ok)
TestProbe_CrossPoolRace               FAIL (loser: database is locked (517))
```

Tidak ada nilai credential, secret, token mentah, atau PII di dokumen ini.
