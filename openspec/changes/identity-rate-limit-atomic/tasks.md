# Tasks: identity-rate-limit-atomic

Rencana eksekusi setelah **G1 + G2 disetujui**. Satu PR kecil (≤ 300 baris perubahan kode), satu branch
`fix/identity-rate-limit-atomic`, base `bf8c3d8f`. Urutan wajib: repo → service → handler → tes → bukti.

| # | Tugas | Path | Verifikasi |
|---|---|---|---|
| T1 | Ganti `Count`+`Increment` dengan `Reserve` (satu transaksi, upsert bersyarat, prune, rollback all-or-nothing) dan `Release` (refund `hits-1` dengan guard `hits > 0`) | `backend/internal/repo/auth_rate_limit_repo.go` | A1, A2, A3 |
| T2 | Definisikan `ReserveRequest`/`ReserveHandle`, ganti antarmuka store, pindahkan hashing kunci ke `AuthRateLimiter.Reserve`/`Release`; hapus `Allowed`/`Record` | `backend/internal/service/auth_ratelimit.go` | A1–A4 |
| T3 | Ganti `rateBlocked`+`recordAttempt` dengan satu `reserve(...)` per handler; login me-refund saat sukses; 429 generik untuk tolak maupun error; log tanpa materi kunci | `backend/internal/transport/auth_handler.go` | A2, A4, A5 |
| T4 | Sesuaikan tes lama yang memakai `Allowed`/`Record` (tanpa melonggarkan asersi I10/I17/I18/I22) | `backend/internal/service/auth_ratelimit_test.go`, `backend/internal/repo/auth_rate_limit_repo_test.go`, `backend/internal/transport/auth_handler_ratelimit_test.go` | seluruh suite |
| T5 | Tambah tes baru A1–A6 sesuai `verification.md` (konkurensi dua pool, injeksi error store, all-or-nothing, refund, non-regresi) | file tes yang sama + `auth_handler_atomic_test.go` | A1–A6 |
| T6 | Bukti: `gofmt -l`, `go vet ./...`, `go test ./... -race -count=1` (paket terkait), run CI, catat id run + SHA | — | semua |

## Batasan eksekusi

- Tidak boleh mengubah angka batas, `TRUSTED_PROXIES`, `clientip.go`, migrasi 017, atau `httpx` error shape.
- Tidak boleh menyentuh `refresh_token_repo.go` / rotasi (change lain sudah live).
- `-race` wajib pada paket `service`, `repo`, `transport`, karena inti perubahan adalah konkurensi.
- Bila A1 (konkurensi dua pool) flaky, hentikan dan lapor — jangan longgarkan asersi jumlah diterima.
- Bila `go test ./...` gagal karena 4 kegagalan baseline yang sudah tercatat di `KNOWN_BASELINE_ISSUES.md`,
  laporkan sebagai baseline, bukan regresi change ini (bandingkan dengan `bf8c3d8f`).

## Bukti yang harus dilampirkan di PR

1. Diff ringkas per file + alasan satu baris.
2. Output `go test -race -run 'RateLimit|Atomic' ./internal/... -v` (nama tes A1–A6 terlihat).
3. Id run CI `backend`/`structure`/`frontend` pada head PR.
4. Pernyataan eksplisit: tidak ada perubahan kontrak API, tidak ada perubahan skema, tidak ada dependensi baru.
