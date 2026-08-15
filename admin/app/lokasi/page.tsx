"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { api, clearToken, getToken } from "@/lib/api";
import AdminShell from "@/components/admin-shell";
import { Card, CardHeader, Empty, ErrorBanner, Pill, Spinner } from "@/components/ui";

type Loc = { location: string; count: number };

const PRIMARY = "#6366f1";

export default function LokasiPage() {
  const router = useRouter();
  const [locs, setLocs] = useState<Loc[]>([]);
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
      const d = await api<{ locations: Loc[] }>("/admin/locations?limit=10");
      setLocs(d.locations);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memuat lokasi");
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

  const maxCount = locs.reduce((m, l) => Math.max(m, l.count), 0);
  const totalCount = locs.reduce((m, l) => m + l.count, 0);

  return (
    <AdminShell title="Lokasi" subtitle="Breakdown lokasi absensi karyawan">
      <ErrorBanner message={error} />

      <div className="mb-6 grid grid-cols-1 gap-4 lg:grid-cols-3">
        {/* Bar chart */}
        <Card className="p-5 lg:col-span-2">
          <h2 className="font-semibold text-slate-900 dark:text-slate-100">Absensi per Lokasi</h2>
          <p className="mb-4 text-xs text-slate-500 dark:text-slate-400">Top 10 lokasi terpadat</p>
          {loading ? (
            <Spinner />
          ) : locs.length === 0 ? (
            <Empty height={260} />
          ) : (
            <ResponsiveContainer width="100%" height={280}>
              <BarChart data={locs} layout="vertical" margin={{ top: 0, right: 16, left: 16, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#eef2f7" horizontal={false} />
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
                  width={180}
                  tick={{ fontSize: 11, fill: "#475569" }}
                  axisLine={false}
                  tickLine={false}
                />
                <Tooltip contentStyle={{ fontSize: 12, borderRadius: 8, border: "1px solid #e2e8f0" }} />
                <Bar dataKey="count" name="Absen" fill={PRIMARY} radius={[0, 6, 6, 0]} barSize={16} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </Card>

        {/* Summary card */}
        <Card className="p-5">
          <h2 className="font-semibold text-slate-900 dark:text-slate-100">Ringkasan</h2>
          <p className="mb-4 text-xs text-slate-500 dark:text-slate-400">10 lokasi teratas</p>
          <div className="space-y-3">
            <div className="rounded-xl bg-indigo-50 p-4 dark:bg-indigo-500/10">
              <div className="text-3xl font-bold text-indigo-700 dark:text-indigo-400">{locs.length}</div>
              <div className="mt-1 text-xs font-medium text-indigo-500 dark:text-indigo-400">Lokasi terdeteksi</div>
            </div>
            <div className="rounded-xl bg-slate-50 p-4 dark:bg-slate-800">
              <div className="text-3xl font-bold text-slate-900 dark:text-slate-100">{totalCount}</div>
              <div className="mt-1 text-xs font-medium text-slate-500 dark:text-slate-400">Total catatan absensi</div>
            </div>
            <p className="text-xs leading-relaxed text-slate-400 dark:text-slate-500">
              Lokasi dihitung dari alamat yang terkirim saat karyawan melakukan absensi.
            </p>
          </div>
        </Card>
      </div>

      {/* Table */}
      <Card>
        <CardHeader title="Daftar Lokasi" subtitle="Diurutkan berdasarkan jumlah absensi" />
        <div className="overflow-x-auto">
          {loading ? (
            <Spinner />
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-100 text-left text-xs font-semibold uppercase tracking-wide text-slate-500 dark:border-slate-800 dark:text-slate-400">
                  <th className="px-6 py-3">#</th>
                  <th className="px-6 py-3">Lokasi</th>
                  <th className="px-6 py-3 text-right">Jumlah Absensi</th>
                  <th className="w-1/3 px-6 py-3">Proporsi</th>
                </tr>
              </thead>
              <tbody>
                {locs.map((l, i) => {
                  const pct = totalCount > 0 ? Math.round((l.count / totalCount) * 100) : 0;
                  return (
                    <tr key={`${l.location}-${i}`} className="border-b border-slate-50 transition-colors hover:bg-slate-50/60 dark:border-slate-800 dark:hover:bg-slate-800/40">
                      <td className="px-6 py-3 text-slate-400 dark:text-slate-500">{i + 1}</td>
                      <td className="px-6 py-3 font-medium text-slate-900 dark:text-slate-100">{l.location}</td>
                      <td className="px-6 py-3 text-right">
                        <Pill className="bg-indigo-100 text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-400">{l.count}</Pill>
                      </td>
                      <td className="px-6 py-3">
                        <div className="flex items-center gap-2">
                          <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800">
                            <div
                              className="h-full rounded-full bg-indigo-500"
                              style={{ width: `${maxCount > 0 ? (l.count / maxCount) * 100 : 0}%` }}
                            />
                          </div>
                          <span className="w-10 text-right text-xs font-semibold text-slate-600 dark:text-slate-300">{pct}%</span>
                        </div>
                      </td>
                    </tr>
                  );
                })}
                {locs.length === 0 && !loading && (
                  <tr>
                    <td colSpan={4} className="px-6 py-10 text-center text-slate-400 dark:text-slate-500">
                      Belum ada data lokasi
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
