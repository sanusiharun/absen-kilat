# Design: identity-hardening

Base: `0413c9cc`. Semua perubahan bersifat aditif dan tidak mengubah kontrak mobile.

## 1. Level privilege dan pemetaan

```
employee(10) < supervisor(20) < hr(30) < super_admin(40)
```

Satu fungsi `privilegeRank(role)` menjadi satu-satunya sumber urutan; tidak ada perbandingan string
role tersebar.

## 2. Guard role

### 2.1 Invarian self-change

- Perubahan role **untuk diri sendiri selalu ditolak** (403), termasuk penurunan. Alasan: kenaikan
  adalah eskalasi, penurunan dapat mengunci diri sendiri di tengah operasi. Satu-satunya jalan adalah
  super admin lain.
- Perubahan role **untuk orang lain**: `rank(new) > rank(actor)` atau `rank(target) >= rank(hr)`
  hanya boleh bila `actor.role == super_admin`.

### 2.2 Guard super admin terakhir (konkurensi)

Cek hitung-ulang dilakukan **di dalam transaksi** yang sama dengan mutasi:

```
BEGIN IMMEDIATE;
  SELECT COUNT(*) FROM users WHERE role='super_admin' AND is_active=1 AND id<>?;
  -- jika 0 dan mutasi menghilangkan super_admin aktif terakhir -> 422
  UPDATE users SET role=?, is_active=? WHERE id=?;   -- kolom terarah, bukan seluruh baris
COMMIT;
```

SQLite: `BEGIN IMMEDIATE` mencegah dua penurunan bersamaan sama-sama lolos. Kolom `users.token_version`
**tidak** dipakai (lihat §3.1: pencabutan lewat tabel sesi), jadi tidak ada kenaikan versi di baris
`users`; update kolom terarah juga menutup lost-update pada field profil yang ditulis `users.Update`.
Untuk Postgres nanti: `SELECT ... FOR UPDATE` pada baris target + advisory lock per tabel `users`. Tes konkurensi: dua request paralel menurunkan dua super admin terakhir ⇒ tepat satu
berhasil (I6).

Invarian ini **tidak boleh bergantung pada jalur tulis**: role dan `is_active` adalah dua field berbeda di
endpoint yang sama (`PATCH /admin/users/{id}`), jadi satu-satunya cara aman adalah menghitung ulang di dalam
transaksi untuk **setiap** mutasi yang menghilangkan `super_admin` aktif — termasuk saat `role` dan
`is_active` dikirim bersamaan dalam satu body. Tidak ada endpoint `DELETE /admin/users`; penonaktifan selalu
lewat `PATCH`. Tes silang-jalur **I6b**: dua request paralel, satu menurunkan role super admin A, satu
menonaktifkan super admin B ⇒ tepat satu berhasil. Tes service-level **I5** menutup kasus
`role`+`is_active` dalam satu body.

### 2.3 Jalur bypass yang harus ditutup

Tabel ini **wajib identik dengan route table nyata** (`backend/internal/authz/routes.go` @ `0413c9cc`):
tidak ada endpoint spekulatif, tidak ada endpoint penulis yang terlewat. Saat ini hanya tiga endpoint yang
bisa mengubah role atau permission; tidak ada `DELETE /admin/users` dan tidak ada
`/admin/access/templates/*`.

| Endpoint | Penjaga baru | Tes |
|---|---|---|
| `POST /admin/users` (body memuat `role`) | guard §2.1 | I16 |
| `PATCH /admin/users/{id}` (body memuat `role`) | guard §2.1 | I2 |
| `PATCH /admin/users/{id}` (body memuat `is_active=false`) | guard §2.2 (silang-jalur) | I6b |
| `PATCH /admin/users/{id}/permissions` | `akses.kelola` (bukan `pengguna.ubah`) | I7, I25 |
| `PATCH /admin/users/{id}/permissions` dengan `id` = aktor | **403** — tanpa ini pemegang `kelola` bisa memberi dirinya `pengguna.ubah` lalu naik role | I21 |
| seed/migrasi | hanya lewat kode, bukan API | I24 (audit route) |

Setiap baris punya tes 403 eksplisit sendiri; satu tes tidak boleh dipakai untuk membuktikan dua jalur.
I25 = view-only actor (`pengguna.ubah` tanpa `akses.kelola`) membaca `GET /admin/users/{id}/permissions`
tetap 200 (memastikan guard permission-write tidak ikut memblokir read).

## 3. Refresh token: rotasi sekali pakai, atomik

### 3.1 Mengapa `token_version` saja tidak cukup

