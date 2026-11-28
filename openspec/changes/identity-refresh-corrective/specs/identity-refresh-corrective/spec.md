## ADDED Requirements

### Requirement: Grace window for benign refresh reuse
The service SHALL reject a reuse of an already-consumed refresh token inside a configured grace window (default 10 seconds) with 401 without revoking the token family, and SHALL treat reuse after that window as replay.

#### Scenario: Retry inside the grace window
- WHEN a rotated refresh token is presented again within the grace window
- THEN the request returns 401
- AND the family stays active and the replacement token remains usable.

#### Scenario: Reuse after the grace window
- WHEN a consumed refresh token is presented after the grace window
- THEN the request returns 401
- AND the whole family is revoked and recorded as a replay audit event.

#### Scenario: Concurrent refresh
- WHEN two refresh requests present the same token simultaneously
- THEN exactly one returns 200 and the other returns 401
- AND the family stays active and the token issued to the winner is usable.

### Requirement: Absolute legacy cut-off
The service SHALL enforce a legacy refresh cut-off that is fixed by configuration and SHALL NOT derive it from process start time.

#### Scenario: Restart does not extend acceptance
- WHEN the service restarts before or after the cut-off
- THEN legacy refresh tokens are accepted or rejected only according to the configured instant, independent of process uptime.

#### Scenario: Cut-off not configured
- WHEN no cut-off is configured
- THEN the legacy path fails closed and rejects legacy tokens.

### Requirement: Credential version bound to session issuance
Refresh sessions SHALL record the credential version in force at issuance, and the service SHALL revoke and reject a session whose recorded version no longer matches the account's current credential version.

#### Scenario: Old-password login racing a password change
- WHEN a login validated against the old password issues a session concurrently with a password change
- THEN the password change increments the credential version atomically with the password hash
- AND the racing session is rejected and its family revoked on first refresh.

#### Scenario: Password change revoke failure
- WHEN session revocation fails during a password change
- THEN the password is not changed and the error is returned.
