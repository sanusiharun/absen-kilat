# Change: identity-hardening

Status: DRAFT revisi 2 (koreksi pasca-review `7225799748`). Base final: `0413c9cc` (main setelah PR #54, #55, #68).
Sumber temuan: review read-only PR #55 (issue #57 comment `7225653761`), discovery #59, dan review `7225799748`.
**Tidak ada kode aplikasi yang boleh ditulis sebelum G1/G2 disetujui owner.**

## Why

Empat celah identitas/otorisasi terbukti ada di `main` (bukan dugaan). Rincian path dan baris:

1. **Eskalasi hak akses.** `AdminService.UpdateUser` (`backend/internal/service/admin_service.go:119-133`)
   hanya menjaga akun sendiri (tidak boleh menonaktifkan diri `:126`, tidak boleh menurunkan role admin
   sendiri `:128`). Tidak ada perbandingan role aktor vs target, sehingga pemegang `pengguna.ubah`
   (template HR punya) dapat menaikkan role pengguna lain atau dirinya sendiri ke `super_admin`.
   PR #55 memetakan endpoint akses ke `pengguna.ubah` (`internal/authz/routes.go:26-29`).
2. **Sesi tidak bisa dicabut.** Refresh token stateless 30 hari tanpa pengecekan `IsActive` dan tanpa
   rotasi/pencabutan; akun yang dinonaktifkan masih bisa memperpanjang sesi.
3. **Verifikasi email memakai user id sebagai token** (shortcut yang diakui di komentar kode).
4. **Tanpa rate limit** pada login/forgot/reset (hanya submit QR yang dibatasi per IP).

## What changes (ringkas, detail di design)

1. **Guard role** dengan invarian eksplisit dan guard konkurensi untuk super admin terakhir.
2. **Permission `akses.kelola`** dipisah dari `pengguna.ubah`, termasuk penutupan **jalur bypass**
   pemberian role/permission di semua endpoint tulis.
3. **Refresh token berotasi, sekali pakai, atomik**, dengan deteksi replay dan pencabutan seluruh
   keluarga token.
4. **Rate limit auth** dengan definisi trusted proxy, perilaku multi-instance, dan respons generik.
5. **Token verifikasi email** acak, kedaluwarsa, sekali pakai, disimpan sebagai hash.

## Koreksi yang diminta review dan bagaimana ditangani

| Poin review | Penanganan |
|---|---|
| `token_version` saja tidak menghasilkan single-use rotation | Diganti desain tabel `refresh_tokens` dengan consume atomik (lihat design §3) |
| Perlu tes concurrent refresh dan replay | `verification.md` I13–I15 |
| Semantik pencabutan | design §3.4 (revoke family, revoke all sesi) |
| Migrasi dan sesi legacy | design §3.5 |
| Invarian self-change role dan guard super admin terakhir | design §2, spec R1/R6 |
| Jalur bypass pemberian permission | design §2.3 (enumerasi endpoint, termasuk override untuk diri sendiri ⇒ 403) |
| Trusted proxy dan multi-instance rate limit | design §4 |
| Ketergantungan melingkar G2/T1 | tasks: rencana tes **sebelum** approval (dokumen), tes merah **sesudah** approval |

## Non-goals

- Tidak menyentuh payroll, absensi, atau UI admin di luar tombol/menu terdampak.
- Tidak mengubah kontrak API mobile kecuali penambahan field status.

## Definition of done

Seluruh tes di `verification.md` (I1–I24) hijau, `go vet` bersih, CI hijau, perilaku super_admin
tidak berubah selain aturan baru yang eksplisit.
