"use client";

/* Halaman Approval (Fase C): daftar absensi yang menunggu persetujuan supervisor/HR,
 * dengan aksi Setujui / Tolak (alasan wajib saat menolak) dan filter tab status. */

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api, clearToken, getToken, mediaURL } from "@/lib/api";
import AdminShell from "@/components/admin-shell";
import { Icon } from "@/components/icons";
import { Card, ErrorBanner, Pill, Spinner } from "@/components/ui";
import { formatDateTime } from "@/lib/format";

type Approval = {
  id: string;
  user_id: string;
  user_name: string;
  user_email: string;
  type: "check_in" | "check_out";
  status: "on_time" | "late";
  source?: string;
  photo_url?: string;
  address?: string;
  distance_m?: number | null;
  location_id?: string;
  occurred_at: string;
};

type StatusTab = "pending" | "approved" | "rejected";

const TABS: { key: StatusTab; label: string }[] = [
  { key: "pending", label: "Menunggu" },
  { key: "approved", label: "Disetujui" },
  { key: "rejected", label: "Ditolak" },
];

const TYPE_META: Record<Approval["type"], { label: string; cls: string }> = {
  check_in: { label: "Masuk", cls: "bg-blue-100 text-blue-700 dark:bg-blue-500/15 dark:text-blue-400" },
  check_out: { label: "Keluar", cls: "bg-violet-100 text-violet-700 dark:bg-violet-500/15 dark:text-violet-400" },
};

const STATUS_TIME_META: Record<Approval["status"], { label: string; cls: string }> = {
  on_time: { label: "Tepat", cls: "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400" },
  late: { label: "Terlambat", cls: "bg-orange-100 text-orange-700 dark:bg-orange-500/15 dark:text-orange-400" },
};

const SOURCE_META: Record<string, { label: string; cls: string }> = {
  qr: { label: "QR", cls: "bg-sky-100 text-sky-700 dark:bg-sky-500/15 dark:text-sky-400" },
  manual: { label: "Manual", cls: "bg-slate-200 text-slate-600 dark:bg-slate-700 dark:text-slate-300" },
  app: { label: "Aplikasi", cls: "bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400" },
};

// distance_m -> "662 km" / "0 m"
function formatDistance(m?: number | null): string {
  if (m === undefined || m === null || Number.isNaN(m)) return "—";
  if (m >= 1000) {
    const km = m / 1000;
    return `${km % 1 === 0 ? km.toFixed(0) : km.toFixed(1).replace(/\.0$/, "")} km`;
  }
  return `${Math.round(m)} m`;
}

// Pesan ramah kalau endpoint backend Fase C belum tersedia (404 NOT_FOUND).
function friendlyError(err: unknown, fallback: string): string {
  const msg = err instanceof Error ? err.message : String(err);
  if (/NOT_FOUND|not found|404/i.test(msg)) return "Fitur ini belum tersedia di server. Silakan coba lagi nanti.";
  return msg || fallback;
}

