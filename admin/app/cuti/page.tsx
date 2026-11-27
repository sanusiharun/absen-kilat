"use client";

/* Halaman Cuti (modul leave): daftar pengajuan cuti dengan tab status,
 * aksi Setujui / Tolak (alasan wajib saat menolak). Akses: supervisor/HR/super_admin. */

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import AdminShell from "@/components/admin-shell";
import { Icon } from "@/components/icons";
import { Card, ErrorBanner, Pill, Spinner } from "@/components/ui";

type Leave = {
  id: string;
  user_id: string;
  user_name?: string;
  user_email?: string;
  leave_type: string;
  start_date: string;
  end_date: string;
  total_days: number;
  reason?: string;
  status: "pending" | "approved" | "rejected";
  approved_by?: string;
  approved_at?: string;
  rejection_reason?: string;
  attachment_url?: string;
  created_at: string;
};

type Tab = "pending" | "all" | "approved" | "rejected";

const TABS: { key: Tab; label: string }[] = [
  { key: "pending", label: "Menunggu" },
  { key: "all", label: "Semua" },
  { key: "approved", label: "Disetujui" },
  { key: "rejected", label: "Ditolak" },
];

const TYPE_META: Record<string, { label: string; cls: string }> = {
  annual: { label: "Cuti Tahunan", cls: "bg-indigo-100 text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-400" },
  sick: { label: "Sakit", cls: "bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400" },
  emergency: { label: "Darurat", cls: "bg-rose-100 text-rose-700 dark:bg-rose-500/15 dark:text-rose-400" },
  maternity: { label: "Melahirkan", cls: "bg-pink-100 text-pink-700 dark:bg-pink-500/15 dark:text-pink-400" },
  paternity: { label: "Ayah", cls: "bg-sky-100 text-sky-700 dark:bg-sky-500/15 dark:text-sky-400" },
};

const STATUS_META: Record<Leave["status"], { label: string; cls: string }> = {
  pending: { label: "Menunggu", cls: "bg-orange-100 text-orange-700 dark:bg-orange-500/15 dark:text-orange-400" },
  approved: { label: "Disetujui", cls: "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400" },
  rejected: { label: "Ditolak", cls: "bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-400" },
};

function typeMeta(code: string) {
  return TYPE_META[code] ?? { label: code, cls: "bg-slate-200 text-slate-600 dark:bg-slate-700 dark:text-slate-300" };
}

function formatDateRange(a: string, b: string): string {
  return `${a} s.d. ${b}`;
}

