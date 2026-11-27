# Shift 24/7 (penjadwalan) — discovery baseline + delta spec draft

Status: **DRAFT / discovery**. Sumber: `main` `d082b13493c80c55660bba8e0ce93a224fa424a1`.
Klasifikasi backlog: **NEW** (belum ada sama sekali di source). Belum ada requirement yang disetujui;
semua aturan di bawah adalah usulan yang harus melewati G1/G2.

## 1. Bukti bahwa fitur ini belum ada

- Tidak ada tabel/kolom shift: `grep -riE 'shift|roster|jadwal' backend/internal/db/migrations/*.sql` → **0 hasil**.
- Tidak ada kode shift: `grep -riE 'shift|roster' backend/internal/**/*.go` (non-test) → **0 hasil**.
- Jam kerja saat ini: `AttendanceService.cutoffHour` dari env `CUTOFF_HOUR` (default 9) —
  `backend/internal/config/config.go:19,46`; dipakai `attendance_service.go:232`.
- Setting `work_start` / `work_end` / `workdays` **tersimpan tetapi tidak mengikat** perhitungan telat.
- Konsekuensi: absensi mengasumsikan satu jam kerja kantor yang sama untuk semua orang. Operasi 24/7
  (kapal, shift malam, jaga bergilir) tidak bisa direpresentasikan.

## 2. Model data yang diusulkan (additive, tanpa mengubah data lama)

```
shifts(
  id TEXT PK, code TEXT UNIQUE, name TEXT,
  start_time TEXT,           -- "08:00"
  end_time   TEXT,           -- "20:00"
  crosses_midnight INTEGER,  -- 1 bila end_time <= start_time
  grace_minutes INTEGER DEFAULT 15,
  break_minutes INTEGER DEFAULT 0,
  location_id TEXT NULL REFERENCES locations(id),
  active INTEGER DEFAULT 1
)

shift_assignments(
  id TEXT PK,
  shift_id TEXT NOT NULL REFERENCES shifts(id),
  user_id TEXT NULL REFERENCES users(id),
  partner_id TEXT NULL REFERENCES partners(id),
  start_date TEXT NOT NULL,  -- YYYY-MM-DD
  end_date   TEXT NOT NULL,  -- inklusif
  source TEXT DEFAULT 'manual',   -- manual | pattern | import
  created_by TEXT, created_at TEXT
)

ALTER TABLE attendances ADD COLUMN shift_id TEXT NULL REFERENCES shifts(id);
```

Semua kolom baru **nullable** atau punya default → data lama tetap valid, tidak ada backfill wajib.

## 3. Aturan yang diusulkan (inferred, butuh persetujuan)

1. **Penentuan shift**: penugasan aktif pada tanggal itu → bila tidak ada, **default shift lokasi** →
   bila tidak ada juga, **fallback ke setting `work_start`/`work_end`/`workdays`** (menutup gap A1–A3).
2. **Telat**: `check_in > shift.start + grace_minutes` (detik, bukan jam bulat). Ini menggantikan
   `cutoffHour`; `CUTOFF_HOUR` dipertahankan hanya sebagai fallback bila shift tidak ada.
3. **Pulang cepat**: `check_out < shift.end - grace_minutes` → status `early_leave` (status baru).
4. **Shift lintas tengah malam**: `attendance.date` = tanggal **mulai** shift. Check-in pukul 23:50 dan
   check-out 07:10 tercatat pada satu baris tanggal yang sama.
5. **Satu absensi per shift per hari**: constraint unik menjadi `(user_id, date, shift_id)`; karyawan
   dengan dua shift dalam satu hari sah memiliki dua baris.
6. **Tanpa check-in**: setelah `shift.end + X` jam, baris absensi tidak dibuat otomatis (tetap
   fail-closed: tidak mengarang data). Status "tidak hadir" dihitung saat query laporan.
7. **Lembur**: `overtime` dihitung dari `check_out - shift.end` bila melewati ambang; tetap modul terpisah.
8. **Hari libur/minggu**: shift pada hari non-`workdays` dianggap lembur penuh (butuh konfirmasi).

## 4. API dan RBAC yang diusulkan

| Endpoint | Peran | Permission baru |
|---|---|---|
| `GET /admin/shifts` · `POST /admin/shifts` · `PATCH /admin/shifts/{id}` | admin/HR | `shift.lihat`, `shift.tambah`, `shift.ubah` |
| `GET /admin/roster?from=&to=` · `POST /admin/roster` (bulk range) | admin/HR/supervisor | `roster.kelola` |
| `DELETE /admin/roster/{id}` | admin/HR | `roster.kelola` |
| `GET /my/shift/today` | semua pegawai | `absensi.lihat` (existing) |

Penegakan di backend (bukan sekadar menyembunyikan menu), mengikuti pola `internal/authz`.

## 5. Rencana migrasi bertahap

| Langkah | Isi | Risiko |
|---|---|---|
| S1 | migrasi `012_shift.sql` (tabel + kolom nullable) | rendah (additive) |
| S2 | domain/repo/service shift + permission | rendah |
| S3 | perhitungan telat/early-leave dari shift + fallback setting | **sedang** — menyentuh `attendance_service`; wajib tes invarian |
| S4 | resolusi tanggal untuk shift lintas tengah malam | **sedang** |
| S5 | roster API + UI admin | rendah |
| S6 | `GET /my/shift/today` + tampilan mobile | rendah |
| S7 | keterkaitan lembur/checkpoint | **tinggi** — lintas modul |

Urutan wajib: S1 → S2 → S3 → (S4 paralel dengan S5) → S6 → S7.

## 6. Pertanyaan yang harus dijawab owner sebelum G1

1. Shift ditentukan **per lokasi** atau **global** lebih dulu? (usulan: per lokasi, karena geofence sudah per lokasi)
2. Berapa **grace default**? (usulan 15 menit)
3. Apakah `CUTOFF_HOUR` dihapus total setelah shift aktif, atau tetap sebagai fallback?
4. Shift pada hari libur dihitung lembur penuh atau tarif khusus?
5. Perlukah pola berulang (mis. 2-2-2 offshore) pada rilis pertama, atau penugasan manual per rentang tanggal?

## 7. Bukti yang belum ada (harus dibuat saat implementasi)

- Tes invarian: telat dari shift, early-leave, lintas tengah malam, fallback setting, dua shift/hari.
- Uji beban: query roster untuk 500 pegawai × 90 hari.