`token_version` (usulan revisi 1) hanya bisa **mencabut semua** token, tidak bisa:
membuat token sekali pakai, mendeteksi pemakaian ulang, atau membatasi pencabutan per keluarga sesi.
Karena itu skema diganti.

### 3.2 Skema (migrasi aditif `012_refresh_tokens.sql`, nomor berikut setelah `011_access_control`)

```
refresh_tokens(
  id TEXT PK,
  user_id TEXT NOT NULL REFERENCES users(id),
  family_id TEXT NOT NULL,           -- satu keluarga = satu login
  token_hash TEXT NOT NULL UNIQUE,   -- sha256(token acak), token mentah tidak disimpan
  issued_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  consumed_at TEXT NULL,
  replaced_by TEXT NULL,
  revoked_at TEXT NULL,
  revoked_reason TEXT NULL,          -- logout | replay | admin | password_change
  ip TEXT, user_agent TEXT
)
INDEX (user_id, family_id), INDEX (expires_at)

email_verification_tokens(
  id TEXT PK,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TEXT NOT NULL,
  consumed_at TEXT NULL,
  created_at TEXT NOT NULL
)
```

Kedua tabel dibuat dalam migrasi `012` yang sama (`CREATE TABLE IF NOT EXISTS`, aditif).

### 3.3 Consume atomik + jendela grace

```
BEGIN IMMEDIATE;
  row := SELECT id, family_id, user_id, expires_at, consumed_at, revoked_at
           FROM refresh_tokens WHERE token_hash=?;
  if row is NULL                -> 401 invalid_grant
  if row.revoked_at IS NOT NULL -> revoke family (idempoten), 401 invalid_grant
  if row.expires_at <= now      -> 401 invalid_grant
  if row.consumed_at IS NOT NULL:
      age := now - row.consumed_at
      if age <= REFRESH_REUSE_GRACE -> 401 invalid_grant, keluarga TETAP HIDUP
      else                          -> revoke family, audit `refresh_replay`, 401
  if user.is_active = 0         -> revoke family, 401
  UPDATE refresh_tokens SET consumed_at=now, replaced_by=:newId
    WHERE id=row.id AND consumed_at IS NULL;      -- 0 baris = kalah balapan
  INSERT successor(same family_id, expires_at=row.expires_at, hash(token baru))
COMMIT;
```

- Pemenang ditentukan oleh `UPDATE ... WHERE consumed_at IS NULL` di dalam transaksi tulis, bukan oleh
  `SELECT`: dua request yang sama-sama lolos klasifikasi awal tetap hanya menghasilkan satu successor
  (`UNIQUE(token_hash)` + `BEGIN IMMEDIATE`), dan yang kalah mengulang klasifikasi lalu jatuh ke jalur
  grace.
- **Grace** (`REFRESH_REUSE_GRACE`, default `10s`, `0s` = ketat): pemakaian ulang token yang baru saja
  dirotasi berarti tabrakan klien (mobile/web mengirim dua refresh), bukan pencurian — 401 tanpa
  mematikan sesi. Penyerang tidak mendapat apa pun dari jalur ini karena token baru tidak pernah
  dikirim ke pemegang token lama. Di luar grace, pemakaian ulang = replay ⇒ seluruh keluarga dicabut.
- Rotasi **tidak** memperpanjang `expires_at`: plafon dihitung dari login pertama family.
- Klien tetap wajib single-flight (satu refresh berjalan per proses); itu tugas T9, bukan alasan
  memperlonggar grace.

### 3.4 Semantik pencabutan

| Aksi | Efek |
|---|---|
| logout | revoke token aktif di keluarga itu saja |
| password berubah / reset | revoke semua keluarga user (`revoked_reason=password_change`) |
| role berubah | revoke semua keluarga user |
| akun dinonaktifkan | revoke semua keluarga + refresh ditolak |
| replay di luar grace | revoke keluarga terkait + audit `refresh_replay` |
| admin mencabut sesi | revoke per `user_id` atau per `family_id` |

### 3.5 Migrasi dan sesi legacy

- Migrasi hanya membuat tabel baru; tabel `users` tidak disentuh.
- Token JWT lama (tanpa baris sesi) diterima sementara lewat jalur terpisah yang: memverifikasi
  signature + `type=refresh` + `exp`, memuat user, **menolak bila `is_active = 0`**, lalu menukarnya
  **satu kali** menjadi family baru (upgrade-on-first-refresh) dengan pencatatan `legacy_refresh`.
