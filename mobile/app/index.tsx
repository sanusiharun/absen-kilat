import { Redirect } from 'expo-router';
import { useEffect, useState } from 'react';

import { getAccessToken } from '@/src/api/tokenStorage';

export default function Index() {
  const [loading, setLoading] = useState(true);
  const [authed, setAuthed] = useState(false);

  useEffect(() => {
    getAccessToken()
      .then((t) => setAuthed(!!t))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return null;
  return <Redirect href={authed ? '/(tabs)' : '/(auth)/login'} />;
}
