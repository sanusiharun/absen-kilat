# Verification: identity-refresh-corrective

Base `591c37b`. Rencana tes boleh ada sebelum approval; eksekusi butuh G1/G2.

## Gate evidence
| Gate | Artefak di-hash (urut) |
|---|---|
| G1 | `proposal.md` |
| G2 | `proposal.md`, `design.md`, `tasks.md`, `specs/*/spec.md` (urut path) |
| G3 | perintah, exit code, hasil tes, run CI, diff scope |

Digest: `python3 tools/validate_sdd.py --digest identity-refresh-corrective G1|G2` (framing panjang big-endian 8 byte, sama dengan change lain). `verification.md` **tidak** ikut di-hash ⇒ setiap laporan gate mencantumkan `PLAN_SHA256 = sha256(verification.md)`. Perubahan isi rencana tes tanpa approval baru = fail closed.

## Matriks tes
| ID | Skenario | Harapan | Ada di base? |
|---|---|---|---|
| C1 | Reuse token yang baru dirotasi pada +2s (dalam grace 10s) | 401; family aktif; token pengganti tetap usable | tidak |
| C2 | Reuse pada +12s (di luar grace) | 401; seluruh family dicabut; audit replay | tidak |
| C3 | Dua refresh paralel token sama | tepat satu 200; yang kalah 401; token pemenang usable | tes lama hanya hitung error |
| C4 | `consumed_at` tidak bisa diparse | fail closed (401), tidak fail open | tidak |
| C5 | Restart proses setelah cut-off legacy | token legacy ditolak; cut-off sama sebelum/sesudah restart | tidak (cut-off relatif proses) |
| C6 | `LEGACY_REFRESH_ACCEPT_UNTIL` tidak diset | jalur legacy menolak (fail closed) | tidak |
| C7 | Login password lama balapan dengan ganti password | sesi hasil balapan 401 pada refresh pertama; family dicabut | tidak (probe `7230473398` membuktikan kebocoran) |
| C8 | `credential_version` token ≠ user | 401 + family dicabut | tidak |
| C9 | `RevokeAllForUser` gagal saat ganti password | error dikembalikan, hash password tidak berubah | sudah ada (`TestRotation_PasswordChangeFailsClosedWhenRevokeFails`) — regresi wajib |
| C10 | Probe lama `7230473398` pada head baru | 6/6 PASS | 4 PASS, 2 FAIL di `9ca67c9` |

## Gate kualitas
`go test ./...` (backend), `go vet ./...`, `gofmt -l` pada file yang disentuh, CI backend/frontend/structure success pada head yang dilaporkan, dan tidak ada perubahan di luar `allowed_paths` (`backend/internal/{repo,service,domain,db}`, `openspec/changes/identity-refresh-corrective`).

## Approval binding
Paket approval WAJIB lewat DM Telegram tepercaya (komentar GitHub bukan approval) dan memuat `change=identity-refresh-corrective`, `gate`, `base_sha`, `head_sha`, digest G1/G2, `PLAN_SHA256`. `base_sha` harus ancestor `head_sha`. Head bergeser = approval baru.
