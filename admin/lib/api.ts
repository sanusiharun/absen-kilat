// API client for the Absen Kilat admin app.
// Token is stored in localStorage (browser-only).

// Default RELATIF: browser memanggil origin yang sama dan Next me-rewrite
// /api/v1/* ke API (lihat next.config.ts). Dengan begitu tidak perlu membuka
// port API ke publik dan tidak ada masalah CORS. Override lewat
// NEXT_PUBLIC_API_URL bila API memang berada di origin lain.
export const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "/api/v1";

// Root URL (tanpa suffix /api/v1), dipakai untuk membangun URL file statis.
export const API_ROOT = API_BASE.replace(/\/api\/v1\/?$/, "");

export const TOKEN_KEY = "absenkilat_admin_token";

// Ubah path relatif foto (mis. "uploads/xxx.jpg" atau "2026/08/xxx.jpg")
// menjadi URL absolut. Path absolut (http...) dikembalikan apa adanya.
export function mediaURL(path?: string | null): string {
  if (!path) return "";
  if (/^https?:\/\//.test(path)) return path;
  const clean = path.replace(/^\//, "");
  return `${API_ROOT}/${clean.startsWith("uploads/") ? clean : `uploads/${clean}`}`;
}

export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(TOKEN_KEY);
}

export function setToken(t: string) {
  window.localStorage.setItem(TOKEN_KEY, t);
}

export function clearToken() {
  window.localStorage.removeItem(TOKEN_KEY);
}

export async function api<T = unknown>(
  path: string,
  options: RequestInit & { auth?: boolean } = {}
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };
  if (options.auth !== false) {
    const t = getToken();
    if (t) headers.Authorization = `Bearer ${t}`;
  }
  const res = await fetch(`${API_BASE}${path}`, { ...options, headers });
  if (!res.ok) {
    let msg = `Request failed (${res.status})`;
    try {
      const body = await res.json();
      if (body?.error?.message) msg = body.error.message;
    } catch {}
    throw new Error(msg);
  }
  return (await res.json()).data;
}
