# Tasks — blocked pending G1/G2

Change ID: `mv-agentic-runtime`. No implementation authorized before G2.

## 1. Skeleton and store
- [ ] 1.1 Create `runtime/store.py` with schema above + migration runner (`PRAGMA user_version`).
- [ ] 1.2 Test: fresh DB creates all tables; re-running migration is a no-op (idempotent).

## 2. Atomic leases
- [ ] 2.1 Implement `lease acquire` using `BEGIN IMMEDIATE` + unique `change_id`.
- [ ] 2.2 Reject acquire when an active lease exists (exit code 20) or `base_sha`/`digest` differ.
- [ ] 2.3 Implement `renew`, `release`, `expire`.
- [ ] 2.4 Test negative: two acquires in parallel processes ⇒ exactly one succeeds.
- [ ] 2.5 Test negative: expired lease blocks dispatch until explicit `expire`.

## 3. Durable dedup
- [ ] 3.1 `dedup check` inserts `INSERT OR IGNORE` and reports first-seen vs seen-before.
- [ ] 3.2 Test: same event id twice ⇒ second call exits 10, no side effects.
- [ ] 3.3 Test: dedup survives process restart (new interpreter, same DB).

## 4. Approval verification
- [ ] 4.1 `approval record` stores author login + id, gate, digest, base SHA, comment URL.
- [ ] 4.2 `approval verify` re-queries GitHub via `gh api` and compares: author id in allowlist,
      author type `User`, digest, base SHA, not revoked.
- [ ] 4.3 Test negative: agent-authored comment ⇒ invalid.
- [ ] 4.4 Test negative: digest mismatch (edited artefact) ⇒ invalid.
- [ ] 4.5 Test negative: base SHA moved ⇒ invalid.
- [ ] 4.6 Test negative: approval copied from another change ⇒ invalid.

## 5. Budget and timeout
- [ ] 5.1 `budget set`/`charge`; unset budget blocks `dispatch` (exit 21).
- [ ] 5.2 Hard wall-clock deadline enforced by `runner` (SIGTERM → SIGKILL at deadline).
- [ ] 5.3 Test: budget exhausted mid-run ⇒ run `failed`, dispatch blocked afterwards.
- [ ] 5.4 Test: unknown usage is recorded as `unknown`, never as 0.

## 6. Worker runner and recovery
- [ ] 6.1 Start worker in its own process group; record PID; heartbeat to `runs`.
- [ ] 6.2 Terminate: SIGTERM, 10s grace, SIGKILL process group.
- [ ] 6.3 Post-run: record commit SHA, diff scope vs `allowed_paths`, exit code.
- [ ] 6.4 Out-of-scope diff ⇒ outcome `failed`, evidence attached, no auto-retry.
- [ ] 6.5 Kill mid-run ⇒ worktree quarantined, outcome `uncertain`, re-dispatch needs human note.
- [ ] 6.6 Test: kill during a long task leaves no duplicate dispatch and no lost commits.

## 7. Coordinator CLI and wiring
- [ ] 7.1 `dispatch` refuses unless lease + approvals + budget all valid (single exit path).
- [ ] 7.2 `status --json` prints lease, budget, approvals and last run.
- [ ] 7.3 Cron integration note: monitor script unchanged; `dispatch` only via explicit verb.

## 8. Tooling kanonik dan kanal approval
- [ ] 8.0 Pakai `python3 tools/validate_sdd.py --digest CHANGE GATE` untuk digest G1/G2;
      jangan menambah algoritma digest alternatif. Rekam nilai digest per gate di issue.
- [ ] 8.0a Implementasi kanal approval human-only (pilihan owner: OOB token / signature / sesi
      terpisah) sebelum dispatch diaktifkan; author id saja **tidak cukup**.
- [ ] 8.0b Bind approval ke `base_sha`; setelah PR #22 merge, rekonsiliasi baseline dan tandai
      approval lama invalid.

## 9. Verification and handover
- [ ] 8.1 Run full negative-test suite; record commands + exit codes.
- [ ] 8.2 Update `HERMES_ONBOARDING.md` steps 4/5/7 from "pending" to "RUN" where evidence exists.
- [ ] 8.3 Record `COST_MANAGEMENT.md` budget defaults concretely (values chosen by owner).
- [ ] 8.4 G3 evidence bundle + G4 release approval request.

## D6 — keputusan owner yang sudah mengikat (2026-11-27)

- **Budget**: 50.000 token per change, wall-clock 1800 s, maksimum 2 attempt transient.
- **Reservasi worst-case**: saat telemetri token `unavailable`, tagih **penuh** 50.000 token.
  Konsekuensi yang harus dilaporkan runtime: dengan budget 50k hanya ada **satu** run per change
  sampai telemetri tersedia.
- **Kanal approval**: DM privat owner, verifikasi `from.id` + `chat.type == "private"`,
  anti-replay `message_id` + jendela kedaluwarsa, binding `change + gate + digest + base_sha`,
  dukung `REVOKE`, audit record. Relay GitHub **bukan** autentikasi manusia.
