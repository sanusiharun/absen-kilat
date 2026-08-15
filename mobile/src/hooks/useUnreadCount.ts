// useUnreadCount: global unread-notification counter shared between the tab
// badge (tabs layout) and the notifications screen. The screen calls
// refreshUnread() after marking items read so the badge updates live.
import { useEffect, useState } from 'react';

import { notificationApi } from '@/src/api/endpoints';

let unread = 0;
const listeners = new Set<(n: number) => void>();

export async function refreshUnread(): Promise<void> {
  try {
    const res = await notificationApi.unreadCount();
    unread = res.unread;
  } catch {
    // keep last known count on network/auth errors
  }
  listeners.forEach((l) => l(unread));
}

export function useUnreadCount(): number {
  const [count, setCount] = useState(unread);

  useEffect(() => {
    listeners.add(setCount);
    setCount(unread);
    refreshUnread();
    return () => {
      listeners.delete(setCount);
    };
  }, []);

  return count;
}
