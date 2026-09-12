"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { api, clearToken, getToken } from "@/lib/api";
import AdminShell from "@/components/admin-shell";
import { Card, CardHeader, Empty, ErrorBanner, LegendRow, Pill, Spinner } from "@/components/ui";
import { Select, Input } from "@/components/fields";
import { formatDate, formatDuration } from "@/lib/format";

type Report = { date: string; present: number; late: number; absent: number; total: number };
type Trend = { date: string; present: number; late: number; absent: number };
type UserReport = {
  user_id: string;
  name: string;
  email: string;
  department: string;
  present_days: number;
  late_days: number;
  absent_days: number;
  check_in_count: number;
  check_out_count: number;
  total_minutes: number;
  total_hours: number;
  last_check_in?: string;
};

const BLUE = "#3b82f6";
const ORANGE = "#f59e0b";
const PINK = "#ec4899";

export default function LaporanPage() {
  const router = useRouter();
  const [days, setDays] = useState(14);
  const [reports, setReports] = useState<Report[]>([]);
  const [trends, setTrends] = useState<Trend[]>([]);
  const [userReports, setUserReports] = useState<UserReport[]>([]);
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    if (!getToken()) {
      router.replace("/login");
      return;
    }
    setLoading(true);
    setError("");
    try {
      const [r, t, u] = await Promise.all([
        api<{ reports: Report[] }>(`/admin/reports?days=${days}`),
        api<{ trends: Trend[] }>(`/admin/trends?days=${days}`),
        api<{ reports: UserReport[] }>(`/admin/reports/users?days=${days}`),
      ]);
      setReports(r.reports);
      setTrends(t.trends);
      setUserReports(u.reports);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memuat laporan");
      if (err instanceof Error && err.message.includes("401")) {
        clearToken();
        router.replace("/login");
      }
    } finally {
      setLoading(false);
    }
  }, [router, days]);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <AdminShell
      title="Laporan"
      subtitle="Ringkasan harian presensi karyawan"
      actions={
        <Select
          value={days}
          onChange={(e) => setDays(Number(e.target.value))}
          pill
          options={[
            { value: "7", label: "7 Hari Terakhir" },
            { value: "14", label: "14 Hari Terakhir" },
            { value: "30", label: "30 Hari Terakhir" },
          ]}
        />
      }
    >
      <ErrorBanner message={error} />

      {/* Trend chart */}
      <Card className="mb-6 p-5">
        <h2 className="font-semibold text-slate-900 dark:text-slate-100">Grafik Tren Presensi</h2>
        <p className="mb-4 text-xs text-slate-500 dark:text-slate-400">{days} hari terakhir</p>
        {loading ? (
          <Spinner />
        ) : trends.length === 0 ? (
          <Empty height={240} />
        ) : (
          <>
            <ResponsiveContainer width="100%" height={260}>
              <LineChart data={trends} margin={{ top: 4, right: 8, left: -22, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#eef2f7" vertical={false} />
                <XAxis
                  dataKey="date"
                  tick={{ fontSize: 10, fill: "#94a3b8" }}
                  tickFormatter={(d: string) => d.slice(5)}
                  axisLine={false}
                  tickLine={false}
                />
                <YAxis
                  tick={{ fontSize: 10, fill: "#94a3b8" }}
                  allowDecimals={false}
                  axisLine={false}
                  tickLine={false}
                />
                <Tooltip
                  contentStyle={{ fontSize: 12, borderRadius: 8, border: "1px solid #e2e8f0" }}
                />
                <Line type="monotone" dataKey="present" name="Hadir" stroke={BLUE} strokeWidth={2} dot={false} />
                <Line type="monotone" dataKey="late" name="Terlambat" stroke={ORANGE} strokeWidth={2} dot={false} />
                <Line type="monotone" dataKey="absent" name="Tidak Hadir" stroke={PINK} strokeWidth={2} dot={false} />
              </LineChart>
            </ResponsiveContainer>
            <LegendRow
              items={[
                { label: "Hadir", color: BLUE },
                { label: "Terlambat", color: ORANGE },
                { label: "Tidak Hadir", color: PINK },
              ]}
            />
          </>
        )}
      </Card>

      {/* Daily summary table */}
      <Card className="mt-6">
        <CardHeader title="Ringkasan Harian" subtitle={`${days} hari terakhir`} />
        <div className="overflow-x-auto">
          {loading ? (
            <Spinner />
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-100 text-left text-xs font-semibold uppercase tracking-wide text-slate-500 dark:border-slate-800 dark:text-slate-400">
                  <th className="px-6 py-3">Tanggal</th>
                  <th className="px-6 py-3 text-center">Total</th>
                  <th className="px-6 py-3 text-center">Hadir</th>
                  <th className="px-6 py-3 text-center">Terlambat</th>
                  <th className="px-6 py-3 text-center">Tidak Hadir</th>
                  <th className="px-6 py-3 text-right">Kehadiran</th>
                </tr>
              </thead>
              <tbody>
                {reports.map((r) => {
                  const pct = r.total > 0 ? Math.round((r.present / r.total) * 100) : 0;
                  return (
                    <tr key={r.date} className="border-b border-slate-50 transition-colors hover:bg-slate-50/60 dark:border-slate-800 dark:hover:bg-slate-800/40">
                      <td className="px-6 py-3 font-medium text-slate-900 dark:text-slate-100">{formatDate(r.date)}</td>
                      <td className="px-6 py-3 text-center text-slate-600 dark:text-slate-300">{r.total}</td>
                      <td className="px-6 py-3 text-center">
                        <Pill className="bg-blue-100 text-blue-700 dark:bg-blue-500/15 dark:text-blue-400">{r.present}</Pill>
                      </td>
                      <td className="px-6 py-3 text-center">
                        <Pill className="bg-orange-100 text-orange-700 dark:bg-orange-500/15 dark:text-orange-400">{r.late}</Pill>
                      </td>
                      <td className="px-6 py-3 text-center">
                        <Pill className="bg-pink-100 text-pink-700 dark:bg-pink-500/15 dark:text-pink-400">{r.absent}</Pill>
                      </td>
                      <td className="px-6 py-3">
                        <div className="flex items-center justify-end gap-2">
                          <div className="h-1.5 w-24 overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800">
                            <div
                              className="h-full rounded-full bg-indigo-500"
                              style={{ width: `${pct}%` }}
                            />
                          </div>
                          <span className="w-10 text-right text-xs font-semibold text-slate-600 dark:text-slate-300">{pct}%</span>
                        </div>
                      </td>
                    </tr>
                  );
                })}
                {reports.length === 0 && !loading && (
                  <tr>
                    <td colSpan={6} className="px-6 py-10 text-center text-slate-400 dark:text-slate-500">
                      Belum ada data laporan
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          )}
        </div>
      </Card>

      {/* Per-user report table */}
      <Card className="mt-6">
        <CardHeader
          title="Laporan per Pengguna"
          subtitle={`Rekap kehadiran tiap karyawan (${days} hari terakhir)`}
          right={
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Cari nama atau email..."
              className="w-56"
            />
          }
        />
        <div className="overflow-x-auto">
          {loading ? (
            <Spinner />
          ) : (
            (() => {
              const q = search.trim().toLowerCase();
              const filtered = q
                ? userReports.filter(
                    (u) =>
                      u.name.toLowerCase().includes(q) ||
                      u.email.toLowerCase().includes(q) ||
                      (u.department || "").toLowerCase().includes(q),
                  )
                : userReports;
              return (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-100 text-left text-xs font-semibold uppercase tracking-wide text-slate-500 dark:border-slate-800 dark:text-slate-400">
                  <th className="px-6 py-3">Nama</th>
                  <th className="px-6 py-3">Departemen</th>
                  <th className="px-6 py-3 text-center">Hari Hadir</th>
                  <th className="px-6 py-3 text-center">Terlambat</th>
                  <th className="px-6 py-3 text-center">Absen</th>
                  <th className="px-6 py-3 text-center">Total Jam</th>
                  <th className="px-6 py-3 text-center">Check-in</th>
                  <th className="px-6 py-3 text-center">Check-out</th>
                  <th className="px-6 py-3 text-right">Terakhir Absen</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((u) => (
                  <tr
                    key={u.user_id}
                    className="border-b border-slate-50 transition-colors hover:bg-slate-50/60 dark:border-slate-800 dark:hover:bg-slate-800/40"
                  >
                    <td className="px-6 py-3">
                      <div className="font-medium text-slate-900 dark:text-slate-100">{u.name}</div>
                      <div className="text-xs text-slate-400 dark:text-slate-500">{u.email}</div>
                    </td>
                    <td className="px-6 py-3 text-slate-600 dark:text-slate-300">
                      {u.department || "-"}
                    </td>
                    <td className="px-6 py-3 text-center">
                      <Pill className="bg-blue-100 text-blue-700 dark:bg-blue-500/15 dark:text-blue-400">
                        {u.present_days}
                      </Pill>
                    </td>
                    <td className="px-6 py-3 text-center">
                      <Pill className="bg-orange-100 text-orange-700 dark:bg-orange-500/15 dark:text-orange-400">
                        {u.late_days}
                      </Pill>
                    </td>
                    <td className="px-6 py-3 text-center">
                      <Pill className="bg-pink-100 text-pink-700 dark:bg-pink-500/15 dark:text-pink-400">
                        {u.absent_days}
                      </Pill>
                    </td>
                    <td
                      className="px-6 py-3 text-center font-medium text-slate-600 dark:text-slate-300"
                      title={`${u.total_hours} jam`}
                    >
                      {formatDuration(u.total_minutes)}
                    </td>
                    <td className="px-6 py-3 text-center text-slate-600 dark:text-slate-300">
                      {u.check_in_count}
                    </td>
                    <td className="px-6 py-3 text-center text-slate-600 dark:text-slate-300">
                      {u.check_out_count}
                    </td>
                    <td className="px-6 py-3 text-right text-xs font-medium text-slate-600 dark:text-slate-300">
                      {u.last_check_in || "-"}
                    </td>
                  </tr>
                ))}
                {filtered.length === 0 && !loading && (
                  <tr>
                    <td colSpan={9} className="px-6 py-10 text-center text-slate-400 dark:text-slate-500">
                      {q ? `Tidak ada hasil untuk "${q}"` : "Belum ada data pengguna"}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
              );
            })()
          )}
        </div>
      </Card>
    </AdminShell>
  );
}
