# Design: identity-refresh-corrective (rev2)

Base `c2a6ee18` (`main` setelah merge PR #74). Rev2 menjawab review `7232428168` atas `cb702d3`.
Semua perubahan menyentuh semantik keamanan runtime ⇒ butuh G1/G2 + approval Telegram sebelum implementasi.

## C1 — Grace window R3 (repo + service + config)
- Grace dibaca dari env `REFRESH_REUSE_GRACE` (default `10s`) dan divalidasi saat start oleh
  `config.ValidateIdentity`: `0 <= grace <= REFRESH_REUSE_GRACE_MAX` (konstanta `60s`, batas maksimum yang
  disetujui). Nilai tidak bisa diparse, negatif, atau melebihi maksimum ⇒ **start gagal** (fail closed,
  bukan default diam-diam). `0s` berarti ketat: tidak ada grace.
- Alur `Rotate` pada `consumed_at` terisi:
  1. Parse `consumed_at` (RFC3339). Gagal parse ⇒ fail closed sebagai replay: `revokeFamilyTx(..., "replay_malformed")`
     + audit `refresh_replay`.
  2. `elapsed := now.Sub(consumedAt)`. `elapsed < 0` (timestamp di masa depan — clock skew atau manipulasi)
     ⇒ **bukan** grace: `revokeFamilyTx(..., "replay")` + audit `refresh_replay_future_timestamp` + 401.
  3. `0 <= elapsed <= grace` ⇒ outcome `domain.RotateReuseInGrace`; commit **tanpa** `revokeFamilyTx`.
  4. `elapsed > grace` ⇒ perilaku sekarang: `revokeFamilyTx(..., "replay")` + audit `refresh_replay`.
- Kemenangan tetap deterministik: `_txlock=immediate` membuat `BEGIN` mengambil write lock, sehingga pemenang
  menulis `consumed_at` sebelum yang kalah membaca.
- Service memetakan `RotateReuseInGrace` → 401 pesan generik yang sama (tidak membocorkan status token) dan
  audit `refresh_grace_reuse` tanpa materi token.
- Yang kalah tidak pernah mencabut family ⇒ token pemenang tetap usable (skenario "concurrent refresh" R3).

## C2 — Cut-off legacy R8: mati secara default
- **Default mati.** `NewAuthService`/wiring tidak pernah menetapkan cut-off relatif-proses. Jalur legacy hanya
  aktif bila `LEGACY_REFRESH_ENABLED=true`; tanpa itu semua refresh legacy → 401 `legacy_disabled`.
- Bila aktif, `LEGACY_REFRESH_ACCEPT_UNTIL` (RFC3339 absolut) **wajib**. Tidak diset atau gagal parse ⇒
  start gagal (`config.ValidateIdentity` mengembalikan error), bukan `legacyDeadline` nol yang diam-diam.
- **Batas rilis yang tidak bisa diperpanjang.** Binary meng-embed tanggal rilis lewat
  `-ldflags "-X main.releaseDate=<RFC3339>"`, diteruskan ke `config.ValidateIdentity`. Validasi menolak start
  bila: `releaseDate` kosong/tidak valid, atau `LEGACY_REFRESH_ACCEPT_UNTIL > releaseDate + 7*24h`. Cut-off
  yang sudah lewat tetap sah (artinya legacy memang kedaluwarsa).
- **Config drift diuji, bukan diasumsikan:** enabled tanpa cut-off, enabled dengan cut-off bukan RFC3339,
  enabled dengan cut-off melewati jendela rilis+7h, dan binary tanpa `releaseDate` semuanya harus gagal start
  dengan pesan eksplisit (C9).
- `SetLegacyRefreshCutoff` dipertahankan hanya untuk test; komentar Go menandainya **bukan** jalur produksi.
- Alternatif yang diizinkan R8 adalah paksa login ulang, bukan memperpanjang token.

## C3 — Versi kredensial: provenance saat verifikasi, penerbitan kondisional
- Migrasi baru `backend/internal/db/migrations/015_credential_version.sql` (additif):
  - `ALTER TABLE users ADD COLUMN credential_version INTEGER NOT NULL DEFAULT 0;`
  - `ALTER TABLE refresh_tokens ADD COLUMN credential_version INTEGER NOT NULL DEFAULT 0;`
- `domain.User.CredentialVersion`, `domain.RefreshToken.CredentialVersion`.
- **Satu jalur penerbitan.** Semua penerbitan (`Login`, `Register`, `VerifyEmail`, `Rotate`, `ConsumeLegacy`)
  lewat `issueTokensTx(ctx, tx, userID, expectedVersion)` di dalam transaksi yang sama:
  `INSERT INTO refresh_tokens (..., credential_version) SELECT ?, ..., ? FROM users WHERE id = ? AND credential_version = ?`.
  `RowsAffected() == 0` ⇒ `ErrCredentialVersionChanged`, rollback. Access token **hanya** di-mint setelah
  INSERT berhasil, sehingga respons mismatch tidak memuat access token maupun refresh token.
- **Verifikasi password menangkap versi, bukan penerbitan.** `Login` membaca
  `SELECT id, password_hash, credential_version FROM users WHERE ...` dalam satu query, menyimpan
  `verAtVerify`, lalu membandingkan bcrypt terhadap hash yang dibaca itu. Versi yang dicap pada sesi adalah
  `verAtVerify` — **bukan** hasil `SELECT` ulang di saat penerbitan. Karena itu validasi password lama tidak
  pernah dicap dengan versi baru.
- **Dua interleaving, keduanya ditolak tanpa sesi baru:**
  - **A** — ganti password commit setelah hash dibaca, sebelum INSERT penerbitan: kondisi
    `credential_version = verAtVerify` tidak cocok ⇒ 0 row ⇒ rollback, 401 generik, audit
    `login_credential_version_mismatch`, tidak ada access/refresh token di respons (C11).
  - **B** — ganti password commit sebelum hash dibaca: hash yang dibaca adalah hash baru ⇒ bcrypt(password lama)
    gagal ⇒ 401 (C12).
  - Varian C (commit di antara baca hash dan bcrypt) identik dengan B karena perbandingan memakai hash yang
    sudah dibaca.
- **Refresh (`Rotate`) tidak "menyembuhkan" versi.** Baris pengganti menyalin versi dari token yang disajikan:
  `INSERT ... SELECT credential_version FROM refresh_tokens WHERE id = ?`, **bukan** dari `users`. Pemeriksaan
  konsumsi membandingkan `refresh_tokens.credential_version` dengan `users.credential_version`; beda ⇒
  `revokeFamilyTx(..., "password_changed")` + `RotateRevoked` (401). Dengan menyalin dari token, sesi yang lolos
  di jendela balapan tetap bernilai versi lama dan tidak pernah naik versi diam-diam.
- **Jalur legacy tidak punya provenance.** JWT legacy tidak membawa klaim versi, jadi provenance-nya tidak bisa
  dibuktikan. `ConsumeLegacy` memperlakukan token legacy sebagai versi `0` dan memakai INSERT kondisional
  `WHERE users.credential_version = 0`. Bila `credential_version > 0` (password berubah setelah migrasi) ⇒
  tolak 401 sebelum marker dibuat dan sebelum family diterbitkan (C13).
- **Ganti/reset password** tetap satu transaksi `UserRepository.ChangePasswordAtomic(ctx, userID, hash)`:
  `UPDATE users SET password_hash = ?, credential_version = credential_version + 1 WHERE id = ?` **dan**
  pencabutan semua family user; error dikembalikan, tidak ditelan.

## Risiko tersisa (dinyatakan, bukan disembunyikan)
1. Access token yang sudah terbit tetap sah sampai `accessTTL` (15 menit) setelah ganti password. Menutupnya butuh
   klaim versi di access token + middleware — change terpisah, tidak diklaim di sini.
2. `_txlock=immediate` mengurangi konkurensi tulis SQLite secara global (sudah ada sebelumnya). Postgres kelak:
   `SELECT … FOR UPDATE`.
3. Grace hanya mencegah pencabutan family; pemakaian ulang tetap 401. `elapsed < 0` sengaja tidak diberi grace
   agar clock skew tidak menjadi celah replay.
4. Token legacy yang terbit setelah migrasi tetapi sebelum ganti password pertama tetap bernilai versi 0 dan
   diterima sampai cut-off — konsisten dengan "tidak ada bukti password berubah", dan tetap dibatasi R8.
