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

import { attendanceApi, Attendance } from '@/src/api/endpoints';
import { colors, spacing } from '@/src/theme';

function formatDate(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });
}

export default function HistoryScreen() {
  const router = useRouter();
  const [date, setDate] = useState(() => new Date());
  const [records, setRecords] = useState<Attendance[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  const load = useCallback(async (d: Date) => {
    setLoading(true);
    try {
      const res = await attendanceApi.list(formatDate(d));
      setRecords(res.records);
    } catch {
      // handled by client
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  useEffect(() => {
    load(date);
  }, [date, load]);

  const shiftDay = (delta: number) => {
    const d = new Date(date);
    d.setDate(d.getDate() + delta);
    setDate(d);
  };

  const dateLabel = date.toLocaleDateString('id-ID', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  });

  return (
    <ScrollView
      style={styles.container}
      contentContainerStyle={styles.content}
      refreshControl={
        <RefreshControl refreshing={refreshing} onRefresh={() => { setRefreshing(true); load(date); }} />
      }
    >
      <View style={styles.dateNav}>
        <Pressable style={styles.navBtn} onPress={() => shiftDay(-1)}>
          <Text style={styles.navText}>←</Text>
        </Pressable>
        <Text style={styles.dateLabel}>{dateLabel}</Text>
        <Pressable style={styles.navBtn} onPress={() => shiftDay(1)}>
          <Text style={styles.navText}>→</Text>
        </Pressable>
      </View>

      {loading ? (
        <ActivityIndicator size="large" color={colors.primary} style={{ marginTop: spacing.xl }} />
      ) : records.length === 0 ? (
        <View style={styles.empty}>
          <Text style={styles.emptyText}>Belum ada absen di tanggal ini.</Text>
        </View>
      ) : (
        records.map((r) => (
          <Pressable
            key={r.id}
            style={styles.card}
            onPress={() => router.push(`/detail?id=${r.id}`)}
          >
            <View style={styles.cardHeader}>
              <Text style={styles.typeText}>
                {r.type === 'check_in' ? '⬇️ Masuk' : '⬆️ Keluar'}
              </Text>
              <Text
                style={[
                  styles.statusBadge,
                  { backgroundColor: r.status === 'on_time' ? '#DCFCE7' : '#FEF3C7' },
                ]}
              >
                {r.status === 'on_time' ? 'Tepat waktu' : 'Terlambat'}
              </Text>
            </View>
            <Text style={styles.timeText}>🕐 {formatTime(r.occurred_at)}</Text>
            {r.latitude && r.longitude ? (
              <Text style={styles.muted}>
                📍 {r.latitude.toFixed(5)}, {r.longitude.toFixed(5)}
              </Text>
            ) : null}
          </Pressable>
        ))
      )}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  content: { padding: spacing.lg, gap: spacing.md },
  dateNav: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' },
  navBtn: { padding: spacing.sm },
  navText: { fontSize: 20, color: colors.primary, fontWeight: '800' },
  dateLabel: { fontSize: 14, fontWeight: '700', color: colors.text, flex: 1, textAlign: 'center' },
  card: {
    backgroundColor: colors.card,
    borderRadius: 12,
    padding: spacing.md,
    gap: spacing.xs,
    borderWidth: 1,
    borderColor: colors.border,
  },
  cardHeader: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  typeText: { fontSize: 15, fontWeight: '700', color: colors.text },
  statusBadge: {
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    borderRadius: 999,
    fontSize: 12,
    fontWeight: '700',
    overflow: 'hidden',
  },
  timeText: { fontSize: 14, color: colors.text },
  muted: { fontSize: 13, color: colors.textMuted },
  empty: { alignItems: 'center', marginTop: spacing.xl },
  emptyText: { color: colors.textMuted },
});
