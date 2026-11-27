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
