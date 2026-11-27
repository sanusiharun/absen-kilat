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
  -- jika 0 dan target aktif super_admin -> 422
  UPDATE users SET role=?, token_version=token_version+1 WHERE id=?;
COMMIT;
```

SQLite: `BEGIN IMMEDIATE` (sudah dipakai pola lease di runtime) mencegah dua penurunan bersamaan
sama-sama lolos. Untuk Postgres nanti: `SELECT ... FOR UPDATE` pada baris target + advisory lock per
tabel users. Tes konkurensi: dua request paralel menurunkan dua super admin terakhir ⇒ tepat satu
berhasil (I6).

### 2.3 Jalur bypass yang harus ditutup

Semua endpoint yang bisa mengubah role atau permission wajib melewati guard yang sama:

| Endpoint | Penjaga baru |
|---|---|
| `POST /admin/users` (dengan `role`) | guard §2.1 |
| `PATCH /admin/users/{id}` (dengan `role`) | guard §2.1 |
| `PATCH /admin/users/{id}/permissions` | `akses.kelola` (bukan `pengguna.ubah`) |
| `PATCH /admin/access/templates/*` (bila ada) | `super_admin` saja |
| seed/migrasi | hanya lewat kode, bukan API |

Setiap endpoint di tabel ini mendapat tes 403 eksplisit (I2, I7, I16).

## 3. Refresh token: rotasi sekali pakai, atomik

### 3.1 Mengapa `token_version` saja tidak cukup

`token_version` (usulan revisi 1) hanya bisa **mencabut semua** token, tidak bisa:
membuat token sekali pakai, mendeteksi pemakaian ulang, atau membatasi pencabutan per keluarga sesi.
Karena itu skema diganti.

### 3.2 Skema (migrasi aditif `013_refresh_tokens.sql`)

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
```

### 3.3 Consume atomik

```
BEGIN IMMEDIATE;
  row := SELECT * FROM refresh_tokens WHERE token_hash=?;
  if row is NULL            -> 401 generic
  if row.revoked_at != NULL -> 401 + revoke whole family (replay on revoked)
  if row.consumed_at != NULL-> REPLAY: revoke family, 401
  if row.expires_at < now   -> 401
  if user.is_active = 0     -> 401 + revoke family
  UPDATE refresh_tokens SET consumed_at=now, replaced_by=newId WHERE id=row.id;
  INSERT INTO refresh_tokens(newId, same family_id, hash(new token), ...);
COMMIT;
```

- Hanya **satu** request yang bisa menangani baris itu (transaksi + `UNIQUE(token_hash)`).
- Replay (token sudah `consumed_at`) mencabut **seluruh keluarga** dan dicatat sebagai audit event.
- Access token tetap pendek (mis. 15 menit) dan tidak disimpan.

### 3.4 Semantik pencabutan

| Aksi | Efek |
|---|---|
| logout | revoke token aktif di keluarga itu saja |
| password berubah / reset | revoke semua keluarga user (`revoked_reason=password_change`) |
| role berubah | revoke semua keluarga user |
| akun dinonaktifkan | revoke semua keluarga + refresh ditolak |
| replay terdeteksi | revoke keluarga terkait + audit |
| admin mencabut sesi | revoke per `user_id` atau per `family_id` |

### 3.5 Migrasi dan sesi legacy

- Migrasi hanya membuat tabel baru; **tidak** menyentuh tabel `users`.
- Token format lama (JWT stateless) tetap diterima **sampai `exp`** maksimal 30 hari, tanpa rotasi,
  dan ditandai di log sebagai `legacy_refresh`. Tidak ada paksa-logout massal saat rilis.
- Setelah `exp` terakhir token legacy lewat, jalur legacy dihapus pada rilis berikutnya.
- Rollback: hentikan penerbitan token baru, biarkan tabel ada (tidak merusak data).

## 4. Rate limit auth

- **Definisi klien**: `X-Forwarded-For` hanya dipercaya bila koneksi berasal dari CIDR yang
  dikonfigurasi (`TRUSTED_PROXIES`, default kosong = tidak mempercayai header apa pun); bila tidak,
  pakai `RemoteAddr`. Ini mencegah pemalsuan header untuk lolos limit.
- **Kunci**: `ip` dan `hash(identifier)` (bukan email mentah, supaya log tidak menyimpan PII).
- **Batas**: 10 percobaan per menit per IP, 5 per menit per akun; jendela bergulir.
- **Multi-instance**: bila nanti ada lebih dari satu instance, limit per-instance bersifat
  best-effort; jalur yang benar adalah penyimpanan bersama (Redis/DB). Status saat ini: **satu
  instance**, jadi in-memory sudah memadai; asumsi ini dicatat di dokumen dan diuji saat deployment
  bertambah.
- **Respons generik**: status, body, dan panjang pesan identik untuk akun ada maupun tidak; jeda
  respons dinormalkan (min 250 ms) agar tidak bisa dipakai enumerasi.

## 5. Token verifikasi email

Random 32 byte base64url; disimpan `sha256` + `expires_at` 24 jam + `consumed_at`; sekali pakai;
mengganti token lama saat kirim ulang.

## 6. Kompatibilitas dan observability

- Semua perubahan aditif; mobile tidak berubah.
- Audit event baru: `role_change`, `permission_change`, `refresh_replay`, `session_revoke`.
- Log tidak memuat token mentah, email mentah, atau hash password.
