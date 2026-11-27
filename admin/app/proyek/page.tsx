"use client";

/* Halaman Proyek & Mitra: master data yang memisahkan karyawan internal dari
 * kru partner (lepas), plus penugasan per nomor proyek/nama kapal.
 * Akses: super_admin / hr. Penugasan = "riwayat penugasan" yang juga tampil di app. */

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import AdminShell from "@/components/admin-shell";
import { Card, CardHeader, ErrorBanner, Pill, Spinner } from "@/components/ui";

type Partner = { id: string; name: string; contact_name?: string; contact_phone?: string; active: boolean };
type Project = {
  id: string;
  code: string;
  name: string;
  vessel_name?: string;
  client_name?: string;
  status: "active" | "done" | "cancelled";
  start_date?: string;
  end_date?: string;
};
type Assignment = {
  id: string;
  user_id: string;
  user_name?: string;
  user_email?: string;
  project_id: string;
  project_code?: string;
  project_name?: string;
  vessel_name?: string;
  partner_id?: string;
  partner_name?: string;
  role?: string;
  start_date: string;
  end_date?: string;
  employment: "internal" | "partner";
};
type AdminUser = { id: string; name?: string; email: string };

type Tab = "projects" | "partners" | "assignments";

const TABS: { key: Tab; label: string }[] = [
  { key: "projects", label: "Proyek / Kapal" },
  { key: "partners", label: "Mitra Kru" },
  { key: "assignments", label: "Penugasan" },
];

const STATUS_CLS: Record<string, string> = {
  active: "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400",
  done: "bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300",
  cancelled: "bg-rose-100 text-rose-700 dark:bg-rose-500/15 dark:text-rose-400",
};

const input =
  "w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-800 outline-none focus:border-indigo-400 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100";

