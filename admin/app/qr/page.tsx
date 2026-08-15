"use client";

/* Halaman PUBLIK absensi via QR (tanpa login).
 * Token diambil dari query string ?t=<token>. Form mengirim multipart ke
 * POST /public/attendance/qr. useSearchParams wajib dibungkus <Suspense>
 * di Next.js — lihat komponen <QrForm> di bawah. */

import { Suspense, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { API_BASE } from "@/lib/api";
import { Icon } from "@/components/icons";

const inputCls =
  "w-full rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-sm text-slate-900 placeholder:text-slate-400 focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100 focus:outline-none";

function QrForm() {
  const sp = useSearchParams();
  const token = useMemo(() => (sp.get("t") || "").trim(), [sp]);

  const [identifier, setIdentifier] = useState("");
  const [name, setName] = useState("");
  const [photo, setPhoto] = useState<File | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // null = belum ada hasil, "success" / "error"
  const [status, setStatus] = useState<"idle" | "success" | "error">("idle");
  const [message, setMessage] = useState("");

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setStatus("idle");
    setMessage("");

    if (!token) {
      setStatus("error");
      setMessage("QR tidak valid — tautan tidak lengkap.");
      return;
    }
    if (!identifier.trim()) {
      setStatus("error");
      setMessage("Email atau kode karyawan wajib diisi.");
      return;
    }

    const fd = new FormData();
    fd.append("token", token);
    // Kirim sebagai email kalau mengandung "@", selain itu sebagai kode karyawan.
    if (identifier.includes("@")) {
      fd.append("email", identifier.trim());
    } else {
      fd.append("employee_code", identifier.trim());
    }
    if (name.trim()) fd.append("name", name.trim());
    if (photo) fd.append("photo", photo);

    setSubmitting(true);
    try {
      // Manual fetch (multipart) — tanpa auth, endpoint publik.
      const res = await fetch(`${API_BASE}/public/attendance/qr`, {
        method: "POST",
        body: fd,
      });

      let body: any = null;
      try {
        body = await res.json();
      } catch {}

      if (res.ok) {
        setStatus("success");
        setMessage("Absen tercatat, menunggu persetujuan supervisor.");
        return;
      }

      const code: string = body?.error?.code || "";
      const errMsg: string = body?.error?.message || "";

      if (res.status === 409) {
        setStatus("error");
        setMessage("Anda sudah absen hari ini.");
      } else if (res.status === 404 && !/resource/i.test(errMsg)) {
        setStatus("error");
        setMessage("Kode/email tidak dikenal. Periksa kembali identitas Anda.");
      } else if (res.status === 401 || res.status === 403) {
        setStatus("error");
        setMessage("QR tidak valid atau sudah tidak aktif.");
      } else if (code === "NOT_FOUND" && /resource/i.test(errMsg)) {
        // Endpoint backend Fase C belum live
        setStatus("error");
        setMessage("Server belum siap. Silakan coba beberapa saat lagi.");
      } else if (res.status === 422 || res.status === 400) {
        setStatus("error");
        setMessage(errMsg || "Data yang dikirim tidak valid. Periksa kembali form Anda.");
      } else {
        setStatus("error");
        setMessage(errMsg || "Terjadi kesalahan. Silakan coba lagi.");
      }
    } catch {
      setStatus("error");
      setMessage("Server belum siap. Silakan coba beberapa saat lagi.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-slate-100 px-4 py-10">
      <div className="w-full max-w-md">
        {/* Logo kecil */}
        <div className="mb-5 flex items-center justify-center gap-2.5">
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-indigo-600 text-base font-bold text-white shadow-sm">
            A
          </div>
          <span className="text-lg font-bold text-slate-900">AbsenKilat</span>
        </div>

        <div className="rounded-2xl bg-white p-6 shadow-sm ring-1 ring-slate-100 sm:p-8">
          <div className="mb-1 flex items-center gap-2 text-indigo-600">
            <Icon name="qrcode" className="h-5 w-5" />
            <h1 className="text-lg font-bold text-slate-900">Absensi QR</h1>
          </div>
          <p className="mb-6 text-sm text-slate-500">
            Isi identitas Anda untuk mencatat kehadiran. Foto dan nama bersifat opsional.
          </p>

          {!token ? (
            <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-4 text-sm font-medium text-amber-700">
              QR tidak valid — tautan tidak lengkap. Silakan pindai ulang QR dari lokasi Anda.
            </div>
          ) : (
            <form onSubmit={submit} className="space-y-4">
              <div>
                <label htmlFor="qr-identifier" className="mb-1.5 block text-sm font-medium text-slate-800">
                  Email atau Kode Karyawan <span className="text-red-500">*</span>
                </label>
                <input
                  id="qr-identifier"
                  type="text"
                  required
                  autoComplete="off"
                  value={identifier}
                  onChange={(e) => setIdentifier(e.target.value)}
                  className={inputCls}
                  placeholder="cth: budi@perusahaan.com atau EMP-001"
                />
              </div>

              <div>
                <label htmlFor="qr-name" className="mb-1.5 block text-sm font-medium text-slate-800">
                  Nama <span className="text-slate-400">(opsional)</span>
                </label>
                <input
                  id="qr-name"
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className={inputCls}
                  placeholder="Nama lengkap Anda"
                />
              </div>

              <div>
                <label htmlFor="qr-photo" className="mb-1.5 block text-sm font-medium text-slate-800">
                  Foto <span className="text-slate-400">(opsional)</span>
                </label>
                <input
                  id="qr-photo"
                  type="file"
                  accept="image/*"
                  onChange={(e) => setPhoto(e.target.files?.[0] || null)}
                  className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-600 file:mr-3 file:rounded-lg file:border-0 file:bg-indigo-50 file:px-3 file:py-1.5 file:text-xs file:font-semibold file:text-indigo-700 hover:file:bg-indigo-100"
                />
              </div>

              <button
                type="submit"
                disabled={submitting}
                className="w-full rounded-lg bg-indigo-600 px-4 py-3 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {submitting ? "Mengirim..." : "Kirim Absensi"}
              </button>
            </form>
          )}

          {status === "success" && (
            <div className="mt-5 flex items-start gap-2.5 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm font-medium text-emerald-700">
              <Icon name="check" className="mt-0.5 h-4 w-4 shrink-0" />
              {message}
            </div>
          )}
          {status === "error" && (
            <div className="mt-5 flex items-start gap-2.5 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm font-medium text-red-700">
              <Icon name="x" className="mt-0.5 h-4 w-4 shrink-0" />
              {message}
            </div>
          )}
        </div>

        <p className="mt-6 text-center text-xs text-slate-400">
          © {new Date().getFullYear()} AbsenKilat · Absensi QR publik
        </p>
      </div>
    </div>
  );
}

export default function QrPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center bg-slate-100">
          <span className="h-6 w-6 animate-spin rounded-full border-2 border-slate-300 border-t-indigo-500" />
        </div>
      }
    >
      <QrForm />
    </Suspense>
  );
}
