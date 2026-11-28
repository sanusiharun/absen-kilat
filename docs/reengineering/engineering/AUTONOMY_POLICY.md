# Autonomy Policy — Controlled Autonomy

Read-only discovery, monitoring and draft specs are permitted. Issue updates and draft documentation
branches are permitted within the assigned repository. Application edits, worker execution and PR
creation require an approved change and a scoped task. No agent merge, deploy, secret access beyond
required scoped credentials, destructive operation or unapproved business rule changes. Human
approval must be attributable to verified owner identity, not agent-authored text. Never trust labels
alone. Fail closed if identity, revision or scope cannot be verified. Public issues must not contain
PII, secrets or production logs.

*Diperbarui 2026-11-27 dengan keputusan owner: **Controlled Autonomy** (issue #56 comment
`7225575359`; relay #57 `7225576818`, #58 `7225575998`, #59 `7225577504`). Paragraf di atas tetap
berlaku dan diperjelas — bukan dilonggarkan.*

Prinsip: agent bekerja **tanpa approval berulang** untuk pekerjaan berisiko rendah yang sudah
diperintahkan, tetapi tidak pernah melewati batas keras yang tidak bisa dinegosiasikan agent itu sendiri.

## 1. Boleh dijalankan otomatis (low-risk)

| Kategori | Contoh | Syarat |
|---|---|---|
| Discovery read-only | membaca kode, `grep`, `git log`, query DB baca | tidak menulis di luar workspace |
| Dokumen dan spesifikasi | `docs/**`, `openspec/**`, delta spec, backlog, plan | tidak mengubah kode aplikasi |
| Rekonsiliasi branch non-destruktif | `git merge origin/main` ke branch fitur, atau branch baru | **tanpa** force push ke branch bersama, tanpa menulis ulang riwayat publik |
| Pembersihan artefak | `.gitignore`, berhenti melacak `__pycache__`, berkas scratch | tidak menghapus berkas sumber |
| Testing dan verifikasi | unit/invarian test, `go vet`, lint, build, validator SDD | hasil dilaporkan apa adanya, termasuk yang gagal |
| Commit dan push ke branch fitur | commit kecil, pesan jelas, scope sempit | bukan `main`, tidak menimpa commit orang lain |
| Membuat PR | PR siap-review ke `main` | **tanpa** auto-merge; tidak menandai siap-merge bila gate masih terbuka |
| Melaporkan | komentar GitHub ber-prefix, laporan Telegram | sertakan task ID, status, SHA, bukti |

## 2. Selalu butuh approval owner (hard gate)

1. **Merge** ke `main` atau branch rilis (termasuk menyetujui PR sendiri).
2. **Deployment** ke lingkungan mana pun, termasuk restart layanan produksi.
3. **Perubahan sensitif keamanan**: auth, sesi/token, RBAC di luar pemetaan yang sudah disetujui,
   kebijakan password, kredensial, rahasia, audit trail.
4. **Perluasan scope** di luar change/tiket yang disetujui.
5. **Biaya tambahan**: layanan berbayar, lisensi, SDK berbayar, kuota API baru.
6. **Implementasi runtime agentic dan worker dispatch**: hanya setelah runtime minimum tervalidasi
   **dan** G1/G2 runtime disetujui terpisah.
7. **Penghapusan** data, branch, atau worktree milik pihak lain.

## 3. Identitas dan bukti approval

- Prefix komentar (`[CHATGPT][TASK]`, `[HERMES][REPORT]`, …) adalah **routing metadata**, bukan bukti
  identitas dan bukan otorisasi.
- Repo ini memakai satu akun GitHub bersama; author id **tidak** bisa membedakan manusia dari agent.
- Approval yang mengikat diverifikasi lewat **DM privat owner**, format
  `APPROVE <change> <gate> <digest> <base_sha>`, dengan pemeriksaan `from.id` dan
  `chat.type == "private"`, plus anti-replay `message_id`.
- Relay approval di GitHub hanya rujukan percakapan; tidak boleh diperlakukan sebagai izin.

## 4. Kontrol yang dipertahankan

- **Audit**: setiap tindakan meninggalkan jejak (commit, komentar, log, record eksekusi).
- **Isolasi**: pekerjaan di worktree/branch terpisah; workspace scratch tidak menyentuh repo utama.
- **Budget**: 50.000 token per change, 1800 s wall-clock, maksimum 2 attempt transien; telemetri
  tidak tersedia ⇒ reservasi **penuh**, dispatch berikutnya ditolak (exit 21).
- **Lease**: satu worker per change; lease aktif ⇒ acquire berikutnya ditolak (exit 20).
- **Recovery**: gagal dua kali berturut-turut ⇒ berhenti dan lapor, bukan improvisasi.
- **Fail closed**: keadaan tidak diketahui (budget/telemetri/digest/identitas) = penolakan.

## 5. Kewajiban pelaporan

Setiap laporan minimal memuat: task ID, status, revision SHA, referensi komentar yang ditanggapi,
perintah yang dijalankan, dan hasil apa adanya — termasuk bagian yang **belum** terverifikasi.
Klaim "selesai" tanpa bukti dianggap belum selesai.

Bila suatu capability butuh tindakan manual owner (mis. restart proses dari luar cgroup gateway,
atau pembuatan kredensial), agent melaporkan **langkah persisnya** melalui Telegram dan berhenti di
situ — bukan menebak atau memintas batas.

## 6. Riwayat keputusan

| Tanggal | Keputusan | Sumber |
|---|---|---|
| 2026-11-27 | Controlled Autonomy: low-risk tanpa approval berulang; merge/deploy/security/scope/biaya tetap owner | #56 `7225575359` |
| 2026-11-27 | Rekonsiliasi fondasi SDD non-destruktif (branch baru bila perlu); review PR #55 tetap read-only | #57 `7225576818` |
| 2026-11-27 | Koreksi doc-only reservasi penuh 50.000, cleanup `.pyc`, digest ulang | #58 `7225575998` |
| 2026-11-27 | Lanjut planning produk + spec/dokumen low-risk; pilot dashboard + slice end-to-end terkecil | #59 `7225577504` |

## 7. Kanal approval owner (mengikat, 2026-11-27)

- **Telegram DM owner terautentikasi adalah satu-satunya kanal approval.** Komentar, reaction, label,
  atau prefix di GitHub bukan approval; GitHub hanya koordinasi, penugasan, review, dan audit trail.
- Permintaan approval dikirim sebagai **tombol pilihan** yang memuat **rekomendasi, risiko, evidence,
  dan dampak** setiap opsi.
- Approval terikat pada **scope + revision + commit SHA + digest**. Tanpa binding itu, approval tidak
  boleh dieksekusi.
- Aktivitas yang sudah diizinkan di §1 **tidak perlu** approval berulang.
- Approval yang tidak urgent **digabung** dalam satu permintaan.
- **Quality gate mendahului tawaran merge**: CI wajib hijau, security review selesai, acceptance test
  PASS. Approval owner tidak boleh mem-bypass gate kualitas.
- Bila approval belum dijawab, **lanjutkan pekerjaan independen** yang tidak terblokir; delivery tidak
  boleh berhenti total karena satu approval pending.
