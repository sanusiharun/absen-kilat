# Delta spec: shift definitions (S1–S2)

Change: `shift-247-s1s2`. Status: **PROPOSED — G2 pending**. Base `bf8c3d8f`.
Cakupan: penyimpanan definisi shift + CRUD admin. Perilaku absensi (telat, early-leave, penentuan
shift saat check-in) **tidak** berubah oleh spec ini.

## R1 — Skema shift tersimpan

The system SHALL store shift definitions with a unique code (case-insensitive), a start and end time
in local `HH:MM`, a derived `crosses_midnight` flag, per-shift grace and break minutes, an optional
location reference, and an active flag.

#### Scenario: Shift baru tersimpan
- GIVEN the `shifts` table exists and no shift has code `MALAM`
- WHEN an admin creates a shift with code `malam`, name `Shift Malam`, `22:00`–`06:00`, grace `15`
- THEN one row is stored with `code = "MALAM"`, `crosses_midnight = 1`, `active = 1`
- AND the response is `201` with the same values

#### Scenario: Kode duplikat ditolak
- GIVEN a shift with code `PAGI` exists
- WHEN another shift is created with code `pagi` or `Pagi`
- THEN the request fails with `422 VALIDATION_ERROR`
- AND no row is written

## R2 — Waktu dan flag diturunkan server

The system SHALL validate `start_time`/`end_time` as `HH:MM` in 24-hour form, SHALL reject equal
start and end, and SHALL derive `crosses_midnight` from the times rather than accepting it from the
client.

#### Scenario: Format jam salah
- WHEN a shift is created with `start_time = "8:0"` or `end_time = "24:00"`
- THEN the request fails with `422 VALIDATION_ERROR`

#### Scenario: Start sama dengan end
- WHEN a shift is created with `start_time == end_time`
- THEN the request fails with `422 VALIDATION_ERROR` (shift 24 jam bukan model yang didukung)

#### Scenario: Klien mengirim crosses_midnight palsu
- WHEN a client sends `crosses_midnight: false` together with `start_time = "20:00"`, `end_time = "08:00"`
- THEN the stored value is `1` (input klien diabaikan)

## R3 — Rentang grace dan break dibatasi

The system SHALL accept `grace_minutes` in `0..120` (default 15) and `break_minutes` in `0..480`
(default 0), and SHALL reject values outside those ranges.

#### Scenario: Grace negatif
- WHEN `grace_minutes = -1` is submitted
- THEN the request fails with `422 VALIDATION_ERROR`

## R4 — Referensi lokasi divalidasi

The system SHALL accept an empty `location_id` (shift berlaku untuk semua lokasi) and SHALL reject a
non-empty `location_id` that does not exist in `locations` with `404 NOT_FOUND`.

#### Scenario: Lokasi tidak dikenal
- WHEN a shift is created with `location_id = "tidak-ada"`
- THEN the response is `404 NOT_FOUND` and no row is written

#### Scenario: Tanpa lokasi
- WHEN `location_id` is omitted
- THEN the shift is stored with `location_id = NULL` and is returned as `""`

## R5 — Akses shift dibatasi permission menu

The system SHALL guard every shift endpoint with a menu permission (`menu.shift.view`,
`menu.shift.create`, `menu.shift.update`), SHALL deny unmapped routes, and SHALL hide the Shift menu
from users without `menu.shift.view`.

#### Scenario: Tanpa permission
- GIVEN a user whose effective permissions do not include `menu.shift.create`
- WHEN the user calls `POST /admin/shifts`
- THEN the response is `403` and no row is written

#### Scenario: Tanpa token
- WHEN `GET /admin/shifts` is called without an `Authorization` header
- THEN the response is `401`

#### Scenario: Menu tersembunyi
- GIVEN a user without any `menu.shift.*` permission
- WHEN the user calls `GET /me/permissions`
- THEN the response does not contain the `shift` menu key

## R6 — Data absensi lama tidak berubah

The system SHALL leave existing attendance rows untouched by the migration (no backfill) and SHALL
not change check-in/check-out behaviour in this change.

#### Scenario: Absensi setelah migrasi
- GIVEN an attendance row created before the migration
- WHEN the migration completes and a check-in is recorded
- THEN both rows keep their previous `status`/`occurred_at` values and `shift_id` is `NULL`

## Non-goals (ditegaskan)

Tidak ada requirement tentang penentuan shift saat check-in, telat/early-leave dari shift, resolusi
tanggal lintas tengah malam, roster, lembur, atau tampilan mobile — semuanya S3–S7 dan butuh spec
terpisah yang disetujui.