export default function ProyekPage() {
  const [tab, setTab] = useState<Tab>("projects");
  const [projects, setProjects] = useState<Project[]>([]);
  const [partners, setPartners] = useState<Partner[]>([]);
  const [assignments, setAssignments] = useState<Assignment[]>([]);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  // project form
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [vessel, setVessel] = useState("");
  const [client, setClient] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  // partner form
  const [partnerName, setPartnerName] = useState("");
  const [partnerContact, setPartnerContact] = useState("");
  // assignment form
  const [aUser, setAUser] = useState("");
  const [aProject, setAProject] = useState("");
  const [aPartner, setAPartner] = useState("");
  const [aRole, setARole] = useState("");
  const [aStart, setAStart] = useState("");
  const [aEnd, setAEnd] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [p, pt, as, u] = await Promise.all([
        api<{ projects: Project[] }>("/admin/projects"),
        api<{ partners: Partner[] }>("/admin/partners"),
        api<{ assignments: Assignment[] }>("/admin/assignments"),
        api<{ users: AdminUser[] }>("/admin/users?status=active"),
      ]);
      setProjects(p.projects ?? []);
      setPartners(pt.partners ?? []);
      setAssignments(as.assignments ?? []);
      setUsers(u.users ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal memuat master data");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- pemuatan data disengaja: effect memanggil loader yang menyalakan state loading.
    void load();
  }, [load]);

  const submitProject = async () => {
    setSaving(true);
    setError("");
    try {
      await api("/admin/projects", {
        method: "POST",
        body: JSON.stringify({
          code,
          name,
          vessel_name: vessel,
          client_name: client,
          start_date: startDate,
          end_date: endDate,
        }),
      });
      setCode(""); setName(""); setVessel(""); setClient(""); setStartDate(""); setEndDate("");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal menyimpan proyek");
    } finally {
      setSaving(false);
    }
  };

  const submitPartner = async () => {
    setSaving(true);
    setError("");
    try {
      await api("/admin/partners", {
        method: "POST",
        body: JSON.stringify({ name: partnerName, contact_name: partnerContact }),
      });
      setPartnerName(""); setPartnerContact("");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal menyimpan mitra");
    } finally {
      setSaving(false);
    }
  };

  const submitAssignment = async () => {
    setSaving(true);
    setError("");
    try {
      await api("/admin/assignments", {
        method: "POST",
        body: JSON.stringify({
          user_id: aUser,
          project_id: aProject,
          partner_id: aPartner,
          role: aRole,
          start_date: aStart,
          end_date: aEnd,
        }),
      });
      setAUser(""); setAProject(""); setAPartner(""); setARole(""); setAStart(""); setAEnd("");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal menyimpan penugasan");
    } finally {
      setSaving(false);
    }
  };

  return (
    <AdminShell title="Proyek & Mitra" subtitle="Nomor proyek / kapal, kru partner, dan riwayat penugasan">
      <ErrorBanner message={error} />

      <div className="mb-5 flex flex-wrap gap-2">
        {TABS.map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`rounded-full px-4 py-1.5 text-sm font-medium transition ${
              tab === t.key
                ? "bg-indigo-600 text-white"
                : "bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {loading ? (
        <Spinner />
      ) : tab === "projects" ? (
        <div className="grid gap-5 lg:grid-cols-[1fr_1.4fr]">
          <Card>
            <CardHeader title="Proyek baru" subtitle="Nomor proyek/SPK unik" />
            <div className="space-y-3">
              <input className={input} placeholder="Nomor proyek (SPK-2026-001)" value={code} onChange={(e) => setCode(e.target.value)} />
              <input className={input} placeholder="Nama pekerjaan" value={name} onChange={(e) => setName(e.target.value)} />
              <input className={input} placeholder="Nama kapal" value={vessel} onChange={(e) => setVessel(e.target.value)} />
              <input className={input} placeholder="Klien" value={client} onChange={(e) => setClient(e.target.value)} />
              <div className="grid grid-cols-2 gap-2">
                <input className={input} type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} />
                <input className={input} type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} />
              </div>
              <button
                disabled={saving || !code.trim() || !name.trim()}
                onClick={() => void submitProject()}
                className="w-full rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-700 disabled:opacity-50"
              >
                Simpan proyek
              </button>
            </div>
          </Card>

          <Card>
            <CardHeader title="Daftar proyek" subtitle={`${projects.length} proyek`} />
            <div className="divide-y divide-slate-100 dark:divide-slate-800">
              {projects.length === 0 ? (
                <p className="py-6 text-center text-sm text-slate-400">Belum ada proyek.</p>
              ) : (
                projects.map((p) => (
                  <div key={p.id} className="flex items-center justify-between gap-3 py-3">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-semibold text-slate-800 dark:text-slate-100">
                        {p.code} · {p.name}
                      </p>
                      <p className="text-xs text-slate-500 dark:text-slate-400">
                        {[p.vessel_name, p.client_name, p.start_date && `${p.start_date}${p.end_date ? `–${p.end_date}` : ""}`]
                          .filter(Boolean)
                          .join(" · ") || "—"}
                      </p>
                    </div>
                    <Pill className={STATUS_CLS[p.status]}>{p.status}</Pill>
                  </div>
                ))
              )}
            </div>
          </Card>
        </div>
      ) : tab === "partners" ? (
        <div className="grid gap-5 lg:grid-cols-[1fr_1.4fr]">
          <Card>
            <CardHeader title="Mitra baru" subtitle="Vendor kru lepas" />
            <div className="space-y-3">
              <input className={input} placeholder="Nama mitra" value={partnerName} onChange={(e) => setPartnerName(e.target.value)} />
              <input className={input} placeholder="Kontak (nama)" value={partnerContact} onChange={(e) => setPartnerContact(e.target.value)} />
              <button
                disabled={saving || !partnerName.trim()}
                onClick={() => void submitPartner()}
                className="w-full rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-700 disabled:opacity-50"
              >
                Simpan mitra
              </button>
            </div>
          </Card>
          <Card>
            <CardHeader title="Daftar mitra" subtitle={`${partners.length} mitra`} />
            <div className="divide-y divide-slate-100 dark:divide-slate-800">
              {partners.length === 0 ? (
                <p className="py-6 text-center text-sm text-slate-400">Belum ada mitra.</p>
              ) : (
                partners.map((p) => (
                  <div key={p.id} className="flex items-center justify-between py-3">
                    <div>
                      <p className="text-sm font-semibold text-slate-800 dark:text-slate-100">{p.name}</p>
                      <p className="text-xs text-slate-500 dark:text-slate-400">{p.contact_name || "—"}</p>
                    </div>
                    <Pill className={p.active ? STATUS_CLS.active : STATUS_CLS.done}>
                      {p.active ? "aktif" : "nonaktif"}
                    </Pill>
                  </div>
                ))
              )}
            </div>
          </Card>
        </div>
      ) : (
        <div className="grid gap-5 lg:grid-cols-[1fr_1.4fr]">
          <Card>
            <CardHeader title="Penugasan baru" subtitle="Internal atau kru mitra" />
            <div className="space-y-3">
              <select className={input} value={aUser} onChange={(e) => setAUser(e.target.value)}>
                <option value="">Pilih karyawan/kru…</option>
                {users.map((u) => (
                  <option key={u.id} value={u.id}>
                    {u.name || u.email}
                  </option>
                ))}
              </select>
              <select className={input} value={aProject} onChange={(e) => setAProject(e.target.value)}>
                <option value="">Pilih proyek…</option>
                {projects.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.code} · {p.name}
                  </option>
                ))}
              </select>
              <select className={input} value={aPartner} onChange={(e) => setAPartner(e.target.value)}>
                <option value="">Internal (tanpa mitra)</option>
                {partners.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
              <input className={input} placeholder="Peran (diver, NDT, rigger…)" value={aRole} onChange={(e) => setARole(e.target.value)} />
              <div className="grid grid-cols-2 gap-2">
                <input className={input} type="date" value={aStart} onChange={(e) => setAStart(e.target.value)} />
                <input className={input} type="date" value={aEnd} onChange={(e) => setAEnd(e.target.value)} />
              </div>
              <button
                disabled={saving || !aUser || !aProject || !aStart}
                onClick={() => void submitAssignment()}
                className="w-full rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-700 disabled:opacity-50"
              >
                Simpan penugasan
              </button>
            </div>
          </Card>

          <Card>
            <CardHeader title="Riwayat penugasan" subtitle={`${assignments.length} penugasan`} />
            <div className="divide-y divide-slate-100 dark:divide-slate-800">
              {assignments.length === 0 ? (
                <p className="py-6 text-center text-sm text-slate-400">Belum ada penugasan.</p>
              ) : (
                assignments.map((a) => (
                  <div key={a.id} className="flex items-center justify-between gap-3 py-3">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-semibold text-slate-800 dark:text-slate-100">
                        {a.user_name || a.user_email || a.user_id}
                      </p>
                      <p className="text-xs text-slate-500 dark:text-slate-400">
                        {[a.project_code, a.project_name, a.vessel_name, a.role, `${a.start_date}${a.end_date ? `–${a.end_date}` : "–…"}`]
                          .filter(Boolean)
                          .join(" · ")}
                      </p>
                    </div>
                    <Pill className={a.employment === "partner" ? STATUS_CLS.active : STATUS_CLS.done}>
                      {a.employment === "partner" ? a.partner_name || "mitra" : "internal"}
                    </Pill>
                  </div>
                ))
              )}
            </div>
          </Card>
        </div>
      )}
    </AdminShell>
  );
}
