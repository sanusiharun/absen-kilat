# Tasks: identity-refresh-corrective

Base `591c37b`. **Tidak ada kode aplikasi sebelum G1/G2 + approval Telegram terverifikasi.**
Urutan wajib: C1 → C2 → C3 → C4 → C5.

## Sebelum approval (dokumen saja, boleh sekarang)
- [x] C0 Spec delta + rencana tes C1–C10 ditulis; digest G1/G2 dan `PLAN_SHA256` dihitung pada head yang di-review (laporan issue).

## Setelah approval (kode)
- [ ] C1 Tes merah lebih dulu: `TestRotation_ReuseWithinGraceNoRevoke`, `TestRotation_ReuseOutsideGraceRevokes`, `TestRotation_GraceWinnerTokenUsable`, `TestRotation_MalformedConsumedAtFailsClosed`; buktikan gagal di base.
- [ ] C2 Implementasi grace (`Rotate` + env `REFRESH_REUSE_GRACE` + outcome `RotateReuseInGrace` + audit `refresh_grace_reuse`) sampai C1 hijau.
- [ ] C3 Cut-off legacy absolut: env `LEGACY_REFRESH_ACCEPT_UNTIL`, hapus default relatif-proses, fail closed bila tidak diset; tes `TestRotation_LegacyCutoffAbsoluteAcrossRestart` sampai hijau.
- [ ] C4 Migrasi `015_credential_version.sql` + `ChangePasswordAtomic` + pemeriksaan versi pada `Rotate`/`ConsumeLegacy`; tes `TestRotation_CredentialVersionMismatchRevokes`, `TestRotation_PasswordChangeRaceOldLoginCannotRefresh`, regresi `TestRotation_PasswordChangeFailsClosedWhenRevokeFails`.
- [ ] C5 Jalankan ulang enam probe dari komentar `7230473398` pada head baru + `go test ./...`, `go vet ./...`, `gofmt -l`; lampirkan keluaran nyata.
- [ ] C6 Laporkan bukti (perintah, exit code, run CI, SHA) di issue; PR tanpa merge, tanpa auto-merge.

## Yang TIDAK boleh dilakukan dalam change ini
Merge/deploy, restart layanan, perubahan credential/secret, perluasan scope ke slice 2b, force-push, atau mengubah konfigurasi job cron.
