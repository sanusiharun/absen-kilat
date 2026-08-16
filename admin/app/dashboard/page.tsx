"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import {
  Line,
  LineChart,
  ResponsiveContainer,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
  PieChart,
  Pie,
  Cell,
  BarChart,
  Bar,
} from "recharts";
import { api, getToken, clearToken } from "@/lib/api";
import AdminShell from "@/components/admin-shell";
import { Icon, type IconName } from "@/components/icons";
import { Avatar, Empty, LegendRow } from "@/components/ui";
import { Select } from "@/components/fields";

type Stats = {
  total_users: number;
  present: number;
  late: number;
  absent: number;
  present_pct: number;
  late_pct: number;
  absent_pct: number;
};
type Trend = { date: string; present: number; late: number; absent: number };
type Loc = { location: string; count: number };
type AdminUser = {
  id: string;
  email: string;
  name: string;
  role: string;
  is_active: boolean;
  is_email_verified: boolean;
  avatar_url?: string;
  created_at?: string;
};
type LeavePending = {
  id: string;
  user_name: string;
  leave_type: string;
  start_date: string;
  end_date: string;
  status: string;
};

const PRIMARY = "#6366f1";
const BLUE = "#3b82f6";
const ORANGE = "#f59e0b";
const PINK = "#ec4899";

type Tone = "indigo" | "green" | "orange" | "pink" | "blue";
const TONE_ICON: Record<Tone, string> = {
  indigo: "bg-indigo-100 text-indigo-600",
  green: "bg-emerald-100 text-emerald-600",
  orange: "bg-orange-100 text-orange-600",
  pink: "bg-pink-100 text-pink-600",
  blue: "bg-blue-100 text-blue-600",
};

