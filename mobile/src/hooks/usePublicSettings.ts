// Hook pengaturan publik (GET /settings/public) dengan cache modul agar tidak
// fetch ulang tiap fokus layar. Kalau endpoint belum ada / gagal → null,
// pemanggil tetap jalan dengan perilaku default ('both').
import { useEffect, useState } from 'react';

import { PublicSettings, settingsApi } from '@/src/api/endpoints';

let cache: PublicSettings | null = null;

export function usePublicSettings(): PublicSettings | null {
  const [settings, setSettings] = useState<PublicSettings | null>(cache);

  useEffect(() => {
    if (cache) return;
    let cancelled = false;
    settingsApi
      .public()
      .then((s) => {
        cache = s;
        if (!cancelled) setSettings(s);
      })
      .catch(() => {
        // endpoint belum tersedia — biarkan null (default 'both')
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return settings;
}
