# Change: identity-hardening

Status: DRAFT (G1/G2 belum diajukan). Base: `0413c9cc` (main setelah PR #54, #55, #68).
Sumber temuan: review read-only PR #55 (issue #57 comment `7225653761`) dan discovery #59.

## Why

Empat celah identitas/otorisasi terbukti ada di `main`, bukan dugaan:

1. **Eskalasi hak akses.** `AdminService.UpdateUser` (`backend/internal/service/admin_service.go:119-133`)
   hanya menjaga akun sendiri (tidak boleh menonaktifkan diri `:126`, tidak boleh menurunkan role admin
   sendiri `:128`). Tidak ada perbandingan role aktor vs target, sehingga pemegang `pengguna.ubah`
   (template HR punya) bisa menaikkan role pengguna lain **atau dirinya sendiri** ke `super_admin`.
   PR #55 memetakan endpoint akses ke `pengguna.ubah` (`internal/authz/routes.go:26-29`), jadi
   kemampuan mengatur menu ikut terbawa.
2. **Refresh token stateless 30 hari** tanpa pengecekan `IsActive` dan tanpa rotasi/pencabutan.
3. **Verifikasi email memakai user id sebagai token** (shortcut yang diakui di komentar kode).
4. **Tanpa rate limit** pada login/forgot/reset (hanya submit QR yang dibatasi).

Dampak: satu akun HR yang disusupi bisa menjadi permanen `super_admin`, sesi tetap hidup walau akun
dinonaktifkan, dan percobaan tebak password tidak dibatasi.

## What changes

Hanya backend; tidak ada perubahan skema wajib kecuali bila rotasi refresh token memerlukan tabel.

1. **Guard eskalasi**: perubahan role yang melibatkan `super_admin` atau `hr` hanya oleh aktor
   `super_admin`; aktor tidak boleh menaikkan role dirinya sendiri; penurunan role admin terakhir
   tetap ditolak.
2. **Permission terpisah**: `akses.kelola` (mengatur menu/permission per user) dipisah dari
   `pengguna.ubah`; pemetaan route akses dipindah ke `akses.kelola`.
3. **Refresh token**: periksa `IsActive` + `role` saat refresh; rotasi token dengan pencabutan token
   lama; refresh token kedaluwarsa bila akun berubah status.
4. **Rate limit auth**: batas per IP + per akun untuk login/forgot/reset, gagal-diam (tanpa membocorkan
   keberadaan akun).
5. **Token verifikasi email**: acak (bukan user id), kedaluwarsa, sekali pakai.

## Non-goals

- Tidak menyentuh payroll, absensi, atau UI admin di luar tombol/menu terdampak.
- Tidak mengubah kontrak API publik yang sudah dipakai mobile, kecuali menambah field status.

## Risks

- Menambah tabel/kolom untuk pencabutan refresh token ⇒ migrasi aditif (nullable), rollout bertahap.
- Guard role bisa memblokir alur admin lama: perlu tes + pesan error jelas, dan super admin tetap bisa.

## Definition of done

Semua tes invarian di `verification.md` hijau, `go vet` bersih, CI hijau, dan perilaku lama untuk
super_admin tidak berubah.
