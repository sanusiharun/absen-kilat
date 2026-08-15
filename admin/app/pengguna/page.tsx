"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { api, clearToken, getToken } from "@/lib/api";
import AdminShell from "@/components/admin-shell";
import { Icon } from "@/components/icons";
import { Avatar, Card, CardHeader, ErrorBanner, Pill, Spinner, inputCls, selectCls } from "@/components/ui";

type AdminUser = {
  id: string;
  email: string;
  name: string;
  avatar_url?: string;
  role: string;
  is_active: boolean;
  is_email_verified: boolean;
  created_at?: string;
};

export default function PenggunaPage() {
  const router = useRouter();
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  // Filters
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");

  // Add-user modal
  const [showModal, setShowModal] = useState(false);
  const [form, setForm] = useState({ name: "", email: "", password: "", role: "employee" });
  const [formError, setFormError] = useState("");
  const [saving, setSaving] = useState(false);

  // Per-row action busy state
  const busy = useRef<Set<string>>(new Set());
  const [, forceRender] = useState(0);

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search), 350);
    return () => clearTimeout(t);
  }, [search]);

  const load = useCallback(async () => {
    if (!getToken()) {
      router.replace("/login");
      return;
    }
    setLoading(true);
    setError("");
    try {
      const params = new URLSearchParams();
      if (debouncedSearch) params.set("search", debouncedSearch);
      if (status) params.set("status", status);
      const qs = params.toString();
      const d = await api<{ users: AdminUser[] }>(`/admin/users${qs ? `?${qs}` : ""}`);
      setUsers(d.users);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memuat pengguna");
      if (err instanceof Error && err.message.includes("401")) {
        clearToken();
        router.replace("/login");
      }
    } finally {
      setLoading(false);
    }
  }, [router, debouncedSearch, status]);

  useEffect(() => {
    load();
  }, [load]);

  async function updateUser(id: string, patch: Record<string, unknown>) {
    if (busy.current.has(id)) return;
    busy.current.add(id);
    forceRender((n) => n + 1);
    setError("");
    setSuccess("");
    try {
      const d = await api<{ user: AdminUser }>(`/admin/users/${id}`, {
        method: "PATCH",
        body: JSON.stringify(patch),
      });
      setUsers((prev) => prev.map((u) => (u.id === id ? d.user : u)));
      setSuccess("Perubahan pengguna berhasil disimpan");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memperbarui pengguna");
    } finally {
      busy.current.delete(id);
      forceRender((n) => n + 1);
    }
  }

  async function createUser(e: React.FormEvent) {
    e.preventDefault();
    setFormError("");
    setSaving(true);
    try {
      await api<{ user: AdminUser }>("/admin/users", {
        method: "POST",
        body: JSON.stringify(form),
      });
      setShowModal(false);
      setForm({ name: "", email: "", password: "", role: "employee" });
      setSuccess("Pengguna baru berhasil ditambahkan");
      load();
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Gagal menambah pengguna");
    } finally {
      setSaving(false);
    }
  }

  const isBusy = (id: string) => busy.current.has(id);

  return (
    <AdminShell title="Pengguna" subtitle="Kelola akun karyawan dan admin">
      <ErrorBanner message={error} />
      {success && (
        <div className="mb-6 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm font-medium text-emerald-700">
          {success}
        </div>
      )}

      <Card>
        <CardHeader
          title="Daftar Pengguna"
          subtitle="Cari, ubah role, atau nonaktifkan akun"
          right={
            <button
              type="button"
              onClick={() => {
                setFormError("");
                setShowModal(true);
              }}
              className="inline-flex items-center gap-1.5 rounded-lg bg-indigo-600 px-3.5 py-2 text-xs font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700"
            >
              <Icon name="plus" className="h-4 w-4" />
              Tambah Pengguna
            </button>
          }
        />

        {/* Filters */}
        <div className="flex flex-wrap items-center gap-3 border-b border-slate-100 px-6 py-4">
          <div className="relative min-w-[220px] flex-1 sm:max-w-xs">
            <Icon
              name="search"
              className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400"
            />
            <input
              type="search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Cari nama atau email..."
              className={`${inputCls} pl-9`}
            />
          </div>
          <select value={status} onChange={(e) => setStatus(e.target.value)} className={selectCls}>
            <option value="">Semua Status</option>
            <option value="active">Aktif</option>
            <option value="inactive">Nonaktif</option>
          </select>
          <span className="ml-auto rounded-full bg-indigo-50 px-3 py-1 text-xs font-semibold text-indigo-700">
            {users.length} pengguna
          </span>
        </div>

        <div className="overflow-x-auto">
          {loading ? (
            <Spinner />
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-100 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
                  <th className="px-6 py-3">Nama</th>
                  <th className="px-6 py-3">Email</th>
                  <th className="px-6 py-3">Role</th>
                  <th className="px-6 py-3">Status</th>
                  <th className="px-6 py-3 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody>
                {users.map((u) => (
                  <tr key={u.id} className="border-b border-slate-50 transition-colors hover:bg-slate-50/60">
                    <td className="px-6 py-3">
                      <div className="flex items-center gap-3">
                        <Avatar name={u.name} src={u.avatar_url} size="h-8 w-8 text-xs" />
                        <span className="font-medium text-slate-900">{u.name || "—"}</span>
                      </div>
                    </td>
                    <td className="px-6 py-3 text-slate-600">{u.email}</td>
                    <td className="px-6 py-3">
                      <select
                        value={u.role}
                        disabled={isBusy(u.id)}
                        onChange={(e) => updateUser(u.id, { role: e.target.value })}
                        className={`${selectCls} py-1 text-xs disabled:opacity-50`}
                      >
                        <option value="employee">Karyawan</option>
                        <option value="admin">Admin</option>
                      </select>
                    </td>
                    <td className="px-6 py-3">
                      <Pill
                        className={
                          u.is_active ? "bg-emerald-100 text-emerald-700" : "bg-red-100 text-red-600"
                        }
                      >
                        <span className={`h-1.5 w-1.5 rounded-full ${u.is_active ? "bg-emerald-500" : "bg-red-500"}`} />
                        {u.is_active ? "Aktif" : "Nonaktif"}
                      </Pill>
                    </td>
                    <td className="px-6 py-3 text-right">
                      <button
                        type="button"
                        disabled={isBusy(u.id)}
                        onClick={() => updateUser(u.id, { is_active: !u.is_active })}
                        className={`rounded-lg px-3 py-1.5 text-xs font-semibold transition-colors disabled:opacity-50 ${
                          u.is_active
                            ? "border border-red-200 text-red-600 hover:bg-red-50"
                            : "border border-emerald-200 text-emerald-600 hover:bg-emerald-50"
                        }`}
                      >
                        {u.is_active ? "Nonaktifkan" : "Aktifkan"}
                      </button>
                    </td>
                  </tr>
                ))}
                {users.length === 0 && !loading && (
                  <tr>
                    <td colSpan={5} className="px-6 py-10 text-center text-slate-400">
                      Belum ada data pengguna
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          )}
        </div>
      </Card>

      {/* Add-user modal */}
      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 p-4">
          <div className="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="text-lg font-bold text-slate-900">Tambah Pengguna</h3>
              <button
                type="button"
                onClick={() => setShowModal(false)}
                className="flex h-8 w-8 items-center justify-center rounded-lg text-slate-400 hover:bg-slate-100 hover:text-slate-700"
              >
                <Icon name="x" className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={createUser} className="space-y-4">
              <div>
                <label htmlFor="u-name" className="mb-1.5 block text-sm font-medium text-slate-800">
                  Nama Lengkap
                </label>
                <input
                  id="u-name"
                  required
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  className={inputCls}
                  placeholder="cth: Budi Santoso"
                />
              </div>
              <div>
                <label htmlFor="u-email" className="mb-1.5 block text-sm font-medium text-slate-800">
                  Email
                </label>
                <input
                  id="u-email"
                  type="email"
                  required
                  value={form.email}
                  onChange={(e) => setForm({ ...form, email: e.target.value })}
                  className={inputCls}
                  placeholder="nama@perusahaan.id"
                />
              </div>
              <div>
                <label htmlFor="u-password" className="mb-1.5 block text-sm font-medium text-slate-800">
                  Kata Sandi
                </label>
                <input
                  id="u-password"
                  type="password"
                  required
                  minLength={8}
                  value={form.password}
                  onChange={(e) => setForm({ ...form, password: e.target.value })}
                  className={inputCls}
                  placeholder="Minimal 8 karakter"
                />
              </div>
              <div>
                <label htmlFor="u-role" className="mb-1.5 block text-sm font-medium text-slate-800">
                  Role
                </label>
                <select
                  id="u-role"
                  value={form.role}
                  onChange={(e) => setForm({ ...form, role: e.target.value })}
                  className={selectCls}
                >
                  <option value="employee">Karyawan</option>
                  <option value="admin">Admin</option>
                </select>
              </div>

              {formError && (
                <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm font-medium text-red-700">
                  {formError}
                </div>
              )}

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowModal(false)}
                  className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50"
                >
                  Batal
                </button>
                <button
                  type="submit"
                  disabled={saving}
                  className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {saving ? "Menyimpan..." : "Simpan"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </AdminShell>
  );
}
