# attendance
Status: DISCOVERY / unapproved. Baseline c85edc9a96c637037f16e838592a6016902670c9.
## Source inventory
- backend/internal/service/attendance_service.go
- backend/internal/transport/attendance_handler.go
- backend/internal/repo/attendance_repo.go
## Required discovery
Document actors, permissions, trigger, normal and alternative flows, validation, persistence, API request/response/error contracts, privacy, UI and test evidence. Verify actual behavior against source and tests; distinguish observed from desired. Business rules and acceptance scenarios remain TBD until reviewed. No implementation authorization.

---

## Discovery 2026-11-27 (P0, read-only)

Sumber: main `c85edc9a96c637037f16e838592a6016902670c9`.

### API (observed)
- `POST /attendance`, `GET /attendance`, `GET /attendance/today`, `GET /attendance/{id}`
  — `backend/internal/transport/attendance_handler.go:45-48`.
- Override admin: `PATCH /admin/attendance/{id}` (`admin_handler.go:69`), service
  `admin_service.go:772` mewajibkan `reason` (422 bila kosong).
- Approval queue: `GET /admin/approvals`, `POST /admin/approvals/{id}` (`admin_handler.go:79-81`),
  scope supervisor = tim sendiri (`admin_service.go:537`).

### Aturan bisnis (observed)
1. Satu check-in & satu check-out per hari: `ErrAlreadyCheckedIn` / `ErrAlreadyCheckedOut`
   (`attendance_service.go:15-19`).
2. Di luar radius **tidak ditolak**: foto jadi wajib dan record masuk `approval_status='pending'`
   (`attendance_service.go:74-77`, `ErrPhotoRequired:23`, `approvalFor():184`).
3. Auto-approve hanya bila: di dalam radius **dan** lokasi bukan `field` **dan** foto ada (`:185`).
4. Status telat dihitung dari `cutoffHour:00` lokal, **bukan** dari setting `work_start`
   (`attendance_service.go:56,228-234`) → jam kerja di `/pengaturan` belum mengikat.
5. Device & IP **dicatat**, bukan ditegakkan (`AttendanceInput.Device/IPAddress:38-39`,
   `migrations/004_device_ip.sql`).
6. `source` = `app` | `qr` | `manual` (`domain/models.go`), QR publik punya rate limit
   per IP 10/menit (`qr_service.go:21-37`).

### Tes yang ada (observed)
`attendance_service_test.go`: `TestCheckInAndCheckOutFlow`, `TestCheckOutWithoutCheckInFails`,
`TestLateStatus`, `TestTodayState`, `TestGeofence` — **tidak ada** tes untuk policy foto,
approval pending, device/IP, QR publik, atau cutoff vs setting.

### Gap (inferred, belum requirement)
| Gap | Dampak | Usulan change |
|---|---|---|
| Jam kerja tidak mengikat (cutoffHour hardcoded) | HR ubah setting tanpa efek | EPIC #23 tiket A1–A3 |
| Belum ada koreksi "lupa absen" | sengketa kehadiran | EPIC #23 tiket B1–B6 |
| Device/IP hanya dicatat | sulit menolak perangkat tak dikenal | `device-binding` (P2) |
| Tidak ada offline buffer | absen gagal di sinyal buruk | `offline-attendance` (P1) |
| Tidak ada anti-mock/liveness | foto/GPS bisa dipalsukan | `attendance-security` (P2) |
| Belum ada penjadwalan shift | tidak cocok untuk operasi 24/7 | `shift-scheduling` (P0 lanjutan) |
| Tidak ada audit perubahan absensi | override hanya menyimpan hasil | `identity-audit` (P0) |

### Unknown
Ambang radius per lokasi vs global (dibaca dari `locations`), perilaku zona waktu klien,
dan apakah `PATCH /admin/attendance` boleh mengubah tanggal (belum diverifikasi runtime).
