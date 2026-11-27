# Tasks: identity-hardening

Semua task butuh G1/G2 + approval owner sebelum kode aplikasi disentuh (AUTONOMY_POLICY §2).

- [ ] T1 Tes invarian dulu (merah di base): HR menaikkan dirinya jadi super_admin → 403; HR menaikkan orang lain jadi super_admin → 403; super_admin tetap boleh; penurunan super_admin terakhir → 422.
- [ ] T2 Implementasi guard eskalasi di `AdminService.UpdateUser`.
- [ ] T3 Permission `akses.kelola` di registry + pemetaan route akses + update template HR.
- [ ] T4 Migrasi aditif `users.token_version` + logika refresh (cek `IsActive`, cocokkan `tv`, naikkan saat password/role berubah).
- [ ] T5 Rate limit login/forgot/reset (per IP + per akun, respons generik).
- [ ] T6 Token verifikasi email acak + kedaluwarsa + sekali pakai.
- [ ] T7 Tes regresi: alur login/refresh/reset normal tetap hijau; mobile tidak berubah.
- [ ] T8 Catat bukti: `go vet`, `go test ./...`, CI hijau, ringkasan di issue.

Urutan wajib: T1 → T2 → T3 → (T4, T5, T6 paralel) → T7 → T8.
