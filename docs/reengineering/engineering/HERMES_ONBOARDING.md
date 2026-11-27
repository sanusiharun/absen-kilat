# Onboarding runbook — NOT RUN
1. Verify Hermes version, service health, GitHub identity and repository access without printing secrets.
2. Configure least-privilege repository token; no admin, merge or deploy privileges where possible.
3. Verify Claude/Codex invocation and authentication independently; use sandbox task, not production.
4. Configure durable event deduplication, atomic lease store, worker termination and budget; single coordinator only.
5. Verify owner identity mapping and approval retrieval; test forged/stale approvals.
6. Dry run: discovery issue → draft spec → blocked before approval → human-approved sandbox task → worker evidence → review → stop before merge.
7. Test restart, duplicate event, expired lease and unavailable worker. Record evidence and unresolved blockers. Enable autonomous dispatch only after human acceptance.