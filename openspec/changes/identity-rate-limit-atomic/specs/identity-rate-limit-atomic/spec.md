# Spec: identity-rate-limit-atomic

Delta atas `identity-hardening` R4. Tidak mengubah R4; menambahkan kewajiban admission atomik.

### A-R1 — Reservasi atomik sebelum operasi terlindungi
The service SHALL decide admit/deny for an authentication attempt in a single atomic operation that both
increments the counter and applies the limit, executed BEFORE the protected operation. The decision SHALL NOT
depend on a separate read performed before the increment.

#### Scenario: parallel burst at the boundary
- GIVEN a counter at 9 attempts in the current window for a key with limit 10
- WHEN 20 attempts arrive concurrently
- THEN exactly 1 is admitted, 19 are rejected with the generic 429
- AND the stored counter never exceeds 10

#### Scenario: no read-then-act window
- GIVEN the time between decision and protected operation is arbitrary
- WHEN the protected operation is slow
- THEN other concurrent attempts are still counted and bounded

### A-R2 — Keputusan meliputi semua scope yang terlibat, all-or-nothing
The service SHALL reserve every participating scope (client IP and account key) within one transaction.
If any scope is exhausted, the reservations already taken by that attempt SHALL be rolled back.

#### Scenario: account exhausted, IP fresh
- GIVEN the account key reached its limit and the IP key has 0 attempts
- WHEN another login for that account arrives from that IP
- THEN the request is rejected with the generic 429
- AND the IP counter remains 0

### A-R3 — Fail closed dengan respons generik
The service SHALL reject the attempt with the same generic 429 used for an exhausted limit when the counter
store fails (write error, busy timeout, context cancellation). The protected operation SHALL NOT run, and no
partial counter increment SHALL remain. Error logs SHALL NOT contain raw IP, email, or key material.

#### Scenario: store failure
- GIVEN the counter store returns an error
- WHEN a login attempt arrives
- THEN the response is the generic 429
- AND the login service is not invoked
- AND the counter row is unchanged

### A-R4 — Refund hanya untuk login sukses
The service SHALL refund the reservation of a successful login, so that only failed login attempts consume
quota as defined by I10. The refund SHALL target the same window the reservation incremented and SHALL NOT
change the response of an already successful login. A refund failure SHALL be recorded and SHALL NOT fail the
request.

#### Scenario: successful logins do not consume quota
- GIVEN an IP with 10 successful logins in the current window
- WHEN a failed login follows
- THEN it is processed (401), not rejected with 429

#### Scenario: refund failure
- GIVEN the refund statement fails after a successful login
- WHEN the login completes
- THEN the response is still 200
- AND at most one extra attempt is counted

### A-R5 — Perilaku yang tidak berubah
The change SHALL preserve existing limits, window width, trusted-proxy derivation, key hashing, and the
indistinguishability of the 429 response. Forgot, reset, and email-verification attempts SHALL continue to
consume quota without refund.

#### Scenario: spoofed forwarded header
- GIVEN a client that is not a trusted proxy sending `X-Forwarded-For`
- WHEN it exceeds the per-IP limit
- THEN it is still limited based on `RemoteAddr`

#### Scenario: identical response
- GIVEN one registered and one unregistered email
- WHEN both are rejected by the limit
- THEN status, body, and body length are identical
