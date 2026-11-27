# Spec: identity-hardening

### R1 — Perubahan role oleh aktor yang tidak berhak
The service SHALL reject any role change where the new role outranks the actor, or where the target
currently holds `hr` or `super_admin`, unless the actor is an active `super_admin`.

#### Scenario: HR promotes an employee to super_admin
- GIVEN an actor with role `hr`
- WHEN the actor sets an employee's role to `super_admin`
- THEN the request is rejected with 403 and the stored role is unchanged

### R6 — Invariant perubahan role diri sendiri
The service SHALL reject every role change targeting the actor's own account, regardless of direction.

#### Scenario: supervisor demotes self
- GIVEN an actor with role `supervisor`
- WHEN the actor sets their own role to `employee`
- THEN the request is rejected with 403

### R7 — Super admin terakhir
The service SHALL reject demoting or deactivating the last active `super_admin`. The check and the
mutation SHALL happen inside one transaction so that two concurrent requests cannot both succeed.

#### Scenario: concurrent demotion
- GIVEN two active super admins and no others
- WHEN two requests demote them at the same time
- THEN exactly one succeeds and the other is rejected with 422

### R2 — Pemisahan permission
The service SHALL require `akses.kelola` (not `pengguna.ubah`) for endpoints that change a user's menu
permissions, SHALL reject `POST /admin/users` and `PATCH /admin/users/{id}` attempts that assign a role
the actor is not entitled to grant, and SHALL reject any permission-override change whose target is the
actor's own account.

#### Scenario: permission write without akses.kelola
- GIVEN an actor with `pengguna.ubah` and without `akses.kelola`
- WHEN they PATCH `/admin/users/{id}/permissions`
- THEN the request is rejected with 403

### R3 — Refresh token sekali pakai dan rotasi atomik
The service SHALL treat a refresh token as single-use. Consuming a token SHALL happen in one
transaction that (a) marks the presented token consumed, and (b) issues exactly one replacement in the
same token family. Presenting an already-consumed token within a configured grace window (default 10
seconds) after it was consumed SHALL be rejected with 401 without revoking the family; presenting it
after that window SHALL be treated as replay, SHALL revoke the whole family, and SHALL be recorded as
an audit event. Rotation SHALL NOT extend the family's expiry.

#### Scenario: concurrent refresh
- GIVEN one valid refresh token
- WHEN two refresh requests arrive simultaneously
- THEN exactly one returns 200 with a new token and the other returns 401
- AND the family stays active, and the newly issued token can still be used

#### Scenario: replay
- GIVEN a refresh token already consumed
- WHEN it is presented again
- THEN the request is rejected with 401 and every token in that family is revoked

### R4 — Rate limit autentikasi
The service SHALL limit authentication attempts per client IP and per account key. The client IP SHALL
be derived from `RemoteAddr` unless the connection originates from a configured trusted proxy. Failure
responses SHALL be identical whether or not the account exists.

#### Scenario: spoofed forwarded header
- GIVEN a client that is not a trusted proxy sending `X-Forwarded-For`
- WHEN it exceeds the per-IP limit
- THEN it is still rate limited based on `RemoteAddr`

### R5 — Token verifikasi email
The service SHALL use a random single-use token that expires within 24 hours, stored only as a hash,
and SHALL NOT use the user identifier as the token.

#### Scenario: replay of verification token
- GIVEN a verification token already consumed
- WHEN it is used again
- THEN the request is rejected with 401

### R8 — Sesi legacy
The service SHALL, until a configured cut-off date, accept a previously issued stateless refresh token
once, exchange it for a new session family, reject it when the account is inactive, and record a
`legacy_refresh` log entry without token material. After the cut-off it SHALL reject legacy tokens. No
mass logout is forced on release.

#### Scenario: legacy token upgraded once
- GIVEN a refresh JWT issued before this change
- WHEN it is presented before the cut-off
- THEN a new session family is issued and the same JWT is rejected if presented again
