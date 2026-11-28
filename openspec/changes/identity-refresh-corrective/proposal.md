# identity-refresh-corrective (rev2)

Status: PROPOSED; G1/G2 pending. Base: `main` @ `c2a6ee18` (PR #74 sudah merge; spec rev1 `cb702d3`
berada di `main`). Rev2 menjawab review `7232428168`: C3 diblokir (provenance versi kredensial), C7 harus
membuktikan tidak ada sesi baru, C2 legacy mati secara default, C1 grace dibatasi, dan `allowed_paths` dikoreksi.

## Masalah
Tiga perilaku wajib spec `identity-hardening` (R3, R8) tidak terpenuhi di `main`, dan satu di antaranya tidak
tercakup R3 sama sekali:

1. **R3 grace tidak ada.** `RefreshTokenRepository.Rotate` (`backend/internal/repo/refresh_token_repo.go:55-63`)
   langsung `revokeFamilyTx(..., "replay")` untuk setiap `consumed_at` terisi. Retry klien yang wajar (timeout,
   double-submit) memaksa logout satu family.
2. **Cut-off legacy relatif proses dan aktif secara default.** `NewAuthService`
   (`backend/internal/service/auth_service.go:62`) menetapkan `legacyDeadline: time.Now().Add(7*24h)`. Setiap
   restart memperpanjang penerimaan token legacy — melanggar R8 ("cut-off SHALL NOT be extended", maksimum
   rilis+7 hari).
3. **Ganti password tidak terserialisasi terhadap penerbitan sesi.** `ChangePassword`
   (`auth_service.go:303-315`) mencabut family lalu meng-`UPDATE` `users` dalam dua transaksi terpisah, dan
   `issueTokens` hanya membaca `users`. Login dengan password lama yang lolos validasi sebelum `UPDATE` tetap
   mendapat family baru yang tidak pernah dicabut. Review `7230894971` menegaskan: *update+revoke atomik saja
   tidak cukup — jalur penerbitan harus memeriksa versi kredensial saat ini*; review `7232428168` menambahkan
   bahwa membaca versi **saat penerbitan** justru mencap validasi basi dengan versi baru.

## Scope
Perbaikan terbatas pada tiga butir di atas, dari `main` terbaru, mempertahankan perbaikan yang sudah lulus
(consume legacy atomik, fail-closed expiry, `_txlock=immediate`, pencabutan fail-closed):

- C1 grace R3 + batas konfigurasi grace + timestamp masa depan fail closed.
- C2 legacy **mati secara default**; bila diaktifkan, cut-off absolut wajib dan tidak boleh melewati
  rilis+7 hari (divalidasi saat start).
- C3 versi kredensial dengan provenance **saat verifikasi password**, penerbitan kondisional dalam transaksi
  yang sama, tanpa token pada mismatch, mencakup dua interleaving dan jalur legacy.

## Non-goals
- Slice 2b (`identity-hardening` T6/T7): rate limit auth, token verifikasi email. I10–I12, I17, I18 tetap terbuka.
- Tidak ada perubahan bentuk API, skema respons, RBAC, atau permission.
- Tidak ada perubahan `tools/validate_sdd.py`.
- Tidak ada merge, deploy, restart layanan, atau worker dispatch dalam change ini.
- Klaim versi kredensial di access token (lihat design, risiko tersisa #1) — bukan bagian change ini.

## Dependensi
Approval owner via DM Telegram yang memuat `change`, `gate`, `base_sha`, `head_sha`, digest G1/G2, dan
`PLAN_SHA256` sebelum kode runtime disentuh (APPROVAL_PROTOCOL + policy koordinator).
