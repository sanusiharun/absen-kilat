"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api, getToken, setTokens } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  // Pesan sesi berakhir dibaca dari query (?expired=1 / ?required=1) lewat
  // window.location supaya halaman tetap bisa diprerender statis.
  const [notice, setNotice] = useState("");
  const [nextPath, setNextPath] = useState("/dashboard");

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    // eslint-disable-next-line react-hooks/set-state-in-effect -- pemuatan data disengaja: effect memanggil loader yang menyalakan state (pola sama dengan PR #22).
    if (params.get("expired") === "1") setNotice("Sesi Anda berakhir. Silakan masuk kembali.");
    else if (params.get("required") === "1") setNotice("Silakan masuk untuk melanjutkan.");
    const next = params.get("next");
    if (next && next.startsWith("/") && !next.startsWith("/login")) setNextPath(next);
    // Sudah punya token dan tidak sedang dipaksa login ulang? langsung lanjut.
    if (!params.get("expired") && !params.get("required") && getToken()) {
      window.location.replace(next && next.startsWith("/") ? next : "/dashboard");
    }
  }, []);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      const d = await api<{ access_token: string; refresh_token?: string }>("/auth/login", {
        method: "POST",
        auth: false,
        body: JSON.stringify({ email, password }),
      });
      // Simpan access + refresh: access hanya 15 menit, refresh 30 hari.
      setTokens(d.access_token, d.refresh_token);
      router.replace(nextPath);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login gagal");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-100 p-4 dark:bg-slate-950">
      <div className="w-full max-w-sm">
        {/* Brand */}
        <div className="mb-6 flex items-center justify-center gap-3">
          <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-indigo-600 text-lg font-bold text-white shadow-sm">
            A
          </div>
          <div className="text-left">
            <div className="text-lg font-bold leading-tight text-slate-900 dark:text-slate-100">AbsenKilat</div>
            <div className="text-xs font-medium text-slate-500 dark:text-slate-400">Admin Dashboard</div>
          </div>
        </div>

        <form
          onSubmit={onSubmit}
          className="rounded-2xl bg-white p-8 shadow-sm ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
        >
          <h1 className="text-2xl font-bold text-slate-900 dark:text-slate-100">Masuk</h1>
          {notice && (
            <div
              role="status"
              className="mt-3 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-300"
            >
              {notice}
            </div>
          )}
          <p className="mb-6 mt-1 text-sm text-slate-600 dark:text-slate-400">
            Gunakan akun admin untuk mengakses dashboard.
          </p>

          <label htmlFor="email" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
            Email
          </label>
          <input
            id="email"
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
            className="mb-4 w-full rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-sm text-slate-900 placeholder:text-slate-400 focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100 dark:placeholder:text-slate-500 dark:focus:border-indigo-500 dark:focus:ring-indigo-500/20"
            placeholder="admin@absenkilat.dev"
          />

          <label htmlFor="password" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
            Kata Sandi
          </label>
          <input
            id="password"
            type="password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            className="mb-5 w-full rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-sm text-slate-900 placeholder:text-slate-400 focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100 dark:placeholder:text-slate-500 dark:focus:border-indigo-500 dark:focus:ring-indigo-500/20"
            placeholder="••••••••"
          />

          {error && (
            <div className="mb-4 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm font-medium text-red-700 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-400">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full rounded-lg bg-indigo-600 py-2.5 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {loading ? "Masuk..." : "Masuk"}
          </button>
        </form>
      </div>
    </div>
  );
}
