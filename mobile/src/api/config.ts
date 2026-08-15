// API configuration. Defaults to the VPS LAN IP; override with
// EXPO_PUBLIC_API_URL in .env for device testing.
export const API_BASE_URL =
  process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8090/api/v1';

export const API_ROOT =
  process.env.EXPO_PUBLIC_API_ROOT ?? 'http://localhost:8090';

// Ubah path relatif foto (mis. "uploads/xxx.jpg" atau "2026/08/xxx.jpg")
// menjadi URL absolut. Path absolut (http...) dikembalikan apa adanya.
export function mediaURL(path?: string | null): string {
  if (!path) return '';
  if (/^https?:\/\//.test(path)) return path;
  const clean = path.replace(/^\//, '');
  return `${API_ROOT}/${clean.startsWith('uploads/') ? clean : `uploads/${clean}`}`;
}
