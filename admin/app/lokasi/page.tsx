"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api, clearToken, getToken } from "@/lib/api";
import AdminShell from "@/components/admin-shell";
import { Icon } from "@/components/icons";
import { Card, CardHeader, ErrorBanner, Pill, Spinner, inputCls } from "@/components/ui";
import { Select } from "@/components/fields";
import { formatDateTime } from "@/lib/format";

type Location = {
  id: string;
  name: string;
  type: "office" | "client" | "field";
  lat: number;
  lng: number;
  radius_m: number;
  active: boolean;
  created_at?: string;
};

type QrToken = {
  id: string;
  location_id: string;
  location_name?: string;
  active: boolean;
  created_at?: string;
};

type FreshQr = {
  token: string;
  url: string;
  dataUrl: string;
  locationName: string;
};

type LocForm = {
  name: string;
  type: "office" | "client" | "field";
  lat: string;
  lng: string;
  radius_m: string;
  active: boolean;
};

const EMPTY_FORM: LocForm = { name: "", type: "office", lat: "", lng: "", radius_m: "100", active: true };

const TYPE_META: Record<Location["type"], { label: string; cls: string }> = {
  office: { label: "Kantor", cls: "bg-indigo-100 text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-400" },
  client: { label: "Klien", cls: "bg-sky-100 text-sky-700 dark:bg-sky-500/15 dark:text-sky-400" },
  field: { label: "Lapangan", cls: "bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400" },
};

