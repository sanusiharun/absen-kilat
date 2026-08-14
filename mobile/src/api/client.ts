// API client: fetch wrapper with auth headers and automatic 401 refresh.
import { API_BASE_URL } from './config';
import {
  clearTokens,
  getAccessToken,
  getRefreshToken,
  saveTokens,
} from './tokenStorage';

export class ApiError extends Error {
  code: string;
  status: number;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.status = status;
  }
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE';
  body?: unknown;
  formData?: FormData;
  auth?: boolean;
}

let refreshing: Promise<string | null> | null = null;

async function refreshAccessToken(): Promise<string | null> {
  const refreshToken = await getRefreshToken();
  if (!refreshToken) return null;

  const res = await fetch(`${API_BASE_URL}/auth/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refreshToken }),
  });
  if (!res.ok) {
    await clearTokens();
    return null;
  }
  const json = (await res.json()) as {
    data: { access_token: string; refresh_token: string };
  };
  await saveTokens(json.data.access_token, json.data.refresh_token);
  return json.data.access_token;
}

export async function apiRequest<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, formData, auth = true } = opts;

  const headers: Record<string, string> = {};
  if (formData) {
    // Let the browser set the multipart boundary.
  } else if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
  }
  if (auth) {
    const token = await getAccessToken();
    if (token) headers['Authorization'] = `Bearer ${token}`;
  }

  let res = await fetch(`${API_BASE_URL}${path}`, {
    method,
    headers,
    body: formData ?? (body !== undefined ? JSON.stringify(body) : undefined),
  });

  // Automatic 401 → refresh → retry once.
  if (res.status === 401 && auth) {
    if (!refreshing) {
      refreshing = refreshAccessToken();
    }
    const newToken = await refreshing;
    refreshing = null;

    if (newToken) {
      headers['Authorization'] = `Bearer ${newToken}`;
      res = await fetch(`${API_BASE_URL}${path}`, {
        method,
        headers,
        body: formData ?? (body !== undefined ? JSON.stringify(body) : undefined),
      });
    } else {
      await clearTokens();
    }
  }

  if (!res.ok) {
    let code = 'UNKNOWN';
    let message = `Request failed (${res.status})`;
    try {
      const json = (await res.json()) as {
        error?: { code?: string; message?: string };
      };
      if (json.error?.code) code = json.error.code;
      if (json.error?.message) message = json.error.message;
    } catch {
      // non-JSON error body
    }
    throw new ApiError(res.status, code, message);
  }

  const json = (await res.json()) as { data: T };
  return json.data;
}

export const api = {
  get: <T>(path: string, opts?: RequestOptions) =>
    apiRequest<T>(path, { ...opts, method: 'GET' }),
  post: <T>(path: string, body?: unknown, opts?: RequestOptions) =>
    apiRequest<T>(path, { ...opts, method: 'POST', body }),
  patch: <T>(path: string, body?: unknown, opts?: RequestOptions) =>
    apiRequest<T>(path, { ...opts, method: 'PATCH', body }),
  upload: <T>(path: string, formData: FormData, opts?: RequestOptions) =>
    apiRequest<T>(path, { ...opts, method: 'POST', formData }),
};
