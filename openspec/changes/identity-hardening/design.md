# Design: identity-hardening

## 1. Guard eskalasi role

Tempat: `AdminService.UpdateUser` sebelum mutasi (`admin_service.go:132`).

```
actor  := userRepo.FindByID(actorID)
target := userRepo.FindByID(targetID)
if up.Role != nil:
    newRole := *up.Role
    if actor.Role != super_admin:
        if newRole in (super_admin, hr) or target.Role in (super_admin, hr):
            return Forbidden("hanya super admin yang boleh mengubah role admin")
        if target.ID == actor.ID:
            return Forbidden("tidak boleh mengubah role sendiri")
    // larangan menurunkan super admin terakhir dipertahankan
```

Aturan tambahan: menonaktifkan akun super_admin terakhir tetap ditolak (sudah ada guardrail akses;
dipakai ulang, bukan diduplikasi).

## 2. Pemisahan permission

- Tambah `akses.kelola` di `internal/authz/registry.go` (menu `pengguna`).
- Route `/admin/users/{id}/permissions` (`routes.go:28-29`) → `akses.kelola`, bukan `pengguna.ubah`.
- Template: `super_admin` (semua), `hr` (tidak lagi otomatis punya `akses.kelola`).

## 3. Refresh token

Opsi A (dipilih, tanpa tabel baru): refresh token = JWT dengan klaim `sid` (session id) + `tv`
(token version) yang disimpan di kolom `users.token_version` (nullable default 0). Refresh memeriksa
`IsActive` dan `tv` cocok; perubahan password/role menaikkan `tv` sehingga semua sesi lama gugur.
Opsi B (ditolak untuk rilis ini): tabel `refresh_tokens` + revocation list — lebih kuat, lebih berat.

## 4. Rate limit auth

Reuse pola pembatas yang sudah ada di QR (`qr_service.go`) — pembatas per IP dan per kunci akun,
jendela 1 menit, batas 10 percobaan; respons selalu generik `401` tanpa membedakan akun ada/tidak.

## 5. Token verifikasi email

Random 32 byte (base64url), disimpan sebagai hash + kedaluwarsa 24 jam + sekali pakai.

## 6. Kompatibilitas

Semua perubahan aditif; mobile tidak berubah. Bila `token_version` belum ada, dianggap 0 sehingga
sesi lama tetap sah sampai migrasi diterapkan.
