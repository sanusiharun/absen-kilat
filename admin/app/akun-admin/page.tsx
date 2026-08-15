"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api, clearToken, getToken } from "@/lib/api";
import AdminShell, { type Me } from "@/components/admin-shell";
import { Icon } from "@/components/icons";
import { Avatar, Card, CardHeader, ErrorBanner, Pill, Spinner, inputCls } from "@/components/ui";

export default function AkunAdminPage() {
  const router = useRouter();
  const [me, setMe] = useState<Me | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // Password form
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [pwError, setPwError] = useState("");
  const [pwSuccess, setPwSuccess] = useState("");
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    if (!getToken()) {
      router.replace("/login");
      return;
    }
    setLoading(true);
    setError("");
    try {
      setMe(await api<Me>("/me"));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memuat profil");
      if (err instanceof Error && err.message.includes("401")) {
        clearToken();
        router.replace("/login");
      }
    } finally {
      setLoading(false);
    }
  }, [router]);

  useEffect(() => {
    load();
  }, [load]);

  async function changePassword(e: React.FormEvent) {
    e.preventDefault();
    setPwError("");
    setPwSuccess("");
    if (newPassword.length < 8) {
      setPwError("Kata sandi baru minimal 8 karakter");
      return;
    }
    if (newPassword !== confirmPassword) {
      setPwError("Konfirmasi kata sandi tidak cocok");
      return;
    }
    setSaving(true);
    try {
      await api("/me/password", {
        method: "POST",
        body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
      });
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      setPwSuccess("Kata sandi berhasil diubah");
    } catch (err) {
      setPwError(err instanceof Error ? err.message : "Gagal mengubah kata sandi");
    } finally {
      setSaving(false);
    }
  }

  const logout = () => {
    clearToken();
    router.replace("/login");
  };

  return (
    <AdminShell title="Akun Admin" subtitle="Profil dan pengaturan keamanan akun">
      <ErrorBanner message={error} />

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Profile */}
        <Card>
          <CardHeader title="Profil" subtitle="Informasi akun admin" />
          {loading ? (
            <Spinner />
          ) : (
            <div className="p-6">
              <div className="mb-6 flex items-center gap-4">
                <Avatar name={me?.name || "Admin"} src={me?.avatar_url} size="h-16 w-16 text-xl" />
                <div>
                  <div className="text-lg font-bold text-slate-900">{me?.name || "—"}</div>
                  <div className="text-sm text-slate-500">{me?.email || "—"}</div>
                  <div className="mt-1.5 flex items-center gap-2">
                    <Pill className="bg-indigo-100 text-indigo-700">
                      {me?.role === "admin" ? "Admin" : me?.role || "—"}
                    </Pill>
                    <Pill
                      className={
                        me?.is_active ? "bg-emerald-100 text-emerald-700" : "bg-red-100 text-red-600"
                      }
                    >
                      {me?.is_active ? "Aktif" : "Nonaktif"}
                    </Pill>
                  </div>
                </div>
              </div>

              <dl className="space-y-3 text-sm">
                <div className="flex items-center justify-between rounded-lg bg-slate-50 px-4 py-3">
                  <dt className="text-slate-500">Email terverifikasi</dt>
                  <dd className="font-medium text-slate-900">
                    {me?.is_email_verified ? "Ya" : "Belum"}
                  </dd>
                </div>
                <div className="flex items-center justify-between rounded-lg bg-slate-50 px-4 py-3">
                  <dt className="text-slate-500">Notifikasi</dt>
                  <dd className="font-medium text-slate-900">
                    {me?.notification_enabled ? "Aktif" : "Nonaktif"}
                  </dd>
                </div>
              </dl>

              <button
                type="button"
                onClick={logout}
                className="mt-6 inline-flex w-full items-center justify-center gap-2 rounded-lg border border-red-200 bg-white px-4 py-2.5 text-sm font-semibold text-red-600 transition-colors hover:bg-red-50"
              >
                <Icon name="logout" className="h-4 w-4" />
                Keluar dari Dashboard
              </button>
            </div>
          )}
        </Card>

        {/* Change password */}
        <Card>
          <CardHeader title="Ganti Kata Sandi" subtitle="Perbarui kata sandi login admin" />
          <form onSubmit={changePassword} className="space-y-4 p-6">
            <div>
              <label htmlFor="pw-current" className="mb-1.5 block text-sm font-medium text-slate-800">
                Kata Sandi Saat Ini
              </label>
              <div className="relative">
                <Icon
                  name="lock"
                  className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400"
                />
                <input
                  id="pw-current"
                  type="password"
                  required
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  autoComplete="current-password"
                  className={`${inputCls} pl-9`}
                  placeholder="••••••••"
                />
              </div>
            </div>
            <div>
              <label htmlFor="pw-new" className="mb-1.5 block text-sm font-medium text-slate-800">
                Kata Sandi Baru
              </label>
              <div className="relative">
                <Icon
                  name="key"
                  className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400"
                />
                <input
                  id="pw-new"
                  type="password"
                  required
                  minLength={8}
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  autoComplete="new-password"
                  className={`${inputCls} pl-9`}
                  placeholder="Minimal 8 karakter"
                />
              </div>
            </div>
            <div>
              <label htmlFor="pw-confirm" className="mb-1.5 block text-sm font-medium text-slate-800">
                Konfirmasi Kata Sandi Baru
              </label>
              <div className="relative">
                <Icon
                  name="key"
                  className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400"
                />
                <input
                  id="pw-confirm"
                  type="password"
                  required
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  autoComplete="new-password"
                  className={`${inputCls} pl-9`}
                  placeholder="Ulangi kata sandi baru"
                />
              </div>
            </div>

            {pwError && (
              <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm font-medium text-red-700">
                {pwError}
              </div>
            )}
            {pwSuccess && (
              <div className="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm font-medium text-emerald-700">
                {pwSuccess}
              </div>
            )}

            <button
              type="submit"
              disabled={saving}
              className="w-full rounded-lg bg-indigo-600 py-2.5 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {saving ? "Menyimpan..." : "Simpan Kata Sandi"}
            </button>
          </form>
        </Card>
      </div>
    </AdminShell>
  );
}
