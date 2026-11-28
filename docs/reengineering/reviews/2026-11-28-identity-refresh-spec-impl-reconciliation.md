# Rekonsiliasi spec rev2 (`identity-refresh-corrective`) vs implementasi live

Tanggal: 2026-11-28 · Reviewer: Hermes coordinator (read-only + docs)
Base: `origin/main` @ `bc0abc7d` (setelah #79 dan #80 merge)
Spec acuan: `openspec/changes/identity-refresh-corrective/**` rev2 (PR #76, head `40c6c27`)
Implementasi acuan: PR #75 `095e120` + koreksi #79 `76a8501`, #80 `75e78ae`

Dokumen ini **tidak** mengubah perilaku. Semua temuan adalah perbandingan dokumen vs kode;
koreksi kode menyentuh auth sehingga butuh approval owner terpisah.

## Ringkasan

| # | Requirement spec rev2 | Status live | Bukti |
|---|---|---|---|
| C1 | Grace reuse default 10 dtk, maksimum 60 dtk, di luar batas ⇒ gagal start | **SESUAI** | `config.go:15-17` `DefaultReuseGrace=10s`, `MaxReuseGrace=60s`; `config.go:118` tolak di luar `0..60s`; `main.go:37` memanggil `ValidateIdentity()` |
| C2a | Cutoff legacy absolut dari env, tidak diturunkan dari uptime | **SESUAI** | `auth_service.go:55` `LEGACY_REFRESH_ACCEPT_UNTIL` RFC3339; `legacyDeadline` nol ⇒ tolak |
| C2b | Legacy hanya aktif bila **eksplisit** di-enable | **BELUM** | Tidak ada `LEGACY_REFRESH_ENABLED`; keaktifan hanya ditentukan ada/tidaknya cutoff |
| C2c | Cutoff tidak boleh > release instant binary + 7 hari | **BELUM** | Tidak ada `releaseDate` yang di-embed; nilai RFC3339 apa pun diterima |
| C2d | Konfigurasi legacy invalid ⇒ **gagal start** | **BELUM** | `auth_service.go:57,62` hanya `Warn`/`Error` lalu lanjut; proses tetap start |
| C3 | Versi dicap dari hasil verifikasi password; penerbitan kondisional `INSERT … SELECT … WHERE credential_version=?`; token di-mint setelah INSERT | **BELUM** | `Login` (`auth_service.go:150`) verifikasi via `FindByEmail`, lalu `issueTokens` (`:391`) `FindByID` **ulang** dan `Create` tanpa syarat; access token di-sign di `:386` sebelum baris refresh disimpan |
| C4 | Rotate menyalin versi dari baris token yang disajikan | **SESUAI** | Rotate membandingkan `tokenVersion` vs `users.credential_version`; #80 menambah `INSERT … SELECT` versi induk |

## Perbedaan yang tersisa (konkret)

### 1. Provenance `credential_version` saat login (paling material)
Spec rev2 menuntut versi dibaca **satu query bersama hash** dan penerbitan kondisional dalam transaksi:
`RowsAffected()==0` ⇒ rollback + 401 tanpa token. Kode live masih dua langkah:
verifikasi hash dari `FindByEmail` → `issueTokens` membaca ulang `FindByID` → `Create` tanpa syarat.
Jendela balapan "password berubah setelah verify, sebelum insert" tetap terbuka dan token bisa dicap versi baru
walau password yang diverifikasi sudah lama. Rotate sudah benar; hanya jalur login/register/verify-email yang belum.

### 2. Flag enable legacy
Spec: default off **karena tidak di-enable**, bukan karena cutoff kebetulan kosong. Operator tidak punya
saklar eksplisit; menghapus satu variabel env = mematikan jalur legacy, tetapi tidak ada penanda niat.

### 3. Batas 7 hari dan release instant
Tidak ada `releaseDate` di binary. Cutoff `2099-01-01T00:00:00Z` akan diterima apa adanya; spec menyebut
kondisi ini harus menolak start.

### 4. Fail-closed saat start untuk config legacy
`LoadLegacyRefreshCutoff` mencatat log lalu lanjut. Dampaknya token legacy ditolak (aman), tetapi operator
tidak mendapat kegagalan start yang spec janjikan — drift konfigurasi lolos diam-diam.

## Rekomendasi

1. **Blocker perilaku**: C3 jalur login — conditional insert + mint setelah sukses. Butuh approval owner (auth).
2. C2b/C2c/C2d satu paket kecil: `LEGACY_REFRESH_ENABLED`, `releaseDate` (ldflags), validasi `+7d` dan
   `ValidateIdentity` mengembalikan error ⇒ start gagal.
3. Sampai itu di-approve, spec rev2 **jangan** dianggap sudah diimplementasikan penuh; status: C1 dan C4 sesuai,
   C2 sebagian, C3 belum.

## Catatan proses

- Saat audit, worktree utama berada di `docs/shift-247-s1s2` (12 commit di belakang `origin/main`); semua bukti
diambil dari `origin/main`, bukan working tree lokal.
- Tidak ada secret/PII; tidak ada perubahan kode aplikasi di PR ini.
