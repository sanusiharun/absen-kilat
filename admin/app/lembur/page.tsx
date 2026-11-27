"use client";

/* Halaman Lembur (modul overtime): daftar klaim lembur dengan tab status,
 * aksi Setujui / Tolak (alasan wajib). Akses: supervisor/HR/super_admin.
 * Menit yang disetujui inilah yang nanti dikalikan tarif oleh modul payroll. */

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import AdminShell from "@/components/admin-shell";
import { Card, ErrorBanner, Pill, Spinner } from "@/components/ui";

type Overtime = {
  id: string;
  user_id: string;
  user_name?: string;
  user_email?: string;
  work_date: string;
  start_at: string;
  end_at: string;
  minutes: number;
  reason?: string;
  status: "pending" | "approved" | "rejected" | "cancelled";
  approved_by?: string;
  rejection_reason?: string;
};

type Tab = "pending" | "all" | "approved" | "rejected";

const TABS: { key: Tab; label: string }[] = [
  { key: "pending", label: "Menunggu" },
  { key: "all", label: "Semua" },
  { key: "approved", label: "Disetujui" },
  { key: "rejected", label: "Ditolak" },
];

const STATUS_CLS: Record<string, string> = {
  pending: "bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400",
  approved: "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400",
  rejected: "bg-rose-100 text-rose-700 dark:bg-rose-500/15 dark:text-rose-400",
  cancelled: "bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300",
};

const STATUS_LABEL: Record<string, string> = {
  pending: "Menunggu",
  approved: "Disetujui",
  rejected: "Ditolak",
  cancelled: "Dibatalkan",
};

function formatMinutes(m: number) {
  const h = Math.floor(m / 60);
  const rest = m % 60;
  return h > 0 ? `${h}j ${rest}m` : `${rest}m`;
}

function formatClock(iso: string) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "-";
  return d.toLocaleTimeString("id-ID", { hour: "2-digit", minute: "2-digit" });
}

export default function LemburPage() {
  const [tab, setTab] = useState<Tab>("pending");
  const [items, setItems] = useState<Overtime[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState<string | null>(null);

  const load = useCallback(async (t: Tab) => {
    setLoading(true);
    setError("");
    try {
      const qs = t === "all" ? "" : `?status=${t}`;
      const res = await api<{ overtimes: Overtime[] }>(`/admin/overtime${qs}`);
      setItems(res.overtimes ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal memuat data lembur");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- pemuatan data disengaja: effect memanggil loader yang menyalakan state loading.
    void load(tab);
  }, [tab, load]);

  const decide = async (id: string, action: "approve" | "reject") => {
    let reason = "";
    if (action === "reject") {
      const input = window.prompt("Alasan penolakan lembur (wajib):");
      if (input === null) return;
      reason = input.trim();
      if (!reason) {
        setError("Alasan penolakan wajib diisi");
        return;
      }
    }
    setBusy(id);
    setError("");
    try {
      await api(`/admin/overtime/${id}/${action}`, {
        method: "POST",
        body: action === "reject" ? JSON.stringify({ reason }) : undefined,
      });
      await load(tab);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Aksi gagal");
    } finally {
      setBusy(null);
    }
  };

  return (
    <AdminShell title="Lembur" subtitle="Klaim lembur kru & karyawan, disetujui supervisor/HR">
      <ErrorBanner message={error} />

      <div className="mb-5 flex flex-wrap gap-2">
        {TABS.map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`rounded-full px-4 py-1.5 text-sm font-medium transition ${
              tab === t.key
                ? "bg-indigo-600 text-white"
                : "bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      <Card>
        {loading ? (
          <Spinner />
        ) : items.length === 0 ? (
          <p className="py-8 text-center text-sm text-slate-400">Belum ada pengajuan lembur.</p>
        ) : (
          <div className="divide-y divide-slate-100 dark:divide-slate-800">
            {items.map((o) => (
              <div key={o.id} className="flex flex-col gap-3 py-4 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold text-slate-800 dark:text-slate-100">
                    {o.user_name || o.user_email || o.user_id}
                  </p>
                  <p className="text-xs text-slate-500 dark:text-slate-400">
                    {o.work_date} · {formatClock(o.start_at)}–{formatClock(o.end_at)} · {formatMinutes(o.minutes)}
                  </p>
                  {o.reason ? (
                    <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">“{o.reason}”</p>
                  ) : null}
                  {o.status === "rejected" && o.rejection_reason ? (
                    <p className="mt-1 text-xs text-rose-500">Alasan ditolak: {o.rejection_reason}</p>
                  ) : null}
                </div>

                <div className="flex shrink-0 items-center gap-2">
                  <Pill className={STATUS_CLS[o.status] ?? STATUS_CLS.cancelled}>
                    {STATUS_LABEL[o.status] ?? o.status}
                  </Pill>
                  {o.status === "pending" ? (
                    <>
                      <button
                        disabled={busy === o.id}
                        onClick={() => void decide(o.id, "approve")}
                        className="rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-emerald-700 disabled:opacity-50"
                      >
                        Setujui
                      </button>
                      <button
                        disabled={busy === o.id}
                        onClick={() => void decide(o.id, "reject")}
                        className="rounded-lg bg-rose-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-rose-700 disabled:opacity-50"
                      >
                        Tolak
                      </button>
                    </>
                  ) : null}
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>
    </AdminShell>
  );
}