export default function DashboardPage() {
  const router = useRouter();
  const [stats, setStats] = useState<Stats | null>(null);
  const [trends, setTrends] = useState<Trend[]>([]);
  const [locs, setLocs] = useState<Loc[]>([]);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [leaves, setLeaves] = useState<LeavePending[]>([]);
  const [error, setError] = useState("");
  const [days, setDays] = useState(7);

  const load = useCallback(async () => {
    if (!getToken()) {
      router.replace("/login");
      return;
    }
    setError("");
    try {
      const [s, t, l, u, lv] = await Promise.all([
        api<Stats>("/admin/stats"),
        api<{ trends: Trend[] }>(`/admin/trends?days=${days}`),
        api<{ locations: Loc[] }>("/admin/locations"),
        api<{ users: AdminUser[] }>("/admin/users"),
        api<{ leaves: LeavePending[] }>("/admin/leave/pending").catch(() => ({ leaves: [] })),
      ]);
      setStats(s);
      setTrends(t.trends);
      setLocs(l.locations);
      setUsers(u.users);
      setLeaves(lv.leaves);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memuat data");
      if (err instanceof Error && err.message.includes("401")) {
        clearToken();
        router.replace("/login");
      }
    }
  }, [router, days]);

  useEffect(() => {
    load();
  }, [load]);

  const donut = stats
    ? [
        { name: "Hadir", value: stats.present, color: BLUE },
        { name: "Terlambat", value: stats.late, color: ORANGE },
        { name: "Tidak Hadir", value: stats.absent, color: PINK },
      ]
    : [];
  const donutTotal = donut.reduce((a, b) => a + b.value, 0);

  const topLocs = [...locs].sort((a, b) => b.count - a.count).slice(0, 5);

  return (
    <AdminShell
      title="Dashboard"
      subtitle="Ringkasan aktivitas dan data absensi"
      actions={
        <>
          {/* Date filter */}
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

          {/* Notifications */}
          <button
            type="button"
            title="Notifikasi"
            className="relative flex h-9 w-9 items-center justify-center rounded-full border border-slate-200 bg-white text-slate-500 transition-colors hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400 dark:hover:bg-slate-700"
          >
            <Icon name="bell" className="h-[18px] w-[18px]" />
            <span className="absolute right-2 top-2 h-2 w-2 rounded-full bg-pink-500" />
          </button>
        </>
      }
    >
      {error && (
        <div className="mb-6 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm font-medium text-red-700 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-400">
          {error}
        </div>
      )}

      {/* Stat cards */}
      <div className="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard
          label="Total Pengguna"
          value={stats?.total_users}
          sub="total akun terdaftar"
          icon="users"
          tone="indigo"
        />
        <StatCard
          label="Presensi Hari Ini"
          value={stats?.present}
          sub={`${stats?.present_pct?.toFixed(1) ?? "0.0"}% dari total`}
          icon="calendar"
          tone="green"
        />
        <StatCard
          label="Terlambat Hari Ini"
          value={stats?.late}
          sub={`${stats?.late_pct?.toFixed(1) ?? "0.0"}% dari total`}
          icon="clock"
          tone="orange"
        />
        <StatCard
          label="Tidak Hadir"
          value={stats?.absent}
          sub={`${stats?.absent_pct?.toFixed(1) ?? "0.0"}% dari total`}
          icon="user-x"
          tone="pink"
        />
      </div>

      {/* Cuti widget */}
      <Link
        href="/cuti"
        className="mb-6 flex flex-col gap-4 rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-100 transition-colors hover:bg-slate-50 dark:bg-slate-900 dark:ring-slate-800 dark:hover:bg-slate-800/60 sm:flex-row sm:items-center sm:justify-between"
      >
        <div className="flex items-center gap-4">
          <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-violet-100 text-violet-600 dark:bg-violet-500/15 dark:text-violet-400">
            <Icon name="calendar" className="h-6 w-6" />
          </div>
          <div>
            <div className="text-sm font-medium text-slate-600 dark:text-slate-300">Pengajuan Cuti</div>
            <div className="mt-1 flex items-baseline gap-2">
              <span className="text-3xl font-bold leading-none text-slate-900 dark:text-slate-100">
                {leaves.length}
              </span>
              <span className="text-sm text-slate-500 dark:text-slate-400">menunggu persetujuan</span>
            </div>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {leaves.slice(0, 3).map((lv) => (
            <span
              key={lv.id}
              className="inline-flex items-center gap-1.5 rounded-full bg-violet-50 px-2.5 py-1 text-xs font-medium text-violet-700 dark:bg-violet-500/15 dark:text-violet-300"
            >
              <span className="h-1.5 w-1.5 rounded-full bg-violet-500" />
              {lv.user_name}
            </span>
          ))}
          {leaves.length > 3 && (
            <span className="text-xs text-slate-400 dark:text-slate-500">+{leaves.length - 3} lainnya</span>
          )}
          <span className="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-3 py-1.5 text-xs font-semibold text-indigo-600 hover:bg-indigo-50 dark:border-slate-700 dark:text-indigo-400 dark:hover:bg-indigo-500/10">
            Kelola Cuti
            <Icon name="chevron-down" className="h-3.5 w-3.5" />
          </span>
        </div>
      </Link>

      {/* Charts row */}
      <div className="mb-6 grid grid-cols-1 gap-4 lg:grid-cols-3">
        {/* Line chart */}
        <div className="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-100 dark:bg-slate-900 dark:ring-slate-800 lg:col-span-1">
          <h2 className="mb-1 font-semibold text-slate-900 dark:text-slate-100">Grafik Presensi</h2>
          <p className="mb-4 text-xs text-slate-500 dark:text-slate-400">{days} hari terakhir</p>
          {trends.length === 0 ? (
            <Empty height={220} />
          ) : (
            <>
              <ResponsiveContainer width="100%" height={200}>
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
                    contentStyle={{
                      fontSize: 12,
                      borderRadius: 8,
                      border: "1px solid #e2e8f0",
                    }}
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
        </div>

        {/* Donut */}
        <div className="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-100 dark:bg-slate-900 dark:ring-slate-800">
          <h2 className="mb-4 font-semibold text-slate-900 dark:text-slate-100">Presensi per Status</h2>
          {donutTotal === 0 ? (
            <Empty height={220} />
          ) : (
            <>
              <div className="relative h-[180px]">
                <ResponsiveContainer width="100%" height="100%">
                  <PieChart>
                    <Pie
                      data={donut}
                      dataKey="value"
                      nameKey="name"
                      cx="50%"
                      cy="50%"
                      innerRadius={55}
                      outerRadius={82}
                      paddingAngle={3}
                      strokeWidth={0}
                    >
                      {donut.map((d) => (
                        <Cell key={d.name} fill={d.color} />
                      ))}
                    </Pie>
                    <Tooltip contentStyle={{ fontSize: 12, borderRadius: 8, border: "1px solid #e2e8f0" }} />
                  </PieChart>
                </ResponsiveContainer>
                <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
                  <div className="text-2xl font-bold text-slate-900 dark:text-slate-100">{donutTotal}</div>
                  <div className="text-xs text-slate-500 dark:text-slate-400">Total Presensi</div>
                </div>
              </div>
              <LegendRow
                items={donut.map((d) => ({ label: d.name, color: d.color, value: d.value }))}
              />
            </>
          )}
        </div>

        {/* Bar chart */}
        <div className="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-100 dark:bg-slate-900 dark:ring-slate-800">
          <h2 className="mb-4 font-semibold text-slate-900 dark:text-slate-100">Absensi per Lokasi (Top 5)</h2>
          {topLocs.length === 0 ? (
            <Empty height={220} />
          ) : (
            <ResponsiveContainer width="100%" height={220}>
              <BarChart data={topLocs} layout="vertical" margin={{ top: 0, right: 12, left: 8, bottom: 0 }}>
                <XAxis
                  type="number"
                  tick={{ fontSize: 10, fill: "#94a3b8" }}
                  allowDecimals={false}
                  axisLine={false}
                  tickLine={false}
                />
                <YAxis
                  type="category"
                  dataKey="location"
                  width={120}
                  tick={{ fontSize: 11, fill: "#475569" }}
                  axisLine={false}
                  tickLine={false}
                />
                <Tooltip contentStyle={{ fontSize: 12, borderRadius: 8, border: "1px solid #e2e8f0" }} />
                <Bar dataKey="count" name="Absen" fill={PRIMARY} radius={[0, 6, 6, 0]} barSize={18} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>

      {/* Users table */}
      <div className="rounded-2xl bg-white shadow-sm ring-1 ring-slate-100 dark:bg-slate-900 dark:ring-slate-800">
        <div className="flex items-center justify-between border-b border-slate-100 px-6 py-4 dark:border-slate-800">
          <div>
            <h2 className="font-semibold text-slate-900 dark:text-slate-100">Manajemen Pengguna</h2>
            <p className="text-xs text-slate-500 dark:text-slate-400">Daftar akun terdaftar di sistem</p>
          </div>
          <span className="rounded-full bg-indigo-50 px-3 py-1 text-xs font-semibold text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-400">
            {users.length} pengguna
          </span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-slate-100 text-left text-xs font-semibold uppercase tracking-wide text-slate-500 dark:border-slate-800 dark:text-slate-400">
                <th className="px-6 py-3">Nama</th>
                <th className="px-6 py-3">Email</th>
                <th className="px-6 py-3">Role</th>
                <th className="px-6 py-3">Status</th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => (
                <tr key={u.id} className="border-b border-slate-50 transition-colors hover:bg-slate-50/60 dark:border-slate-800 dark:hover:bg-slate-800/40">
                  <td className="px-6 py-3">
                    <div className="flex items-center gap-3">
                      <Avatar name={u.name} src={u.avatar_url} size="h-8 w-8 text-xs" />
                      <span className="font-medium text-slate-900 dark:text-slate-100">{u.name || "—"}</span>
                    </div>
                  </td>
                  <td className="px-6 py-3 text-slate-600 dark:text-slate-300">{u.email}</td>
                  <td className="px-6 py-3">
                    <span
                      className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${
                        u.role === "admin"
                          ? "bg-indigo-100 text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-400"
                          : "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300"
                      }`}
                    >
                      {u.role === "admin" ? "Admin" : "Karyawan"}
                    </span>
                  </td>
                  <td className="px-6 py-3">
                    <span
                      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ${
                        u.is_active ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400" : "bg-red-100 text-red-600 dark:bg-red-500/15 dark:text-red-400"
                      }`}
                    >
                      <span className={`h-1.5 w-1.5 rounded-full ${u.is_active ? "bg-emerald-500" : "bg-red-500"}`} />
                      {u.is_active ? "Aktif" : "Nonaktif"}
                    </span>
                  </td>
                </tr>
              ))}
              {users.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-6 py-10 text-center text-slate-400 dark:text-slate-500">
                    Belum ada data pengguna
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </AdminShell>
  );
}

/* ---------- Sub-components ---------- */

function StatCard({
  label,
  value,
  sub,
  icon,
  tone,
}: {
  label: string;
  value?: number;
  sub?: string;
  icon: IconName;
  tone: Tone;
}) {
  return (
    <div className="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-100 dark:bg-slate-900 dark:ring-slate-800">
      <div className="flex items-center justify-between">
        <div className="min-w-0">
          <div className="text-3xl font-bold leading-none text-slate-900 dark:text-slate-100">{value ?? "—"}</div>
          <div className="mt-2 text-sm font-medium text-slate-600 dark:text-slate-300">{label}</div>
        </div>
        <div className={`flex h-12 w-12 shrink-0 items-center justify-center rounded-xl ${TONE_ICON[tone]}`}>
          <Icon name={icon} className="h-6 w-6" />
        </div>
      </div>
      <div className="mt-3 text-xs text-slate-400 dark:text-slate-500">{sub}</div>
    </div>
  );
}
