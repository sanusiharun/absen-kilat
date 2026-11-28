# Controlled Autonomy Policy v1 — snapshot & digest

File ini adalah **salinan kanonik** dari draft policy pada issue #56 comment `7225604464`, yang
disetujui owner (DM Telegram, 2026-11-27) dengan digest yang dikutip pemilik:
`sha256:50da828d3c961fc4aaa5f04344141d3281c669a2c5fcd4e1522ac95a63f2ef90`.

## Catatan verifikasi digest (jujur)

- Digest di atas muncul **verbatim** di komentar sumber `7225604464`, jadi teks dan digest saling
  konsisten di titik itu.
- Digest **tidak dapat direproduksi ulang** oleh sesi ini: metode kanonikalisasi yang dipakai
  penyusun draft tidak tercatat (tidak ada `sha256sum` atas bentuk yang jelas). Beberapa bentuk wajar
  (3 bullet apa adanya, tanpa newline akhir, tanpa prefiks `- `, gabung spasi, sisa body setelah
  judul draft) **tidak** menghasilkan `50da828d…`.
- Karena itu, untuk ke depan, file ini **mendefinisikan** bentuk kanonik (lihat bawah) sehingga
  approval bisa diverifikasi ulang secara deterministik. Digest `50da828d…` diperlakukan sebagai
  **declared digest** yang terikat pada komentar sumber, bukan hasil hitung ulang.

## Bentuk kanonik yang berlaku mulai sekarang

1. Isi file ini **mulai dari judul `# Controlled Autonomy Policy` sampai akhir file** (blok
   `POLICY-CANONICAL-BEGIN` … `POLICY-CANONICAL-END` di bawah).
2. Encoding UTF-8, line ending **LF**, tanpa spasi di ujung baris, diakhiri **satu** newline.
3. Digest = `sha256` atas byte blok tersebut.

<!-- POLICY-CANONICAL-BEGIN -->
- ALLOWED: read-only discovery; docs/spec edits; test runs; commit+push to feat/* and docs/*; open PR (no merge); non-destructive branch reconciliation.
- FORBIDDEN W/O SEPARATE APPROVAL: merge to main, deploy, secrets/credentials, security-policy change, scope expansion, worker dispatch/runtime change, extra billing.
- CONTROLS KEPT: coordinator.db audit + [HERMES][REPORT] with commit SHA per write; per-tick budget/lease; no force-push/history rewrite; shared account prefix != human auth.
<!-- POLICY-CANONICAL-END -->

## Penerapan operasional

| Aspek | Nilai |
|---|---|
| Job cron | `a9a4ac25da16` — bounded-write coordinator, setiap 5 menit, monitor deterministik |
| Kapabilitas tulis | dokumen `docs/**` dan `openspec/**`, test/lint/vet/build, commit+push ke `feat/*`/`docs/*`, buka PR, rekonsiliasi non-destruktif |
| Tetap butuh owner | merge, deploy, credentials/secrets, security policy/RBAC di luar pemetaan disetujui, perluasan scope, runtime/dispatch, biaya tambahan, force-push |
| Larangan self-modification | agent **tidak** boleh mengubah konfigurasi job cron-nya sendiri |
| Audit | `coordinator.db` (status + catatan + SHA) dan komentar `[HERMES][REPORT]` per write |
| Fail closed | identitas/digest/scope tidak jelas ⇒ berhenti dan lapor |
