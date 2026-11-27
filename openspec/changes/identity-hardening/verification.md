# Verification: identity-hardening

## Gate evidence

| Gate | Kebutuhan |
|---|---|
| G1 | proposal + design disetujui owner (digest + base SHA) |
| G2 | tasks + spec disetujui, tes invarian sudah ditulis dan merah di base |
| G3 | bukti eksekusi: perintah, exit code, hasil tes, CI |

## Matriks tes negatif

| ID | Skenario | Harapan |
|---|---|---|
| I1 | HR mengubah role dirinya menjadi `super_admin` | 403, role tidak berubah |
| I2 | HR mengubah role pengguna lain menjadi `super_admin` | 403 |
| I3 | HR mengubah role `employee` menjadi `supervisor` | **diizinkan** (bukan admin role) |
| I4 | Super admin mengubah role siapa pun | 200 |
| I5 | Menurunkan super admin terakhir | 422 (guardrail lama tetap) |
| I6 | Aktor tanpa `akses.kelola` memanggil PATCH permissions | 403 |
| I7 | Aktor dengan `pengguna.ubah` tapi tanpa `akses.kelola` memanggil PATCH permissions | 403 |
| I8 | Refresh token akun yang dinonaktifkan | 401 |
| I9 | Refresh token setelah password berubah | 401 (tv naik) |
| I10 | 11 percobaan login gagal dalam 1 menit | percobaan ke-11 ditolak generik |
| I11 | Token verifikasi email dipakai dua kali | percobaan kedua 401 |
| I12 | Token verifikasi email kedaluwarsa (>24 jam) | 401 |

## Bukti yang harus dilampirkan

`go vet ./...` (0), `go test ./...` (semua paket ok), CI run id + status, dan diff scope vs
`allowed_paths`. Tidak ada merge/deploy tanpa approval owner.
