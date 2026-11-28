# Tasks: identity-refresh-corrective (rev2)

Base `c2a6ee18`. **Tidak ada kode aplikasi sebelum G1/G2 + approval Telegram terverifikasi.**
Urutan wajib: C1 → C2 → C3 → C4 → C5.

## Sebelum approval (dokumen saja, boleh sekarang)
- [x] C0 Spec delta + rencana tes ditulis; digest G1/G2 dan `PLAN_SHA256` dihitung pada head yang di-review.
- [x] C0b Rev2: C3 provenance, C7 semantik, C2 default-off, C1 bounds, `allowed_paths` dikoreksi.

## Setelah approval (kode)
- [ ] C1 Tes merah lebih dulu (grace): `TestRotation_ReuseWithinGraceNoRevoke`,
      `TestRotation_ReuseOutsideGraceRevokes`, `TestRotation_GraceWinnerTokenUsable`,
      `TestRotation_MalformedConsumedAtFailsClosed`, `TestRotation_FutureConsumedAtFailsClosed`; buktikan gagal di base.
- [ ] C1b Wiring config: `backend/internal/config/config.go` (`REFRESH_REUSE_GRACE`, batas `0..60s`) +
      `backend/cmd/api/main.go`; tes `TestConfig_GraceBoundsRejected` (negatif, di atas maks, tak bisa diparse) ⇒ start gagal.
- [ ] C2 Implementasi grace (`Rotate` + outcome `RotateReuseInGrace` + audit `refresh_grace_reuse`,
      `refresh_replay_future_timestamp`) sampai C1/C1b hijau.
- [ ] C3 Legacy default-off: `LEGACY_REFRESH_ENABLED` + `LEGACY_REFRESH_ACCEPT_UNTIL` absolut wajib;
      hapus default relatif-proses; tes `TestConfig_LegacyDisabledByDefault`,
      `TestConfig_LegacyCutoffRequired`,
      `TestRotation_LegacyCutoffAbsoluteAcrossRestart`.
- [ ] C3b Config drift legacy: `releaseDate` di-embed via `-ldflags` dan diteruskan ke `config.ValidateIdentity`;
      tes `TestConfig_LegacyCutoffExceedsReleaseWindowFailsClosed`,
      `TestConfig_LegacyReleaseDateMissingFailsClosed`, `TestConfig_LegacyCutoffInvalidFormatFailsClosed`.
- [ ] C4 Migrasi `015_credential_version.sql` + `ChangePasswordAtomic` + `issueTokensTx` kondisional
      (`WHERE users.credential_version = ?`) untuk Login/Register/VerifyEmail/Rotate; `Rotate` menyalin versi
      dari baris token, bukan dari `users`; `ConsumeLegacy` versi 0 kondisional.
- [ ] C4b Tes merah C3 lebih dulu: `TestRotation_CredentialVersionMismatchRevokes`,
      `TestRotation_PasswordChangeRaceOldLoginNoUsableSession` (interleaving A: tidak ada token, tidak ada baris),
      `TestRotation_PasswordChangeBeforeVerifyRejectsLogin` (interleaving B),
      `TestRotation_LegacyAfterCredentialChangeRejected`, regresi
      `TestRotation_PasswordChangeFailsClosedWhenRevokeFails`.
- [ ] C5 Jalankan ulang enam probe dari komentar `7230473398` pada head baru + `go test ./...`, `go vet ./...`,
      `gofmt -l`; lampirkan keluaran nyata.
- [ ] C6 Laporkan bukti (perintah, exit code, run CI, SHA, `git diff --stat` vs `allowed_paths`) di issue;
      PR tanpa merge, tanpa auto-merge.

## Yang TIDAK boleh dilakukan dalam change ini
Merge/deploy, restart layanan, perubahan credential/secret, perluasan scope ke slice 2b, force-push, atau
mengubah konfigurasi job cron.
