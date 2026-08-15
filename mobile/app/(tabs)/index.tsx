import { Ionicons } from '@expo/vector-icons';
import { useFocusEffect, useRouter } from 'expo-router';
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
import { SafeAreaView } from 'react-native-safe-area-context';

import {
  attendanceApi,
  Attendance,
  TodayState,
  AbsenLocation,
  locationApi,
} from '@/src/api/endpoints';
import { Avatar } from '@/src/components/Avatar';
import { LocationPicker } from '@/src/components/LocationPicker';
import { useProfile } from '@/src/hooks/useProfile';
import { usePublicSettings } from '@/src/hooks/usePublicSettings';
import { colors, spacing } from '@/src/theme';
import {
  clearAbsenLocation,
  getSavedAbsenLocation,
  saveAbsenLocation,
  LocationChoice,
} from '@/src/utils/absenLocation';

function fmtTime(iso?: string): string {
  if (!iso) return '--:--';
  return new Date(iso).toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });
}

function statusBadge(rec?: Attendance) {
  if (!rec) {
    return { label: 'Belum', bg: '#F1F5F9', color: colors.textMuted };
  }
  if (rec.status === 'on_time') {
    return { label: 'Tepat Waktu', bg: colors.successSoft, color: colors.success };
  }
  return { label: 'Terlambat', bg: colors.dangerSoft, color: colors.danger };
}

const DAY_NAMES = ['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Min'];