- **Mekanisme "satu kali" (bukan sekadar niat).** JWT legacy bersifat stateless, jadi pemakaian ulang hanya
  bisa dicegah dengan state. Migrasi `012` menambah tabel ketiga:

  ```
  legacy_refresh_consumed(
    token_hash TEXT PRIMARY KEY,      -- sha256(JWT utuh), token mentah tidak disimpan
    user_id    TEXT NOT NULL REFERENCES users(id),
    family_id  TEXT NOT NULL,         -- family hasil penukaran
    consumed_at TEXT NOT NULL
  )
  ```

  Urutan wajib, semuanya dalam satu `BEGIN IMMEDIATE`:

  ```
  BEGIN IMMEDIATE;
    verifikasi signature, type=refresh, exp > now         -- gagal -> 401, tidak ada tulis
    if now > LEGACY_REFRESH_ACCEPT_UNTIL                  -> 401 legacy_expired
    INSERT INTO legacy_refresh_consumed(...)              -- PK conflict = sudah pernah dipakai
      -- conflict -> revoke family_id yang tercatat di baris lama,
      --             audit `legacy_refresh_replay`, 401 invalid_grant
    user := SELECT ... ; if is_active = 0                 -> ROLLBACK, 401
    buat family + token pertama
  COMMIT;
  ```

  `ROLLBACK` pada akun nonaktif **sengaja** tidak menyimpan penanda (tidak ada family yang pernah dibuatkan
  untuk akun nonaktif, dan akun yang diaktifkan kembali masih boleh menukar sekali di dalam jendela
  migrasi). Konflik penanda dicabut-family berlaku tanpa melihat status akun: replay selalu fail closed.
  `PRIMARY KEY` + `BEGIN IMMEDIATE` adalah yang menentukan pemenang, bukan `SELECT` lebih dulu: dua
  permintaan paralel dengan JWT yang sama hanya bisa menghasilkan satu insert, jadi hanya satu family.
- **Tanpa grace.** Berbeda dari R3, jalur legacy **tidak** punya jendela `REFRESH_REUSE_GRACE`. Tabrakan
  double-refresh klien di jalur legacy berarti 401 + family dicabut + login ulang sekali; itu konsekuensi
  yang diterima dan dibatasi oleh single-flight klien (T9) serta oleh umur jendela migrasi yang pendek.
  Memberi grace di sini akan membuat JWT legacy bisa dipakai berulang — persis yang dilarang.
- Jendela penerimaan dibatasi `LEGACY_REFRESH_ACCEPT_UNTIL` (default: 7 hari setelah deploy, diisi saat
  rilis). Setelah itu jalur legacy mati; token sisa ditolak dan pengguna login ulang. Baris
  `legacy_refresh_consumed` dipangkas 7 hari setelah cut-off (tidak ada lagi penukaran yang mungkin).
- Bila metrik `legacy_refresh` sudah nol, jalur legacy + tabel penandanya dihapus pada rilis berikutnya.
- Rollback: hentikan penerbitan token baru, biarkan tabel ada (tidak merusak data).

## 4. Rate limit auth

- **Definisi klien**: `X-Forwarded-For` hanya dipercaya bila koneksi berasal dari CIDR yang
  dikonfigurasi (`TRUSTED_PROXIES`, default kosong = tidak mempercayai header apa pun); bila tidak,
  pakai `RemoteAddr`. Ini mencegah pemalsuan header untuk lolos limit.
- **Kunci**: `ip` dan `hash(identifier)` (bukan email mentah, supaya log tidak menyimpan PII).
- **Batas**: 10 percobaan per menit per IP, 5 per menit per akun; jendela bergulir.
- **Multi-instance**: dengan penghitung in-memory, batas efektif menjadi N× pada N instance dan
  penyerang bisa memilih instance. Karena itu **horizontal scaling diblokir** sampai penghitung
  bersama (Redis/DB) tersedia; status sekarang satu instance, dan asumsi ini ditulis di sini agar tidak
  jadi kejutan operasional. Mode bersama wajib disertai tes dua proses (I22) sebelum instance kedua
  dinyalakan.
- **Respons generik**: status, body, dan panjang pesan identik untuk akun ada maupun tidak; jeda
  respons dinormalkan (min 250 ms) agar tidak bisa dipakai enumerasi.

## 5. Token verifikasi email

Random 32 byte base64url; disimpan `sha256` + `expires_at` 24 jam + `consumed_at`; sekali pakai;
mengganti token lama saat kirim ulang.

## 6. Kompatibilitas dan observability

- Semua perubahan aditif; mobile tidak berubah.
- Audit event baru: `role_change`, `permission_change`, `refresh_replay`, `session_revoke`.
- Log tidak memuat token mentah, email mentah, atau hash password.
