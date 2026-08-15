"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api, clearToken, getToken } from "@/lib/api";
import AdminShell from "@/components/admin-shell";
import { Icon } from "@/components/icons";
import { Avatar, Card, CardHeader, ErrorBanner, Pill, Spinner, inputCls, selectCls } from "@/components/ui";
import { formatDateTime } from "@/lib/format";

type AdminAttendance = {
  id: string;
  user_id: string;
  user_name: string;
  user_email: string;
  type: "check_in" | "check_out";
  status: "on_time" | "late";
  photo_url?: string;
  address?: string;
  latitude?: number | null;
  longitude?: number | null;
  device?: string;
  ip_address?: string;
  distance_m?: number | null;
  occurred_at: string;
};

export default function AbsensiPage() {
  const router = useRouter();
  const [rows, setRows] = useState<AdminAttendance[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // Filters
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [status, setStatus] = useState("");
  const [type, setType] = useState("");
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");

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
  }, [router, from, to, status, type, debouncedSearch]);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <AdminShell title="Absensi" subtitle="Riwayat seluruh absensi karyawan">
      <ErrorBanner message={error} />

      <Card>
        <CardHeader
          title="Riwayat Absensi"
          subtitle={`${total} catatan ditemukan`}
        />

        {/* Filters */}
        <div className="flex flex-wrap items-end gap-3 border-b border-slate-100 px-6 py-4">
          <div>
            <label className="mb-1 block text-[11px] font-semibold uppercase tracking-wide text-slate-400">
              Dari
            </label>
            <input type="date" value={from} onChange={(e) => setFrom(e.target.value)} className={selectCls} />
          </div>
          <div>
            <label className="mb-1 block text-[11px] font-semibold uppercase tracking-wide text-slate-400">
              Sampai
            </label>
            <input type="date" value={to} onChange={(e) => setTo(e.target.value)} className={selectCls} />
          </div>
          <div>
            <label className="mb-1 block text-[11px] font-semibold uppercase tracking-wide text-slate-400">
              Status
            </label>
            <select value={status} onChange={(e) => setStatus(e.target.value)} className={selectCls}>
              <option value="">Semua Status</option>
              <option value="on_time">Tepat Waktu</option>
              <option value="late">Terlambat</option>
            </select>
          </div>
          <div>
            <label className="mb-1 block text-[11px] font-semibold uppercase tracking-wide text-slate-400">
              Tipe
            </label>
            <select value={type} onChange={(e) => setType(e.target.value)} className={selectCls}>
              <option value="">Semua Tipe</option>
              <option value="check_in">Masuk</option>
              <option value="check_out">Keluar</option>
            </select>
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
                <tr className="border-b border-slate-100 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
                  <th className="px-6 py-3">Pengguna</th>
                  <th className="px-6 py-3">Tipe</th>
                  <th className="px-6 py-3">Status</th>
                  <th className="px-6 py-3">Waktu</th>
                  <th className="px-6 py-3">Lokasi</th>
                  <th className="px-6 py-3">Device</th>
                  <th className="px-6 py-3">IP</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((a) => (
                  <tr key={a.id} className="border-b border-slate-50 transition-colors hover:bg-slate-50/60">
                    <td className="px-6 py-3">
                      <div className="flex items-center gap-3">
                        <Avatar name={a.user_name} size="h-8 w-8 text-xs" />
                        <div className="min-w-0">
                          <div className="truncate font-medium text-slate-900">{a.user_name || "—"}</div>
                          <div className="truncate text-xs text-slate-500">{a.user_email}</div>
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-3">
                      <Pill
                        className={
                          a.type === "check_in"
                            ? "bg-blue-100 text-blue-700"
                            : "bg-violet-100 text-violet-700"
                        }
                      >
                        {a.type === "check_in" ? "Masuk" : "Keluar"}
                      </Pill>
                    </td>
                    <td className="px-6 py-3">
                      <Pill
                        className={
                          a.status === "on_time"
                            ? "bg-emerald-100 text-emerald-700"
                            : "bg-orange-100 text-orange-700"
                        }
                      >
                        {a.status === "on_time" ? "Tepat Waktu" : "Terlambat"}
                      </Pill>
                    </td>
                    <td className="px-6 py-3 whitespace-nowrap text-slate-600">
                      {formatDateTime(a.occurred_at)}
                    </td>
                    <td className="max-w-[200px] truncate px-6 py-3 text-slate-600" title={a.address}>
                      {a.address || "—"}
                    </td>
                    <td className="px-6 py-3 text-slate-600">{a.device || "—"}</td>
                    <td className="px-6 py-3 font-mono text-xs text-slate-500">{a.ip_address || "—"}</td>
                  </tr>
                ))}
                {rows.length === 0 && !loading && (
                  <tr>
                    <td colSpan={7} className="px-6 py-10 text-center text-slate-400">
                      Belum ada data absensi
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          )}
        </div>
      </Card>
    </AdminShell>
  );
}
