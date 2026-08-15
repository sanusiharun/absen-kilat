"use client";

/* ---------- Form fields konsisten indigo: Select & DateInput ---------- */

import { Icon } from "@/components/icons";

type SelectOption = { value: string; label: string };

export function Select({
  value,
  onChange,
  options,
  children,
  className = "",
  selectClassName = "",
  id,
  disabled,
  pill = false,
}: {
  value: string | number;
  onChange: (e: React.ChangeEvent<HTMLSelectElement>) => void;
  options?: SelectOption[];
  children?: React.ReactNode;
  className?: string;
  selectClassName?: string;
  id?: string;
  disabled?: boolean;
  pill?: boolean;
}) {
  return (
    <div className={`relative ${className}`}>
      <select
        id={id}
        value={value}
        onChange={onChange}
        disabled={disabled}
        className={`w-full cursor-pointer appearance-none border border-slate-300 bg-white text-sm text-slate-800 shadow-sm transition-colors focus:border-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100 dark:focus:border-indigo-500 dark:focus:ring-indigo-500/20 ${
          pill
            ? "rounded-full py-1.5 pl-3 pr-8 text-xs font-medium"
            : "rounded-lg py-2 pl-3 pr-9"
        } ${selectClassName}`}
      >
        {options
          ? options.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))
          : children}
      </select>
      <Icon
        name="chevron-down"
        className="pointer-events-none absolute right-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400"
      />
    </div>
  );
}

export function DateInput({
  value,
  onChange,
  id,
  className = "",
}: {
  value: string;
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  id?: string;
  className?: string;
}) {
  return (
    <input
      id={id}
      type="date"
      value={value}
      onChange={onChange}
      className={`w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-800 shadow-sm transition-colors focus:border-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-100 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100 dark:[color-scheme:dark] dark:focus:border-indigo-500 dark:focus:ring-indigo-500/20 ${className}`}
    />
  );
}
