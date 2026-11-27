# Minimum viable agentic runtime (single coordinator)

Status: PROPOSED; G1/G2 pending. Change ID: `mv-agentic-runtime`.
Baseline: `c85edc9a96c637037f16e838592a6016902670c9` (discovery on source main).
Coordinator branch: `reengineering/sdd-foundation`.

## Problem

`TASK_DELEGATION.md`, `RECOVERY_PROTOCOL.md`, `COST_MANAGEMENT.md` and
`APPROVAL_PROTOCOL.md` describe controls that exist only as prose. Today: no atomic
lease, no durable event dedup, no worker termination, no budget enforcement and no
verifiable approval record. Because of that, autonomous dispatch stays disabled and
every phase needs a human in the loop for mechanics a script could do safely.

## Goal

A minimal, dependency-free runtime that makes the *existing* policies enforceable, so
that a single coordinator can run one worker per change with fail-closed behaviour:

1. **Atomic lease** — one active worker per change, no double dispatch.
2. **Durable dedup** — the same event (issue comment, CI result, dispatch) is never acted on twice.
3. **Worker termination + recovery** — a lease can be expired, a worker killed, and an
   uncertain worktree quarantined without losing unreviewed work.
4. **Approval binding** — an approval is only valid if it is human-authored by the
   configured owner, names the change + gate, and its SHA-256 digest matches the
   artefacts *and* the base SHA at dispatch time.
5. **Budget / timeout fail-closed** — no dispatch when a budget is unset, exceeded or
   unreadable; every run has a hard deadline and a recorded outcome.

## Non-goals

- No multi-coordinator support, no distributed lock, no message broker, no vector store.
- No new infrastructure: Python 3 standard library + SQLite file only.
- No changes to application behaviour, API, schema of the app, or deployment.
- No self-approval, no automatic merge, no autonomous deploy.
- No LLM cost optimisation beyond "do not call a model for unchanged state".

## Scope of deliverables (after G2)

- `runtime/` Python package (stdlib + `sqlite3`): lease store, dedup store, budget ledger,
  approval verifier, worker runner adapter, CLI.
- SQLite schema (single file, WAL) with migration versioning.
- Tests, including **negative tests** (forged approval, replayed event, expired lease,
  duplicate dispatch, budget exhaustion, worker that ignores termination).
- Runbook in `docs/reengineering/engineering/HERMES_ONBOARDING.md` updated to "RUN" for
  the steps that become executable, with recorded evidence.

## Owner decisions incorporated (revision 2, 2026-11-27)

- **D3 budget:** 50.000 token per change, target wall-clock 30 menit, maksimum 2 attempt
  transien. Budget kosong / tidak terbaca / terlampaui ⇒ dispatch **ditolak** (fail closed).
- **D3 timeout vs telemetry:** batas waktu adalah **hard timeout** yang bisa ditegakkan
  (SIGTERM→SIGKILL). Pemakaian token bergantung pada telemetri worker yang **tidak selalu
  tersedia**; runtime wajib membedakan `telemetry=reported` vs `telemetry=unavailable` dan
  tidak boleh memperlakukan "tidak diketahui" sebagai 0. Bila telemetri tidak tersedia,
  runtime menagih batas atas yang disepakati (worst-case) atau menolak dispatch berikutnya —
  pilihannya dicatat di design.
- **D4 bootstrap:** satu eksekusi manual terkontrol untuk membangun `runtime/`, dengan lease +
  digest + base SHA dicatat di komentar issue. Setelah itu dispatch hanya lewat runtime.
- **Approval identity:** identitas approval **tidak boleh** hanya mengandalkan author id,
  karena agen dan owner memakai akun GitHub yang sama (token owner). Dirancang kanal approval
  human-only (lihat design) sebelum dispatch diaktifkan.
- **Digest kanonik:** memakai `tools/validate_sdd.py --digest CHANGE GATE` (bukan algoritma
  alternatif). Digest G1/G2 dihitung per gate setelah artefak final.
- **Base SHA binding:** approval terikat base SHA; setelah PR #22 merge baseline berubah dan
  approval lama otomatis tidak valid sampai direkonsiliasi.

## Dependencies and assumptions

- GitHub remains the durable task record; the runtime is a *local* execution ledger.
- Owner allowlist is a fixed configured identity (`sanusiharun`, GitHub id 39834005).
- CI remains authoritative for build/test signals until reconciled.
- Digest format: SHA-256 over sorted `path\0sha256(content)` lines of the reviewed artefact set.

## Risks

- **False sense of control**: a green runtime test does not prove policy enforcement.
  Mitigation: negative tests are mandatory, and every dispatch records lease id + digest + base SHA.
- **Scope creep into a workflow engine.** Mitigation: interfaces below are the contract; anything
  outside it needs a separate change.
- **Killing a worker leaves a dirty worktree.** Mitigation: quarantine directory + never reset
  unreviewed commits (matches `RECOVERY_PROTOCOL.md`).

## Acceptance (G3 evidence)

- Negative tests listed in `specs/coordinator-runtime/spec.md` all fail-closed.
- Dry run: dispatch a read-only sandbox task, kill it mid-run, recover, and prove no duplicate
  dispatch and no lost work; record commands + exit codes in the issue.
