"use client";

/* Shared admin layout: sidebar (with active state), mobile top bar, header.
 * Every admin page renders inside <AdminShell title=... subtitle=...>. */

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { UnauthorizedError, api, getToken, redirectToLogin } from "@/lib/api";
import { useTheme } from "@/lib/theme";
import { Icon, type IconName } from "@/components/icons";
import { Avatar } from "@/components/ui";

export type Me = {
  id: string;
  email: string;
  name: string;
  avatar_url?: string;
  role: string;
  is_active: boolean;
  is_email_verified: boolean;
  notification_enabled: boolean;
};

type NavItem = { label: string; href: string; icon: IconName };

// Menu yang boleh dibuka datang dari server (GET /me/permissions), bukan dari
// daftar hardcoded di frontend. Dengan begitu akses bisa diatur per user tanpa
// menyentuh kode admin; backend tetap menolak route yang tidak diizinkan.
type MenuAccess = { key: string; label: string; href: string; actions: string[] };

const MENU_ICONS: Record<string, IconName> = {
  dashboard: "dashboard",
  pengguna: "users",
  absensi: "calendar-check",
  laporan: "chart-bar",
  lokasi: "map-pin",
  device: "device",
  cuti: "calendar",
  lembur: "calendar-check",
  proyek: "map-pin",
  approval: "check-list",
  pengaturan: "settings",
  "akun-admin": "user",
};

// Menu yang ditampilkan pada grup bawah (bukan grup "Menu Utama").
const BOTTOM_MENU_KEYS = ["pengaturan"];

const ROLE_KEY = "absenkilat_role";
const ADMIN_ROLES = ["super_admin", "hr", "supervisor"];

function toNavItem(menu: MenuAccess): NavItem {
  return { label: menu.label, href: menu.href, icon: MENU_ICONS[menu.key] ?? "dashboard" };
}

