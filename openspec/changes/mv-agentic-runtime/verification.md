# Verification — mv-agentic-runtime

G1: PENDING. G2: PENDING. G3: NOT RUN. G4: PENDING.
No implementation authorized. Implementation must not start until an approval from the
authenticated owner Telegram DM (configured chat id, `chat.type == "private"`, anti-replay
`message_id`) names gate G1/G2, this change id, the canonical digest, base SHA, reviewed head SHA
and `plan_sha256`.

## Canonical digest

Digest dihitung dengan `python3 tools/validate_sdd.py --digest CHANGE GATE` (kanonik): untuk
setiap artefak, prefiks panjang 8-byte big-endian + path relatif posix, lalu prefiks yang sama +
byte file, semuanya ke satu SHA-256. G1 = `proposal.md`; G2 = `proposal.md` + `design.md` +
`tasks.md` + `specs/*/spec.md`. Tidak ada algoritma digest alternatif.

Karena `verification.md` tidak tercakup G2, setiap approval juga mengikat
`PLAN_SHA256 = sha256sum openspec/changes/<change>/verification.md` (nilai dicatat di laporan gate,
bukan di file ini). Nilai G1/G2 sebelum revisi 2: G1 `b41a5ddf…`, G2 `43e0afbe…`. Setiap revisi
wajib mencatat ulang nilai di issue; approval lama otomatis tidak valid.

## Required evidence per gate

| Gate | Evidence to record here |
|---|---|
| G1 | Telegram message id + chat id, canonical digest, base SHA, head SHA, `plan_sha256`, date |
| G2 | same as G1, digest covers proposal+design+tasks+specs |
| G3 | commands, revisions, exit codes, negative-test results, dry-run transcript |
| G4 | human merge/release authorization reference |

## Negative test matrix (must all fail closed)

| # | Scenario | Expected |
|---|---|---|
| N1 | Second acquire for the same change while lease active | exit 20, no second lease row |
| N2 | Dispatch with expired lease | refused |
| N3 | Same GitHub event id replayed | exit 10, no duplicate action |
| N4 | Approval relayed by GitHub comment, agent account or bot update | rejected |
| N5 | Approval digest mismatch after artefact edit | rejected |
| N6 | Base or head SHA moved since approval | rejected |
| N7 | Budget unset | dispatch refused (exit 21) |
| N8 | Budget exhausted mid-run | run failed, further dispatch refused |
| N9 | Worker ignores SIGTERM past grace | SIGKILL on process group, worktree quarantined |
| N10 | Worker writes outside `allowed_paths` | run failed, evidence attached, no auto-retry |
| N11 | Token telemetry unavailable | run records `telemetry=unavailable` **and** `reservation=50000`; remaining budget 0 |
| N13 | Dispatch after reservation-exhausted budget | exits 21, no worker started, no automatic retry |
| N14 | Crash between reservation hold and worker launch | no worker, exit 21, reservation stays held |
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
- Pada head `ab08930` artefak masih identik dengan revisi 5 (historis; digantikan revisi 6 di bawah).
- Kanal approval: **DM privat owner** terverifikasi (`from.id` = chat id owner, `chat.type == "private"`, anti-replay `message_id`); komentar GitHub `[CHATGPT]`/`[HERMES]` dan token bersama **bukan** approval dan tidak dapat di-ingest.
- Digest policy: `sha256:50da828d…` hanya **declared** (verbatim di komentar sumber `7225604464`), **tidak dapat direproduksi** dari repo. Bentuk kanonik di `CONTROLLED_AUTONOMY_V1.md` masih ambigu (judul-sampai-EOF vs blok marker) dan **menunggu konfirmasi owner**; tidak ada gate yang boleh bersandar pada digest policy sampai dikonfirmasi. Mismatch yang masih terbuka: `APPROVAL_PROTOCOL.md` menyebut komentar GitHub human-authored sebagai approval — perlu diselaraskan ke D5 lewat approval owner terpisah, tidak diubah dalam slice ini.
- G1/G2 tetap PENDING sampai approval Telegram menyebut change id + gate + digest + base/head + `plan_sha256`.

## Revisi 6 — koreksi review #58 (Telegram-only ingress, digest kanonik, reservasi atomik)

- Base: `456eb1ad9732613fca56dca3b0c0c8dd8e18b652` (main, PR #69 sudah merge; diverifikasi ancestor)
- Perubahan: schema/CLI/threat model + R3 + tasks §4 tidak lagi menerima identitas GitHub;
  digest key decision 6 diselaraskan ke framing `tools/validate_sdd.py`; approval mengikat
  `head_sha` dan `plan_sha256`; reservasi token atomik sebelum launch + fail closed saat crash (N14).
- `validate_sdd.py --digest mv-agentic-runtime G1` → `230db064b25417dd2dfc0e852a0bc9ecf2d1907f5a0f835edc44185dc5043fde` (tidak berubah; proposal tidak disentuh)
- `validate_sdd.py --digest mv-agentic-runtime G2` → `83947cced146c81db2278a741c1667208669651af49301774b08a589389fa4cc`
- `validate_sdd.py` → STRUCTURE PASS; `python3 -m unittest discover -s tools/tests` → 5 tests OK
- `PLAN_SHA256` dihitung pada commit revisi ini dan dicatat di laporan GitHub, bukan di file ini.
- Tidak ada kode aplikasi, worker dispatch, merge, deploy, atau biaya.
