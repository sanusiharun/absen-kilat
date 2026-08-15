import { Ionicons } from '@expo/vector-icons';
import { useFocusEffect } from 'expo-router';
import { useCallback, useState } from 'react';
import {
  ActivityIndicator,
  Pressable,
  RefreshControl,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';

import { notificationApi, Notification } from '@/src/api/endpoints';
import { refreshUnread } from '@/src/hooks/useUnreadCount';
import { colors, spacing } from '@/src/theme';

function fmtRelative(iso: string): string {
  const d = new Date(iso);
  const now = new Date();
  const sameDay = d.toDateString() === now.toDateString();
  if (sameDay) {
    return d.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });
  }
  return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short' });
}

export default function NotificationsScreen() {
  const [items, setItems] = useState<Notification[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  const load = useCallback(async () => {
    try {
      const res = await notificationApi.list();
      setItems(res.notifications);
      refreshUnread();
    } catch {
      // handled by client
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  useFocusEffect(
    useCallback(() => {
      load();
    }, [load])
  );

  async function markRead(n: Notification) {
    if (n.is_read) return;
    try {
      await notificationApi.markRead(n.id);
      setItems((prev) => prev.map((x) => (x.id === n.id ? { ...x, is_read: true } : x)));
      refreshUnread();
    } catch {
      // biarkan
    }
  }

  async function markAll() {
    try {
      await notificationApi.markAllRead();
      setItems((prev) => prev.map((x) => ({ ...x, is_read: true })));
      refreshUnread();
    } catch {
      // biarkan
    }
  }

  const unreadCount = items.filter((n) => !n.is_read).length;

  return (
    <ScrollView
      style={styles.container}
      contentContainerStyle={styles.content}
      refreshControl={
        <RefreshControl
          refreshing={refreshing}
          onRefresh={() => {
            setRefreshing(true);
            load();
          }}
        />
      }
    >
      <View style={styles.headerRow}>
        <Text style={styles.header}>Notifikasi</Text>
        {items.length > 0 ? (
          <Pressable onPress={markAll} hitSlop={8}>
            <Text style={styles.markAll}>Tandai semua dibaca</Text>
          </Pressable>
        ) : null}
      </View>

      {loading ? (
        <ActivityIndicator size="large" color={colors.primary} style={{ marginTop: spacing.xl }} />
      ) : items.length === 0 ? (
        <View style={styles.empty}>
          <Ionicons name="notifications-off-outline" size={44} color={colors.border} />
          <Text style={styles.emptyText}>Tidak ada notifikasi.</Text>
          <Text style={styles.emptySub}>Notifikasi absen dan sistem akan muncul di sini.</Text>
        </View>
      ) : (
        <>
          {unreadCount > 0 ? (
            <Text style={styles.unreadLabel}>{unreadCount} belum dibaca</Text>
          ) : null}
          {items.map((n) => (
            <Pressable key={n.id} style={styles.card} onPress={() => markRead(n)}>
              <View
                style={[
                  styles.iconCircle,
                  { backgroundColor: n.is_read ? colors.bg : colors.primarySoft },
                ]}
              >
                <Ionicons
                  name={n.is_read ? 'mail-open-outline' : 'mail-unread-outline'}
                  size={20}
                  color={n.is_read ? colors.textMuted : colors.primary}
                />
              </View>
              <View style={styles.cardBody}>
                <View style={styles.cardTitleRow}>
                  <Text style={[styles.cardTitle, !n.is_read && styles.cardTitleUnread]}>
                    {n.title}
                  </Text>
                  {!n.is_read ? <View style={styles.unreadDot} /> : null}
                </View>
                <Text style={styles.cardBodyText}>{n.body}</Text>
                <Text style={styles.cardTime}>{fmtRelative(n.created_at)}</Text>
              </View>
            </Pressable>
          ))}
        </>
      )}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  content: { padding: spacing.lg, gap: spacing.md, paddingBottom: spacing.xl },
  headerRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: spacing.xs,
  },
  header: { fontSize: 24, fontWeight: '800', color: colors.text },
  markAll: { color: colors.primary, fontWeight: '600', fontSize: 13 },
  unreadLabel: { fontSize: 12, fontWeight: '600', color: colors.textMuted, marginTop: -spacing.xs },
  card: {
    backgroundColor: colors.card,
    borderRadius: 14,
    padding: spacing.md,
    flexDirection: 'row',
    gap: spacing.sm,
    borderWidth: 1,
    borderColor: colors.border,
  },
  iconCircle: {
    width: 40,
    height: 40,
    borderRadius: 12,
    alignItems: 'center',
    justifyContent: 'center',
  },
  cardBody: { flex: 1 },
  cardTitleRow: { flexDirection: 'row', alignItems: 'center', gap: spacing.xs },
  cardTitle: { fontSize: 14, fontWeight: '600', color: colors.textMuted, flexShrink: 1 },
  cardTitleUnread: { color: colors.text, fontWeight: '700' },
  unreadDot: { width: 8, height: 8, borderRadius: 4, backgroundColor: colors.primary },
  cardBodyText: { fontSize: 13, color: colors.text, marginTop: 2, lineHeight: 18 },
  cardTime: { fontSize: 12, color: colors.textMuted, marginTop: 4 },
  empty: { alignItems: 'center', marginTop: spacing.xl, gap: spacing.sm },
  emptyText: { color: colors.textMuted, fontSize: 15, fontWeight: '600' },
  emptySub: { color: colors.textMuted, fontSize: 13, textAlign: 'center' },
});
