# Spec: identity-hardening

### R1 — Perubahan role
The service SHALL reject a role change that assigns `super_admin` or `hr` unless the actor is an
active `super_admin`. The service SHALL reject any role change targeting the actor's own account
when the new role has a higher privilege level. The service SHALL preserve the existing guarantee
that the last active `super_admin` cannot be demoted or deactivated.

#### Scenario: HR promotes self
- GIVEN an actor with role `hr`
- WHEN the actor sets their own role to `super_admin`
- THEN the request is rejected with 403 and the stored role is unchanged

#### Scenario: super admin promotes an employee
- GIVEN an actor with role `super_admin`
- WHEN the actor sets an `employee` to `supervisor`
- THEN the request succeeds

### R2 — Permission separation
The service SHALL require `akses.kelola` (not `pengguna.ubah`) for endpoints that change a user's
menu permissions.

#### Scenario: HR without akses.kelola
- GIVEN an actor with `pengguna.ubah` and without `akses.kelola`
- WHEN they PATCH `/admin/users/{id}/permissions`
- THEN the request is rejected with 403

### R3 — Session invalidation
The service SHALL reject a refresh token when the account is inactive or when the token version does
not match the stored value. Password or role changes SHALL increment the token version.

#### Scenario: deactivated account refreshes
- GIVEN a valid refresh token for an account that has been deactivated
- WHEN refresh is attempted
- THEN the request is rejected with 401

### R4 — Auth rate limiting
The service SHALL limit authentication attempts per client IP and per account key, and SHALL return a
generic failure that does not reveal whether the account exists.

#### Scenario: brute force
- GIVEN 10 failed logins within one minute from one IP
- WHEN an 11th attempt arrives
- THEN it is rejected without revealing account existence

### R5 — Email verification token
The service SHALL use a random single-use token that expires, and SHALL NOT use the user identifier
as the token.

#### Scenario: replay
- GIVEN a verification token already consumed
- WHEN it is used again
- THEN the request is rejected with 401