export default function ApprovalPage() {
  const router = useRouter();
  const [tab, setTab] = useState<StatusTab>("pending");
  const [rows, setRows] = useState<Approval[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  // Modal tolak
  const [rejectRow, setRejectRow] = useState<Approval | null>(null);
  const [reason, setReason] = useState("");
  const [rejectError, setRejectError] = useState("");
  const [acting, setActing] = useState(false);
  const [busyId, setBusyId] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!getToken()) {
      router.replace("/login");
      return;
    }
    setLoading(true);
    setError("");
    try {
      const d = await api<{ approvals: Approval[]; total: number }>(
        `/admin/approvals?status=${tab}&limit=100&offset=0`,
      );
      setRows(d.approvals || []);
      setTotal(d.total ?? (d.approvals || []).length);
    } catch (err) {
      setError(friendlyError(err, "Gagal memuat daftar approval"));
      setRows([]);
      setTotal(0);
      if (err instanceof Error && /401|403/.test(err.message)) {
        clearToken();
        router.replace("/login");
      }
    } finally {
      setLoading(false);
    }
  }, [router, tab]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- pemuatan data disengaja: effect memanggil loader yang menyalakan state loading.
    load();
  }, [load]);

  function openReject(a: Approval) {
    setRejectRow(a);
    setReason("");
    setRejectError("");
  }

  async function doApprove(a: Approval) {
    setBusyId(a.id);
    setError("");
    setSuccess("");
    try {
      await api(`/admin/approvals/${a.id}`, {
        method: "POST",
        body: JSON.stringify({ action: "approve" }),
      });
      setSuccess(`Absensi ${a.user_name || "karyawan"} disetujui`);
      load();
    } catch (err) {
      setError(friendlyError(err, "Gagal menyetujui absensi"));
    } finally {
      setBusyId(null);
    }
  }

  async function submitReject(e: React.FormEvent) {
    e.preventDefault();
    if (!rejectRow) return;
    setRejectError("");
    if (!reason.trim()) {
      setRejectError("Alasan wajib diisi untuk menolak absensi");
      return;
    }
    setActing(true);
    try {
      await api(`/admin/approvals/${rejectRow.id}`, {
        method: "POST",
        body: JSON.stringify({ action: "reject", reason: reason.trim() }),
      });
      setSuccess(`Absensi ${rejectRow.user_name || "karyawan"} ditolak`);
      setRejectRow(null);
      setReason("");
      load();
    } catch (err) {
      setRejectError(friendlyError(err, "Gagal menolak absensi"));
    } finally {
      setActing(false);
    }
  }

  return (
    <AdminShell title="Approval" subtitle="Tinjau dan setujui absensi yang menunggu persetujuan">
      <ErrorBanner message={error} />
      {success && (
        <div className="mb-6 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm font-medium text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/40 dark:text-emerald-400">
          {success}
        </div>
      )}

      <Card>
        {/* Filter tab */}
        <div className="flex flex-wrap items-center gap-1 border-b border-slate-100 px-6 pt-4 dark:border-slate-800">
          {TABS.map((t) => {
            const active = tab === t.key;
            return (
              <button
                key={t.key}
                type="button"
                onClick={() => setTab(t.key)}
                className={`rounded-t-lg border-b-2 px-4 py-2.5 text-sm font-semibold transition-colors ${
                  active
                    ? "border-indigo-600 text-indigo-700 dark:border-indigo-400 dark:text-indigo-400"
                    : "border-transparent text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200"
                }`}
              >
                {t.label}
              </button>
            );
          })}
          <span className="ml-auto hidden pb-1 text-xs text-slate-400 dark:text-slate-500 sm:block">
            {total} catatan
          </span>
        </div>

        <div className="overflow-x-auto">
          {loading ? (
            <Spinner label="Memuat approval..." />
          ) : rows.length === 0 ? (
            <div className="flex flex-col items-center px-6 py-14 text-center">
              <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-100 text-slate-400 dark:bg-slate-800 dark:text-slate-500">
                <Icon name="check-list" className="h-7 w-7" />
              </div>
              <p className="mt-4 text-sm font-semibold text-slate-700 dark:text-slate-200">
                Tidak ada {TABS.find((t) => t.key === tab)?.label.toLowerCase()} approval
              </p>
              <p className="mt-1 text-xs text-slate-400 dark:text-slate-500">
                {tab === "pending"
                  ? "Semua absensi sudah ditinjau. Keren!"
                  : "Belum ada riwayat approval pada status ini."}
              </p>
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-100 text-left text-xs font-semibold uppercase tracking-wide text-slate-500 dark:border-slate-800 dark:text-slate-400">
                  <th className="px-6 py-3">Pengguna</th>
                  <th className="px-6 py-3">Tipe</th>
                  <th className="px-6 py-3">Status Waktu</th>
                  <th className="px-6 py-3">Source</th>
                  <th className="px-6 py-3">Foto</th>
                  <th className="px-6 py-3">Jarak</th>
                  <th className="px-6 py-3">Waktu</th>
                  <th className="px-6 py-3 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((a) => {
                  const tm = TYPE_META[a.type] || TYPE_META.check_in;
                  const st = STATUS_TIME_META[a.status] || STATUS_TIME_META.on_time;
                  const src = SOURCE_META[a.source || "app"] || SOURCE_META.app;
                  const photo = mediaURL(a.photo_url);
                  return (
                    <tr
                      key={a.id}
                      className="border-b border-slate-50 transition-colors hover:bg-slate-50/60 dark:border-slate-800 dark:hover:bg-slate-800/40"
                    >
                      <td className="px-6 py-3">
                        <div className="font-medium text-slate-900 dark:text-slate-100">{a.user_name || "—"}</div>
                        <div className="truncate text-xs text-slate-500 dark:text-slate-400">{a.user_email}</div>
                      </td>
                      <td className="px-6 py-3">
                        <Pill className={tm.cls}>{tm.label}</Pill>
                      </td>
                      <td className="px-6 py-3">
                        <Pill className={st.cls}>{st.label}</Pill>
                      </td>
                      <td className="px-6 py-3">
                        <Pill className={src.cls}>{src.label}</Pill>
                      </td>
                      <td className="px-6 py-3">
                        {photo ? (
                          // eslint-disable-next-line @next/next/no-img-element
                          <img
                            src={photo}
                            alt="Foto absensi"
                            className="h-10 w-10 rounded-lg object-cover ring-1 ring-slate-200 dark:ring-slate-700"
                          />
                        ) : (
                          <span className="text-xs text-slate-300 dark:text-slate-600">—</span>
                        )}
                      </td>
                      <td className="px-6 py-3 whitespace-nowrap text-slate-600 dark:text-slate-300">
                        {formatDistance(a.distance_m)}
                      </td>
                      <td className="px-6 py-3 whitespace-nowrap text-slate-600 dark:text-slate-300">
                        {formatDateTime(a.occurred_at)}
                      </td>
                      <td className="px-6 py-3">
                        <div className="flex items-center justify-end gap-2">
                          {tab === "pending" ? (
                            <>
                              <button
                                type="button"
                                disabled={busyId === a.id}
                                onClick={() => doApprove(a)}
                                className="inline-flex items-center gap-1 rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-semibold text-white shadow-sm transition-colors hover:bg-emerald-700 disabled:opacity-50"
                              >
                                <Icon name="check" className="h-3.5 w-3.5" />
                                Setujui
                              </button>
                              <button
                                type="button"
                                disabled={busyId === a.id}
                                onClick={() => openReject(a)}
                                className="inline-flex items-center gap-1 rounded-lg border border-red-200 px-3 py-1.5 text-xs font-semibold text-red-600 transition-colors hover:bg-red-50 disabled:opacity-50 dark:border-red-900/60 dark:text-red-400 dark:hover:bg-red-950/40"
                              >
                                <Icon name="x" className="h-3.5 w-3.5" />
                                Tolak
                              </button>
                            </>
                          ) : (
                            <span className="text-xs text-slate-300 dark:text-slate-600">—</span>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
        </div>
      </Card>

      {/* Modal Tolak (alasan wajib) */}
      {rejectRow && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 p-4 dark:bg-slate-950/70">
          <div className="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl dark:bg-slate-900 dark:ring-1 dark:ring-slate-800">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="text-lg font-bold text-slate-900 dark:text-slate-100">Tolak Absensi</h3>
              <button
                type="button"
                onClick={() => setRejectRow(null)}
                className="flex h-8 w-8 items-center justify-center rounded-lg text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-200"
              >
                <Icon name="x" className="h-4 w-4" />
              </button>
            </div>
            <div className="mb-4 rounded-xl bg-slate-50 px-4 py-3 text-sm dark:bg-slate-800">
              <span className="font-semibold text-slate-900 dark:text-slate-100">{rejectRow.user_name || "—"}</span>
              <span className="text-slate-500 dark:text-slate-400"> · {rejectRow.user_email}</span>
            </div>

            <form onSubmit={submitReject} className="space-y-4">
              <div>
                <label htmlFor="reject-reason" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                  Alasan Penolakan <span className="text-red-500">*</span>
                </label>
                <textarea
                  id="reject-reason"
                  rows={4}
                  required
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  className="w-full resize-none rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100 dark:placeholder:text-slate-500 dark:focus:border-indigo-500 dark:focus:ring-indigo-500/20"
                  placeholder="cth: Foto tidak sesuai, lokasi di luar radius..."
                />
              </div>

              {rejectError && (
                <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm font-medium text-red-700 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-400">
                  {rejectError}
                </div>
              )}

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setRejectRow(null)}
                  className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
                >
                  Batal
                </button>
                <button
                  type="submit"
                  disabled={acting}
                  className="rounded-lg bg-red-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {acting ? "Menyimpan..." : "Tolak Absensi"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </AdminShell>
  );
}
