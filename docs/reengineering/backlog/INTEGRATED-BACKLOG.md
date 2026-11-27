# Integrated backlog — AbsenKilat HRMS

Status: DISCOVERY/PLANNING (bukan requirement yang disetujui). Baseline sumber:
`c85edc9a96c637037f16e838592a6016902670c9` (main). Owner direction: unified parallel delivery —
fondasi agentic (#58), penyelarasan fitur existing, dan discovery/spec fitur baru berjalan paralel.
**Tidak boleh** berisi identitas klien, nomor proposal komersial, harga, atau data pribadi.

Klasifikasi: **REUSE** (sudah ada, dipakai apa adanya) · **ADJUST** (ada, perlu perubahan kecil) ·
**NEW** (belum ada) · **DEFER** (ditunda dengan alasan, tidak dihapus).

---

## 1. Matriks fitur → klasifikasi → bukti sumber

| # | Fitur | Klasifikasi | Bukti sumber (main `c85edc9a`) | Gap utama |
|---|---|---|---|---|
| 1 | Identitas & sesi (login, refresh, peran) | **ADJUST** | `backend/internal/transport/auth_handler.go`, `service/auth_service.go`, `db/migrations/003_roles.sql`, `007_rbac_qr.sql` | eskalasi peran, refresh stateless, rate limit auth, verifikasi email = user id |
| 2 | RBAC & scope (peran, tim) | **ADJUST** | `transport/middleware_auth.go`, `service/admin_service.go:119`, `repo/user_repo.go:120` | belum ada permission per menu (sedang dikerjakan di PR #55), guard eskalasi |
| 3 | Absensi harian (check-in/out, telat) | **REUSE + ADJUST** | `transport/attendance_handler.go` (`/attendance`, `/today`, `/{id}`), `service/attendance_service.go:78/126/228`, `attendance_service_test.go` (5 tes) | jam kerja hardcoded `cutoffHour`, belum ada checkpoint/regularisasi |
| 4 | Geofence radius | **REUSE** | `service/attendance_service.go` (`geoResult.Outside`, `DistanceM`), `migrations/005_geofence.sql` | belum ada polygon geofencing |
| 5 | Foto bukti + kebijakan foto | **REUSE** | `ErrPhotoRequired`, `photoRequired()`, `approvalFor()` | belum ada liveness/anti-replay foto |
| 6 | Device & IP binding | **ADJUST** | `migrations/004_device_ip.sql`, `AttendanceInput.Device/IPAddress` | hanya dicatat, belum ada aturan penolakan/allowlist perangkat |
| 7 | Approval absensi (di luar radius → pending) | **REUSE** | `approvalFor()`, `admin_service.go:512–537` (approval queue, scope supervisor) | belum ada SLA/eskalasi |
| 8 | QR per lokasi | **REUSE** | `transport/qr_handler.go`, `service/qr_service.go` (rate limit per IP 10/menit), `migrations/007_rbac_qr.sql` | token QR belum kedaluwarsa otomatis; belum ada audit pemakaian |
| 9 | Cuti & izin (5 tipe, kuota, approval) | **REUSE** | `service/leave_service.go`, `migrations/008_leave.sql`, `leave_service_test.go` | belum ada jenis "izin" terpisah & "post-dive rest" |
| 10 | Lembur | **REUSE** | `service/overtime_service.go`, `migrations/009_overtime.sql` | belum ada kategori darurat/night-shift & multiplier |
| 11 | Mitra & proyek/kapal + penugasan | **REUSE** | `service/project_service.go`, `migrations/010_partner_project.sql` | belum ada tarif harian mitra (honorarium) |
| 12 | Shift 24/7 dinamis | **NEW** | tidak ada di source | butuh model shift, roster, penugasan shift, override |
| 13 | Payroll: gaji pokok + tunjangan (hazard/offshore/meal/transport) | **NEW** | hanya `MonthlyMinutes` lembur sebagai bahan | butuh komponen gaji, tarif, periode, kunci payroll |
| 14 | Slip gaji PDF | **NEW** | tidak ada | butuh generator dokumen + template + penyimpanan |
| 15 | Honorarium harian mitra | **NEW** | `assignments` ada, tarif tidak | butuh tarif per orang/proyek + perhitungan hari hadir |
| 16 | Procurement (PR → approval bertingkat → RFQ → PO → BAST → 3-way match) | **NEW** | tidak ada | modul besar; butuh vendor master + dokumen |
| 17 | Pajak: PPh 21 TER + withholding non-karyawan + e-Bupot | **NEW** | tidak ada | wajib verifikasi aturan resmi terbaru sebelum spec |
| 18 | ESS Android/iOS | **ADJUST** | `mobile/` (Expo SDK 54): home, cuti, riwayat, profil, scan | belum ada slip gaji, pengajuan lembur dari app, offline |
| 19 | Offline attendance buffer + sync | **NEW** | tidak ada | butuh antrean lokal + integritas/timestamp |
| 20 | Anti-mock / root / jailbreak / face liveness | **NEW** | tidak ada | butuh SDK/dependency baru + keputusan biaya |
| 21 | Dashboard & reporting | **ADJUST** | `admin/app/dashboard`, `/laporan`, `admin_service.go` reports | belum ada laporan biaya tenaga kerja per proyek; export CSV/XLSX belum ada (EPIC #23 tiket D1–D4) |
| 22 | Polygon geofencing | **DEFER (LOW)** | — | alasan owner: potensi biaya; **tidak dihapus** |
| 23 | Notifikasi/reminder | **ADJUST** | `migration/002_notifications.sql`, `notification_service.go` | belum ada push server-side; reminder lokal belum ada (EPIC #23 tiket E1–E2) |

## 2. Dependency graph (ringkas)

```
identitas & sesi ──► RBAC per-menu ──► semua modul admin
        │
        ├──► absensi (foto/geofence/approval) ──► shift 24/7 ──► jadwal & roster
        │            │                                   │
        │            └──► checkpoint/regularisasi        └──► lembur per shift
        │
        ├──► cuti & izin ──┐
        ├──► lembur ───────┼──► PAYROLL (gaji + tunjangan) ──► slip gaji PDF ──► PAJAK (PPh21 TER, e-Bupot)
        └──► mitra/proyek ─┘            │
                                        └──► honorarium mitra (hari hadir × tarif)
                                              │
                                              └──► procurement (PO/BAST) ──► 3-way matching
ESS mobile: butuh absensi + cuti + slip gaji + offline buffer
Security (mock/root/liveness): butuh keputusan SDK + biaya
```

**Critical path ke nilai bisnis tertinggi:** identitas/RBAC → absensi+shift → payroll → slip gaji →
pajak. Procurement dan security device berada di jalur paralel yang lebih panjang.

## 3. Prasyarat minimum per kelompok implementasi

| Kelompok | Prasyarat wajib sebelum coding |
|---|---|
| Identitas/RBAC | G1/G2 change `identity-hardening`; baseline bersih (PR #22 merged, #55 direkonsiliasi) |
| Shift 24/7 | keputusan model shift (per lokasi vs global), dampak ke `statusFor()` & absensi existing |
| Payroll | keputusan periode penggajian, komponen gaji, sumber hari hadir (absensi vs rekap manual), kebijakan pembulatan |
| Pajak | verifikasi aturan resmi terbaru (PPh 21 TER, e-Bupot) — **tidak boleh** dari memori model |
| Procurement | vendor master + kebijakan approval bertingkat + format dokumen |
| ESS/offline | strategi sinkronisasi + integritas data (timestamp server, idempotensi) |
| Security device | keputusan biaya SDK (liveness) + kebijakan penolakan perangkat |
| Semua | runtime agentic (#58) **tidak** memblokir produk; dispatch manual tetap bisa |

## 4. Penilaian milestone (usulan proposal: minggu 1–2 FSD, 3–4 staging, 5–6 mobile, 7 UAT, 8 go-live)

| Minggu | Aktivitas | Realistis? | Catatan |
|---|---|---|---|
| 1–2 | FSD/delta spec untuk P0 (identitas, absensi, shift, cuti, lembur, dashboard) | **Cukup realistis** bila hanya P0 | 23 fitur tidak mungkin di-FSD-kan dalam 2 minggu |
| 3–4 | Staging P0 | **Berisiko** | butuh shift 24/7 (NEW) + keputusan DB; baseline saat ini masih merah (4 tes) |
| 5–6 | Mobile | **Berisiko** | ESS butuh endpoint slip gaji & lembur yang belum ada |
| 7 | UAT | **Berisiko** | kalau P0 belum stabil, UAT jadi pengujian fungsional pertama |
| 8 | Go-live | **Tidak realistis untuk seluruh scope** | realistis hanya untuk P0 terbatas (absensi+cuti+lembur+dashboard) |

**Kesimpulan:** milestone 8 minggu hanya layak untuk **P0 terbatas**. Payroll/pajak/procurement/security
butuh fase terpisah.

## 5. Cost risks

1. **Face liveness SDK** — biaya per-verifikasi atau lisensi; belum ada estimasi.
2. **Polygon geofencing** — sudah dinyatakan LOW karena biaya.
3. **Pajak** — perubahan aturan tahunan ⇒ biaya pemeliharaan berulang.
4. **Offline sync + anti-mock** — menambah kompleksitas mobile (dan matrix perangkat untuk QA).
5. **Runtime agentic (#58)** — biaya token; sudah dibatasi 50k/run, 30 menit, 2 attempt.

## 6. Rencana eksekusi paralel (tanpa menunggu blokir)

| Jalur | Pekerjaan sekarang | Blokir? |
|---|---|---|
| A. Fondasi agentic | #58 draft → menunggu G1/G2 | ya (keputusan owner) |
| B. Baseline hygiene | PR #22 (ESLint per-call-site) → CI; review #55 | tidak |
| C. Discovery produk | attendance ✔, qr-geofence, dashboard-reporting | tidak |
| D. Spec P0 | delta spec per fitur (OpenSpec) setelah discovery masing-masing | tidak |

## 7. Yang belum dan tidak akan ditulis di repo publik ini

Identitas klien, nomor proposal komersial (`PROP-…`), harga/penawaran, dan data pribadi apa pun.
