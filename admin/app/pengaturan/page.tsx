"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api, clearToken, getToken } from "@/lib/api";
import AdminShell from "@/components/admin-shell";
import { Card, ErrorBanner, Spinner, inputCls } from "@/components/ui";
import { Select } from "@/components/fields";

type AppSettings = {
  work_start: string;
  work_end: string;
  absen_mode: "both" | "checkin_only";
  workdays: string[];
  photo_policy: "radius" | "always" | "off";
};

const DAYS: { value: string; label: string }[] = [
  { value: "mon", label: "Senin" },
  { value: "tue", label: "Selasa" },
  { value: "wed", label: "Rabu" },
  { value: "thu", label: "Kamis" },
  { value: "fri", label: "Jumat" },
  { value: "sat", label: "Sabtu" },
  { value: "sun", label: "Minggu" },
];

export default function PengaturanPage() {
  const router = useRouter();
  const [settings, setSettings] = useState<AppSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    if (!getToken()) {
      router.replace("/login");
      return;
    }
    setLoading(true);
    setError("");
    try {
      const d = await api<{ settings: AppSettings }>("/admin/settings");
      setSettings(d.settings);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memuat pengaturan");
      if (err instanceof Error && err.message.includes("401")) {
        clearToken();
        router.replace("/login");
      }
    } finally {
      setLoading(false);
    }
  }, [router]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- pemuatan data disengaja: effect memanggil loader yang menyalakan state loading.
    load();
  }, [load]);

  function toggleDay(day: string) {
    if (!settings) return;
    const has = settings.workdays.includes(day);
    setSettings({
      ...settings,
      workdays: has
        ? settings.workdays.filter((d) => d !== day)
        : [...settings.workdays, day],
    });
  }

  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (!settings) return;
    setSaving(true);
    setError("");
    setSuccess("");
    try {
      const d = await api<{ settings: AppSettings }>("/admin/settings", {
        method: "PUT",
        body: JSON.stringify({
          work_start: settings.work_start,
          work_end: settings.work_end,
          absen_mode: settings.absen_mode,
          workdays: settings.workdays,
          photo_policy: settings.photo_policy,
        }),
      });
      setSettings(d.settings);
      setSuccess("Pengaturan berhasil disimpan");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menyimpan pengaturan");
    } finally {
      setSaving(false);
    }
  }

  const set = <K extends keyof AppSettings>(key: K, value: AppSettings[K]) => {
    if (settings) setSettings({ ...settings, [key]: value });
  };

  return (
    <AdminShell title="Pengaturan" subtitle="Atur jam kerja dan kebijakan absensi">
      <ErrorBanner message={error} />
      {success && (
        <div className="mb-6 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm font-medium text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/40 dark:text-emerald-400">
          {success}
        </div>
      )}

      {loading ? (
        <Card className="p-6">
          <Spinner />
        </Card>
      ) : !settings ? (
        <Card className="p-6 text-sm text-slate-500 dark:text-slate-400">
          Pengaturan tidak tersedia.
        </Card>
      ) : (
        <form onSubmit={save} className="max-w-2xl space-y-6">
          {/* Jam kerja */}
          <Card className="p-6">
            <h2 className="font-semibold text-slate-900 dark:text-slate-100">Jam Kerja</h2>
            <p className="mb-5 text-xs text-slate-500 dark:text-slate-400">
              Jam mulai dan selesai kerja yang dipakai untuk penilaian keterlambatan.
            </p>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div>
                <label htmlFor="s-work-start" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                  Jam Mulai
                </label>
                <input
                  id="s-work-start"
                  type="time"
                  required
                  value={settings.work_start}
                  onChange={(e) => set("work_start", e.target.value)}
                  className={inputCls}
                />
              </div>
              <div>
                <label htmlFor="s-work-end" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                  Jam Selesai
                </label>
                <input
                  id="s-work-end"
                  type="time"
                  required
                  value={settings.work_end}
                  onChange={(e) => set("work_end", e.target.value)}
                  className={inputCls}
                />
              </div>
            </div>
          </Card>

          {/* Mode absensi */}
          <Card className="p-6">
            <h2 className="font-semibold text-slate-900 dark:text-slate-100">Mode Absensi</h2>
            <p className="mb-4 text-xs text-slate-500 dark:text-slate-400">
              Apakah karyawan wajib absen masuk dan pulang, atau cukup absen masuk saja.
            </p>
            <div className="space-y-3">
              <label className="flex cursor-pointer items-start gap-3 rounded-xl border border-slate-200 p-4 transition-colors has-checked:border-indigo-500 has-checked:bg-indigo-50/60 dark:border-slate-700 dark:has-checked:border-indigo-500 dark:has-checked:bg-indigo-500/10">
                <input
                  type="radio"
                  name="absen_mode"
                  checked={settings.absen_mode === "both"}
                  onChange={() => set("absen_mode", "both")}
                  className="mt-0.5 h-4 w-4 accent-indigo-600"
                />
                <span>
                  <span className="block text-sm font-semibold text-slate-900 dark:text-slate-100">Datang + Pulang</span>
                  <span className="block text-xs text-slate-500 dark:text-slate-400">
                    Karyawan absen saat masuk dan saat pulang kerja.
                  </span>
                </span>
              </label>
              <label className="flex cursor-pointer items-start gap-3 rounded-xl border border-slate-200 p-4 transition-colors has-checked:border-indigo-500 has-checked:bg-indigo-50/60 dark:border-slate-700 dark:has-checked:border-indigo-500 dark:has-checked:bg-indigo-500/10">
                <input
                  type="radio"
                  name="absen_mode"
                  checked={settings.absen_mode === "checkin_only"}
                  onChange={() => set("absen_mode", "checkin_only")}
                  className="mt-0.5 h-4 w-4 accent-indigo-600"
                />
                <span>
                  <span className="block text-sm font-semibold text-slate-900 dark:text-slate-100">Datang Saja</span>
                  <span className="block text-xs text-slate-500 dark:text-slate-400">
                    Cukup satu absensi saat datang, tanpa absen pulang.
                  </span>
                </span>
              </label>
            </div>
          </Card>

          {/* Hari kerja */}
          <Card className="p-6">
            <h2 className="font-semibold text-slate-900 dark:text-slate-100">Hari Kerja</h2>
            <p className="mb-4 text-xs text-slate-500 dark:text-slate-400">
              Hari dalam seminggu yang dihitung sebagai hari kerja.
            </p>
            <div className="flex flex-wrap gap-2">
              {DAYS.map((d) => {
                const on = settings.workdays.includes(d.value);
                return (
                  <button
                    key={d.value}
                    type="button"
                    onClick={() => toggleDay(d.value)}
                    className={`rounded-full px-4 py-2 text-sm font-semibold transition-colors ${
                      on
                        ? "bg-indigo-600 text-white shadow-sm"
                        : "border border-slate-300 text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
                    }`}
                  >
                    {d.label}
                  </button>
                );
              })}
            </div>
          </Card>

          {/* Kebijakan foto */}
          <Card className="p-6">
            <h2 className="font-semibold text-slate-900 dark:text-slate-100">Kebijakan Foto</h2>
            <p className="mb-4 text-xs text-slate-500 dark:text-slate-400">
              Kapan foto wajib diambil saat absensi.
            </p>
            <div className="max-w-xs">
              <Select
                value={settings.photo_policy}
                onChange={(e) => set("photo_policy", e.target.value as AppSettings["photo_policy"])}
                options={[
                  { value: "radius", label: "Sesuai radius lokasi" },
                  { value: "always", label: "Selalu wajib foto" },
                  { value: "off", label: "Tanpa foto" },
                ]}
              />
            </div>
          </Card>

          <div className="flex justify-end gap-2">
            <button
              type="submit"
              disabled={saving}
              className="rounded-lg bg-indigo-600 px-5 py-2.5 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {saving ? "Menyimpan..." : "Simpan Pengaturan"}
            </button>
          </div>
        </form>
      )}
    </AdminShell>
  );
}
