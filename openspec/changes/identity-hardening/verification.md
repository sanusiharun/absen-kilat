# Verification: identity-hardening

Base: `456eb1ad` (main setelah PR #69; CI backend/frontend/structure success). Rencana tes ini **boleh** ada sebelum approval (dokumen); eksekusi tes butuh G1/G2.

## Gate evidence

Definisi gate di bawah ini **harus identik dengan** `tools/validate_sdd.py` (revisi 3 sebelumnya salah:
design dicantumkan di G1 padahal tidak ikut di-hash, dan `verification.md` dicantumkan di G2 padahal tidak
pernah di-hash). Kanonik, apa adanya di kode:

| Gate | Artefak yang di-hash (urut) | Lainnya |
|---|---|---|
| G1 | `proposal.md` | design, tasks, spec, rencana tes **tidak** di-hash |
| G2 | `proposal.md`, `design.md`, `tasks.md`, `specs/*/spec.md` (urut path) | `verification.md` **tidak** di-hash |
| G3 | — | perintah, exit code, hasil tes, run CI, diff scope vs `allowed_paths` |

Framing digest (sama untuk G1/G2): `sha256` atas, untuk setiap berkas berurutan, `len(path) ‖ path` lalu
`len(content) ‖ content`, dengan panjang sebagai big-endian 8 byte dan `path` relatif terhadap direktori
change (posix). Perintah: `python3 tools/validate_sdd.py --digest identity-hardening G1|G2`.

**Celah yang diakui:** karena `verification.md` tidak di-hash, rencana tes bisa dilemahkan setelah G2 tanpa
mengubah digest. Mitigasi yang dipakai sekarang (tanpa mengubah tool, sehingga digest change lain tidak
bergeser): setiap laporan gate **wajib mencantumkan `PLAN_SHA256`** = `sha256(verification.md)`, sehingga
versi rencana tes yang disetujui bisa dibuktikan byte-identik. Opsi yang lebih kuat — memasukkan
`verification.md` ke G2 di validator — menggeser digest semua change yang sedang berjalan dan karena itu
butuh persetujuan owner terpisah; belum dilakukan.

Yang dilaporkan di setiap gerbang: `base_sha` (commit yang di-review), `head_sha`, digest G1/G2 penuh,
`PLAN_SHA256`, keluaran `validate_sdd.py` (termasuk `STRUCTURE PASS`), dan `pytest tools/tests`.

## Paket approval dan binding plan hash

Paket approval final WAJIB dikirim owner lewat **DM Telegram tepercaya** (laporan GitHub saja bukan approval)
dan memuat eksplisit: `change=identity-hardening`, `gate=G1|G2`, `base_sha`, `head_sha` yang di-review,
digest G1/G2 penuh, dan `PLAN_SHA256 = sha256(verification.md)`.

- Implementasi/G3 hanya boleh mulai bila `sha256(verification.md)` pada head kerja **sama persis** dengan
  `PLAN_SHA256` di paket approval; beda → fail closed, minta approval ulang.
- `base_sha` harus ancestor dari `head_sha` (`git merge-base --is-ancestor`); head yang di-review tidak boleh
  bergeser tanpa approval baru.
- Slice ini **tidak** mengubah validator repo-wide (`tools/validate_sdd.py`); memasukkan `verification.md`
  ke G2 tetap usulan terpisah yang butuh persetujuan owner.

## Kebijakan jendela migrasi legacy

- `LEGACY_REFRESH_ACCEPT_UNTIL` = tanggal rilis aktual + **maksimum 7 hari**, tidak boleh diperpanjang.
- `exp` JWT legacy tidak boleh diubah/diperpanjang untuk mengakomodasi jendela migrasi.
- Alternatif bila 7 hari tidak cukup: paksa login ulang (forced reauth), bukan memperpanjang token.

## Matriks tes negatif (role dan permission)

| ID | Skenario | Harapan |
|---|---|---|
| I1 | HR mengubah role dirinya menjadi `super_admin` | 403, role tidak berubah |
| I2 | HR mengubah role orang lain menjadi `super_admin` lewat `PATCH /admin/users/{id}` | 403, role tidak berubah |
| I3 | HR mengubah `employee` menjadi `supervisor` | 200 (bukan admin role, dan `rank(new) <= rank(actor)`) |
| I4 | Super admin mengubah role siapa pun | 200 |
| I5 | Mutasi yang menyisakan 0 super_admin aktif, diuji di level service (lewat HTTP hanya bisa terjadi konkuren, I6) | 422 `last_super_admin`, baris `users` tidak berubah |
| I6 | **Konkurensi**: dua request paralel menurunkan dua super admin terakhir (dua-duanya lewat jalur role) | tepat satu 200, satu 422 |
| I6b | **Konkurensi silang-jalur**: paralel, satu `PATCH` menurunkan role super admin A, satu `PATCH` menonaktifkan super admin B | tepat satu berhasil; tidak pernah 0 `super_admin` aktif |
| I7 | Aktor dengan `pengguna.ubah` tanpa `akses.kelola` memanggil PATCH permissions | 403 |
| I16 | `POST /admin/users` dengan `role=super_admin` oleh HR | 403 (jalur bypass tertutup) |

## Matriks tes sesi dan token

| ID | Skenario | Harapan |
|---|---|---|
| I8 | Refresh token akun yang dinonaktifkan | 401, keluarga token dicabut |
| I9 | Refresh setelah password berubah | 401 (semua keluarga dicabut) |
| I13 | **Dua refresh paralel dengan token sama** (dalam grace) | tepat satu 200; yang kalah 401; keluarga tetap hidup dan token baru bisa dipakai |
| I14 | Replay token yang sudah dipakai | 401 dan **seluruh keluarga dicabut** |
| I15 | Pemakaian token dari keluarga yang sudah dicabut | 401, tercatat audit `refresh_replay` |
| I19 | Token legacy (JWT lama) dipakai sebelum cut-off | 200 + family baru; pemakaian kedua token lama itu 401; log `legacy_refresh` tanpa token |
| I19b | Token legacy untuk akun nonaktif | 401, tidak ada family dibuat |
| I19c | **Dua request paralel dengan satu JWT legacy** | tepat satu 200; yang kalah 401; family hasil pemenang dicabut + audit `legacy_refresh_replay` |
| I19d | Pemakaian kedua JWT legacy berurutan (bukan paralel) | 401, family pertama dicabut, audit `legacy_refresh_replay` |
| I21 | Pemegang `akses.kelola` PATCH permissions miliknya sendiri | 403, tidak ada override tersimpan |
| I25 | Aktor `pengguna.ubah` tanpa `akses.kelola` membaca `GET /admin/users/{id}/permissions` | 200 (guard tulis tidak ikut memblokir baca) |
| I22 | Mode penghitung bersama: dua proses, 5+5 percobaan | percobaan ke-11 ditolak secara agregat |
| I23 | Batas grace: token dirotasi lalu dipakai ulang pada +9s dan +12s | +9s → 401 tanpa cabut keluarga; +12s → 401 + keluarga tercabut |
| I24 | Audit route: setiap `/admin/*` punya pemetaan izin; tidak ada route penulis role/is_active/permission yang lolos | tes gagal bila ada route tanpa mapping |

## Matriks tes rate limit dan verifikasi

| ID | Skenario | Harapan |
|---|---|---|
| I10 | 11 percobaan login gagal dalam 1 menit (IP sama) | percobaan ke-11 ditolak generik |
| I17 | `X-Forwarded-For` dipalsukan dari klien tak-terpercaya | limit memakai `RemoteAddr` (pemalsuan tidak menolong) |
| I18 | Respons untuk email terdaftar vs tidak terdaftar | status, body, dan panjang identik |
| I11 | Token verifikasi email dipakai dua kali | percobaan kedua 401 |
| I12 | Token verifikasi email kedaluwarsa (>24 jam) | 401 |
| I20 | Log aplikasi setelah semua tes | tidak memuat token mentah, email mentah, atau password |

## Bukti yang dilampirkan

`go vet ./...` (0) · `go test ./...` (semua paket ok) · run CI (id + status) · daftar endpoint yang
disentuh · konfirmasi tidak ada perubahan kontrak mobile.
