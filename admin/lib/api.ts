// API client for the Absen Kilat admin app.
//
// Sesi: access token hanya berlaku 15 menit, refresh token 30 hari. Klien ini
// menyimpan keduanya, memperbarui access token sebelum kedaluwarsa, dan bila
// server tetap menolak (401) ia membersihkan sesi lalu mengarahkan ke /login
// dengan pesan yang jelas — bukan meninggalkan dashboard setengah kosong.

// Default RELATIF: browser memanggil origin yang sama dan Next me-rewrite
// /api/v1/* ke API (lihat next.config.ts). Dengan begitu tidak perlu membuka
// port API ke publik dan tidak ada masalah CORS. Override lewat
// NEXT_PUBLIC_API_URL bila API memang berada di origin lain.
export const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "/api/v1";

// Root URL (tanpa suffix /api/v1), dipakai untuk membangun URL file statis.
export const API_ROOT = API_BASE.replace(/\/api\/v1\/?$/, "");

export const TOKEN_KEY = "absenkilat_admin_token";
export const REFRESH_KEY = "absenkilat_admin_refresh";

// Access token diperbarui kalau sisa umurnya tinggal <= 60 detik.
const EXPIRY_MARGIN_S = 60;

/** Dilempar saat sesi tidak bisa dipulihkan lagi (sudah diarahkan ke /login). */
export class UnauthorizedError extends Error {
  constructor(message = "Sesi Anda berakhir. Silakan masuk kembali.") {
    super(message);
    this.name = "UnauthorizedError";
  }
}

// Ubah path relatif foto (mis. "uploads/xxx.jpg" atau "2026/08/xxx.jpg")
// menjadi URL absolut. Path absolut (http...) dikembalikan apa adanya.
export function mediaURL(path?: string | null): string {
  if (!path) return "";
  if (/^https?:\/\//.test(path)) return path;
  const clean = path.replace(/^\//, "");
  return `${API_ROOT}/${clean.startsWith("uploads/") ? clean : `uploads/${clean}`}`;
}

// ---------------------------------------------------------------- token store

export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(TOKEN_KEY);
}

export function getRefreshToken(): string | null {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(REFRESH_KEY);
}

export function setTokens(access: string, refresh?: string | null) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(TOKEN_KEY, access);
  if (refresh) window.localStorage.setItem(REFRESH_KEY, refresh);
}

/** @deprecated pakai setTokens supaya refresh token ikut tersimpan. */
export function setToken(t: string) {
  setTokens(t);
}

export function clearTokens() {
  if (typeof window === "undefined") return;
  window.localStorage.removeItem(TOKEN_KEY);
  window.localStorage.removeItem(REFRESH_KEY);
}

/** @deprecated pakai clearTokens. */
export function clearToken() {
  clearTokens();
}

// ---------------------------------------------------------------- expiry math

/** Sisa umur token (detik) dari klaim exp, atau null kalau tak terbaca. */
export function tokenSecondsLeft(token: string | null): number | null {
  if (!token) return null;
  const parts = token.split(".");
  if (parts.length < 2) return null;
  try {
    const payload = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const padded = payload + "=".repeat((4 - (payload.length % 4)) % 4);
    const json = JSON.parse(atob(padded));
    if (typeof json?.exp !== "number") return null;
    return json.exp - Math.floor(Date.now() / 1000);
  } catch {
    return null;
  }
}

// ---------------------------------------------------------------- refresh flow

let refreshInFlight: Promise<string | null> | null = null;

/** Tukar refresh token dengan access token baru (single-flight per tab). */
export async function refreshAccessToken(): Promise<string | null> {
  if (refreshInFlight) return refreshInFlight;
  const refresh = getRefreshToken();
  if (!refresh) return null;

  refreshInFlight = (async () => {
    try {
      const res = await fetch(`${API_BASE}/auth/refresh`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refresh }),
      });
      if (!res.ok) return null;
      const body = await res.json();
      const access = body?.data?.access_token as string | undefined;
      const nextRefresh = body?.data?.refresh_token as string | undefined;
      if (!access) return null;
      setTokens(access, nextRefresh ?? null);
      return access;
    } catch {
      return null;
    } finally {
      refreshInFlight = null;
    }
  })();

  return refreshInFlight;
}

/** Bersihkan sesi lalu arahkan ke halaman login dengan alasan yang jelas. */
export function redirectToLogin(reason: "expired" | "required" = "expired") {
  if (typeof window === "undefined") return;
  clearTokens();
  if (window.location.pathname.startsWith("/login")) return;
  const next = window.location.pathname + window.location.search;
  const target = `/login?${reason}=1` + (next ? `&next=${encodeURIComponent(next)}` : "");
  window.location.replace(target);
}

// ---------------------------------------------------------------- requests

async function rawFetch(path: string, options: RequestInit, token: string | null) {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };
  if (token) headers.Authorization = `Bearer ${token}`;
  return fetch(`${API_BASE}${path}`, { ...options, headers });
}

async function errorMessage(res: Response): Promise<string> {
  try {
    const body = await res.json();
    const code = body?.error?.code as string | undefined;
    const message = body?.error?.message as string | undefined;
    if (code === "UNAUTHORIZED") return "Sesi Anda berakhir. Silakan masuk kembali.";
    if (message) return message;
  } catch {}
  return `Request failed (${res.status})`;
}

export async function api<T = unknown>(
  path: string,
  options: RequestInit & { auth?: boolean } = {}
): Promise<T> {
  const useAuth = options.auth !== false;

  // Perbarui lebih dulu kalau access token hampir habis, supaya user tidak
  // sempat melihat 401 sama sekali.
  if (useAuth) {
    const left = tokenSecondsLeft(getToken());
    if (left !== null && left <= EXPIRY_MARGIN_S && getRefreshToken()) {
      await refreshAccessToken();
    }
  }

  let res = await rawFetch(path, options, useAuth ? getToken() : null);

  if (res.status === 401 && useAuth) {
    const renewed = await refreshAccessToken();
    if (renewed) res = await rawFetch(path, options, renewed);
    if (res.status === 401) {
      redirectToLogin("expired");
      throw new UnauthorizedError();
    }
  }

  if (!res.ok) throw new Error(await errorMessage(res));
  return (await res.json()).data;
}