export default function AdminShell({
  title,
  subtitle,
  actions,
  children,
}: {
  title: string;
  subtitle?: string;
  actions?: React.ReactNode;
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const { theme, toggle } = useTheme();
  const [me, setMe] = useState<Me | null>(null);
  const [role, setRole] = useState<string | null>(null);
  const [menus, setMenus] = useState<MenuAccess[]>([]);
  const [menusLoaded, setMenusLoaded] = useState(false);
  // true saat sesi tidak bisa dipulihkan: tampilkan panel "sesi berakhir",
  // bukan dashboard setengah kosong dengan banner error.
  const [sessionEnded, setSessionEnded] = useState(false);

  const loadMe = useCallback(async () => {
    if (!getToken()) {
      redirectToLogin("required");
      return;
    }
    try {
      const data = await api<Me>("/me");
      setMe(data);
      setRole(data.role);
      try {
        window.localStorage.setItem(ROLE_KEY, data.role);
      } catch {}
      // Menu diambil terpisah supaya kegagalan endpoint permission tidak
      // mengeluarkan user dari sesi yang valid.
      try {
        const perms = await api<{ menus: MenuAccess[] }>("/me/permissions");
        setMenus(perms.menus ?? []);
      } catch {
        setMenus([]);
      } finally {
        setMenusLoaded(true);
      }
    } catch (err) {
      if (err instanceof UnauthorizedError) {
        // lib/api sudah membersihkan token & mengarahkan ke /login.
        setSessionEnded(true);
        return;
      }
      if (err instanceof Error && /401/.test(err.message)) {
        redirectToLogin("expired");
        setSessionEnded(true);
        return;
      }
      // Gagal memuat /me tanpa 401: biarkan dashboard memakai data kosong.
      setMe(null);
    } finally {
      setMenusLoaded(true);
    }
  }, []);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- pemuatan data disengaja: effect memanggil loader yang menyalakan state loading.
    loadMe();
  }, [loadMe]);

  const logout = () => {
    try {
      window.localStorage.removeItem(ROLE_KEY);
    } catch {}
    redirectToLogin("required");
  };

  // Akses ditolak jika role sudah termuat dan bukan admin (employee / tak dikenal).
  const denied = role !== null && !ADMIN_ROLES.includes(role);
  // Nav dirender dari permission server. Saat masih memuat, jangan tampilkan
  // menu default apa pun supaya tidak ada menu "berkedip" yang ternyata ditolak.
  const visibleMenus = denied || !menusLoaded ? [] : menus;
  const navItems = visibleMenus.filter((m) => !BOTTOM_MENU_KEYS.includes(m.key)).map(toNavItem);
  const navBottom = visibleMenus.filter((m) => BOTTOM_MENU_KEYS.includes(m.key)).map(toNavItem);

  const isActive = (href: string) =>
    href === "/dashboard" ? pathname === href : pathname.startsWith(href);

  const navLink = (item: NavItem) => {
    const active = isActive(item.href);
    return (
      <Link
        key={item.href}
        href={item.href}
        className={`mb-1 flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left transition-colors ${
          active
            ? "bg-indigo-50 font-semibold text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-400"
            : "font-medium text-slate-600 hover:bg-slate-50 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-100"
        }`}
      >
        <Icon name={item.icon} className="h-[18px] w-[18px]" />
        {item.label}
      </Link>
    );
  };

  const themeToggleBtn = (
    <button
      type="button"
      title={theme === "light" ? "Ganti ke mode gelap" : "Ganti ke mode terang"}
      onClick={toggle}
      className="flex h-9 w-9 items-center justify-center rounded-full border border-slate-200 bg-white text-slate-500 transition-colors hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400 dark:hover:bg-slate-700"
    >
      <Icon name={theme === "light" ? "moon" : "sun"} className="h-[18px] w-[18px]" />
    </button>
  );

  return (
    <div className="min-h-screen bg-gray-100 dark:bg-slate-950">
      {/* ===== Sidebar (desktop) ===== */}
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-64 flex-col border-r border-slate-200 bg-white lg:flex dark:border-slate-800 dark:bg-slate-900">
        {/* Brand */}
        <div className="flex items-center gap-3 border-b border-slate-100 px-5 py-5 dark:border-slate-800">
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-indigo-600 text-base font-bold text-white shadow-sm">
            A
          </div>
          <div>
            <div className="text-sm font-bold leading-tight text-slate-900 dark:text-slate-100">AbsenKilat</div>
            <div className="text-xs text-slate-500 dark:text-slate-400">Admin Dashboard</div>
          </div>
        </div>

        {/* Nav */}
        <nav className="flex-1 overflow-y-auto px-3 py-4 text-sm">
          <div className="mb-1.5 px-3 text-[11px] font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500">
            Menu Utama
          </div>
          {navItems.map(navLink)}

          {navBottom.length > 0 && (
            <>
              <div className="mb-1.5 mt-5 px-3 text-[11px] font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500">
                Pengaturan
              </div>
              {navBottom.map(navLink)}
            </>
          )}
        </nav>

        {/* Account info section (clickable -> profil login) + logout */}
        <div className="border-t border-slate-100 p-4 dark:border-slate-800">
          <Link
            href="/akun-admin"
            className="flex items-center gap-3 rounded-xl bg-slate-50 p-3 transition-colors hover:bg-slate-100 dark:bg-slate-800 dark:hover:bg-slate-700"
          >
            <Avatar name={me?.name || "Admin"} src={me?.avatar_url} size="h-10 w-10 text-sm" />
            <div className="min-w-0 flex-1">
              <div className="truncate text-sm font-semibold text-slate-900 dark:text-slate-100">
                {me?.name || "Admin"}
              </div>
              <div className="truncate text-xs text-slate-500 dark:text-slate-400">{me?.email || ""}</div>
            </div>
            <button
              type="button"
              onClick={(e) => {
                e.preventDefault();
                logout();
              }}
              title="Keluar"
              className="flex h-8 w-8 items-center justify-center rounded-lg text-slate-400 transition-colors hover:bg-slate-200 hover:text-slate-700 dark:text-slate-500 dark:hover:bg-slate-700 dark:hover:text-slate-200"
            >
              <Icon name="logout" className="h-4 w-4" />
            </button>
          </Link>
        </div>
      </aside>

      {/* ===== Main ===== */}
      <div className="lg:pl-64">
        {/* Mobile top bar */}
        <div className="flex items-center justify-between border-b border-slate-200 bg-white px-4 py-3 lg:hidden dark:border-slate-800 dark:bg-slate-900">
          <div className="flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-indigo-600 text-sm font-bold text-white">
              A
            </div>
            <span className="text-sm font-bold text-slate-900 dark:text-slate-100">AbsenKilat</span>
          </div>
          <div className="flex items-center gap-2">
            {themeToggleBtn}
            <button
              type="button"
              onClick={logout}
              className="rounded-lg border border-slate-300 px-3 py-1.5 text-xs font-medium text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
            >
              Logout
            </button>
          </div>
        </div>

        {/* Header */}
        <header className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 bg-white px-4 py-4 md:px-8 dark:border-slate-800 dark:bg-slate-900">
          <div>
            <h1 className="text-xl font-bold text-slate-900 dark:text-slate-100">{title}</h1>
            {subtitle && <p className="text-sm text-slate-500 dark:text-slate-400">{subtitle}</p>}
          </div>
          <div className="flex flex-wrap items-center gap-2 md:gap-3">
            {actions}
            {themeToggleBtn}
          </div>
        </header>

        <main className="p-4 md:p-8">
          {sessionEnded ? (
            <div className="mx-auto flex max-w-md flex-col items-center rounded-2xl bg-white px-6 py-14 text-center shadow-sm ring-1 ring-slate-100 dark:bg-slate-900 dark:ring-slate-800">
              <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-amber-100 text-amber-600 dark:bg-amber-500/15 dark:text-amber-400">
                <Icon name="clock" className="h-8 w-8" />
              </div>
              <h2 className="mt-5 text-xl font-bold text-slate-900 dark:text-slate-100">Sesi berakhir</h2>
              <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
                Sesi Anda sudah berakhir karena tidak ada aktivitas dalam waktu lama. Silakan masuk kembali —
                data yang tadi sedang dibuka akan dimuat ulang setelah Anda masuk.
              </p>
              <button
                type="button"
                onClick={() => redirectToLogin("expired")}
                className="mt-6 inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-5 py-2.5 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700"
              >
                <Icon name="logout" className="h-4 w-4" />
                Masuk kembali
              </button>
            </div>
          ) : denied ? (
            <div className="mx-auto flex max-w-md flex-col items-center rounded-2xl bg-white px-6 py-14 text-center shadow-sm ring-1 ring-slate-100 dark:bg-slate-900 dark:ring-slate-800">
              <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-red-100 text-red-600 dark:bg-red-500/15 dark:text-red-400">
                <Icon name="lock" className="h-8 w-8" />
              </div>
              <h2 className="mt-5 text-xl font-bold text-slate-900 dark:text-slate-100">Akses Ditolak</h2>
              <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
                Akun Anda tidak memiliki izin untuk mengakses dashboard admin. Silakan hubungi administrator jika
                menurut Anda ini sebuah kesalahan.
              </p>
              <button
                type="button"
                onClick={logout}
                className="mt-6 inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-5 py-2.5 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-indigo-700"
              >
                <Icon name="logout" className="h-4 w-4" />
                Keluar
              </button>
            </div>
          ) : (
            children
          )}
        </main>
      </div>
    </div>
  );
}
