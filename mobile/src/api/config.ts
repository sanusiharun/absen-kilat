// API configuration. Defaults to the VPS LAN IP; override with
// EXPO_PUBLIC_API_URL in .env for device testing.
export const API_BASE_URL =
  process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8090/api/v1';

export const API_ROOT =
  process.env.EXPO_PUBLIC_API_ROOT ?? 'http://localhost:8090';