export default function HomeScreen() {
  const router = useRouter();
  const { profile } = useProfile();
  const settings = usePublicSettings();
  const [today, setToday] = useState<TodayState | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  // FASE B: dialog "Absen Dimana?" — pilih lokasi dari GET /locations.
  const [pickerVisible, setPickerVisible] = useState(false);
  const [locations, setLocations] = useState<AbsenLocation[]>([]);
  const [locationsLoading, setLocationsLoading] = useState(false);
  const [savedLocation, setSavedLocation] = useState<LocationChoice | null>(null);
  const [pendingTarget, setPendingTarget] = useState<'checkin' | 'checkout'>('checkin');

  const load = useCallback(async () => {
    try {
      setToday(await attendanceApi.today());
    } catch {
      // auth errors handled by client; user redirected on next mount
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

  // Tekan CTA absen: tampilkan pilihan lokasi. Kalau GET /locations kosong atau
  // gagal → fallback alur absen biasa (tanpa pilihan lokasi).
  const openAbsen = useCallback(
    async (target: 'checkin' | 'checkout') => {
      setPendingTarget(target);
      try {
        setSavedLocation(await getSavedAbsenLocation());
      } catch {
        setSavedLocation(null);
      }
      setLocationsLoading(true);
      try {
        const res = await locationApi.list();
        if (res.locations.length === 0) {
          router.push(target === 'checkin' ? '/checkin' : '/checkout');
          return;
        }
        setLocations(res.locations);
        setPickerVisible(true);
      } catch {
        // /locations belum tersedia (backend masih dikerjakan) — absen biasa.
        router.push(target === 'checkin' ? '/checkin' : '/checkout');
      } finally {
        setLocationsLoading(false);
      }
    },
    [router]
  );

  const onPickLocation = useCallback(
    async (choice: LocationChoice | null) => {
      setPickerVisible(false);
      const target = pendingTarget;
      if (choice) {
        await saveAbsenLocation(choice);
        router.push({
          pathname: target === 'checkin' ? '/checkin' : '/checkout',
          params: {
            location_id: choice.id,
            location_name: choice.name,
            location_type: choice.type ?? '',
          },
        });
      } else {
        await clearAbsenLocation();
        router.push(target === 'checkin' ? '/checkin' : '/checkout');
      }
    },
    [pendingTarget, router]
  );

  if (loading) {
    return (
      <SafeAreaView style={styles.safe} edges={['top']}>
        <View style={styles.center}>
          <ActivityIndicator size="large" color={colors.primary} />
        </View>
      </SafeAreaView>
    );
  }

  const hour = new Date().getHours();
  const greeting =
    hour < 11 ? 'Selamat pagi' : hour < 15 ? 'Selamat siang' : hour < 18 ? 'Selamat sore' : 'Selamat malam';
  const firstName = profile?.name?.trim().split(' ')[0] ?? 'Karyawan';

  const checkInRec = today?.records.find((r) => r.type === 'check_in');
  const checkOutRec = today?.records.find((r) => r.type === 'check_out');
  const inBadge = statusBadge(checkInRec);
  const outBadge = statusBadge(checkOutRec);

  // FASE B: hormati absen_mode dari settings — 'checkin_only' tidak menampilkan
  // CTA check-out; setelah check-in dianggap selesai.
  const checkinOnly = settings?.absen_mode === 'checkin_only';
  const canCheckIn = today ? !today.checked_in : true;
  const canCheckOut = checkinOnly ? false : today ? today.checked_in && !today.checked_out : false;
  const doneToday = today
    ? checkinOnly
      ? today.checked_in
      : today.checked_in && today.checked_out
    : false;

  // Strip mingguan: Senin s/d Minggu, hari ini disorot.
  const weekStart = new Date();
  weekStart.setDate(weekStart.getDate() - ((weekStart.getDay() + 6) % 7));
  const weekDays = Array.from({ length: 7 }, (_, i) => {
    const d = new Date(weekStart);
    d.setDate(weekStart.getDate() + i);
    return d;
  });
  const now = new Date();

  return (
    <SafeAreaView style={styles.safe} edges={['top']}>
      <ScrollView
        style={styles.container}
        contentContainerStyle={styles.content}
        refreshControl={<RefreshControl refreshing={refreshing} onRefresh={() => { setRefreshing(true); load(); }} />}
      >
        {/* Header sapaan */}
        <View style={styles.header}>
          <View style={styles.headerText}>
            <Text style={styles.greeting}>Halo, {firstName} 👋</Text>
            <Text style={styles.greetingSub}>
              {greeting} —{' '}
              {now.toLocaleDateString('id-ID', { weekday: 'long', day: 'numeric', month: 'long' })}
            </Text>
          </View>
          <Avatar size={46} />
        </View>

        {/* Kartu status hari ini */}
        <View style={styles.card}>
          <View style={styles.cardTitleRow}>
            <Text style={styles.cardTitle}>Status Hari Ini</Text>
            <Text style={styles.cardDate}>
              {now.toLocaleDateString('id-ID', { day: 'numeric', month: 'short' })}
            </Text>
          </View>

          <View style={styles.statusRow}>
            <View style={[styles.statusIcon, { backgroundColor: colors.successSoft }]}>
              <Ionicons name="arrow-down-circle" size={22} color={colors.success} />
            </View>
            <View style={styles.statusInfo}>
              <Text style={styles.statusLabel}>Absen Masuk</Text>
              <Text style={styles.statusTime}>{fmtTime(checkInRec?.occurred_at)}</Text>
            </View>
            <View style={[styles.badge, { backgroundColor: inBadge.bg }]}>
              <Text style={[styles.badgeText, { color: inBadge.color }]}>{inBadge.label}</Text>
            </View>
          </View>

          <View style={styles.divider} />

          <View style={styles.statusRow}>
            <View style={[styles.statusIcon, { backgroundColor: colors.warningSoft }]}>
              <Ionicons name="arrow-up-circle" size={22} color={colors.warning} />
            </View>
            <View style={styles.statusInfo}>
              <Text style={styles.statusLabel}>Absen Keluar</Text>
              <Text style={styles.statusTime}>{fmtTime(checkOutRec?.occurred_at)}</Text>
            </View>
            <View style={[styles.badge, { backgroundColor: outBadge.bg }]}>
              <Text style={[styles.badgeText, { color: outBadge.color }]}>{outBadge.label}</Text>
            </View>
          </View>
        </View>

        {/* CTA besar */}
        {doneToday ? (
          <Pressable
            style={[styles.cta, styles.ctaDone]}
            onPress={() => router.push('/(tabs)/history')}
          >
            <View style={styles.ctaIcon}>
              <Ionicons name="checkmark" size={30} color="#FFFFFF" />
            </View>
            <View style={styles.ctaText}>
              <Text style={styles.ctaTitle}>
                {checkinOnly ? 'Absen masuk selesai ✅' : 'Absen hari ini selesai ✅'}
              </Text>
              <Text style={styles.ctaSub}>
                {checkinOnly
                  ? 'Absen masuk sudah tercatat. Terima kasih!'
                  : 'Masuk & keluar sudah tercatat. Terima kasih!'}
              </Text>
            </View>
          </Pressable>
        ) : (
          <Pressable
            style={styles.cta}
            onPress={() => openAbsen(canCheckIn ? 'checkin' : 'checkout')}
          >
            <View style={styles.ctaIcon}>
              <Ionicons name="finger-print" size={30} color="#FFFFFF" />
            </View>
            <View style={styles.ctaText}>
              <Text style={styles.ctaTitle}>Absen Sekarang</Text>
              <Text style={styles.ctaSub}>
                {canCheckIn
                  ? checkinOnly
                    ? 'Absen masuk sekarang'
                    : 'Absen masuk / keluar'
                  : 'Absen keluar sekarang'}
              </Text>
            </View>
            <Ionicons name="chevron-forward" size={22} color="rgba(255,255,255,0.8)" />
          </Pressable>
        )}

        {/* Menu cepat */}
        <View style={styles.quickRow}>
          <Pressable style={styles.quickCard} onPress={() => router.push('/(tabs)/history')}>
            <View style={[styles.quickIcon, { backgroundColor: colors.primarySoft }]}>
              <Ionicons name="calendar-outline" size={22} color={colors.primary} />
            </View>
            <Text style={styles.quickTitle}>Riwayat Absen</Text>
            <Text style={styles.quickSub}>Lihat riwayat</Text>
          </Pressable>
          <Pressable style={styles.quickCard} onPress={() => router.push('/(tabs)/profile')}>
            <View style={[styles.quickIcon, { backgroundColor: colors.primarySoft }]}>
              <Ionicons name="person-outline" size={22} color={colors.primary} />
            </View>
            <Text style={styles.quickTitle}>Profil Saya</Text>
            <Text style={styles.quickSub}>Kelola akun</Text>
          </Pressable>
        </View>

        {/* Strip mingguan */}
        <View style={styles.card}>
          <Text style={styles.cardTitle}>Minggu Ini</Text>
          <View style={styles.weekRow}>
            {weekDays.map((d, i) => {
              const isToday = d.toDateString() === now.toDateString();
              return (
                <View key={i} style={styles.weekCell}>
                  <Text style={[styles.weekDayName, isToday && { color: colors.primary }]}>
                    {DAY_NAMES[i]}
                  </Text>
                  <View style={[styles.weekDayCircle, isToday && styles.weekDayToday]}>
                    <Text style={[styles.weekDayNum, isToday && { color: '#FFFFFF' }]}>
                      {d.getDate()}
                    </Text>
                  </View>
                </View>
              );
            })}
          </View>
        </View>
      </ScrollView>

      <LocationPicker
        visible={pickerVisible}
        locations={locations}
        loading={locationsLoading}
        selectedId={savedLocation?.id ?? null}
        onSelect={onPickLocation}
        onClose={() => setPickerVisible(false)}
      />
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: colors.bg },
  container: { flex: 1, backgroundColor: colors.bg },
  content: { padding: spacing.lg, gap: spacing.md, paddingBottom: spacing.xl },
  center: { flex: 1, justifyContent: 'center', alignItems: 'center', backgroundColor: colors.bg },
  header: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' },
  headerText: { flex: 1, paddingRight: spacing.md },
  greeting: { fontSize: 22, fontWeight: '800', color: colors.text },
  greetingSub: { fontSize: 13, color: colors.textMuted, marginTop: 2 },
  card: {
    backgroundColor: colors.card,
    borderRadius: 16,
    padding: spacing.md,
    borderWidth: 1,
    borderColor: colors.border,
  },
  cardTitleRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: spacing.sm,
  },
  cardTitle: { fontSize: 15, fontWeight: '700', color: colors.text },
  cardDate: { fontSize: 13, color: colors.textMuted },
  statusRow: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm, paddingVertical: spacing.xs },
  statusIcon: {
    width: 40,
    height: 40,
    borderRadius: 12,
    alignItems: 'center',
    justifyContent: 'center',
  },
  statusInfo: { flex: 1 },
  statusLabel: { fontSize: 13, color: colors.textMuted },
  statusTime: { fontSize: 18, fontWeight: '800', color: colors.text, marginTop: 1 },
  badge: {
    paddingHorizontal: spacing.sm,
    paddingVertical: 4,
    borderRadius: 999,
  },
  badgeText: { fontSize: 12, fontWeight: '700' },
  divider: { height: 1, backgroundColor: colors.border, marginVertical: spacing.xs },
  cta: {
    backgroundColor: colors.primary,
    borderRadius: 18,
    padding: spacing.lg,
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    shadowColor: colors.primaryDark,
    shadowOpacity: 0.35,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: 6 },
    elevation: 6,
  },
  ctaDone: { backgroundColor: colors.success },
  ctaIcon: {
    width: 52,
    height: 52,
    borderRadius: 26,
    backgroundColor: 'rgba(255,255,255,0.22)',
    alignItems: 'center',
    justifyContent: 'center',
  },
  ctaText: { flex: 1 },
  ctaTitle: { color: '#FFFFFF', fontSize: 18, fontWeight: '800' },
  ctaSub: { color: 'rgba(255,255,255,0.85)', fontSize: 13, marginTop: 2 },
  quickRow: { flexDirection: 'row', gap: spacing.md },
  quickCard: {
    flex: 1,
    backgroundColor: colors.card,
    borderRadius: 16,
    padding: spacing.md,
    borderWidth: 1,
    borderColor: colors.border,
    gap: spacing.xs,
  },
  quickIcon: {
    width: 42,
    height: 42,
    borderRadius: 12,
    alignItems: 'center',
    justifyContent: 'center',
    marginBottom: spacing.xs,
  },
  quickTitle: { fontSize: 14, fontWeight: '700', color: colors.text },
  quickSub: { fontSize: 12, color: colors.textMuted },
  weekRow: { flexDirection: 'row', justifyContent: 'space-between', marginTop: spacing.sm },
  weekCell: { alignItems: 'center', gap: 6 },
  weekDayName: { fontSize: 12, fontWeight: '600', color: colors.textMuted },
  weekDayCircle: {
    width: 34,
    height: 34,
    borderRadius: 17,
    alignItems: 'center',
    justifyContent: 'center',
  },
  weekDayToday: { backgroundColor: colors.primary },
  weekDayNum: { fontSize: 14, fontWeight: '700', color: colors.text },
});
