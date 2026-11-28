# Draft design — mv-agentic-runtime

Single process, single machine, single coordinator. Python 3 standard library plus
`sqlite3`. No network services; GitHub is reached only through the `gh` CLI (already
authenticated) and is treated as an untrusted data source.

## Components

```
runtime/
  store.py        SQLite schema + migrations (WAL, foreign_keys=ON)
  lease.py        acquire / renew / release / expire  (atomic, single-writer)
  dedup.py        event_id -> processed_at (INSERT OR IGNORE = dedup)
  budget.py       per-change budget ledger (tokens, wall-clock, attempts)
  approvals.py    verify owner identity, gate, digest, base SHA binding
  runner.py       worker adapter: start, heartbeat, terminate, quarantine
  cli.py          coordinator verbs (see interfaces)
tests/            positive + negative tests (unittest)
```

## SQLite schema (single file `runtime.db`)

```sql
CREATE TABLE leases (
  change_id   TEXT PRIMARY KEY,      -- one active lease per change
  lease_id    TEXT NOT NULL UNIQUE,
  owner       TEXT NOT NULL,         -- coordinator id
  worker      TEXT NOT NULL,         -- claude | codex
  task_ids    TEXT NOT NULL,         -- JSON array
  allowed_paths TEXT NOT NULL,       -- JSON array
  base_sha    TEXT NOT NULL,
  digest      TEXT NOT NULL,         -- approved spec digest (canonical validator)
  plan_sha256 TEXT NOT NULL,         -- sha256 of verification.md, bound at approval
  issued_at   TEXT NOT NULL,
  expires_at  TEXT NOT NULL,
  state       TEXT NOT NULL CHECK (state IN ('active','released','expired','quarantined')),
  pid         INTEGER
);

CREATE TABLE events (               -- durable dedup
  event_id    TEXT PRIMARY KEY,     -- e.g. gh:issue_comment:7219979023
  kind        TEXT NOT NULL,
  seen_at     TEXT NOT NULL
);

CREATE TABLE budget (
  change_id   TEXT PRIMARY KEY,
  tokens_limit INTEGER,             -- NULL => dispatch forbidden (fail closed)
  tokens_used  INTEGER NOT NULL DEFAULT 0,
  tokens_reserved INTEGER NOT NULL DEFAULT 0, -- held before worker launch
  wall_limit_s INTEGER,
  wall_used_s  INTEGER NOT NULL DEFAULT 0,
  attempts     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE approvals (
  change_id    TEXT NOT NULL,
  gate         TEXT NOT NULL CHECK (gate IN ('G1','G2','G3','G4')),
  channel      TEXT NOT NULL CHECK (channel = 'telegram_dm'), -- only authenticated ingress
  chat_id      INTEGER NOT NULL,    -- owner DM chat id, taken from the Telegram update
  message_id   INTEGER NOT NULL,    -- anti-replay identity
  message_hash TEXT NOT NULL,       -- sha256 of normalized approval text
  digest       TEXT NOT NULL,       -- canonical validator digest
  base_sha     TEXT NOT NULL,
  head_sha     TEXT NOT NULL,       -- exact reviewed head
  plan_sha256  TEXT NOT NULL,       -- sha256 of verification.md (test plan)
  created_at   TEXT NOT NULL,
  revoked_at   TEXT,
  PRIMARY KEY (change_id, gate),
  UNIQUE (channel, chat_id, message_id)
);

CREATE TABLE runs (
  run_id      TEXT PRIMARY KEY,
  change_id   TEXT NOT NULL,
  lease_id    TEXT NOT NULL,
  started_at  TEXT NOT NULL,
  ended_at    TEXT,
  exit_code   INTEGER,
  outcome     TEXT,                 -- verified | failed | killed | uncertain
  note        TEXT
);
```

## Interfaces (the contract)

CLI (all verbs idempotent, all fail closed, machine-readable `--json`):

```
runtime lease acquire  --change ID --worker W --base-sha SHA --digest D \
                       --task-ids 1,2 --paths "backend/**,docs/**" --ttl 45m
runtime lease renew    --change ID --lease-id L --ttl 45m
runtime lease release  --change ID --lease-id L --outcome verified|failed|killed
runtime lease expire   --change ID                     # mark expired + quarantine worktree
runtime dedup check    --event-id E                    # exit 0 unseen, exit 10 already seen
runtime budget set     --change ID --tokens N --wall 2h
runtime budget charge  --change ID --tokens N --wall-seconds S
runtime approval ingest --change ID --gate G1            # consume one authenticated Telegram DM
runtime approval verify --change ID --gate G1           # exit 0 valid, 11 invalid/missing
runtime dispatch       --change ID                     # refuses unless lease+approvals+budget ok
runtime status         --change ID [--json]
```

