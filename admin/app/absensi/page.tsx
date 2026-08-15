"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api, clearToken, getToken } from "@/lib/api";
import AdminShell from "@/components/admin-shell";
import { Icon } from "@/components/icons";
import { Avatar, Card, CardHeader, ErrorBanner, Pill, Spinner, inputCls } from "@/components/ui";
import { DateInput, Select } from "@/components/fields";
import { formatDateTime } from "@/lib/format";

type AdminAttendance = {
  id: string;
  user_id: string;
  user_name: string;
  user_email: string;
  type: "check_in" | "check_out";
  status: "on_time" | "late";
  approval_status: "pending" | "approved" | "rejected";
  source?: string;
  photo_url?: string;
  address?: string;
  latitude?: number | null;
  longitude?: number | null;
  device?: string;
  ip_address?: string;
  distance_m?: number | null;
  occurred_at: string;
};

type OverrideForm = {
  type: "check_in" | "check_out";
  status: "on_time" | "late";
  approval_status: "pending" | "approved" | "rejected";
  occurred_at: string; // nilai datetime-local
  reason: string;
};

const APPROVAL_META: Record<AdminAttendance["approval_status"], { label: string; cls: string }> = {
  pending: { label: "Menunggu", cls: "bg-orange-100 text-orange-700 dark:bg-orange-500/15 dark:text-orange-400" },
  approved: { label: "Disetujui", cls: "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400" },
  rejected: { label: "Ditolak", cls: "bg-red-100 text-red-600 dark:bg-red-500/15 dark:text-red-400" },
};

const SOURCE_META: Record<string, { label: string; cls: string }> = {
  qr: { label: "QR", cls: "bg-sky-100 text-sky-700 dark:bg-sky-500/15 dark:text-sky-400" },
  manual: { label: "Manual", cls: "bg-slate-200 text-slate-600 dark:bg-slate-700 dark:text-slate-300" },
  app: { label: "Aplikasi", cls: "bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400" },
};

