"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api, clearToken, getToken } from "@/lib/api";
import AdminShell from "@/components/admin-shell";
import { Icon } from "@/components/icons";
import { Avatar, Card, CardHeader, ErrorBanner, Pill, Spinner, inputCls } from "@/components/ui";
import { Select } from "@/components/fields";

type AdminUser = {
  id: string;
  email: string;
  name: string;
  avatar_url?: string;
  role: string;
  is_active: boolean;
  is_email_verified: boolean;
  nip?: string;
  phone?: string;
  department?: string;
  position?: string;
  address?: string;
  join_date?: string;
  manager_id?: string;
  employee_code?: string;
  created_at?: string;
};

type EditForm = {
  name: string;
  email: string;
  role: string;
  is_active: boolean;
  nip: string;
  phone: string;
  department: string;
  position: string;
  address: string;
  join_date: string;
  employee_code: string;
  manager_id: string;
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
  const [busyIds, setBusyIds] = useState<Set<string>>(() => new Set());

  // Edit-user modal
  const [editUser, setEditUser] = useState<AdminUser | null>(null);
  const [editForm, setEditForm] = useState<EditForm | null>(null);
  const [editFormError, setEditFormError] = useState("");
  const [editSaving, setEditSaving] = useState(false);
  const [managers, setManagers] = useState<AdminUser[]>([]);

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

  // Scroll-lock body saat modal edit terbuka (cegah background scroll)
  useEffect(() => {
    if (editUser) {
      const prev = document.body.style.overflow;
      document.body.style.overflow = "hidden";
      return () => {
        document.body.style.overflow = prev;
      };
    }
  }, [editUser]);

  async function updateUser(id: string, patch: Record<string, unknown>) {
    if (busyIds.has(id)) return;
    setBusyIds((prev) => new Set(prev).add(id));
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
      setBusyIds((prev) => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
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

  async function openEdit(u: AdminUser) {
    setEditFormError("");
    setEditForm({
      name: u.name,
      email: u.email,
      role: u.role,
      is_active: u.is_active,
      nip: u.nip || "",
      phone: u.phone || "",
      department: u.department || "",
      position: u.position || "",
      address: u.address || "",
      join_date: u.join_date || "",
      employee_code: u.employee_code || "",
      manager_id: u.manager_id || "",
    });
    setEditUser(u);
    // Muat daftar calon manager (semua pengguna aktif) sekali saja.
    if (managers.length === 0) {
      try {
        const d = await api<{ users: AdminUser[] }>("/admin/users?status=active");
        setManagers(d.users);
      } catch {
        setManagers([]);
      }
    }
  }

  async function saveEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!editUser || !editForm) return;
    setEditFormError("");
    setEditSaving(true);
    try {
      const d = await api<{ user: AdminUser }>(`/admin/users/${editUser.id}`, {
        method: "PATCH",
        body: JSON.stringify({
          name: editForm.name.trim(),
          email: editForm.email.trim(),
          role: editForm.role,
          is_active: editForm.is_active,
          nip: editForm.nip.trim(),
          phone: editForm.phone.trim(),
          department: editForm.department.trim(),
          position: editForm.position.trim(),
          address: editForm.address.trim(),
          join_date: editForm.join_date,
          employee_code: editForm.employee_code.trim(),
          manager_id: editForm.manager_id,
        }),
      });
      setUsers((prev) => prev.map((x) => (x.id === editUser.id ? d.user : x)));
      setSuccess("Data pengguna berhasil diperbarui");
      setEditUser(null);
      setEditForm(null);
    } catch (err) {
      setEditFormError(err instanceof Error ? err.message : "Gagal memperbarui pengguna");
    } finally {
      setEditSaving(false);
    }
  }

  const isBusy = (id: string) => busyIds.has(id);

  return (
    <AdminShell title="Pengguna" subtitle="Kelola akun karyawan dan admin">
      <ErrorBanner message={error} />
      {success && (
        <div className="mb-6 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm font-medium text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/40 dark:text-emerald-400">
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
        <div className="flex flex-wrap items-center gap-3 border-b border-slate-100 px-6 py-4 dark:border-slate-800">
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
          <Select
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            className="w-40"
            options={[
              { value: "", label: "Semua Status" },
              { value: "active", label: "Aktif" },
              { value: "inactive", label: "Nonaktif" },
            ]}
          />
          <span className="ml-auto rounded-full bg-indigo-50 px-3 py-1 text-xs font-semibold text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-400">
            {users.length} pengguna
          </span>
        </div>

        <div className="overflow-x-auto">
          {loading ? (
            <Spinner />
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-100 text-left text-xs font-semibold uppercase tracking-wide text-slate-500 dark:border-slate-800 dark:text-slate-400">
                  <th className="px-6 py-3">Nama</th>
                  <th className="px-6 py-3">Email</th>
                  <th className="px-6 py-3">Role</th>
                  <th className="px-6 py-3">Status</th>
                  <th className="px-6 py-3 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody>
                {users.map((u) => (
                  <tr key={u.id} className="border-b border-slate-50 transition-colors hover:bg-slate-50/60 dark:border-slate-800 dark:hover:bg-slate-800/40">
                    <td className="px-6 py-3">
                      <div className="flex items-center gap-3">
                        <Avatar name={u.name} src={u.avatar_url} size="h-8 w-8 text-xs" />
                        <div className="min-w-0">
                          <div className="font-medium text-slate-900 dark:text-slate-100">{u.name || "—"}</div>
                          {u.nip && <div className="text-xs text-slate-400 dark:text-slate-500">{u.nip}</div>}
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-3 text-slate-600 dark:text-slate-300">{u.email}</td>
                    <td className="px-6 py-3">
                      <Select
                        value={u.role}
                        disabled={isBusy(u.id)}
                        onChange={(e) => updateUser(u.id, { role: e.target.value })}
                        className="w-32"
                        selectClassName="py-1 text-xs"
                        options={[
                          { value: "employee", label: "Karyawan" },
                          { value: "admin", label: "Admin" },
                        ]}
                      />
                    </td>
                    <td className="px-6 py-3">
                      <Pill
                        className={
                          u.is_active
                            ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400"
                            : "bg-red-100 text-red-600 dark:bg-red-500/15 dark:text-red-400"
                        }
                      >
                        <span className={`h-1.5 w-1.5 rounded-full ${u.is_active ? "bg-emerald-500" : "bg-red-500"}`} />
                        {u.is_active ? "Aktif" : "Nonaktif"}
                      </Pill>
                    </td>
                    <td className="px-6 py-3 text-right">
                      <div className="flex items-center justify-end gap-2">
                        <button
                          type="button"
                          disabled={isBusy(u.id)}
                          onClick={() => openEdit(u)}
                          className="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-3 py-1.5 text-xs font-semibold text-slate-600 transition-colors hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
                        >
                          <Icon name="edit" className="h-3.5 w-3.5" />
                          Edit
                        </button>
                        <button
                          type="button"
                          disabled={isBusy(u.id)}
                          onClick={() => updateUser(u.id, { is_active: !u.is_active })}
                          className={`rounded-lg px-3 py-1.5 text-xs font-semibold transition-colors disabled:opacity-50 ${
                            u.is_active
                              ? "border border-red-200 text-red-600 hover:bg-red-50 dark:border-red-900/60 dark:text-red-400 dark:hover:bg-red-950/40"
                              : "border border-emerald-200 text-emerald-600 hover:bg-emerald-50 dark:border-emerald-900/60 dark:text-emerald-400 dark:hover:bg-emerald-950/40"
                          }`}
                        >
                          {u.is_active ? "Nonaktifkan" : "Aktifkan"}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
                {users.length === 0 && !loading && (
                  <tr>
                    <td colSpan={5} className="px-6 py-10 text-center text-slate-400 dark:text-slate-500">
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
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 p-4 dark:bg-slate-950/70">
          <div className="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl dark:bg-slate-900 dark:ring-1 dark:ring-slate-800">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="text-lg font-bold text-slate-900 dark:text-slate-100">Tambah Pengguna</h3>
              <button
                type="button"
                onClick={() => setShowModal(false)}
                className="flex h-8 w-8 items-center justify-center rounded-lg text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-200"
              >
                <Icon name="x" className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={createUser} className="space-y-4">
              <div>
                <label htmlFor="u-name" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
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
                <label htmlFor="u-email" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
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
                <label htmlFor="u-password" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
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
                <label htmlFor="u-role" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                  Role
                </label>
                <Select
                  id="u-role"
                  value={form.role}
                  onChange={(e) => setForm({ ...form, role: e.target.value })}
                  options={[
                    { value: "employee", label: "Karyawan" },
                    { value: "admin", label: "Admin" },
                  ]}
                />
              </div>

              {formError && (
                <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm font-medium text-red-700 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-400">
                  {formError}
                </div>
              )}

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowModal(false)}
                  className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
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
      {/* Edit-user modal */}
      {editUser && editForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 p-4 dark:bg-slate-950/70">
          <div className="max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-2xl bg-white p-6 shadow-xl dark:bg-slate-900 dark:ring-1 dark:ring-slate-800">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="text-lg font-bold text-slate-900 dark:text-slate-100">Edit Pengguna</h3>
              <button
                type="button"
                onClick={() => setEditUser(null)}
                className="flex h-8 w-8 items-center justify-center rounded-lg text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-200"
              >
                <Icon name="x" className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={saveEdit} className="space-y-4">
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div>
                  <label htmlFor="e-name" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                    Nama Lengkap
                  </label>
                  <input
                    id="e-name"
                    required
                    value={editForm.name}
                    onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
                    className={inputCls}
                  />
                </div>
                <div>
                  <label htmlFor="e-email" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                    Email
                  </label>
                  <input
                    id="e-email"
                    type="email"
                    required
                    value={editForm.email}
                    onChange={(e) => setEditForm({ ...editForm, email: e.target.value })}
                    className={inputCls}
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div>
                  <label htmlFor="e-role" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                    Role
                  </label>
                  <Select
                    id="e-role"
                    value={editForm.role}
                    onChange={(e) => setEditForm({ ...editForm, role: e.target.value })}
                    options={[
                      { value: "employee", label: "Karyawan" },
                      { value: "admin", label: "Admin" },
                    ]}
                  />
                </div>
                <div className="flex items-end pb-1">
                  <label className="flex cursor-pointer items-center gap-3">
                    <input
                      type="checkbox"
                      checked={editForm.is_active}
                      onChange={(e) => setEditForm({ ...editForm, is_active: e.target.checked })}
                      className="h-4 w-4 rounded accent-indigo-600"
                    />
                    <span className="text-sm font-medium text-slate-800 dark:text-slate-200">Akun aktif</span>
                  </label>
                </div>
              </div>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div>
                  <label htmlFor="e-nip" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                    NIP
                  </label>
                  <input
                    id="e-nip"
                    value={editForm.nip}
                    onChange={(e) => setEditForm({ ...editForm, nip: e.target.value })}
                    className={inputCls}
                    placeholder="Nomor induk pegawai"
                  />
                </div>
                <div>
                  <label htmlFor="e-code" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                    Kode Karyawan
                  </label>
                  <input
                    id="e-code"
                    value={editForm.employee_code}
                    onChange={(e) => setEditForm({ ...editForm, employee_code: e.target.value })}
                    className={inputCls}
                    placeholder="cth: EMP-001"
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div>
                  <label htmlFor="e-phone" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                    No. HP
                  </label>
                  <input
                    id="e-phone"
                    value={editForm.phone}
                    onChange={(e) => setEditForm({ ...editForm, phone: e.target.value })}
                    className={inputCls}
                    placeholder="08xxxxxxxxxx"
                  />
                </div>
                <div>
                  <label htmlFor="e-join" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                    Tanggal Bergabung
                  </label>
                  <input
                    id="e-join"
                    type="date"
                    value={editForm.join_date}
                    onChange={(e) => setEditForm({ ...editForm, join_date: e.target.value })}
                    className={`${inputCls} dark:[color-scheme:dark]`}
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div>
                  <label htmlFor="e-dept" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                    Departemen
                  </label>
                  <input
                    id="e-dept"
                    value={editForm.department}
                    onChange={(e) => setEditForm({ ...editForm, department: e.target.value })}
                    className={inputCls}
                    placeholder="cth: Engineering"
                  />
                </div>
                <div>
                  <label htmlFor="e-pos" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                    Jabatan
                  </label>
                  <input
                    id="e-pos"
                    value={editForm.position}
                    onChange={(e) => setEditForm({ ...editForm, position: e.target.value })}
                    className={inputCls}
                    placeholder="cth: Staff IT"
                  />
                </div>
              </div>

              <div>
                <label htmlFor="e-manager" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                  Manager / Atasan
                </label>
                <Select
                  id="e-manager"
                  value={editForm.manager_id}
                  onChange={(e) => setEditForm({ ...editForm, manager_id: e.target.value })}
                >
                  <option value="">— Tanpa manager —</option>
                  {managers
                    .filter((m) => m.id !== editUser.id)
                    .map((m) => (
                      <option key={m.id} value={m.id}>
                        {m.name || m.email}
                      </option>
                    ))}
                </Select>
              </div>

              <div>
                <label htmlFor="e-address" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                  Alamat
                </label>
                <textarea
                  id="e-address"
                  rows={2}
                  value={editForm.address}
                  onChange={(e) => setEditForm({ ...editForm, address: e.target.value })}
                  className={`${inputCls} resize-none`}
                  placeholder="Alamat tempat tinggal"
                />
              </div>

              {editFormError && (
                <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm font-medium text-red-700 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-400">
                  {editFormError}
                </div>
              )}

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setEditUser(null)}
                  className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
                >
                  Batal
                </button>
                <button
                  type="submit"
                  disabled={editSaving}
                  className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {editSaving ? "Menyimpan..." : "Simpan"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </AdminShell>
  );
}
