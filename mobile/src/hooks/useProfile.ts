// useProfile: shared profile state with a manual refresh trigger.
import { useCallback, useEffect, useState } from 'react';

import { profileApi, User } from '@/src/api/endpoints';

export function useProfile() {
  const [profile, setProfile] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      setProfile(await profileApi.me());
    } catch {
      // unauthenticated or network error; keep last known profile
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { profile, loading, refresh };
}
