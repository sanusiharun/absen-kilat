"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api, clearToken, getToken } from "@/lib/api";
import AdminShell from "@/components/admin-shell";
import { Icon } from "@/components/icons";
import { Card, CardHeader, Empty, ErrorBanner, Pill, Spinner } from "@/components/ui";

type DeviceStat = { device: string; count: number };

export default function DevicePage() {
  const router = useRouter();
  const [devices, setDevices] = useState<DeviceStat[]>([]);
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
      const d = await api<{ devices: DeviceStat[] }>("/admin/devices");
      setDevices(d.devices);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memuat device");
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

  const maxCount = devices.reduce((m, d) => Math.max(m, d.count), 0);
  const totalCount = devices.reduce((m, d) => m + d.count, 0);

  return (
    <AdminShell title="Device" subtitle="Perangkat yang digunakan untuk absensi">
      <ErrorBanner message={error} />

      {/* Summary cards */}
      <div className="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div className="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-100 dark:bg-slate-900 dark:ring-slate-800">
          <div className="flex items-center justify-between">
            <div className="min-w-0">
              <div className="text-3xl font-bold leading-none text-slate-900 dark:text-slate-100">{devices.length}</div>
              <div className="mt-2 text-sm font-medium text-slate-600 dark:text-slate-300">Jenis Perangkat</div>
            </div>
            <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-indigo-100 text-indigo-600 dark:bg-indigo-500/15 dark:text-indigo-400">
              <Icon name="device" className="h-6 w-6" />
            </div>
          </div>
        </div>
        <div className="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-100 dark:bg-slate-900 dark:ring-slate-800">
          <div className="flex items-center justify-between">
            <div className="min-w-0">
              <div className="text-3xl font-bold leading-none text-slate-900 dark:text-slate-100">{totalCount}</div>
              <div className="mt-2 text-sm font-medium text-slate-600 dark:text-slate-300">Total Absensi</div>
            </div>
            <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-emerald-100 text-emerald-600 dark:bg-emerald-500/15 dark:text-emerald-400">
              <Icon name="calendar-check" className="h-6 w-6" />
            </div>
          </div>
        </div>
        <div className="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-100 dark:bg-slate-900 dark:ring-slate-800">
          <div className="flex items-center justify-between">
            <div className="min-w-0">
              <div className="text-3xl font-bold leading-none text-slate-900 dark:text-slate-100">
                {devices[0]?.device || "—"}
              </div>
              <div className="mt-2 text-sm font-medium text-slate-600 dark:text-slate-300">Perangkat Terpopuler</div>
            </div>
            <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-orange-100 text-orange-600 dark:bg-orange-500/15 dark:text-orange-400">
              <Icon name="chart-bar" className="h-6 w-6" />
            </div>
          </div>
        </div>
      </div>

      <Card>
        <CardHeader
          title="Daftar Perangkat"
          subtitle="Diurutkan berdasarkan jumlah penggunaan"
          right={
            <span className="rounded-full bg-indigo-50 px-3 py-1 text-xs font-semibold text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-400">
              {devices.length} perangkat
            </span>
          }
        />
        <div className="overflow-x-auto">
          {loading ? (
            <Spinner />
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-100 text-left text-xs font-semibold uppercase tracking-wide text-slate-500 dark:border-slate-800 dark:text-slate-400">
                  <th className="px-6 py-3">#</th>
                  <th className="px-6 py-3">Perangkat</th>
                  <th className="px-6 py-3 text-right">Jumlah Absensi</th>
                  <th className="w-1/3 px-6 py-3">Proporsi</th>
                </tr>
              </thead>
              <tbody>
                {devices.map((d, i) => {
                  const pct = totalCount > 0 ? Math.round((d.count / totalCount) * 100) : 0;
                  return (
                    <tr key={d.device} className="border-b border-slate-50 transition-colors hover:bg-slate-50/60 dark:border-slate-800 dark:hover:bg-slate-800/40">
                      <td className="px-6 py-3 text-slate-400 dark:text-slate-500">{i + 1}</td>
                      <td className="px-6 py-3">
                        <div className="flex items-center gap-3">
                          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400">
                            <Icon name="device" className="h-4 w-4" />
                          </div>
                          <span className="font-medium text-slate-900 dark:text-slate-100">{d.device}</span>
                        </div>
                      </td>
                      <td className="px-6 py-3 text-right">
                        <Pill className="bg-indigo-100 text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-400">{d.count}</Pill>
                      </td>
                      <td className="px-6 py-3">
                        <div className="flex items-center gap-2">
                          <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800">
                            <div
                              className="h-full rounded-full bg-indigo-500"
                              style={{ width: `${maxCount > 0 ? (d.count / maxCount) * 100 : 0}%` }}
                            />
                          </div>
                          <span className="w-10 text-right text-xs font-semibold text-slate-600 dark:text-slate-300">{pct}%</span>
                        </div>
                      </td>
                    </tr>
                  );
                })}
                {devices.length === 0 && !loading && (
                  <tr>
                    <td colSpan={4} className="px-6 py-10 text-center text-slate-400 dark:text-slate-500">
                      Belum ada data perangkat
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