Python API mirrors the verbs (`LeaseStore`, `DedupStore`, `BudgetLedger`,
`ApprovalVerifier`, `WorkerRunner`) so tests exercise the same paths as the CLI.
`approval ingest` accepts **no** author, comment or identity argument: the chat id, message id
and text hash are read from the already-authenticated Telegram update. A caller-supplied
identity is never accepted.

## Key decisions

1. **Single writer.** SQLite in WAL with `BEGIN IMMEDIATE` for lease acquisition. The
   uniqueness of `leases.change_id` is the atomicity primitive: a second acquire for an
   active lease raises instead of queueing. No distributed lease until a second
   coordinator exists (out of scope).
2. **Dedup keyed by GitHub event id**, not by content hash, so an edited comment is a new
   event while a re-delivered one is not.
3. **Approval validity is a tuple** `(change_id, gate, channel = telegram_dm, chat_id =
   configured owner, message_id, message_hash, digest, base_sha, head_sha, plan_sha256)`; any
   drift invalidates. Identity comes from the authenticated Telegram update, never from a CLI
   argument, and GitHub comments are reference only. `revoked_at` allows withdrawal without
   deleting history.
4. **Budget default is "unset ⇒ forbid"** (fail closed), matching `COST_MANAGEMENT.md`.
5. **Worker termination**: SIGTERM → grace 10s → SIGKILL on the recorded PID **and** its
   process group; after a kill the worktree is moved to `quarantine/<change>-<ts>/` and the
   run is recorded `killed`/`uncertain`; re-dispatch requires an explicit human note.
6. **Digest** is the canonical one produced by `python3 tools/validate_sdd.py --digest CHANGE
   GATE`: for each artefact, an 8-byte big-endian length prefix followed by the relative posix
   path, then the same prefix followed by the file bytes, all fed to one SHA-256. G1 covers
   `proposal.md`; G2 covers `proposal.md` + `design.md` + `tasks.md` + `specs/*/spec.md`.
   No alternative or parallel digest algorithm is allowed. Because the canonical digest does
   not cover `verification.md`, the approval also binds `plan_sha256 = sha256sum
   openspec/changes/<change>/verification.md`, and `verify` recomputes both before dispatch.

## Budget, timeout and telemetry (revision 2)

| Parameter | Nilai | Penegakan |
|---|---|---|
| `tokens_limit` | **50.000 / change** | charge per run; bila terlampaui → run `failed` |
| `wall_limit_s` | **1800** (target 30 menit) | hard deadline runner: SIGTERM → 10s → SIGKILL process group |
| `attempts` | **2** (transien saja) | attempt ke-3 ditolak; kegagalan deterministik tidak di-retry |
| budget tidak diset/tidak terbaca | — | **dispatch ditolak** (fail closed, exit 21) |

**Telemetri token tidak setara timeout.** Wall-clock selalu bisa ditegakkan; pemakaian token
hanya diketahui bila worker melaporkannya. Runtime mencatat `telemetry` per run:
`reported` (angka dari worker) atau `unavailable`. Untuk `unavailable` runtime memakai
**billing worst-case** (batas atas yang disepakati owner) agar anggaran tidak pernah "terlihat
kosong padahal terpakai". Nilai `unknown` tidak pernah ditulis sebagai 0.

**Reservasi atomik sebelum launch.** `dispatch` holds the worst-case amount
(`tokens_used + tokens_reserved + reservation ≤ tokens_limit`) in the same `BEGIN IMMEDIATE`
transaction that creates the lease; only after the hold commits is a worker started. If the hold
cannot be written, budget is unreadable, or the coordinator crashes between hold and start, the
dispatch fails closed (exit 21), no worker runs, and the reservation stays held until an explicit
human release. A crash during a run leaves the run `uncertain` and never returns the reservation
automatically.

## Approval identity — kanal human-only (revision 2)

