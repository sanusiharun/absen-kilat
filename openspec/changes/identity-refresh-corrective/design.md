# Design: identity-refresh-corrective

Base `591c37b`. Semua perubahan menyentuh semantik keamanan runtime ⇒ butuh G1/G2 + approval Telegram sebelum implementasi.

## C1 — Grace window R3 (repo + service)
- `Rotate` menerima parameter grace (default `10s`, dari env `REFRESH_REUSE_GRACE`, di-parse `time.ParseDuration`; nilai tidak valid ⇒ start gagal, bukan default diam-diam).
- Alur `consumed_at` terisi:
  1. Parse `consumed_at` (RFC3339). Gagal parse ⇒ **fail closed** sebagai replay (cabut family, `RotateReplay`).
  2. `now.Sub(consumedAt) <= grace` ⇒ outcome baru `domain.RotateReuseInGrace`; commit transaksi **tanpa** `revokeFamilyTx`.
  3. Di luar grace ⇒ perilaku sekarang: `revokeFamilyTx(..., "replay")` + audit `refresh_replay`.
- Kemenangan tetap deterministik: `_txlock=immediate` membuat `BEGIN` mengambil write lock, sehingga pemenang menulis `consumed_at` sebelum yang kalah membaca.
- Service memetakan `RotateReuseInGrace` → 401 pesan generik yang sama (tidak membocorkan status token) dan audit `refresh_grace_reuse` tanpa materi token.
- Yang kalah tidak pernah mencabut family ⇒ token pemenang tetap usable (skenario "concurrent refresh" R3).

## C2 — Cut-off legacy absolut R8
- Hapus default relatif-proses di `NewAuthService`. Cut-off dibaca dari env `LEGACY_REFRESH_ACCEPT_UNTIL` (RFC3339) saat start.
- Env tidak ada/format salah ⇒ `legacyDeadline` dibiarkan **nol** dan jalur legacy **menolak** semua token JWT legacy (fail closed + `slog.Error` sekali saat start). Alternatif yang diizinkan R8 adalah paksa login ulang, bukan memperpanjang token.
- `SetLegacyRefreshCutoff` dipertahankan untuk test, tetapi nilainya tidak lagi dihitung dari `time.Now()`.
- Nilai cut-off yang dioperasikan = tanggal rilis aktual + maksimum 7 hari; tanggal rilis adalah fakta manusia, tidak di-hardcode oleh agen.

## C3 — Versi kredensial (serialisasi ganti password vs penerbitan)
- Migrasi baru `backend/internal/db/migrations/015_credential_version.sql` (additif):
  - `ALTER TABLE users ADD COLUMN credential_version INTEGER NOT NULL DEFAULT 0;`
  - `ALTER TABLE refresh_tokens ADD COLUMN credential_version INTEGER NOT NULL DEFAULT 0;`
- `domain.User.CredentialVersion`, `domain.RefreshToken.CredentialVersion`.
- **Penerbitan**: `Rotate` dan `ConsumeLegacy` menyisipkan baris pengganti dengan
  `INSERT … SELECT ?, …, credential_version FROM users WHERE id = ?` di dalam transaksi yang sama. Versi yang tercatat karena itu konsisten dengan password yang berlaku pada saat commit — bukan hasil baca terpisah.
- **Konsumsi**: sebelum menerbitkan pengganti, `Rotate` membandingkan `refresh_tokens.credential_version` dengan `users.credential_version`. Beda ⇒ `revokeFamilyTx(..., "password_changed")` + `RotateRevoked` (401). Ini mematikan sesi yang terbit di jendela balapan pada refresh pertama.
- **Ganti/reset password**: satu transaksi di `UserRepository.ChangePasswordAtomic(ctx, userID, hash)`: `UPDATE users SET password_hash = ?, credential_version = credential_version + 1 WHERE id = ?` **dan** pencabutan semua family user. Error dikembalikan, tidak ditelan (sifat fail-closed yang sudah ada dipertahankan). `ChangePassword`/`ResetPassword` memanggil ini alih-alih dua operasi terpisah.
- Login lama yang lolos validasi sebelum commit tetap mungkin mendapat token (transaksi tidak bisa dibatalkan), tetapi token itu bernilai versi lama dan ditolak pada refresh pertama, sekaligus family-nya dicabut.

## Risiko tersisa (dinyatakan, bukan disembunyikan)
1. Access token yang sudah terbit tetap sah sampai `accessTTL` (15 menit) setelah ganti password. Menutup celah ini butuh klaim versi di access token + pemeriksaan di middleware (perubahan transport) — diusulkan sebagai change terpisah, tidak diklaim di sini.
2. `_txlock=immediate` berlaku untuk semua transaksi; mengurangi konkurensi tulis SQLite. Pindah ke Postgres kelak: `SELECT … FOR UPDATE`.
3. Grace 10 detik tidak membuat token yang sudah dikonsumsi bisa dipakai ulang: pemakaian ulang tetap 401; grace hanya mencegah pencabutan family.
