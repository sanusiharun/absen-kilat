# identity-refresh-corrective

Status: PROPOSED; G1/G2 pending. Base: `main` @ `591c37b` (PR #71 sudah merge; temuan review `7230894971`/`7232132757` berada di `main`).

## Masalah
Tiga perilaku wajib spec `identity-hardening` (R3, R8) tidak terpenuhi di `main`, dan satu di antaranya tidak tercakup R3 sama sekali:

1. **R3 grace tidak ada.** `RefreshTokenRepository.Rotate` (`backend/internal/repo/refresh_token_repo.go:55-63`) langsung `revokeFamilyTx(..., "replay")` untuk setiap `consumed_at` terisi. Retry klien yang wajar (timeout, double-submit) memaksa logout satu family.
2. **Cut-off legacy relatif proses.** `NewAuthService` (`backend/internal/service/auth_service.go:62`) menetapkan `legacyDeadline: time.Now().Add(7*24h)`. Setiap restart memperpanjang penerimaan token legacy — melanggar R8 ("cut-off SHALL NOT be extended", maksimum rilis+7 hari).
3. **Ganti password tidak terserialisasi terhadap penerbitan sesi.** `ChangePassword` (`auth_service.go:303-315`) mencabut family lalu meng-`UPDATE` `users` dalam dua transaksi terpisah, dan `issueTokens` hanya membaca `users`. Login dengan password lama yang lolos validasi sebelum `UPDATE` tetap mendapat family baru yang tidak pernah dicabut. Review `7230894971` menegaskan: *update+revoke atomik saja tidak cukup — jalur penerbitan harus memeriksa versi kredensial saat ini*.

## Scope
Perbaikan terbatas pada tiga butir di atas, dari `main` terbaru, mempertahankan perbaikan yang sudah lulus (consume legacy atomik, fail-closed expiry, `_txlock=immediate`, pencabutan fail-closed).

## Non-goals
- Slice 2b (`identity-hardening` T6/T7): rate limit auth, token verifikasi email. I10–I12, I17, I18 tetap terbuka.
- Tidak ada perubahan bentuk API, skema respons, RBAC, atau permission.
- Tidak ada perubahan `tools/validate_sdd.py`.
- Tidak ada merge, deploy, restart layanan, atau worker dispatch dalam change ini.
- Klaim versi kredensial di access token (lihat design, risiko tersisa #1) — bukan bagian change ini.

## Dependensi
Approval owner via DM Telegram yang memuat `change`, `gate`, `base_sha`, `head_sha`, digest G1/G2, dan `PLAN_SHA256` sebelum kode runtime disentuh (APPROVAL_PROTOCOL + policy koordinator).
