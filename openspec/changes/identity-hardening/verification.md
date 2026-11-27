# Verification: identity-hardening

Base: `0413c9cc`. Rencana tes ini **boleh** ada sebelum approval (dokumen); eksekusi tes butuh G1/G2.

## Gate evidence

| Gate | Kebutuhan |
|---|---|
| G1 | proposal + design disetujui owner (digest + base SHA final) |
| G2 | tasks + spec + **rencana tes ini** disetujui; tes merah ditulis setelah G2 |
| G3 | perintah, exit code, hasil tes, run CI, diff scope vs `allowed_paths` |

## Matriks tes negatif (role dan permission)

| ID | Skenario | Harapan |
|---|---|---|
| I1 | HR mengubah role dirinya menjadi `super_admin` | 403, role tidak berubah |
| I2 | HR mengubah role orang lain menjadi `super_admin` | 403 |
| I3 | HR mengubah `employee` menjadi `supervisor` | 200 (bukan admin role, dan `rank(new) <= rank(actor)`) |
| I4 | Super admin mengubah role siapa pun | 200 |
| I5 | Menurunkan super admin terakhir | 422 |
| I6 | **Konkurensi**: dua request paralel menurunkan dua super admin terakhir | tepat satu 200, satu 422 |
| I7 | Aktor dengan `pengguna.ubah` tanpa `akses.kelola` memanggil PATCH permissions | 403 |
| I16 | `POST /admin/users` dengan `role=super_admin` oleh HR | 403 (jalur bypass tertutup) |

## Matriks tes sesi dan token

| ID | Skenario | Harapan |
|---|---|---|
| I8 | Refresh token akun yang dinonaktifkan | 401, keluarga token dicabut |
| I9 | Refresh setelah password berubah | 401 (semua keluarga dicabut) |
| I13 | **Dua refresh paralel dengan token sama** | tepat satu 200; yang kalah 401 |
| I14 | Replay token yang sudah dipakai | 401 dan **seluruh keluarga dicabut** |
| I15 | Pemakaian token dari keluarga yang sudah dicabut | 401, tercatat audit `refresh_replay` |
| I19 | Token legacy (JWT lama) masih diterima sampai `exp` | 200, log menandai `legacy_refresh` |

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
