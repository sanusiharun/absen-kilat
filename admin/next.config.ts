import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Allow the VPS public IP (and localhost) as a dev host so static chunks and
  // HMR are not blocked by Next's anti-DNS-rebinding protection when the admin
  // app is previewed over the public IP / tunnel.
  allowedDevOrigins: ["43.157.243.110"],

  // API dan file upload dilayani dari origin yang SAMA dengan dashboard.
  // Browser user hanya membuka port 3000; kalau frontend memanggil :8090
  // langsung, login gagal ("Failed to fetch") pada jaringan yang memblokir port
  // lain. Dengan rewrite ini NEXT_PUBLIC_API_URL cukup "/api/v1" (relatif).
  async rewrites() {
    const api = process.env.API_ORIGIN ?? "http://127.0.0.1:8090";
    return [
      { source: "/api/v1/:path*", destination: `${api}/api/v1/:path*` },
      { source: "/uploads/:path*", destination: `${api}/uploads/:path*` },
      { source: "/healthz", destination: `${api}/healthz` },
    ];
  },
};

export default nextConfig;
