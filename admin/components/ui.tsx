"use client";

/* ---------- Small shared UI pieces (consistent indigo/slate style) ---------- */

import { avatarColor, initials } from "@/components/icons";

export function Avatar({ name, src, size }: { name: string; src?: string; size: string }) {
  if (src) {
    return (
      // eslint-disable-next-line @next/next/no-img-element
      <img src={src} alt={name} className={`${size} shrink-0 rounded-full object-cover ring-2 ring-white`} />
    );
  }
  return (
    <div
      className={`${size} flex shrink-0 items-center justify-center rounded-full font-semibold text-white ${avatarColor(
        name || "?",
      )}`}
    >
      {initials(name || "?")}
    </div>
  );
}

export function Empty({ height = 160 }: { height?: number }) {
  return (
    <div className="flex items-center justify-center text-xs text-slate-400" style={{ height: `${height}px` }}>
      Belum ada data
    </div>
  );
}

export function Card({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return <div className={`rounded-2xl bg-white shadow-sm ring-1 ring-slate-100 ${className}`}>{children}</div>;
}

export function CardHeader({
  title,
  subtitle,
  right,
}: {
  title: string;
  subtitle?: string;
  right?: React.ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-100 px-6 py-4">
      <div>
        <h2 className="font-semibold text-slate-900">{title}</h2>
        {subtitle && <p className="text-xs text-slate-500">{subtitle}</p>}
      </div>
      {right}
    </div>
  );
}

export function Pill({ className = "", children }: { className?: string; children: React.ReactNode }) {
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ${className}`}>
      {children}
    </span>
  );
}

export function LegendRow({
  items,
}: {
  items: { label: string; color: string; value?: number }[];
}) {
  return (
    <div className="mt-4 flex flex-wrap items-center gap-x-4 gap-y-1">
      {items.map((it) => (
        <span key={it.label} className="flex items-center gap-1.5 text-xs text-slate-600">
          <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: it.color }} />
          {it.label}
          {it.value !== undefined && <span className="font-semibold text-slate-900">{it.value}</span>}
        </span>
      ))}
    </div>
  );
}

export function ErrorBanner({ message }: { message: string }) {
  if (!message) return null;
  return (
    <div className="mb-6 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm font-medium text-red-700">
      {message}
    </div>
  );
}

export function SuccessBanner({ message }: { message: string }) {
  if (!message) return null;
  return (
    <div className="mb-6 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm font-medium text-emerald-700">
      {message}
    </div>
  );
}

export function Spinner({ label = "Memuat..." }: { label?: string }) {
  return (
    <div className="flex items-center justify-center gap-2 py-10 text-sm text-slate-400">
      <span className="h-4 w-4 animate-spin rounded-full border-2 border-slate-300 border-t-indigo-500" />
      {label}
    </div>
  );
}

/* Shared form control classes */
export const inputCls =
  "w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100 focus:outline-none";

export const selectCls =
  "appearance-none rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 focus:border-indigo-400 focus:outline-none";
