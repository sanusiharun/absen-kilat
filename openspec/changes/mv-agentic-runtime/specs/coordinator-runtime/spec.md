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
The runtime SHALL accept an approval only when the author is in the configured owner
allowlist, the GitHub author type is `User`, the gate matches, the digest matches the
recomputed artefact digest, and the base SHA equals the base SHA of the approval record.
The runtime SHALL expose revocation without deleting history.
Agent-authored comments SHALL NOT be accepted as approval.

#### Scenario: forged approval
- GIVEN an approval comment authored by an agent account
- WHEN verify runs
- THEN it exits 11 and dispatch remains blocked

#### Scenario: stale approval after edit
- GIVEN an approval for digest `D1`
- WHEN an artefact changes so the recomputed digest is `D2`
- THEN verify exits 11 and the affected gate is invalidated

### R3a — Approval channel is human-only
The runtime SHALL NOT treat a GitHub comment author id as proof of human approval.
The runtime SHALL accept approval only from a channel that agents cannot write to
(out-of-band owner message with `APPROVE <change> <gate> <digest> <base_sha>`, or a signature
verifiable with a key absent from the worker host).
Until such a channel is configured, `dispatch` SHALL remain refused.

#### Scenario: agent-authored relay is not approval
- GIVEN a comment carrying a `[CHATGPT][TASK]` prefix that claims owner approval
- WHEN approval verify runs
- THEN it exits 11 and dispatch remains refused

### R4 — Budget and timeout fail-closed
The runtime SHALL refuse dispatch when a budget is unset, exhausted or unreadable.
The runtime SHALL enforce a wall-clock deadline per run and record the outcome.
The runtime SHALL record unknown usage as unknown, never as zero.
Default budget values SHALL be 50.000 tokens per change and 1800 s wall-clock, with at most two
transient attempts. A hard wall-clock deadline SHALL be enforced regardless of token telemetry;
when telemetry is `unavailable` the runtime SHALL charge a worst-case bound rather than zero.

#### Scenario: unset budget
- GIVEN no budget for change `X`
- WHEN dispatch runs
- THEN it exits 21 and starts no worker

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
