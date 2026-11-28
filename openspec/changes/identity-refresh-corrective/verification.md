# Verification: identity-refresh-corrective (rev2)

Base `c2a6ee18`. Rencana tes boleh ada sebelum approval; eksekusi butuh G1/G2.

## Gate evidence
| Gate | Artefak di-hash (urut) |
|---|---|
| G1 | `proposal.md` |
| G2 | `proposal.md`, `design.md`, `tasks.md`, `specs/*/spec.md` (urut path) |
| G3 | perintah, exit code, hasil tes, run CI, diff scope |

Digest: `python3 tools/validate_sdd.py --digest identity-refresh-corrective G1|G2` (framing panjang big-endian
8 byte, sama dengan change lain). `verification.md` **tidak** ikut di-hash ⇒ setiap laporan gate mencantumkan
`PLAN_SHA256 = sha256(verification.md)`. Perubahan isi rencana tes tanpa approval baru = fail closed.

## Matriks tes
| ID | Skenario | Harapan | Ada di base? |
|---|---|---|---|
| C1 | Reuse token yang baru dirotasi pada +2s (dalam grace 10s) | 401; family aktif; token pengganti tetap usable | tidak |
| C2 | Reuse pada +12s (di luar grace) | 401; seluruh family dicabut; audit replay | tidak |
| C3 | Dua refresh paralel token sama | tepat satu 200; yang kalah 401; token pemenang usable | tes lama hanya hitung error |
| C4 | `consumed_at` tidak bisa diparse | fail closed (401), tidak fail open | tidak |
| C5 | `consumed_at` di masa depan (elapsed < 0) | 401; family dicabut; audit `refresh_replay_future_timestamp`; grace tidak dipakai | tidak |
| C6 | `REFRESH_REUSE_GRACE` negatif / > 60s / tak bisa diparse | start gagal dengan error eksplisit | tidak |
| C7 | Login password lama, ganti password commit **setelah** verifikasi hash dan **sebelum** INSERT penerbitan (interleaving A) | 401; **tidak ada access token maupun refresh token** yang dikembalikan atau tersimpan; tidak ada baris `refresh_tokens` baru | tidak (probe `7230473398` membuktikan kebocoran) |
| C8 | Ganti password commit **sebelum** pembacaan hash (interleaving B) | 401; hash baru tidak cocok dengan password lama | tidak |
| C9 | Sesi yang tercatat dengan `credential_version` lama di-refresh | 401 + family dicabut; token pengganti tidak pernah naik ke versi baru | tidak |
| C10 | Token legacy setelah `credential_version` naik | 401 sebelum marker dibuat dan sebelum family terbit | tidak |
| C11 | `LEGACY_REFRESH_ENABLED` tidak diset | semua refresh legacy 401; tidak ada family baru | tidak (jalur legacy aktif default) |
| C12 | Legacy enabled, `LEGACY_REFRESH_ACCEPT_UNTIL` tidak diset / bukan RFC3339 | start gagal dengan error eksplisit | tidak |
| C13 | Legacy enabled, cut-off > `releaseDate` + 7 hari; atau binary tanpa `releaseDate` (config drift) | start gagal dengan error eksplisit | tidak |
| C14 | Restart proses sebelum/sesudah cut-off legacy | keputusan hanya menurut instant konfigurasi, bukan uptime | tidak (cut-off relatif proses) |
| C15 | `RevokeAllForUser` gagal saat ganti password | error dikembalikan, hash password tidak berubah | sudah ada (`TestRotation_PasswordChangeFailsClosedWhenRevokeFails`) — regresi wajib |
| C16 | Probe lama `7230473398` pada head baru | 6/6 PASS | 4 PASS, 2 FAIL di `9ca67c9` |

## Gate kualitas
`go test ./...` (backend), `go vet ./...`, `gofmt -l` pada file yang disentuh, CI backend/frontend/structure
success pada head yang dilaporkan.

## allowed_paths (dikoreksi rev2)
Diff gate korektif **hanya** boleh menyentuh:

- `backend/internal/repo/**` — `refresh_token_repo.go`, `user_repo.go`, `*_test.go`
- `backend/internal/service/**` — `auth_service.go`, `auth_rotation_test.go`
- `backend/internal/domain/**`
- `backend/internal/db/**` — termasuk `migrations/015_credential_version.sql`
- `backend/internal/config/**` — env baru + validasi start (`config.go`, `config_test.go`)
- `backend/cmd/api/main.go` — wiring `releaseDate` + validasi config
- `openspec/changes/identity-refresh-corrective/**`

Di luar daftar itu (termasuk frontend, `.github/workflows/`, `tools/validate_sdd.py`) ⇒ pelanggaran scope,
PR ditolak. Test berada di paket yang sama dengan kode yang diuji, sehingga tidak butuh entri terpisah;
`backend/internal/config/config_test.go` dan `backend/internal/service/auth_rotation_test.go` adalah path tes
konkret yang diverifikasi lewat `git diff --stat` sebelum digest diajukan.

## Approval binding
Paket approval WAJIB lewat DM Telegram tepercaya (komentar GitHub bukan approval) dan memuat
`change=identity-refresh-corrective`, `gate`, `base_sha`, `head_sha`, digest G1/G2, `PLAN_SHA256`.
`base_sha` harus ancestor `head_sha`. Head bergeser = approval baru.