export default function CutiPage() {
  const [tab, setTab] = useState<Tab>("pending");
  const [leaves, setLeaves] = useState<Leave[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [acting, setActing] = useState<string | null>(null);
  const [rejectId, setRejectId] = useState<string | null>(null);
  const [rejectReason, setRejectReason] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      if (tab === "pending") {
        const d = await api<{ leaves: Leave[] }>("/admin/leave/pending");
        setLeaves(d.leaves);
      } else if (tab === "all") {
        const d = await api<{ leaves: Leave[] }>("/admin/leave/all");
        setLeaves(d.leaves);
      } else {
        const d = await api<{ leaves: Leave[] }>(`/admin/leave?status=${tab}`);
        setLeaves(d.leaves);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal memuat data cuti");
    } finally {
      setLoading(false);
    }
  }, [tab]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- pemuatan data disengaja: effect memanggil loader yang menyalakan state loading.
    load();
  }, [load]);

  const approve = async (id: string) => {
    setActing(id);
    setError("");
    try {
      await api(`/admin/leave/${id}/approve`, { method: "POST" });
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal menyetujui");
    } finally {
      setActing(null);
    }
  };

  const openReject = (id: string) => {
    setRejectId(id);
    setRejectReason("");
  };

  const submitReject = async () => {
    if (!rejectId) return;
    if (!rejectReason.trim()) {
      setError("Alasan penolakan wajib diisi");
      return;
    }
    setActing(rejectId);
    setError("");
    try {
      await api(`/admin/leave/${rejectId}/reject`, {
        method: "POST",
        body: JSON.stringify({ reason: rejectReason.trim() }),
      });
      setRejectId(null);
      setRejectReason("");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal menolak");
    } finally {
      setActing(null);
    }
  };

  const pendingCount = tab === "pending" ? leaves.length : undefined;

  return (
    <AdminShell
      title="Cuti"
      subtitle="Persetujuan pengajuan cuti karyawan"
      actions={
        <button
          type="button"
          onClick={load}
          className="inline-flex items-center gap-2 rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-600 transition-colors hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
        >
          <Icon name="plus" className="h-4 w-4" />
          Muat ulang
        </button>
      }
    >
      {/* Tabs */}
      <div className="mb-5 flex flex-wrap gap-2">
        {TABS.map((t) => {
          const active = t.key === tab;
          return (
            <button
              key={t.key}
              type="button"
              onClick={() => setTab(t.key)}
              className={`rounded-lg px-4 py-2 text-sm font-semibold transition-colors ${
                active
                  ? "bg-indigo-600 text-white shadow-sm"
                  : "bg-white text-slate-600 ring-1 ring-slate-200 hover:bg-slate-50 dark:bg-slate-900 dark:text-slate-300 dark:ring-slate-800 dark:hover:bg-slate-800"
              }`}
            >
              {t.label}
              {t.key === "pending" && pendingCount ? (
                <span className="ml-2 rounded-full bg-orange-500 px-1.5 py-0.5 text-[11px] font-bold text-white">
                  {pendingCount}
                </span>
              ) : null}
            </button>
          );
        })}
      </div>

      {error ? <ErrorBanner message={error} /> : null}

      {loading ? (
        <Spinner label="Memuat pengajuan cuti..." />
      ) : leaves.length === 0 ? (
        <Card className="py-14 text-center">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-100 text-slate-400 dark:bg-slate-800">
            <Icon name="calendar" className="h-7 w-7" />
          </div>
          <p className="text-sm font-medium text-slate-500 dark:text-slate-400">
            Tidak ada pengajuan cuti pada tab ini.
          </p>
        </Card>
      ) : (
        <div className="grid gap-4">
          {leaves.map((l) => {
            const tm = typeMeta(l.leave_type);
            const sm = STATUS_META[l.status];
            return (
              <Card key={l.id} className="p-5">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <Pill className={tm.cls}>{tm.label}</Pill>
                      <Pill className={sm.cls}>{sm.label}</Pill>
                      <span className="text-sm font-semibold text-slate-700 dark:text-slate-200">
                        {l.total_days} hari
                      </span>
                    </div>
                    <div className="mt-2 text-sm font-semibold text-slate-900 dark:text-slate-100">
                      {l.user_name || l.user_email || l.user_id}
                    </div>
                    <div className="text-xs text-slate-500 dark:text-slate-400">
                      {formatDateRange(l.start_date, l.end_date)}
                    </div>
                  </div>
                  <div className="text-right text-xs text-slate-400">
                    {l.created_at?.slice(0, 10)}
                  </div>
                </div>

                {l.reason ? (
                  <p className="mt-3 rounded-lg bg-slate-50 px-3 py-2 text-sm text-slate-600 dark:bg-slate-800/60 dark:text-slate-300">
                    {l.reason}
                  </p>
                ) : null}

                {l.attachment_url ? (
                  <a
                    href={l.attachment_url}
                    target="_blank"
                    rel="noreferrer"
                    className="mt-2 inline-flex items-center gap-1 text-xs font-medium text-indigo-600 hover:underline dark:text-indigo-400"
                  >
                    <Icon name="plus" className="h-3.5 w-3.5" /> Lihat lampiran
                  </a>
                ) : null}

                {l.status === "rejected" && l.rejection_reason ? (
                  <p className="mt-2 text-xs text-red-600 dark:text-red-400">
                    Alasan tolak: {l.rejection_reason}
                  </p>
                ) : null}

                {l.status === "pending" ? (
                  <div className="mt-4 flex flex-wrap gap-2">
                    <button
                      type="button"
                      disabled={acting === l.id}
                      onClick={() => approve(l.id)}
                      className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-emerald-700 disabled:opacity-60"
                    >
                      <Icon name="check" className="h-4 w-4" />
                      Setujui
                    </button>
                    <button
                      type="button"
                      disabled={acting === l.id}
                      onClick={() => openReject(l.id)}
                      className="inline-flex items-center gap-1.5 rounded-lg bg-red-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-red-700 disabled:opacity-60"
                    >
                      <Icon name="x" className="h-4 w-4" />
                      Tolak
                    </button>
                  </div>
                ) : null}
              </Card>
            );
          })}
        </div>
      )}

      {/* Modal tolak */}
      {rejectId ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
          <div className="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl dark:bg-slate-900">
            <h3 className="text-base font-bold text-slate-900 dark:text-slate-100">Tolak Pengajuan Cuti</h3>
            <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
              Berikan alasan penolakan (wajib).
            </p>
            <textarea
              value={rejectReason}
              onChange={(e) => setRejectReason(e.target.value)}
              rows={3}
              className="mt-3 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 outline-none focus:border-indigo-500 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              placeholder="Mis. Kuota tidak cukup / tanggal bentrok dengan proyek"
            />
            <div className="mt-4 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setRejectId(null)}
                className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={submitReject}
                disabled={acting === rejectId}
                className="rounded-lg bg-red-600 px-4 py-2 text-sm font-semibold text-white hover:bg-red-700 disabled:opacity-60"
              >
                Tolak
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </AdminShell>
  );
}
