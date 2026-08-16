import { Ionicons } from '@expo/vector-icons';
import { useRouter } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import {
  ActivityIndicator,
  Alert,
  Pressable,
  RefreshControl,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { leaveApi, type Leave } from '@/src/api/endpoints';
import { colors, spacing } from '@/src/theme';

const TYPE_META: Record<string, { label: string; color: string }> = {
  annual: { label: 'Cuti Tahunan', color: colors.primary },
  sick: { label: 'Sakit', color: colors.warning },
  emergency: { label: 'Darurat', color: colors.danger },
  maternity: { label: 'Melahirkan', color: '#DB2777' },
  paternity: { label: 'Ayah', color: '#0EA5E9' },
};

function typeMeta(code: string) {
  return TYPE_META[code] ?? { label: code, color: colors.textMuted };
}

function statusMeta(status: Leave['status']) {
  switch (status) {
    case 'approved':
      return { label: 'Disetujui', color: colors.success, bg: colors.successSoft };
    case 'rejected':
      return { label: 'Ditolak', color: colors.danger, bg: colors.dangerSoft };
    default:
      return { label: 'Menunggu', color: colors.warning, bg: colors.warningSoft };
  }
}

export default function CutiScreen() {
  const router = useRouter();
  const [leaves, setLeaves] = useState<Leave[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  const load = useCallback(async () => {
    try {
      const d = await leaveApi.list();
      setLeaves(d.leaves);
    } catch (e) {
      Alert.alert('Gagal', e instanceof Error ? e.message : 'Memuat data cuti gagal');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const cancel = (id: string) => {
    Alert.alert('Batalkan Pengajuan', 'Yakin ingin membatalkan cuti ini?', [
      { text: 'Batal', style: 'cancel' },
      {
        text: 'Ya, Batalkan',
        style: 'destructive',
        onPress: async () => {
          try {
            await leaveApi.cancel(id);
            await load();
          } catch (e) {
            Alert.alert('Gagal', e instanceof Error ? e.message : 'Batalkan gagal');
          }
        },
      },
    ]);
  };

  return (
    <SafeAreaView style={styles.safe} edges={['top']}>
      <View style={styles.header}>
        <Text style={styles.title}>Cuti Saya</Text>
        <Pressable style={styles.addBtn} onPress={() => router.push('/apply-cuti')}>
          <Ionicons name="add" size={18} color="#FFFFFF" />
          <Text style={styles.addText}>Ajukan</Text>
        </Pressable>
      </View>

      <ScrollView
        style={styles.container}
        contentContainerStyle={styles.content}
        refreshControl={
          <RefreshControl refreshing={refreshing} onRefresh={load} tintColor={colors.primary} />
        }
      >
        {loading ? (
          <ActivityIndicator style={{ marginTop: spacing.xl }} color={colors.primary} />
        ) : leaves.length === 0 ? (
          <View style={styles.empty}>
            <Ionicons name="calendar-outline" size={42} color={colors.textMuted} />
            <Text style={styles.emptyText}>Belum ada pengajuan cuti</Text>
            <Pressable style={styles.emptyBtn} onPress={() => router.push('/apply-cuti')}>
              <Text style={styles.emptyBtnText}>Ajukan Cuti Sekarang</Text>
            </Pressable>
          </View>
        ) : (
          leaves.map((l) => {
            const tm = typeMeta(l.leave_type);
            const sm = statusMeta(l.status);
            return (
              <View key={l.id} style={styles.card}>
                <View style={styles.cardTop}>
                  <View style={[styles.tag, { backgroundColor: `${tm.color}1A` }]}>
                    <Text style={[styles.tagText, { color: tm.color }]}>{tm.label}</Text>
                  </View>
                  <View style={[styles.statusBadge, { backgroundColor: sm.bg }]}>
                    <Text style={[styles.statusText, { color: sm.color }]}>{sm.label}</Text>
                  </View>
                </View>
                <Text style={styles.range}>
                  {l.start_date} s.d. {l.end_date}
                </Text>
                <Text style={styles.days}>{l.total_days} hari</Text>
                {l.reason ? <Text style={styles.reason}>{l.reason}</Text> : null}
                {l.status === 'rejected' && l.rejection_reason ? (
                  <Text style={styles.reject}>Alasan: {l.rejection_reason}</Text>
                ) : null}
                {l.status === 'pending' ? (
                  <Pressable style={styles.cancelBtn} onPress={() => cancel(l.id)}>
                    <Text style={styles.cancelText}>Batalkan</Text>
                  </Pressable>
                ) : null}
              </View>
            );
          })
        )}
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: colors.bg },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: spacing.lg,
    paddingTop: spacing.lg,
    paddingBottom: spacing.md,
  },
  title: { fontSize: 22, fontWeight: '800', color: colors.text },
  addBtn: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
    backgroundColor: colors.primary,
    paddingHorizontal: spacing.md,
    paddingVertical: 9,
    borderRadius: 12,
  },
  addText: { color: '#FFFFFF', fontWeight: '700', fontSize: 14 },
  container: { flex: 1 },
  content: { paddingHorizontal: spacing.lg, paddingBottom: spacing.xl, gap: spacing.md },
  empty: {
    alignItems: 'center',
    marginTop: spacing.xl * 1.5,
    gap: spacing.sm,
  },
  emptyText: { color: colors.textMuted, fontSize: 15 },
  emptyBtn: {
    marginTop: spacing.sm,
    backgroundColor: colors.primarySoft,
    paddingHorizontal: spacing.lg,
    paddingVertical: 12,
    borderRadius: 12,
  },
  emptyBtnText: { color: colors.primary, fontWeight: '700' },
  card: {
    backgroundColor: colors.card,
    borderRadius: 16,
    borderWidth: 1,
    borderColor: colors.border,
    padding: spacing.md,
    gap: 6,
  },
  cardTop: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' },
  tag: { paddingHorizontal: spacing.sm, paddingVertical: 5, borderRadius: 999 },
  tagText: { fontSize: 13, fontWeight: '700' },
  statusBadge: { paddingHorizontal: spacing.sm, paddingVertical: 5, borderRadius: 999 },
  statusText: { fontSize: 12, fontWeight: '700' },
  range: { fontSize: 15, fontWeight: '700', color: colors.text, marginTop: 2 },
  days: { fontSize: 13, color: colors.textMuted },
  reason: { fontSize: 13, color: colors.text, marginTop: 2 },
  reject: { fontSize: 12, color: colors.danger, marginTop: 2 },
  cancelBtn: {
    alignSelf: 'flex-start',
    marginTop: spacing.xs,
    paddingHorizontal: spacing.md,
    paddingVertical: 8,
    borderRadius: 10,
    borderWidth: 1,
    borderColor: colors.danger,
  },
  cancelText: { color: colors.danger, fontWeight: '700', fontSize: 13 },
});
