# Tasks: identity-hardening

Base: `0413c9cc`. **Tidak ada kode aplikasi sebelum G1/G2 disetujui owner.**
Ketergantungan melingkar G2/T1 diperbaiki: rencana tes ditulis **sebelum** approval (dokumen),
tes merah ditulis **sesudah** approval.

## Sebelum approval (boleh sekarang, dokumen saja)

- [x] T0 Rencana tes lengkap I1–I24 di `verification.md` (tanpa kode).

## Setelah G1/G2 disetujui (kode)

- [ ] T1 Tulis tes invarian **merah** untuk I1–I7 (role/guard) lebih dulu, buktikan gagal di base.
- [ ] T2 Implementasi guard role (§2.1) + guard konkurensi super admin terakhir (§2.2) sampai I1–I6 hijau.
- [ ] T3 Permission `akses.kelola` di `internal/authz/registry.go`, pemetaan route akses, update template HR, dan tutup semua jalur bypass (§2.3) sampai I7, I16, dan I21 hijau.
- [ ] T4 Migrasi `012_refresh_tokens.sql` (refresh_tokens + email_verification_tokens) + consume atomik + jendela grace `REFRESH_REUSE_GRACE` + `expires_at` tidak diperpanjang (§3.2–3.3) + semantik pencabutan (§3.4).
- [ ] T5 Sesi legacy (§3.5): terima sekali lalu tukar jadi family baru, tolak bila akun nonaktif, batas `LEGACY_REFRESH_ACCEPT_UNTIL`, log `legacy_refresh`, tanpa paksa-logout (I19, I19b).
- [ ] T6 Rate limit auth dengan trusted proxy, kunci ter-hash, respons generik (§4) sampai I10, I17, I18, I22 hijau.
- [ ] T7 Token verifikasi email acak + hash + kedaluwarsa + sekali pakai sampai I11, I12 hijau.
- [ ] T8 Tes konkurensi: dua refresh paralel (I13) dan replay (I14) dan pencabutan keluarga (I15).
- [ ] T9 Regresi: login/refresh/reset normal, mobile tidak berubah; `go vet`, `go test ./...`, CI hijau.
- [ ] T10 Laporkan bukti (perintah, exit code, run CI) di issue; PR tanpa merge.

Urutan wajib: T1 → T2 → T3 → (T4, T6, T7 paralel) → T5 → T8 → T9 → T10.
