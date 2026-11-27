"use client";

/* Editor akses menu per user.
 *
 * Model yang dipakai (lihat internal/authz): template role + override per user.
 * Tiap aksi punya tiga status: IKUT ROLE (inherit), IZINKAN (allow), TOLAK (deny).
 * Deny selalu menang. "Akses efektif" dihitung di klien HANYA untuk tampilan —
 * backend tetap yang menentukan (GET/PATCH /admin/users/{id}/permissions).
 */
import { useCallback, useEffect, useMemo, useState } from "react";

import { Icon } from "@/components/icons";
import { Spinner } from "@/components/ui";
import { api } from "@/lib/api";

type Item = {
  permission: string;
  menu_key: string;
  menu_label: string;
  action: string;
  role_granted: boolean;
  override: string; // "" | "allow" | "deny"
  effective: boolean;
};

type Access = {
  user_id: string;
  name: string;
  email: string;
  role: string;
  items: Item[];
  permissions: string[];
};

const ACTION_LABEL: Record<string, string> = {
  view: "Lihat",
  create: "Tambah",
  update: "Ubah",
  delete: "Hapus",
  export: "Unduh",
  approve: "Setujui",
  override: "Override",
};

const ROLE_LABEL: Record<string, string> = {
  super_admin: "Super Admin",
  hr: "HR",
  supervisor: "Supervisor",
  employee: "Karyawan",
};