export default function LokasiPage() {
  const router = useRouter();
  const [locs, setLocs] = useState<Location[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  // Modal state: null = tertutup, "add" = tambah, selain itu id lokasi yang diedit
  const [modal, setModal] = useState<string | null>(null);
  const [form, setForm] = useState<LocForm>(EMPTY_FORM);
  const [formError, setFormError] = useState("");
  const [saving, setSaving] = useState(false);
  const [busyId, setBusyId] = useState<string | null>(null);

  // ---- QR management (Fase C) ----
  const [qrTokens, setQrTokens] = useState<QrToken[]>([]);
  const [qrLoading] = useState(false);
  const [qrError, setQrError] = useState("");
  const [qrSuccess, setQrSuccess] = useState("");
  const [qrBusy, setQrBusy] = useState<string | null>(null); // id token / lokasi yang sedang diproses
  const [freshQr, setFreshQr] = useState<FreshQr | null>(null);
  const [qrCache, setQrCache] = useState<Record<string, string>>({}); // location_id -> dataUrl
  const [copied, setCopied] = useState(false);

  const loadQrTokens = useCallback(async () => {
    if (!getToken()) return;
    try {
      const d = await api<{ tokens: QrToken[] }>("/admin/qr");
      setQrTokens(d.tokens || []);
      setQrError("");
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      if (/NOT_FOUND|not found|404/i.test(msg)) {
        setQrError("Fitur QR belum tersedia di server. Silakan coba lagi nanti.");
      } else if (!/401|403/.test(msg)) {
        setQrError(msg);
      }
    }
  }, []);

  useEffect(() => {
    loadQrTokens();
  }, [loadQrTokens]);

  const load = useCallback(async () => {
    if (!getToken()) {
      router.replace("/login");
      return;
    }
    setLoading(true);
    setError("");
    try {
      const d = await api<{ locations: Location[] }>("/admin/locations?all=1");
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

  function openAdd() {
    setForm(EMPTY_FORM);
    setFormError("");
    setModal("add");
  }

  function openEdit(l: Location) {
    setForm({
      name: l.name,
      type: l.type,
      lat: String(l.lat),
      lng: String(l.lng),
      radius_m: String(l.radius_m),
      active: l.active,
    });
    setFormError("");
    setModal(l.id);
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setFormError("");
    setSaving(true);
    try {
      const body = {
        name: form.name.trim(),
        type: form.type,
        lat: Number(form.lat),
        lng: Number(form.lng),
        radius_m: Number(form.radius_m),
        active: form.active,
      };
      if (modal === "add") {
        await api<{ location: Location }>("/admin/locations", {
          method: "POST",
          body: JSON.stringify(body),
        });
        setSuccess("Lokasi baru berhasil ditambahkan");
      } else if (modal) {
        await api<{ location: Location }>(`/admin/locations/${modal}`, {
          method: "PATCH",
          body: JSON.stringify(body),
        });
        setSuccess("Lokasi berhasil diperbarui");
      }
      setModal(null);
      load();
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Gagal menyimpan lokasi");
    } finally {
      setSaving(false);
    }
  }

  async function toggleActive(l: Location) {
    setBusyId(l.id);
    setError("");
    setSuccess("");
    try {
      await api<{ location: Location }>(`/admin/locations/${l.id}`, {
        method: "PATCH",
        body: JSON.stringify({ active: !l.active }),
      });
      setSuccess(l.active ? "Lokasi dinonaktifkan" : "Lokasi diaktifkan kembali");
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal mengubah status lokasi");
    } finally {
      setBusyId(null);
    }
  }

  // ---- QR management handlers ----

  async function generateQr(l: Location) {
    setQrBusy(l.id);
    setQrError("");
    setQrSuccess("");
    setCopied(false);
    try {
      const d = await api<{ token: string; url: string }>("/admin/qr", {
        method: "POST",
        body: JSON.stringify({ location_id: l.id }),
      });
      // Generate image QR dari URL publik (pakai npm qrcode; kalau gagal -> fallback teks URL saja)
      let dataUrl = "";
      try {
        const QRCode = (await import("qrcode")).default;
        dataUrl = await QRCode.toDataURL(d.url, { width: 240, margin: 2 });
      } catch {
        dataUrl = "";
      }
      setQrCache((prev) => ({ ...prev, [l.id]: dataUrl }));
      setFreshQr({ token: d.token, url: d.url, dataUrl, locationName: l.name });
      setQrSuccess(`QR untuk "${l.name}" berhasil dibuat`);
      loadQrTokens();
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setQrError(
        /NOT_FOUND|not found|404/i.test(msg)
          ? "Fitur QR belum tersedia di server. Silakan coba lagi nanti."
          : msg,
      );
    } finally {
      setQrBusy(null);
    }
  }

  async function revokeQr(t: QrToken) {
    setQrBusy(t.id);
    setQrError("");
    setQrSuccess("");
    try {
      await api<{ ok: boolean }>(`/admin/qr/${t.id}`, { method: "DELETE" });
      setQrSuccess("Token QR dicabut (revoke)");
      loadQrTokens();
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setQrError(
        /NOT_FOUND|not found|404/i.test(msg)
          ? "Fitur QR belum tersedia di server. Silakan coba lagi nanti."
          : msg,
      );
    } finally {
      setQrBusy(null);
    }
  }

  async function copyQrUrl(url: string) {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback lama: pilih teks
      setQrError("Gagal menyalin otomatis. Silakan salin URL secara manual.");
    }
  }

  // Tampilkan QR yang sudah dibuat di sesi ini (tanpa URL karena GET /admin/qr tidak mengembalikan token).
  function viewCachedQr(t: QrToken) {
    const cached = qrCache[t.location_id];
    if (!cached) return;
    setFreshQr({
      token: "",
      url: freshQr && freshQr.dataUrl === cached ? freshQr.url : "",
      dataUrl: cached,
      locationName: t.location_name || "Lokasi",
    });
  }

  const total = locs.length;
  const activeCount = locs.filter((l) => l.active).length;

  return (
    <AdminShell title="Lokasi" subtitle="Kelola titik lokasi absensi">
      <ErrorBanner message={error} />
      {success && (
        <div className="mb-6 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm font-medium text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/40 dark:text-emerald-400">
          {success}
        </div>
      )}

      {/* Ringkasan kecil */}
      <div className="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div className="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-100 dark:bg-slate-900 dark:ring-slate-800">
          <div className="text-3xl font-bold text-slate-900 dark:text-slate-100">{total}</div>
          <div className="mt-1 text-xs font-medium text-slate-500 dark:text-slate-400">Total lokasi</div>
        </div>
        <div className="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-100 dark:bg-slate-900 dark:ring-slate-800">
          <div className="text-3xl font-bold text-emerald-600 dark:text-emerald-400">{activeCount}</div>
          <div className="mt-1 text-xs font-medium text-slate-500 dark:text-slate-400">Lokasi aktif</div>
        </div>
        <div className="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-slate-100 dark:bg-slate-900 dark:ring-slate-800">
          <div className="text-3xl font-bold text-slate-400">{total - activeCount}</div>
          <div className="mt-1 text-xs font-medium text-slate-500 dark:text-slate-400">Lokasi nonaktif</div>
        </div>
      </div>

      <Card>
        <CardHeader
          title="Daftar Lokasi"
          subtitle="Tambah, ubah, atau nonaktifkan titik lokasi"
          right={
            <button
              type="button"
              onClick={openAdd}
              className="inline-flex items-center gap-1.5 rounded-lg bg-indigo-600 px-3.5 py-2 text-xs font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700"
            >
              <Icon name="plus" className="h-4 w-4" />
              Tambah Lokasi
            </button>
          }
        />

        <div className="overflow-x-auto">
          {loading ? (
            <Spinner />
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-100 text-left text-xs font-semibold uppercase tracking-wide text-slate-500 dark:border-slate-800 dark:text-slate-400">
                  <th className="px-6 py-3">Nama</th>
                  <th className="px-6 py-3">Tipe</th>
                  <th className="px-6 py-3">Lat / Lng</th>
                  <th className="px-6 py-3 text-right">Radius</th>
                  <th className="px-6 py-3">Status</th>
                  <th className="px-6 py-3 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody>
                {locs.map((l) => (
                  <tr key={l.id} className="border-b border-slate-50 transition-colors hover:bg-slate-50/60 dark:border-slate-800 dark:hover:bg-slate-800/40">
                    <td className="px-6 py-3 font-medium text-slate-900 dark:text-slate-100">{l.name}</td>
                    <td className="px-6 py-3">
                      <Pill className={TYPE_META[l.type].cls}>{TYPE_META[l.type].label}</Pill>
                    </td>
                    <td className="px-6 py-3 font-mono text-xs text-slate-600 dark:text-slate-300">
                      {l.lat.toFixed(4)}, {l.lng.toFixed(4)}
                    </td>
                    <td className="px-6 py-3 text-right text-slate-600 dark:text-slate-300">{l.radius_m} m</td>
                    <td className="px-6 py-3">
                      <Pill
                        className={
                          l.active
                            ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400"
                            : "bg-red-100 text-red-600 dark:bg-red-500/15 dark:text-red-400"
                        }
                      >
                        <span className={`h-1.5 w-1.5 rounded-full ${l.active ? "bg-emerald-500" : "bg-red-500"}`} />
                        {l.active ? "Aktif" : "Nonaktif"}
                      </Pill>
                    </td>
                    <td className="px-6 py-3">
                      <div className="flex items-center justify-end gap-2">
                        <button
                          type="button"
                          disabled={busyId === l.id}
                          onClick={() => openEdit(l)}
                          className="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-3 py-1.5 text-xs font-semibold text-slate-600 transition-colors hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
                        >
                          <Icon name="edit" className="h-3.5 w-3.5" />
                          Edit
                        </button>
                        <button
                          type="button"
                          disabled={busyId === l.id}
                          onClick={() => toggleActive(l)}
                          className={`rounded-lg px-3 py-1.5 text-xs font-semibold transition-colors disabled:opacity-50 ${
                            l.active
                              ? "border border-red-200 text-red-600 hover:bg-red-50 dark:border-red-900/60 dark:text-red-400 dark:hover:bg-red-950/40"
                              : "border border-emerald-200 text-emerald-600 hover:bg-emerald-50 dark:border-emerald-900/60 dark:text-emerald-400 dark:hover:bg-emerald-950/40"
                          }`}
                        >
                          {l.active ? "Nonaktifkan" : "Aktifkan"}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
                {locs.length === 0 && !loading && (
                  <tr>
                    <td colSpan={6} className="px-6 py-10 text-center text-slate-400 dark:text-slate-500">
                      Belum ada data lokasi
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          )}
        </div>
      </Card>

      {/* ===== Manajemen QR Absensi (Fase C) ===== */}
      <div className="mt-8">
        {qrError && (
          <div className="mb-4 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm font-medium text-red-700 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-400">
            {qrError}
          </div>
        )}
        {qrSuccess && (
          <div className="mb-4 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm font-medium text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/40 dark:text-emerald-400">
            {qrSuccess}
          </div>
        )}

        <Card>
          <CardHeader
            title="QR Absensi"
            subtitle="Buat token QR per lokasi aktif untuk absensi publik tanpa login"
          />

          {/* Tombol buat QR per lokasi aktif */}
          <div className="border-b border-slate-100 px-6 py-4 dark:border-slate-800">
            <div className="mb-2 text-[11px] font-semibold uppercase tracking-wide text-slate-400 dark:text-slate-500">
              Lokasi Aktif
            </div>
            {locs.filter((l) => l.active).length === 0 ? (
              <p className="text-xs text-slate-400 dark:text-slate-500">
                Tidak ada lokasi aktif. Aktifkan lokasi terlebih dahulu untuk membuat QR.
              </p>
            ) : (
              <div className="flex flex-wrap gap-2">
                {locs
                  .filter((l) => l.active)
                  .map((l) => (
                    <button
                      key={l.id}
                      type="button"
                      disabled={qrBusy === l.id}
                      onClick={() => generateQr(l)}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-indigo-200 bg-indigo-50 px-3 py-2 text-xs font-semibold text-indigo-700 transition-colors hover:bg-indigo-100 disabled:opacity-50 dark:border-indigo-500/30 dark:bg-indigo-500/10 dark:text-indigo-400 dark:hover:bg-indigo-500/20"
                    >
                      <Icon name="qrcode" className="h-4 w-4" />
                      {qrBusy === l.id ? "Membuat..." : `Buat QR · ${l.name}`}
                    </button>
                  ))}
              </div>
            )}
          </div>

          {/* Daftar token */}
          <div className="overflow-x-auto">
            {qrLoading ? (
              <Spinner label="Memuat token QR..." />
            ) : qrTokens.length === 0 ? (
              <div className="flex flex-col items-center px-6 py-10 text-center">
                <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-slate-100 text-slate-400 dark:bg-slate-800 dark:text-slate-500">
                  <Icon name="qrcode" className="h-6 w-6" />
                </div>
                <p className="mt-3 text-sm font-semibold text-slate-700 dark:text-slate-200">Belum ada token QR</p>
                <p className="mt-1 text-xs text-slate-400 dark:text-slate-500">
                  Klik &quot;Buat QR&quot; pada salah satu lokasi aktif di atas.
                </p>
              </div>
            ) : (
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-slate-100 text-left text-xs font-semibold uppercase tracking-wide text-slate-500 dark:border-slate-800 dark:text-slate-400">
                    <th className="px-6 py-3">Lokasi</th>
                    <th className="px-6 py-3">Status</th>
                    <th className="px-6 py-3">Dibuat</th>
                    <th className="px-6 py-3 text-right">Aksi</th>
                  </tr>
                </thead>
                <tbody>
                  {qrTokens.map((t) => (
                    <tr
                      key={t.id}
                      className="border-b border-slate-50 transition-colors hover:bg-slate-50/60 dark:border-slate-800 dark:hover:bg-slate-800/40"
                    >
                      <td className="px-6 py-3 font-medium text-slate-900 dark:text-slate-100">
                        {t.location_name || t.location_id || "—"}
                      </td>
                      <td className="px-6 py-3">
                        <Pill
                          className={
                            t.active
                              ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400"
                              : "bg-red-100 text-red-600 dark:bg-red-500/15 dark:text-red-400"
                          }
                        >
                          {t.active ? "Aktif" : "Nonaktif"}
                        </Pill>
                      </td>
                      <td className="px-6 py-3 whitespace-nowrap text-slate-600 dark:text-slate-300">
                        {t.created_at ? formatDateTime(t.created_at) : "—"}
                      </td>
                      <td className="px-6 py-3">
                        <div className="flex items-center justify-end gap-2">
                          {qrCache[t.location_id] && (
                            <button
                              type="button"
                              onClick={() => viewCachedQr(t)}
                              className="inline-flex items-center gap-1 rounded-lg border border-slate-200 px-3 py-1.5 text-xs font-semibold text-slate-600 transition-colors hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
                            >
                              <Icon name="qrcode" className="h-3.5 w-3.5" />
                              Lihat QR
                            </button>
                          )}
                          <button
                            type="button"
                            disabled={qrBusy === t.id}
                            onClick={() => revokeQr(t)}
                            className="inline-flex items-center gap-1 rounded-lg border border-red-200 px-3 py-1.5 text-xs font-semibold text-red-600 transition-colors hover:bg-red-50 disabled:opacity-50 dark:border-red-900/60 dark:text-red-400 dark:hover:bg-red-950/40"
                          >
                            {qrBusy === t.id ? "Memproses..." : "Revoke"}
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </Card>
      </div>

      {/* Modal preview QR */}
      {freshQr && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 p-4 dark:bg-slate-950/70">
          <div className="w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl dark:bg-slate-900 dark:ring-1 dark:ring-slate-800">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="text-lg font-bold text-slate-900 dark:text-slate-100">QR Absensi</h3>
              <button
                type="button"
                onClick={() => setFreshQr(null)}
                className="flex h-8 w-8 items-center justify-center rounded-lg text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-200"
              >
                <Icon name="x" className="h-4 w-4" />
              </button>
            </div>
            <p className="mb-4 text-sm text-slate-600 dark:text-slate-300">
              QR untuk <span className="font-semibold text-slate-900 dark:text-slate-100">{freshQr.locationName}</span>.
              Karyawan memindai QR ini lalu mengisi form absensi publik.
            </p>
            <div className="flex justify-center rounded-2xl border border-slate-100 bg-white p-4 dark:border-slate-800">
              {freshQr.dataUrl ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={freshQr.dataUrl} alt={`QR Absensi ${freshQr.locationName}`} className="h-48 w-48" />
              ) : (
                <div className="flex h-48 w-48 items-center justify-center rounded-xl bg-slate-50 text-xs text-slate-400 dark:bg-slate-800 dark:text-slate-500">
                  Gambar QR tidak tersedia
                </div>
              )}
            </div>
            <div className="mt-4">
              <div className="mb-1 text-[11px] font-semibold uppercase tracking-wide text-slate-400 dark:text-slate-500">
                URL Publik
              </div>
              <div className="flex items-center gap-2">
                <input
                  readOnly
                  value={freshQr.url}
                  onFocus={(e) => e.target.select()}
                  placeholder={freshQr.url ? "" : "URL tidak tersedia untuk token lama"}
                  className="w-full rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 font-mono text-xs text-slate-600 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
                />
                <button
                  type="button"
                  disabled={!freshQr.url}
                  onClick={() => copyQrUrl(freshQr.url)}
                  className="inline-flex shrink-0 items-center gap-1 rounded-lg bg-indigo-600 px-3 py-2 text-xs font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Icon name={copied ? "check" : "copy"} className="h-3.5 w-3.5" />
                  {copied ? "Tersalin" : "Salin"}
                </button>
              </div>
            </div>
            <div className="mt-5 flex justify-end">
              <button
                type="button"
                onClick={() => setFreshQr(null)}
                className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
              >
                Tutup
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Modal tambah/edit lokasi */}
      {modal !== null && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 p-4 dark:bg-slate-950/70">
          <div className="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl dark:bg-slate-900 dark:ring-1 dark:ring-slate-800">
            <div className="mb-4 flex items-center justify-between">
              <h3 className="text-lg font-bold text-slate-900 dark:text-slate-100">
                {modal === "add" ? "Tambah Lokasi" : "Edit Lokasi"}
              </h3>
              <button
                type="button"
                onClick={() => setModal(null)}
                className="flex h-8 w-8 items-center justify-center rounded-lg text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-200"
              >
                <Icon name="x" className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={submit} className="space-y-4">
              <div>
                <label htmlFor="l-name" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                  Nama Lokasi
                </label>
                <input
                  id="l-name"
                  required
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  className={inputCls}
                  placeholder="cth: Kantor Pusat"
                />
              </div>
              <div>
                <label htmlFor="l-type" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                  Tipe
                </label>
                <Select
                  id="l-type"
                  value={form.type}
                  onChange={(e) => setForm({ ...form, type: e.target.value as LocForm["type"] })}
                  options={[
                    { value: "office", label: "Kantor" },
                    { value: "client", label: "Klien" },
                    { value: "field", label: "Lapangan" },
                  ]}
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label htmlFor="l-lat" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                    Latitude
                  </label>
                  <input
                    id="l-lat"
                    required
                    type="number"
                    step="any"
                    value={form.lat}
                    onChange={(e) => setForm({ ...form, lat: e.target.value })}
                    className={inputCls}
                    placeholder="-6.2088"
                  />
                </div>
                <div>
                  <label htmlFor="l-lng" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                    Longitude
                  </label>
                  <input
                    id="l-lng"
                    required
                    type="number"
                    step="any"
                    value={form.lng}
                    onChange={(e) => setForm({ ...form, lng: e.target.value })}
                    className={inputCls}
                    placeholder="106.8456"
                  />
                </div>
              </div>
              <div>
                <label htmlFor="l-radius" className="mb-1.5 block text-sm font-medium text-slate-800 dark:text-slate-200">
                  Radius (meter)
                </label>
                <input
                  id="l-radius"
                  required
                  type="number"
                  min={1}
                  value={form.radius_m}
                  onChange={(e) => setForm({ ...form, radius_m: e.target.value })}
                  className={inputCls}
                />
              </div>
              <label className="flex cursor-pointer items-center gap-3">
                <input
                  type="checkbox"
                  checked={form.active}
                  onChange={(e) => setForm({ ...form, active: e.target.checked })}
                  className="h-4 w-4 rounded accent-indigo-600"
                />
                <span className="text-sm font-medium text-slate-800 dark:text-slate-200">Lokasi aktif</span>
              </label>

              {formError && (
                <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm font-medium text-red-700 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-400">
                  {formError}
                </div>
              )}

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setModal(null)}
                  className="rounded-lg border border-slate-300 px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
                >
                  Batal
                </button>
                <button
                  type="submit"
                  disabled={saving}
                  className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {saving ? "Menyimpan..." : "Simpan"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </AdminShell>
  );
}
