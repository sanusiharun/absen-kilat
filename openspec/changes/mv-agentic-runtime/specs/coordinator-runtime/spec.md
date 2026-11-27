# Coordinator runtime — delta spec

Change: `mv-agentic-runtime`. Delta over no prior spec (new capability).
Requirement IDs are stable and referenced by `tasks.md`.

## ADDED

### R1 — Atomic single lease per change
The runtime SHALL allow at most one active lease per change id at any time.
The runtime SHALL reject a second acquire while a lease is active.
The runtime SHALL bind a lease to `base_sha`, approved `digest`, `worker`, `task_ids` and `allowed_paths`.

#### Scenario: second acquire is rejected
- GIVEN an active lease for change `X`
- WHEN another acquire for `X` runs
- THEN it exits 20 and no additional lease row is created

#### Scenario: mismatched base SHA is rejected
- GIVEN an approval recorded at base `A`
- WHEN acquire runs with base `B`
- THEN it is rejected and the mismatch is recorded

### R2 — Durable event deduplication
The runtime SHALL persist processed event ids in durable storage.
The runtime SHALL treat a repeated event id as already processed without side effects.
The runtime SHALL preserve dedup state across process restarts.

#### Scenario: replayed event
- GIVEN event `E` already processed
- WHEN `E` is delivered again
- THEN the runtime exits 10 and performs no action

### R3 — Approval identity and digest binding
The runtime SHALL accept an approval only when it arrives on the authenticated owner Telegram DM
(R3a), the gate matches, the canonical digest matches the recomputed artefact digest, the base SHA
equals the approval base, the head SHA equals the reviewed head, and `plan_sha256` matches the
recomputed `verification.md` hash.
No identity SHALL be accepted from a CLI argument, issue text or GitHub comment.
The runtime SHALL expose revocation without deleting history.
Agent-authored comments SHALL NOT be accepted as approval.

#### Scenario: forged approval
- GIVEN a GitHub comment or Telegram relay authored by an agent account
- WHEN approval verify runs
- THEN it exits 11 and dispatch remains blocked

#### Scenario: stale approval after edit
- GIVEN an approval for digest `D1`
- WHEN an artefact changes so the recomputed digest is `D2`
- THEN verify exits 11 and the affected gate is invalidated

### R3a — Approval channel is human-only
The runtime SHALL NOT treat a GitHub comment author id as proof of human approval.
The runtime SHALL accept approval only from the **owner's private Telegram DM**, verified by
`from.id` == configured owner chat id AND `chat.type == "private"`, carrying
`APPROVE <change> <gate> <digest> <base_sha>`.
The runtime SHALL reject forwarded messages, group/channel messages, bot/agent-originated
messages, and any approval quoted from GitHub text.
The runtime SHALL prevent replay by recording `message_id` and expiring an approval window.
Until such a channel is configured, `dispatch` SHALL remain refused.

#### Scenario: agent-authored relay is not approval
- GIVEN a comment carrying a `[CHATGPT][TASK]` prefix that claims owner approval
- WHEN approval verify runs
- THEN it exits 11 and dispatch remains refused

### R4 — Budget and timeout fail-closed
The runtime SHALL refuse dispatch when a budget is unset, exhausted or unreadable.
The runtime SHALL reserve the run's worst-case token amount atomically in the same transaction
that creates the lease, before any worker starts, and SHALL fail closed (exit 21, no worker) if the
reservation cannot be committed or if the coordinator crashes between reservation and launch.
The runtime SHALL NOT return a reservation automatically.
The runtime SHALL enforce a wall-clock deadline per run and record the outcome.
The runtime SHALL record unknown usage as unknown, never as zero.
Default budget values SHALL be 50.000 tokens per change and 1800 s wall-clock, with at most two
transient attempts. A hard wall-clock deadline SHALL be enforced regardless of token telemetry;
when telemetry is `unavailable` the runtime SHALL reserve the **full 50.000-token budget** for that
run rather than zero, and SHALL record `reservation=50000` together with `telemetry=unavailable`.
A run whose reservation consumes the whole budget SHALL leave zero remaining budget for the change;
every subsequent dispatch for that change SHALL exit 21, start no worker, and SHALL NOT be retried
automatically.

#### Scenario: unset budget
- GIVEN no budget for change `X`
- WHEN dispatch runs
- THEN it exits 21 and starts no worker

#### Scenario: telemetry unavailable reserves the full budget
- GIVEN a budget of 50.000 tokens for change `X`
- WHEN a run finishes without token telemetry
- THEN the run record contains `telemetry=unavailable` and `reservation=50000`
- AND the remaining budget for `X` is 0

#### Scenario: dispatch after reservation-exhausted budget
- GIVEN the previous run for `X` reserved the full 50.000 tokens
- WHEN dispatch runs again for `X`
- THEN it exits 21, starts no worker, and records no automatic retry

### R5 — Worker termination and recovery
The runtime SHALL start a worker in its own process group and record its PID.
On timeout or cancellation the runtime SHALL send SIGTERM, wait a bounded grace period,
then SIGKILL the process group.
After an abnormal end the runtime SHALL mark the run `uncertain` and quarantine the worktree.
The runtime SHALL NOT reset or delete unreviewed commits.
Re-dispatch after an abnormal end SHALL require an explicit human note.

#### Scenario: worker ignores termination
- GIVEN a worker that ignores SIGTERM
- WHEN the deadline passes
- THEN the process group is SIGKILLed, the worktree is quarantined and the run is `uncertain`

### R6 — Scope enforcement
The runtime SHALL compare the produced diff against the lease `allowed_paths`.
A diff outside `allowed_paths` SHALL mark the run failed.
The runtime SHALL NOT auto-retry deterministic failures.

#### Scenario: out-of-scope write
- GIVEN a lease allowing `backend/**`
- WHEN the worker modifies `mobile/**`
- THEN the run fails and the evidence is attached

### R7 — Human-visible state
The runtime SHALL expose `status --json` with lease, budget, approvals and last run.
The runtime SHALL NOT merge, deploy or self-approve.

#### Scenario: status is authoritative for mechanics only
- GIVEN a completed run
- WHEN `status --json` is printed
- THEN it reports lease/budget/approval facts but never a PASS verdict for the change