// ISO -> nilai untuk input datetime-local (zona lokal)
function toLocalInput(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// nilai datetime-local -> ISO (UTC)
function fromLocalInput(v: string): string {
  const d = new Date(v);
  return Number.isNaN(d.getTime()) ? "" : d.toISOString();
}

export default function AbsensiPage() {
  const router = useRouter();
  const [rows, setRows] = useState<AdminAttendance[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  // Filters
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [status, setStatus] = useState("");
  const [type, setType] = useState("");
  const [approval, setApproval] = useState("");
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");

  // Override modal
  const [overrideRow, setOverrideRow] = useState<AdminAttendance | null>(null);
  const [ovForm, setOvForm] = useState<OverrideForm | null>(null);
  const [ovError, setOvError] = useState("");
  const [ovSaving, setOvSaving] = useState(false);

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
      if (from) params.set("from", from);
      if (to) params.set("to", to);
      if (status) params.set("status", status);
      if (type) params.set("type", type);
      if (approval) params.set("approval_status", approval);
      if (debouncedSearch) params.set("search", debouncedSearch);
      const qs = params.toString();
      const d = await api<{ attendances: AdminAttendance[]; total: number }>(
        `/admin/attendance${qs ? `?${qs}` : ""}`,
      );
      setRows(d.attendances);
      setTotal(d.total);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memuat absensi");
      if (err instanceof Error && err.message.includes("401")) {
        clearToken();
        router.replace("/login");
      }
    } finally {
      setLoading(false);
    }
  }, [router, from, to, status, type, approval, debouncedSearch]);

  useEffect(() => {
    load();
  }, [load]);

  function openOverride(a: AdminAttendance) {
    setOvError("");
    setOvForm({
      type: a.type,
      status: a.status,
      approval_status: a.approval_status || "pending",
      occurred_at: toLocalInput(a.occurred_at),
      reason: "",
    });
    setOverrideRow(a);
  }

  async function submitOverride(e: React.FormEvent) {
    e.preventDefault();
    if (!overrideRow || !ovForm) return;
    setOvError("");
    if (!ovForm.reason.trim()) {
      setOvError("Alasan wajib diisi");
      return;
    }
    setOvSaving(true);
    try {
      await api(`/admin/attendance/${overrideRow.id}`, {
        method: "PATCH",
        body: JSON.stringify({
          type: ovForm.type,
          status: ovForm.status,
          approval_status: ovForm.approval_status,
          occurred_at: fromLocalInput(ovForm.occurred_at),
          reason: ovForm.reason.trim(),
        }),
      });
      setSuccess("Override absensi berhasil disimpan");
      setOverrideRow(null);
      setOvForm(null);
      load();
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Gagal menyimpan override";
      setOvError(msg.includes("VALIDATION") || msg.includes("alasan") ? "Alasan wajib diisi" : msg);
    } finally {
      setOvSaving(false);
    }
  }

  return (
    <AdminShell title="Absensi" subtitle="Riwayat seluruh absensi karyawan">
      <ErrorBanner message={error} />
      {success && (
        <div className="mb-6 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm font-medium text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/40 dark:text-emerald-400">
          {success}
        </div>
      )}

      <Card>
        <CardHeader title="Riwayat Absensi" subtitle={`${total} catatan ditemukan`} />

        {/* Filters */}
        <div className="flex flex-wrap items-end gap-3 border-b border-slate-100 px-6 py-4 dark:border-slate-800">
          <div>
            <label className="mb-1 block text-[11px] font-semibold uppercase tracking-wide text-slate-400 dark:text-slate-500">
              Dari
            </label>
            <DateInput value={from} onChange={(e) => setFrom(e.target.value)} className="w-36" />
          </div>
          <div>
            <label className="mb-1 block text-[11px] font-semibold uppercase tracking-wide text-slate-400 dark:text-slate-500">
              Sampai
            </label>
            <DateInput value={to} onChange={(e) => setTo(e.target.value)} className="w-36" />
          </div>
          <div>
            <label className="mb-1 block text-[11px] font-semibold uppercase tracking-wide text-slate-400 dark:text-slate-500">
              Status
            </label>
            <Select
              value={status}
              onChange={(e) => setStatus(e.target.value)}
              className="w-36"
              options={[
                { value: "", label: "Semua Status" },
                { value: "on_time", label: "Tepat Waktu" },
                { value: "late", label: "Terlambat" },
              ]}
            />
          </div>
          <div>
            <label className="mb-1 block text-[11px] font-semibold uppercase tracking-wide text-slate-400 dark:text-slate-500">
              Tipe
            </label>
            <Select
              value={type}
              onChange={(e) => setType(e.target.value)}
              className="w-36"
              options={[
                { value: "", label: "Semua Tipe" },
                { value: "check_in", label: "Masuk" },
                { value: "check_out", label: "Keluar" },
              ]}
            />
          </div>
          <div>
            <label className="mb-1 block text-[11px] font-semibold uppercase tracking-wide text-slate-400 dark:text-slate-500">
              Approval
            </label>
            <Select
              value={approval}
              onChange={(e) => setApproval(e.target.value)}
              className="w-36"
              options={[
                { value: "", label: "Semua Approval" },
                { value: "pending", label: "Menunggu" },
                { value: "approved", label: "Disetujui" },
                { value: "rejected", label: "Ditolak" },
              ]}
            />
          </div>
          <div className="relative min-w-[200px] flex-1 sm:max-w-xs">
            <Icon
              name="search"
              className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400"
            />
            <input
              type="search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Cari nama / email..."
              className={`${inputCls} pl-9`}
            />
          </div>
        </div>

        <div className="overflow-x-auto">
          {loading ? (
            <Spinner />
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-100 text-left text-xs font-semibold uppercase tracking-wide text-slate-500 dark:border-slate-800 dark:text-slate-400">
                  <th className="px-6 py-3">Pengguna</th>
                  <th className="px-6 py-3">Tipe</th>
                  <th className="px-6 py-3">Status</th>
                  <th className="px-6 py-3">Approval</th>
                  <th className="px-6 py-3">Waktu</th>
                  <th className="px-6 py-3">Lokasi</th>
                  <th className="px-6 py-3">Device</th>
                  <th className="px-6 py-3">IP</th>
                  <th className="px-6 py-3 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((a) => {
                  const appr = APPROVAL_META[a.approval_status] || APPROVAL_META.pending;
                  const src = SOURCE_META[a.source || "app"] || SOURCE_META.app;
                  return (
                    <tr key={a.id} className="border-b border-slate-50 transition-colors hover:bg-slate-50/60 dark:border-slate-800 dark:hover:bg-slate-800/40">
                      <td className="px-6 py-3">
                        <div className="flex items-center gap-3">
                          <Avatar name={a.user_name} size="h-8 w-8 text-xs" />
                          <div className="min-w-0">
                            <div className="truncate font-medium text-slate-900 dark:text-slate-100">{a.user_name || "—"}</div>
                            <div className="truncate text-xs text-slate-500 dark:text-slate-400">{a.user_email}</div>
                          </div>
                        </div>
                      </td>
                      <td className="px-6 py-3">
                        <Pill
                          className={
                            a.type === "check_in"
                              ? "bg-blue-100 text-blue-700 dark:bg-blue-500/15 dark:text-blue-400"
                              : "bg-violet-100 text-violet-700 dark:bg-violet-500/15 dark:text-violet-400"
                          }
                        >
                          {a.type === "check_in" ? "Masuk" : "Keluar"}
                        </Pill>
                      </td>
                      <td className="px-6 py-3">
                        <Pill
                          className={
                            a.status === "on_time"
                              ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400"
                              : "bg-orange-100 text-orange-700 dark:bg-orange-500/15 dark:text-orange-400"
                          }
                        >
                          {a.status === "on_time" ? "Tepat Waktu" : "Terlambat"}
                        </Pill>
                      </td>
                      <td className="px-6 py-3">
                        <div className="flex items-center gap-1.5">
                          <Pill className={appr.cls}>{appr.label}</Pill>
                          <Pill className={src.cls}>{src.label}</Pill>
                        </div>
                      </td>
                      <td className="px-6 py-3 whitespace-nowrap text-slate-600 dark:text-slate-300">
                        {formatDateTime(a.occurred_at)}
                      </td>
                      <td className="max-w-[180px] truncate px-6 py-3 text-slate-600 dark:text-slate-300" title={a.address}>
                        {a.address || "—"}
                      </td>
                      <td className="px-6 py-3 text-slate-600 dark:text-slate-300">{a.device || "—"}</td>
                      <td className="px-6 py-3 font-mono text-xs text-slate-500 dark:text-slate-400">{a.ip_address || "—"}</td>
                      <td className="px-6 py-3 text-right">
                        <button
                          type="button"
                          onClick={() => openOverride(a)}
                          className="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-3 py-1.5 text-xs font-semibold text-slate-600 transition-colors hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
                        >
                          <Icon name="edit" className="h-3.5 w-3.5" />
                          Override
                        </button>
                      </td>
                    </tr>
                  );
                })}
                {rows.length === 0 && !loading && (
                  <tr>
                    <td colSpan={9} className="px-6 py-10 text-center text-slate-400 dark:text-slate-500">
                      Belum ada data absensi
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          )}
        </div>
      </Card>

      {/* Modal Override */}
      {overrideRow && ovForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 p-4 dark:bg-slate-950/70">
          <div className="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl dark:bg-slate-900 dark:ring-1 dark:ring-slate-800">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="text-lg font-bold text-slate-900 dark:text-slate-100">Override Absensi</h3>
              <button
                type="button"
                onClick={() => setOverrideRow(null)}
                className="flex h-8 w-8 items-center justify-center rounded-lg text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-200"
              >
                <Icon name="x" className="h-4 w-4" />
              </button>
            </div>
            <div className="mb-4 rounded-xl bg-slate-50 px-4 py-3 text-sm dark:bg-slate-800">
              <span className="font-semibold text-slate-900 dark:text-slate-100">{overrideRow.user_name || "—"}</span>
              <span className="text-slate-500 dark:text-slate-400"> · {overrideRow.user_email}</span>
            </div>

            <form onSubmit={submitOverride} className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label htmlFor="o-type" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                    Tipe
                  </label>
                  <Select
                    id="o-type"
                    value={ovForm.type}
                    onChange={(e) => setOvForm({ ...ovForm, type: e.target.value as OverrideForm["type"] })}
                    options={[
                      { value: "check_in", label: "Masuk" },
                      { value: "check_out", label: "Keluar" },
                    ]}
                  />
                </div>
                <div>
                  <label htmlFor="o-status" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                    Status
                  </label>
                  <Select
                    id="o-status"
                    value={ovForm.status}
                    onChange={(e) => setOvForm({ ...ovForm, status: e.target.value as OverrideForm["status"] })}
                    options={[
                      { value: "on_time", label: "Tepat Waktu" },
                      { value: "late", label: "Terlambat" },
                    ]}
                  />
                </div>
              </div>

              <div>
                <label htmlFor="o-time" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                  Waktu Absen
                </label>
                <input
                  id="o-time"
                  type="datetime-local"
                  required
                  value={ovForm.occurred_at}
                  onChange={(e) => setOvForm({ ...ovForm, occurred_at: e.target.value })}
                  className={`${inputCls} dark:[color-scheme:dark]`}
                />
              </div>

              <div>
                <label htmlFor="o-approval" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                  Status Approval
                </label>
                <Select
                  id="o-approval"
                  value={ovForm.approval_status}
                  onChange={(e) => setOvForm({ ...ovForm, approval_status: e.target.value as OverrideForm["approval_status"] })}
                  options={[
                    { value: "pending", label: "Menunggu" },
                    { value: "approved", label: "Disetujui" },
                    { value: "rejected", label: "Ditolak" },
                  ]}
                />
              </div>

              <div>
                <label htmlFor="o-reason" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                  Alasan <span className="text-red-500">*</span>
                </label>
                <textarea
                  id="o-reason"
                  rows={3}
                  required
                  value={ovForm.reason}
                  onChange={(e) => setOvForm({ ...ovForm, reason: e.target.value })}
                  className={`${inputCls} resize-none`}
                  placeholder="Alasan override wajib diisi..."
                />
              </div>

              {ovError && (
                <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm font-medium text-red-700 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-400">
                  {ovError}
                </div>
              )}

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setOverrideRow(null)}
                  className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
                >
                  Batal
                </button>
                <button
                  type="submit"
                  disabled={ovSaving}
                  className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {ovSaving ? "Menyimpan..." : "Simpan Override"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </AdminShell>
  );
}
