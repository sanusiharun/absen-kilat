import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Allow the VPS public IP (and localhost) as a dev host so static chunks and
  // HMR are not blocked by Next's anti-DNS-rebinding protection when the admin
  // app is previewed over the public IP / tunnel.
  allowedDevOrigins: ["43.157.243.110"],
};

export default nextConfig;