Masalah: komentar dari agen dan dari owner memakai akun GitHub yang sama, jadi `author_id`
tidak membuktikan manusia. **Owner decision (D5, 2026-11-27): kanal = Telegram owner DM.**

Format pesan (kanal ini): `APPROVE <change> <gate> <digest> <base_sha> <head_sha> <plan_sha256>`.
Pesan 4-field lama (tanpa `head_sha`/`plan_sha256`) tidak berlaku dan ditolak.

Verifikasi yang wajib diimplementasikan (bukan sekadar "chat-nya privat"):

| Aspek | Aturan |
|---|---|
| Identitas pengirim | `message.from.id` == chat id owner yang dikonfigurasi (bukan username, bukan nama tampilan) |
| Jenis chat | DM privat (`chat.type == "private"`), tolak grup/channel/forward |
| Anti-replay | `message_id` + hash isi dicatat; pesan yang sama tidak bisa dipakai dua kali; window kedaluwarsa (mis. 30 menit) |
| Binding | change id + gate + digest + base SHA + head SHA + `plan_sha256` harus cocok persis; digest dan plan hash dihitung ulang saat verifikasi |
| Pemisahan dari agen | pesan yang berasal dari bot/agen (mis. dikirim via API dengan flag bot) ditolak; runtime hanya membaca DM manusia, dan **tidak pernah** menerima approval yang dikutip dari GitHub |
| Pencabutan | owner bisa mengirim `REVOKE <change> <gate>`; record ditandai, bukan dihapus |
| Audit | record approval menyimpan message_id, waktu, chat id (tanpa isi sensitif), digest, base SHA |

Catatan penting: relay GitHub (prefix `[CHATGPT][…]`) **bukan** autentikasi manusia dan tidak
dapat di-ingest sebagai approval — dipakai hanya sebagai rujukan percakapan. `approval ingest`
tidak menerima identitas dari pemanggil; bila update Telegram tidak lolos pemeriksaan chat id,
jenis chat, atau replay, ingest gagal dan `dispatch` tetap ditolak.

Sampai kanal DM terkonfigurasi dan diverifikasi, dispatch tetap disabled (fail closed).

**Worst-case bound (DIPUTUSKAN owner, comment 7222715922):** reservasi **penuh** 50.000 token
per run ketika telemetri `unavailable`, sehingga anggaran tidak pernah terlihat tersisa padahal
sudah terpakai. Konsekuensinya: dengan budget 50k hanya ada **satu** run per change
sampai telemetri tersedia; runtime harus melaporkan batas ini, bukan menyembunyikannya.

## Threat model

| Threat | Control |
|---|---|
| Agent-authored or forged approval | approval exists only as an authenticated Telegram DM from the configured owner chat; identity is taken from the update, never from caller arguments or issue text; GitHub comments are reference only |
| Approval replay after edits | digest + base_sha bound; recompute before dispatch |
| Prompt injection in issue/PR/log text | text treated as data; runtime never derives commands from repo content; dispatch requires an explicit change ID the human named |
| Duplicate dispatch / double worker | unique lease row + dedup on event id |
| Stolen or over-privileged token | least-privilege token plan (#57); runtime never reads credentials, only invokes `gh` |
| Runaway cost | budget set required; charge per run; stop at limit |
| Worker escapes its scope | allowed_paths recorded in lease and diffed post-run; out-of-scope diff ⇒ run `failed`, evidence recorded |
| Orphaned/killed worker keeps writing | process-group kill + post-run branch head check before accepting results |
| Local state tampering | runtime.db caches approvals; every `verify` re-checks channel, chat id, message id/hash, digest, base/head and plan hash before dispatch, and GitHub is never consulted for approval |

## Out of scope / open questions

- Multi-coordinator + distributed lock (needs a different primitive).
- Where runtime.db lives in CI (if ever needed).
- Whether `openspec validate` replaces the repo's own `tools/validate_sdd.py` (see #57 plan).

## 9. Isolasi kredensial (keputusan owner 2026-11-27)

Runtime menerima `credential_path` dari konfigurasi. Path divalidasi: ditolak bila berada di dalam
repository atau di dalam workspace agent (lihat `docs/reengineering/engineering/CREDENTIAL_ISOLATION.md`).
Kredensial disuntikkan lewat environment saat eksekusi worker, tidak ditulis ke disk oleh runtime dan
tidak pernah muncul di log/komentar. Identitas koordinator, worker, dan deployment terpisah.