export default function MenuAccessModal({
  userId,
  onClose,
}: {
  userId: string;
  onClose: () => void;
}) {
  const [access, setAccess] = useState<Access | null>(null);
  const [draft, setDraft] = useState<Record<string, string>>({});
  const [original, setOriginal] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const d = await api<{ access: Access }>(`/admin/users/${userId}/permissions`);
      setAccess(d.access);
      const map: Record<string, string> = {};
      d.access.items.forEach((it) => {
        map[it.permission] = it.override;
      });
      setOriginal(map);
      setDraft(map);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal memuat akses");
    } finally {
      setLoading(false);
    }
  }, [userId]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- pemuatan data disengaja: effect memanggil loader yang menyalakan state (pola sama dengan PR #22).
    load();
  }, [load]);

  const menus = useMemo(() => {
    if (!access) return [];
    const groups = new Map<string, { key: string; label: string; items: Item[] }>();
    access.items.forEach((it) => {
      const g = groups.get(it.menu_key) ?? { key: it.menu_key, label: it.menu_label, items: [] };
      g.items.push(it);
      groups.set(it.menu_key, g);
    });
    return [...groups.values()];
  }, [access]);

  const effectiveCount = useMemo(() => {
    if (!access) return 0;
    return access.items.filter((it) => {
      const ov = draft[it.permission] ?? "";
      if (ov === "deny") return false;
      if (ov === "allow") return true;
      return it.role_granted;
    }).length;
  }, [access, draft]);

  const changed = useMemo(
    () => Object.keys(draft).filter((k) => (draft[k] ?? "") !== (original[k] ?? "")),
    [draft, original]
  );

  const setEffect = (permission: string, effect: string) => {
    setNotice("");
    setDraft((prev) => ({ ...prev, [permission]: effect }));
  };

  const resetMenu = (items: Item[]) => {
    setNotice("");
    setDraft((prev) => {
      const next = { ...prev };
      items.forEach((it) => {
        next[it.permission] = "";
      });
      return next;
    });
  };

  const save = async () => {
    setSaving(true);
    setError("");
    setNotice("");
    try {
      const changes = changed.map((p) => ({ permission: p, effect: draft[p] ?? "" }));
      const d = await api<{ access: Access }>(`/admin/users/${userId}/permissions`, {
        method: "PATCH",
        body: JSON.stringify({ changes }),
      });
      setAccess(d.access);
      const map: Record<string, string> = {};
      d.access.items.forEach((it) => {
        map[it.permission] = it.override;
      });
      setOriginal(map);
      setDraft(map);
      setNotice("Akses menu tersimpan.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal menyimpan akses");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-slate-900/50 p-4 py-10 backdrop-blur-sm">
      <div className="w-full max-w-3xl rounded-2xl bg-white shadow-xl dark:bg-slate-900">
        <div className="flex items-start justify-between gap-4 border-b border-slate-100 px-6 py-5 dark:border-slate-800">
          <div>
            <h3 className="text-lg font-bold text-slate-900 dark:text-slate-100">Akses Menu</h3>
            <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
              {access ? `${access.name || access.email} · role ${ROLE_LABEL[access.role] ?? access.role}` : "Memuat…"}
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg p-2 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-200"
            aria-label="Tutup"
          >
            <Icon name="x" className="h-5 w-5" />
          </button>
        </div>

        <div className="px-6 py-5">
          {loading ? (
            <div className="flex items-center gap-3 py-10 text-sm text-slate-500">
              <Spinner /> Memuat akses…
            </div>
          ) : (
            <>
              <div className="mb-4 flex flex-wrap items-center gap-3 rounded-xl bg-slate-50 px-4 py-3 text-sm dark:bg-slate-800/60">
                <span className="font-semibold text-slate-700 dark:text-slate-200">
                  Akses efektif: {effectiveCount} izin
                </span>
                <span className="text-slate-500 dark:text-slate-400">
                  {changed.length > 0 ? `${changed.length} perubahan belum disimpan` : "Belum ada perubahan"}
                </span>
              </div>

              {error && (
                <div className="mb-4 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-400">
                  {error}
                </div>
              )}
              {notice && (
                <div className="mb-4 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/40 dark:text-emerald-400">
                  {notice}
                </div>
              )}

              <div className="space-y-3">
                {menus.map((menu) => (
                  <div
                    key={menu.key}
                    className="rounded-xl border border-slate-200 px-4 py-3 dark:border-slate-700"
                  >
                    <div className="flex items-center justify-between gap-3">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-semibold text-slate-800 dark:text-slate-100">
                          {menu.label}
                        </span>
                        <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-slate-500 dark:bg-slate-800 dark:text-slate-400">
                          {menu.key}
                        </span>
                      </div>
                      <button
                        type="button"
                        onClick={() => resetMenu(menu.items)}
                        className="text-xs font-semibold text-indigo-600 hover:underline dark:text-indigo-400"
                      >
                        Kembalikan ke role
                      </button>
                    </div>

                    <div className="mt-3 space-y-2">
                      {menu.items.map((it) => {
                        const ov = draft[it.permission] ?? "";
                        const isDeny = ov === "deny";
                        const effective = isDeny ? false : ov === "allow" ? true : it.role_granted;
                        return (
                          <div
                            key={it.permission}
                            className="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-slate-50/70 px-3 py-2 dark:bg-slate-800/40"
                          >
                            <div className="flex items-center gap-2 text-sm">
                              <span
                                className={`inline-flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-bold ${
                                  effective
                                    ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-400"
                                    : "bg-slate-200 text-slate-500 dark:bg-slate-700 dark:text-slate-400"
                                }`}
                              >
                                {effective ? "✓" : "–"}
                              </span>
                              <span className="font-medium text-slate-700 dark:text-slate-200">
                                {ACTION_LABEL[it.action] ?? it.action}
                              </span>
                              {ov !== "" && (
                                <span
                                  className={`rounded-full px-2 py-0.5 text-[10px] font-bold uppercase ${
                                    ov === "deny"
                                      ? "bg-red-100 text-red-700 dark:bg-red-500/20 dark:text-red-400"
                                      : "bg-indigo-100 text-indigo-700 dark:bg-indigo-500/20 dark:text-indigo-300"
                                  }`}
                                >
                                  override {ov}
                                </span>
                              )}
                              {ov === "" && (
                                <span className="text-[11px] text-slate-400 dark:text-slate-500">
                                  {it.role_granted ? "dari role" : "tidak diberikan role"}
                                </span>
                              )}
                            </div>

                            <div className="flex items-center gap-1">
                              {[
                                { key: "", label: "Ikut role" },
                                { key: "allow", label: "Izinkan" },
                                { key: "deny", label: "Tolak" },
                              ].map((opt) => {
                                const active = ov === opt.key;
                                return (
                                  <button
                                    key={opt.key}
                                    type="button"
                                    onClick={() => setEffect(it.permission, opt.key)}
                                    className={`rounded-lg px-2.5 py-1 text-[11px] font-semibold transition-colors ${
                                      active
                                        ? opt.key === "deny"
                                          ? "bg-red-600 text-white"
                                          : opt.key === "allow"
                                            ? "bg-indigo-600 text-white"
                                            : "bg-slate-700 text-white"
                                        : "border border-slate-200 text-slate-600 hover:bg-white dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
                                    }`}
                                  >
                                    {opt.label}
                                  </button>
                                );
                              })}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>

        <div className="flex items-center justify-end gap-2 border-t border-slate-100 px-6 py-4 dark:border-slate-800">
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg border border-slate-200 px-4 py-2 text-sm font-semibold text-slate-600 transition-colors hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
          >
            Tutup
          </button>
          <button
            type="button"
            disabled={saving || changed.length === 0}
            onClick={save}
            className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-indigo-700 disabled:opacity-50"
          >
            {saving ? "Menyimpan…" : "Simpan akses"}
          </button>
        </div>
      </div>
    </div>
  );
}
