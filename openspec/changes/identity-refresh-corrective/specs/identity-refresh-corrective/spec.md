## ADDED Requirements

### Requirement: Grace window for benign refresh reuse
The service SHALL reject a reuse of an already-consumed refresh token inside a configured grace window
(default 10 seconds, hard maximum 60 seconds) with 401 without revoking the token family, and SHALL treat reuse
after that window, reuse with a malformed consumption timestamp, or reuse with a consumption timestamp in the
future as replay.

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

#### Scenario: Grace configuration outside approved bounds
- WHEN the configured grace is negative, unparseable, or greater than the approved maximum
- THEN the service fails to start instead of falling back to a default.

#### Scenario: Consumption timestamp in the future
- WHEN a consumed refresh token carries a `consumed_at` later than the current time
- THEN the request returns 401
- AND the whole family is revoked and recorded as a replay audit event, without applying grace.

### Requirement: Absolute legacy cut-off
The legacy refresh path SHALL be disabled unless explicitly enabled, and when enabled SHALL enforce a cut-off
that is fixed by configuration, SHALL NOT derive it from process start time, and SHALL NOT accept a cut-off
later than seven days after the release instant embedded in the running binary.

#### Scenario: Legacy path disabled by default
- WHEN legacy refresh is not explicitly enabled
- THEN every legacy refresh token is rejected with 401 and no session is issued.

#### Scenario: Restart does not extend acceptance
- WHEN the service restarts before or after the cut-off
- THEN legacy refresh tokens are accepted or rejected only according to the configured instant, independent of
  process uptime.

#### Scenario: Enabled without a usable cut-off
- WHEN legacy refresh is enabled but no cut-off is configured, the value is not an absolute RFC3339 instant,
  or the value is later than the embedded release instant plus seven days
- THEN the service fails to start with an explicit configuration error.

### Requirement: Credential version bound to session issuance
Refresh sessions SHALL record the credential version observed when the presented password (or refresh token)
was verified, and the service SHALL issue a session only if that version still matches the account at the
moment of issuance, rejecting the request without returning any access or refresh token otherwise.

#### Scenario: Old-password login racing a password change
- WHEN a login verifies the old password and the password change commits before the session is issued
- THEN the password change increments the credential version atomically with the password hash
- AND the login fails with 401 and no access or refresh token is returned or persisted.

#### Scenario: Password change before verification
- WHEN the password change commits before the login reads the stored hash
- THEN the old password no longer matches and the login fails with 401.

#### Scenario: Session issued in the race window cannot be refreshed
- WHEN a session was recorded with a credential version that no longer matches the account
- THEN the next refresh returns 401 and the family is revoked.

#### Scenario: Legacy token after a credential change
- WHEN a legacy refresh token has no verifiable credential-version provenance and the account's credential
  version has advanced
- THEN the legacy refresh is rejected with 401 and no session or marker is created.

#### Scenario: Password change revoke failure
- WHEN session revocation fails during a password change
- THEN the password is not changed and the error is returned.
