# Isolasi credential — kebijakan dan temuan

Keputusan owner (2026-11-27, DM): **otomasi rotasi credential DITUNDA**. Prioritas: **isolasi
credential, least privilege, dan pemisahan akses** antara koordinator, worker, dan deployment.
Credential disimpan **di luar repository** dan **di luar workspace agent**. Memakai infrastruktur
yang sudah ada, **tanpa biaya tambahan**. **Rotasi manual tetap wajib** bila ada indikasi exposure
atau compromise.

## 1. Prinsip

1. **Satu identitas, satu tujuan.** Token koordinator tidak boleh dipakai worker; token deployment
   tidak boleh dipakai koordinator.
2. **Least privilege.** Setiap token hanya punya scope yang benar-benar dipakai (mis. `issues:write`
   untuk koordinator, tanpa `contents:write` bila tidak perlu).
3. **Di luar repo dan di luar workspace agent.** Rahasia tidak berada di direktori kerja agent,
   tidak di repo, tidak di komentar, tidak di log.
4. **Fail closed.** Bila credential tidak tersedia atau tidak jelas asalnya, pekerjaan berhenti.

## 2. Temuan current state (jujur, apa adanya)

| Temuan | Risiko | Status |
|---|---|---|
| `CREDENTIALS.txt` (password MySQL/Postgres) ada di riwayat `golang-banking-platform` yang **publik** | tinggi — harus dianggap sudah bocor | **belum dirotasi** (sesuai keputusan owner: manual, bila ada indikasi exposure) |
| Token `gh` yang dipakai koordinator = token owner dengan scope luas | sedang–tinggi — satu token untuk semua peran | perlu pemisahan (butuh pembuatan token baru oleh owner) |
| Secret worker di `~/.agentic/secrets.env` — **di dalam workspace agent** | sedang — melanggar prinsip #3 | perlu dipindah keluar workspace |
| `ADMIN_PASSWORD` di unit `absen-api` bukan lagi password aktif akun | rendah, tapi menyesatkan | catat, jangan dipakai sebagai sumber kebenaran |
| Kredensial demo/staff tersimpan di dokumen handoff | sedang | review ulang, jangan diperluas |

## 3. Target pemisahan peran

| Peran | Identitas | Scope yang diizinkan | Lokasi rahasia |
|---|---|---|---|
| Koordinator | token Git terbatas (fine-grained) | `issues:write`, `pull_requests:read`, `contents:read` pada repo AbsenKilat | di luar workspace agent, dibaca lewat environment oleh service, bukan file di repo |
| Worker | token terpisah, hanya untuk branch-nya | `contents:write` pada repo yang sama, tanpa `issues:write` | di luar workspace, disuntik saat eksekusi |
| Deployment | kredensial server (SSH/sudo) terpisah | hanya host target | di luar workspace; tidak dipakai koordinator |

Catatan: pembuatan token/identitas baru **memerlukan tindakan owner** (tidak bisa dibuat agent), dan
belum boleh dieksekusi sebelum approval karena menyentuh security policy.

## 4. Dampak ke runtime G1/G2

Change `mv-agentic-runtime` yang sudah disetujui **perlu tambahan requirement** karena runtime-lah yang
menjalankan worker dan membaca budget/lease:

- **R9 (baru)** — Runtime SHALL menerima path kredensial dari konfigurasi, SHALL menolak start bila
  path berada di dalam workspace agent atau di dalam repository, dan SHALL tidak pernah menulis
  kredensial ke log/komentar.
- **N14 (tes negatif baru)** — konfigurasi yang menunjuk path kredensial di dalam workspace/repo ⇒
  runtime menolak (exit 21) tanpa menjalankan worker.
- Karena spec berubah, **digest G1/G2 `mv-agentic-runtime` berubah** dan perlu disetujui ulang.

## 5. Langkah yang bisa dieksekusi tanpa approval baru (low-risk, dokumentasi)

- Dokumen ini (kebijakan + temuan).
- Menambahkan requirement R9 + tes N14 ke change runtime (dokumen).
- Mencatat larangan menaruh rahasia di workspace agent pada `AUTONOMY_POLICY`.

## 6. Langkah yang MEMBUTUHKAN approval owner (security-sensitive)

1. Membuat token Git terbatas untuk koordinator dan token terpisah untuk worker.
2. Memindahkan `~/.agentic/secrets.env` keluar workspace agent dan menyuntikkannya lewat environment.
3. Rotasi manual password yang pernah bocor (`CREDENTIALS.txt`) dan pembersihan riwayat repo publik.
4. Perubahan konfigurasi service yang menyentuh kredensial (`absen-api`, gateway Hermes).
