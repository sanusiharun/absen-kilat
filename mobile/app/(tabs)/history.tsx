import { Ionicons } from '@expo/vector-icons';
import { useFocusEffect, useRouter } from 'expo-router';
import { useCallback, useRef, useState } from 'react';
import {
  ActivityIndicator,
  Pressable,
  RefreshControl,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { attendanceApi, Attendance } from '@/src/api/endpoints';
import { ApprovalBadge } from '@/src/components/ApprovalBadge';
import { colors, spacing } from '@/src/theme';

const DAY_NAMES = ['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Min'];
const MONTH_NAMES = [
  'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
  'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
];

function fmtDate(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

function fmtTime(iso: string): string {
  return new Date(iso).toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });
}

function fmtDayLabel(d: Date): string {
  return d.toLocaleDateString('id-ID', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });
}

export default function HistoryScreen() {
  const router = useRouter();
  const [month, setMonth] = useState(() => {
    const d = new Date();
    return new Date(d.getFullYear(), d.getMonth(), 1);
  });
  const [selectedDate, setSelectedDate] = useState(() => new Date());
  const [records, setRecords] = useState<Attendance[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [dots, setDots] = useState<Set<string>>(new Set());
  const [monthLoading, setMonthLoading] = useState(false);
  const cacheRef = useRef<Record<string, Set<string>>>({});

  const loadDay = useCallback(async (d: Date) => {
    setLoading(true);
    try {
      const res = await attendanceApi.list(fmtDate(d));
      setRecords(res.records);
    } catch {
      // handled by client
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  // Tandai tanggal yang punya record di bulan yang ditampilkan.
  const loadDots = useCallback(async (m: Date) => {
    const key = `${m.getFullYear()}-${m.getMonth()}`;
    if (cacheRef.current[key]) {
      setDots(cacheRef.current[key]);
      return;
    }
    setMonthLoading(true);
    const found = new Set<string>();
    const now = new Date();
    const year = m.getFullYear();
    const monthIdx = m.getMonth();
    const lastDay = new Date(year, monthIdx + 1, 0).getDate();
    const endDay =
      year === now.getFullYear() && monthIdx === now.getMonth() ? now.getDate() : lastDay;
    try {
      await Promise.all(
        Array.from({ length: endDay }, (_, i) => {
          const dateStr = `${year}-${String(monthIdx + 1).padStart(2, '0')}-${String(i + 1).padStart(2, '0')}`;
          return attendanceApi
            .list(dateStr)
            .then((res) => {
              if (res.records.length > 0) found.add(dateStr);
            })
            .catch(() => {});
        })
      );
      cacheRef.current[key] = found;
      setDots(found);
    } catch {
      // biarkan dots kosong
    } finally {
      setMonthLoading(false);
    }
  }, []);

  // Berjalan saat screen fokus (termasuk kembali dari detail) dan saat
  // selectedDate/month berubah selagi screen fokus.
  useFocusEffect(
    useCallback(() => {
      loadDay(selectedDate);
      loadDots(month);
    }, [loadDay, loadDots, selectedDate, month])
  );

  const shiftMonth = (delta: number) => {
    const m = new Date(month);
    m.setMonth(m.getMonth() + delta);
    setMonth(m);
  };

  const shiftDay = (delta: number) => {
    const d = new Date(selectedDate);
    d.setDate(d.getDate() + delta);
    setSelectedDate(d);
  };

  const year = month.getFullYear();
  const monthIdx = month.getMonth();
  const firstOffset = (new Date(year, monthIdx, 1).getDay() + 6) % 7;
  const daysInMonth = new Date(year, monthIdx + 1, 0).getDate();
  const now = new Date();

  const cells: (Date | null)[] = [
    ...Array.from({ length: firstOffset }, () => null),
    ...Array.from({ length: daysInMonth }, (_, i) => new Date(year, monthIdx, i + 1)),
  ];

  return (
    <SafeAreaView style={styles.safe} edges={['top']}>
      <ScrollView
        style={styles.container}
        contentContainerStyle={styles.content}
        refreshControl={
          <RefreshControl
            refreshing={refreshing}
            onRefresh={() => {
              setRefreshing(true);
              loadDay(selectedDate);
              cacheRef.current = {};
              loadDots(month);
            }}
          />
        }
      >
      <Text style={styles.header}>Riwayat Absen</Text>

      {/* Kalender mini */}
      <View style={styles.card}>
        <View style={styles.monthNav}>
          <Pressable style={styles.navBtn} onPress={() => shiftMonth(-1)} hitSlop={8}>
            <Ionicons name="chevron-back" size={20} color={colors.primary} />
          </Pressable>
          <Text style={styles.monthLabel}>
            {MONTH_NAMES[monthIdx]} {year}
          </Text>
          <Pressable style={styles.navBtn} onPress={() => shiftMonth(1)} hitSlop={8}>
            <Ionicons name="chevron-forward" size={20} color={colors.primary} />
          </Pressable>
        </View>
        {monthLoading ? (
          <ActivityIndicator color={colors.primary} style={{ marginTop: spacing.sm }} />
        ) : null}
        <View style={styles.weekHeader}>
          {DAY_NAMES.map((d) => (
            <Text key={d} style={styles.weekHeaderText}>{d}</Text>
          ))}
        </View>
        <View style={styles.grid}>
          {cells.map((d, i) => {
            if (!d) return <View key={`b${i}`} style={styles.cell} />;
            const dateStr = fmtDate(d);
            const isToday = dateStr === fmtDate(now);
            const isSelected = dateStr === fmtDate(selectedDate);
            const hasRecord = dots.has(dateStr);
            return (
              <Pressable
                key={dateStr}
                style={styles.cell}
                onPress={() => {
                  setSelectedDate(d);
                  if (month.getMonth() !== d.getMonth() || month.getFullYear() !== d.getFullYear()) {
                    setMonth(new Date(d.getFullYear(), d.getMonth(), 1));
                  }
                }}
              >
                <View
                  style={[
                    styles.dayCircle,
                    isToday && styles.dayCircleToday,
                    isSelected && !isToday && styles.dayCircleSelected,
                  ]}
                >
                  <Text
                    style={[
                      styles.dayNum,
                      isToday && { color: '#FFFFFF', fontWeight: '800' },
                      isSelected && !isToday && { color: colors.primary, fontWeight: '700' },
                    ]}
                  >
                    {d.getDate()}
                  </Text>
                </View>
                {hasRecord && !isToday ? <View style={styles.dot} /> : <View style={styles.dotSpacer} />}
              </Pressable>
            );
          })}
        </View>
      </View>

      {/* Navigasi hari */}
      <View style={styles.dayNav}>
        <Pressable style={styles.navBtn} onPress={() => shiftDay(-1)} hitSlop={8}>
          <Ionicons name="chevron-back" size={20} color={colors.primary} />
        </Pressable>
        <Text style={styles.dayLabel}>{fmtDayLabel(selectedDate)}</Text>
        <Pressable style={styles.navBtn} onPress={() => shiftDay(1)} hitSlop={8}>
          <Ionicons name="chevron-forward" size={20} color={colors.primary} />
        </Pressable>
      </View>

      {loading ? (
        <ActivityIndicator size="large" color={colors.primary} style={{ marginTop: spacing.xl }} />
      ) : records.length === 0 ? (
        <View style={styles.empty}>
          <Ionicons name="calendar-outline" size={40} color={colors.border} />
          <Text style={styles.emptyText}>Belum ada absen di tanggal ini.</Text>
        </View>
      ) : (
        records.map((r) => {
          const isIn = r.type === 'check_in';
          const late = r.status === 'late';
          const badgeBg = late ? colors.dangerSoft : isIn ? colors.successSoft : colors.warningSoft;
          const badgeColor = late ? colors.danger : isIn ? colors.success : colors.warning;
          return (
            <Pressable key={r.id} style={styles.recordCard} onPress={() => router.push(`/detail?id=${r.id}`)}>
              <View style={styles.recordIcon}>
                <Ionicons
                  name={isIn ? 'arrow-down-circle' : 'arrow-up-circle'}
                  size={22}
                  color={badgeColor}
                />
              </View>
              <View style={styles.recordInfo}>
                <Text style={styles.recordType}>{isIn ? 'Absen Masuk' : 'Absen Keluar'}</Text>
                <Text style={styles.recordTime}>{fmtTime(r.occurred_at)} WIB</Text>
              </View>
              <View style={styles.recordBadges}>
                <View style={[styles.badge, { backgroundColor: badgeBg }]}>
                  <Text style={[styles.badgeText, { color: badgeColor }]}>
                    {late ? 'Terlambat' : 'Tepat Waktu'}
                  </Text>
                </View>
                <ApprovalBadge status={r.approval_status} />
              </View>
            </Pressable>
          );
        })
      )}
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: colors.bg },
  container: { flex: 1, backgroundColor: colors.bg },
  content: { padding: spacing.lg, gap: spacing.md, paddingBottom: spacing.xl },
  header: { fontSize: 24, fontWeight: '800', color: colors.text, marginBottom: spacing.xs },
  card: {
    backgroundColor: colors.card,
    borderRadius: 16,
    padding: spacing.md,
    borderWidth: 1,
    borderColor: colors.border,
  },
  monthNav: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' },
  navBtn: { padding: spacing.xs },
  monthLabel: { fontSize: 15, fontWeight: '700', color: colors.text },
  weekHeader: { flexDirection: 'row', marginTop: spacing.md },
  weekHeaderText: {
    flex: 1,
    textAlign: 'center',
    fontSize: 11,
    fontWeight: '700',
    color: colors.textMuted,
  },
  grid: { flexDirection: 'row', flexWrap: 'wrap', marginTop: spacing.xs },
  cell: {
    width: '14.2857%',
    alignItems: 'center',
    paddingVertical: 4,
  },
  dayCircle: {
    width: 32,
    height: 32,
    borderRadius: 16,
    alignItems: 'center',
    justifyContent: 'center',
  },
  dayCircleToday: { backgroundColor: colors.primary },
  dayCircleSelected: { backgroundColor: colors.primarySoft },
  dayNum: { fontSize: 13, fontWeight: '600', color: colors.text },
  dot: { width: 5, height: 5, borderRadius: 3, backgroundColor: colors.primary, marginTop: 2 },
  dotSpacer: { width: 5, height: 5, marginTop: 2 },
  dayNav: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    backgroundColor: colors.card,
    borderRadius: 14,
    borderWidth: 1,
    borderColor: colors.border,
    paddingHorizontal: spacing.sm,
    paddingVertical: spacing.sm,
  },
  dayLabel: { flex: 1, textAlign: 'center', fontSize: 13, fontWeight: '700', color: colors.text },
  recordCard: {
    backgroundColor: colors.card,
    borderRadius: 14,
    padding: spacing.md,
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    borderWidth: 1,
    borderColor: colors.border,
  },
  recordIcon: {
    width: 40,
    height: 40,
    borderRadius: 12,
    backgroundColor: colors.bg,
    alignItems: 'center',
    justifyContent: 'center',
  },
  recordInfo: { flex: 1 },
  recordType: { fontSize: 15, fontWeight: '700', color: colors.text },
  recordTime: { fontSize: 13, color: colors.textMuted, marginTop: 1 },
  recordBadges: { alignItems: 'flex-end', gap: 4 },
  badge: { paddingHorizontal: spacing.sm, paddingVertical: 4, borderRadius: 999 },
  badgeText: { fontSize: 11, fontWeight: '700' },
  empty: { alignItems: 'center', marginTop: spacing.xl, gap: spacing.sm },
  emptyText: { color: colors.textMuted, fontSize: 14 },
});
