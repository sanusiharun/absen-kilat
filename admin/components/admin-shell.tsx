"use client";

/* Shared admin layout: sidebar (with active state), mobile top bar, header.
 * Every admin page renders inside <AdminShell title=... subtitle=...>. */

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { api, clearToken, getToken, mediaURL } from "@/lib/api";
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

const NAV_UTAMA: NavItem[] = [
  { label: "Dashboard", href: "/dashboard", icon: "dashboard" },
  { label: "Pengguna", href: "/pengguna", icon: "users" },
  { label: "Absensi", href: "/absensi", icon: "calendar-check" },
  { label: "Laporan", href: "/laporan", icon: "chart-bar" },
  { label: "Lokasi", href: "/lokasi", icon: "map-pin" },
  { label: "Device", href: "/device", icon: "device" },
];

const NAV_PENGATURAN: NavItem[] = [{ label: "Akun Admin", href: "/akun-admin", icon: "user" }];

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
  const router = useRouter();
  const { theme, toggle } = useTheme();
  const [me, setMe] = useState<Me | null>(null);

  const loadMe = useCallback(async () => {
    if (!getToken()) {
      router.replace("/login");
      return;
    }
    try {
      setMe(await api<Me>("/me"));
    } catch (err) {
      if (err instanceof Error && /401|403/.test(err.message)) {
        clearToken();
        router.replace("/login");
        return;
      }
      setMe(null);
    }
  }, [router]);

  useEffect(() => {
    loadMe();
  }, [loadMe]);

  const logout = () => {
    clearToken();
    router.replace("/login");
  };

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
          {NAV_UTAMA.map(navLink)}

          <div className="mb-1.5 mt-5 px-3 text-[11px] font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500">
            Pengaturan
          </div>
          {NAV_PENGATURAN.map(navLink)}
        </nav>

        {/* Admin profile */}
        <div className="border-t border-slate-100 p-4 dark:border-slate-800">
          <div className="flex items-center gap-3 rounded-xl bg-slate-50 p-3 dark:bg-slate-800">
            <Avatar name={me?.name || "Admin"} src={me?.avatar_url} size="h-10 w-10 text-sm" />
            <div className="min-w-0 flex-1">
              <div className="truncate text-sm font-semibold text-slate-900 dark:text-slate-100">
                {me?.name || "Admin"}
              </div>
              <div className="truncate text-xs text-slate-500 dark:text-slate-400">{me?.email || ""}</div>
            </div>
            <button
              type="button"
              onClick={logout}
              title="Keluar"
              className="flex h-8 w-8 items-center justify-center rounded-lg text-slate-400 transition-colors hover:bg-slate-200 hover:text-slate-700 dark:text-slate-500 dark:hover:bg-slate-700 dark:hover:text-slate-200"
            >
              <Icon name="logout" className="h-4 w-4" />
            </button>
          </div>
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

        <main className="p-4 md:p-8">{children}</main>
      </div>
    </div>
  );
}
