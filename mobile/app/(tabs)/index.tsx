import { useRouter } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import {
  ActivityIndicator,
  Pressable,
  RefreshControl,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';

import { attendanceApi, TodayState } from '@/src/api/endpoints';
import { Avatar } from '@/src/components/Avatar';
import { colors, spacing } from '@/src/theme';

export default function HomeScreen() {
  const router = useRouter();
  const [today, setToday] = useState<TodayState | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  const load = useCallback(async () => {
    try {
      setToday(await attendanceApi.today());
    } catch {
      // auth error handled by client (clears tokens); user redirected on next mount
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  const onRefresh = useCallback(() => {
    setRefreshing(true);
    load();
  }, [load]);

  useEffect(() => {
    load();
  }, [load]);

  if (loading) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" color={colors.primary} />
      </View>
    );
  }

  const canCheckIn = today ? !today.checked_in : true;
  const canCheckOut = today ? today.checked_in && !today.checked_out : false;

  return (
    <ScrollView
      style={styles.container}
      contentContainerStyle={styles.content}
      refreshControl={<RefreshControl refreshing={refreshing} onRefresh={onRefresh} />}
    >
      <View style={styles.header}>
        <Avatar size={48} />
        <View>
          <Text style={styles.greeting}>Absen Kilat</Text>
          <Text style={styles.date}>{new Date().toLocaleDateString('id-ID', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })}</Text>
        </View>
      </View>

      {today?.checked_in && today.checked_out ? (
        <View style={[styles.card, styles.doneCard]}>
          <Text style={styles.doneTitle}>✅ Absen hari ini selesai</Text>
          <Text style={styles.doneSub}>Masuk & keluar sudah tercatat. Terima kasih!</Text>
        </View>
      ) : null}

      <View style={styles.card}>
        <Text style={styles.cardTitle}>Aksi Absen</Text>
        {canCheckIn ? (
          <Pressable
            style={[styles.actionBtn, { backgroundColor: colors.success }]}
            onPress={() => router.push('/checkin')}
          >
            <Text style={styles.actionText}>📷 Absen Masuk</Text>
          </Pressable>
        ) : null}
        {canCheckOut ? (
          <Pressable
            style={[styles.actionBtn, { backgroundColor: colors.warning }]}
            onPress={() => router.push('/checkout')}
          >
            <Text style={styles.actionText}>📷 Absen Keluar</Text>
          </Pressable>
        ) : null}
        {!canCheckIn && !canCheckOut ? (
          <Text style={styles.muted}>Semua absen hari ini sudah tercatat.</Text>
        ) : null}
      </View>

      <Pressable style={styles.linkRow} onPress={() => router.push('/(tabs)/history')}>
        <Text style={styles.linkText}>Lihat Riwayat Absen →</Text>
      </Pressable>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  content: { padding: spacing.lg, gap: spacing.lg },
  center: { flex: 1, justifyContent: 'center', alignItems: 'center', backgroundColor: colors.bg },
  header: { flexDirection: 'row', alignItems: 'center', gap: spacing.md },
  greeting: { fontSize: 20, fontWeight: '800', color: colors.text },
  date: { fontSize: 13, color: colors.textMuted, marginTop: 2 },
  card: {
    backgroundColor: colors.card,
    borderRadius: 14,
    padding: spacing.lg,
    gap: spacing.md,
    borderWidth: 1,
    borderColor: colors.border,
  },
  doneCard: { backgroundColor: '#F0FDF4', borderColor: '#BBF7D0' },
  doneTitle: { fontSize: 16, fontWeight: '700', color: colors.success },
  doneSub: { fontSize: 13, color: colors.textMuted },
  cardTitle: { fontSize: 15, fontWeight: '700', color: colors.text },
  actionBtn: {
    borderRadius: 12,
    paddingVertical: 18,
    alignItems: 'center',
  },
  actionText: { color: '#fff', fontSize: 16, fontWeight: '800' },
  muted: { color: colors.textMuted, fontSize: 14 },
  linkRow: { alignItems: 'center', paddingVertical: spacing.sm },
  linkText: { color: colors.primary, fontWeight: '600' },
});
