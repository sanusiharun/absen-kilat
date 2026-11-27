# Verification — mv-agentic-runtime

G1: PENDING. G2: PENDING. G3: NOT RUN. G4: PENDING.
No implementation authorized. Implementation must not start until a human-authored
approval (verified owner `sanusiharun`, id 39834005) names gate G1/G2, this change id and
the SHA-256 digest of the reviewed artefacts.

## Canonical digest

Digest dihitung dengan `python3 tools/validate_sdd.py --digest CHANGE GATE` (kanonik).
Nilai G1/G2 sebelum revisi 2: G1 `b41a5ddf…`, G2 `43e0afbe…`. Setelah revisi 2 nilai berubah
dan wajib dicatat ulang di issue; approval lama otomatis tidak valid.

## Required evidence per gate

| Gate | Evidence to record here |
|---|---|
| G1 | approval comment URL, author login+id, digest, base SHA, date |
| G2 | approval comment URL, digest of proposal+design+tasks+specs, base SHA |
| G3 | commands, revisions, exit codes, negative-test results, dry-run transcript |
| G4 | human merge/release authorization reference |

## Negative test matrix (must all fail closed)

| # | Scenario | Expected |
|---|---|---|
| N1 | Second acquire for the same change while lease active | exit 20, no second lease row |
| N2 | Dispatch with expired lease | refused |
| N3 | Same GitHub event id replayed | exit 10, no duplicate action |
| N4 | Approval authored by an agent account | rejected |
| N5 | Approval digest mismatch after artefact edit | rejected |
| N6 | Base SHA moved since approval | rejected |
| N7 | Budget unset | dispatch refused (exit 21) |
| N8 | Budget exhausted mid-run | run failed, further dispatch refused |
| N9 | Worker ignores SIGTERM past grace | SIGKILL on process group, worktree quarantined |
| N10 | Worker writes outside `allowed_paths` | run failed, evidence attached, no auto-retry |
| N11 | Token telemetry unavailable | run records `telemetry=unavailable` **and** `reservation=50000`; remaining budget 0 |
| N13 | Dispatch after reservation-exhausted budget | exits 21, no worker started, no automatic retry |
| N12 | Approval relayed by an agent account (shared token) | rejected; requires human-only channel |

## Positive dry run (after G2)

1. Read-only sandbox task on a scratch branch; record lease id, base SHA, digest.
2. Kill it mid-run; confirm quarantine + `uncertain` outcome.
3. Re-dispatch with human note; confirm no duplicate work and no lost commits.
4. Attach `status --json` before/after and the full command transcript.

## Skipped checks policy

Any check not executed must be listed with the reason. Static inspection never substitutes
for execution (`VERIFICATION.md`).

## Digest kanonik (revisi 5, base final)

- Base final: `456eb1ad9732613fca56dca3b0c0c8dd8e18b652` (main setelah PR #69; CI backend/frontend/structure success)
- Rekonsiliasi: `origin/main` sudah ancestor branch (`git merge-base --is-ancestor` ok) — merge `1fa4f04`/`43a8c4f`, tanpa force push
- Koreksi doc-only: `815e6fa` (reservasi penuh 50000, fail closed, N13)
- Cleanup artefak: `5fc9ec4` (`__pycache__`/`*.pyc` tidak lagi dilacak)
- `validate_sdd.py --digest mv-agentic-runtime G1` -> `230db064b25417dd2dfc0e852a0bc9ecf2d1907f5a0f835edc44185dc5043fde`
- `validate_sdd.py --digest mv-agentic-runtime G2` -> `2e7742802a232948259672d1299e1daf453e096289cef561edfc1442e4b01db3`
- `validate_sdd.py` -> STRUCTURE PASS · `python3 -m unittest discover -s tools/tests` -> 5 tests OK

Digest pra-revisi (b41a5ddf…/43e0afbe…/1ce67194…/e1645da2…) **tidak berlaku lagi** dan tidak boleh dipakai untuk persetujuan.

## Gate check ulang #58 — base `d082b134` diganti ke main terverifikasi

- Base lama `d082b13493c80c55660bba8e0ce93a224fa424a1` masih ancestor, tetapi bukan base verifikasi terbaru.
- Base verifikasi terbaru: `456eb1ad9732613fca56dca3b0c0c8dd8e18b652` (main setelah PR #69).
- Head saat gate check: `ab0893075bebc0a072f16c9b165518f7ab867df7` (branch `docs/sdd-mv-agentic-runtime`).
- Artefak change tidak berubah sejak revisi 5: G1/G2 dihitung ulang pada head ini dan hasilnya identik.
- Kanal approval: komentar human-authored owner `sanusiharun` (id 39834005) di issue; `[CHATGPT]`/`[HERMES]` dan token bersama bukan approval.
- Digest policy yang mengikat sesi ini: `sha256:50da828d3c961fc4aaa5f04344141d3281c669a2c5fcd4e1522ac95a63f2ef90` (declared; lihat `docs/reengineering/engineering/CONTROLLED_AUTONOMY_V1.md`).
- G1/G2 tetap PENDING sampai approval manusia menyebut change id + digest + gate. Tidak ada implementasi runtime, worker dispatch, merge, atau deploy.
