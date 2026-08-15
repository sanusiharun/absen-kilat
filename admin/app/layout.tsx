import type { Metadata } from "next";
import { Geist } from "next/font/google";
import "./globals.css";

const geist = Geist({ variable: "--font-geist", subsets: ["latin"] });

export const metadata: Metadata = {
  title: "AbsenKilat Admin",
  description: "Admin dashboard Absen Kilat",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="id" className={`${geist.variable} antialiased`}>
      <body className="min-h-screen bg-gray-100 font-sans text-slate-900">{children}</body>
    </html>
  );
}
