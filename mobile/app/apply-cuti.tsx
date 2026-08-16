import { Ionicons } from '@expo/vector-icons';
import { useRouter } from 'expo-router';
import { useEffect, useState } from 'react';
import {
  ActivityIndicator,
  Alert,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { leaveApi, type LeaveType } from '@/src/api/endpoints';
import { colors, spacing } from '@/src/theme';

function todayISO() {
  return new Date().toISOString().slice(0, 10);
}

export default function ApplyCutiScreen() {
  const router = useRouter();
  const [types, setTypes] = useState<LeaveType[]>([]);
  const [selected, setSelected] = useState<string>('');
  const [start, setStart] = useState(todayISO());
  const [end, setEnd] = useState(todayISO());
  const [extraDays, setExtraDays] = useState('');
  const [reason, setReason] = useState('');
  const [loadingTypes, setLoadingTypes] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    leaveApi
      .types()
      .then((d) => {
        setTypes(d.leave_types);
        if (d.leave_types[0]) setSelected(d.leave_types[0].code);
      })
      .catch((e) => Alert.alert('Gagal', e instanceof Error ? e.message : 'Memuat tipe cuti'))
      .finally(() => setLoadingTypes(false));
  }, []);

  const selectedType = types.find((t) => t.code === selected);
  const needsExtra = selectedType
    ? selectedType.code === 'paternity' || selectedType.code === 'maternity'
    : false;
  const maxExtra = selectedType?.max_extra_days ?? 0;

  const submit = async () => {
    if (!selected) return;
    if (start > end) {
      Alert.alert('Tanggal salah', 'Tanggal mulai tidak boleh setelah tanggal selesai.');
      return;
    }
    const extra = Number(extraDays) || 0;
    if (needsExtra && extra <= 0) {
      Alert.alert('Hari tambahan', `Cuti ini membutuhkan hari tambahan (mis. paternity 3/5/7, maternity 90).`);
      return;
    }
    if (extra > maxExtra) {
      Alert.alert('Kelebihan', `Maksimal tambahan ${maxExtra} hari.`);
      return;
    }
    setSubmitting(true);
    try {
      await leaveApi.apply({
        leave_type: selected,
        start_date: start,
        end_date: end,
        reason: reason.trim() || undefined,
        extra_days: extra > 0 ? extra : undefined,
      });
      Alert.alert('Berhasil', 'Pengajuan cuti dikirim, menunggu persetujuan.', [
        { text: 'OK', onPress: () => router.replace('/cuti') },
      ]);
    } catch (e) {
      Alert.alert('Gagal', e instanceof Error ? e.message : 'Pengajuan gagal');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <SafeAreaView style={styles.safe} edges={['top']}>
      <View style={styles.header}>
        <Pressable onPress={() => router.back()} style={styles.back}>
          <Ionicons name="chevron-back" size={24} color={colors.text} />
        </Pressable>
        <Text style={styles.title}>Ajukan Cuti</Text>
        <View style={{ width: 32 }} />
      </View>

      <ScrollView style={styles.container} contentContainerStyle={styles.content}>
        {loadingTypes ? (
          <ActivityIndicator style={{ marginTop: spacing.xl }} color={colors.primary} />
        ) : (
          <>
            <Text style={styles.label}>Jenis Cuti</Text>
            <View style={styles.typeWrap}>
              {types.map((t) => {
                const active = t.code === selected;
                return (
                  <Pressable
                    key={t.code}
                    style={[styles.typeChip, active && styles.typeChipActive]}
                    onPress={() => setSelected(t.code)}
                  >
                    <Text style={[styles.typeChipText, active && styles.typeChipTextActive]}>
                      {t.name}
                    </Text>
                    {t.default_days > 0 ? (
                      <Text style={[styles.typeChipSub, active && styles.typeChipSubActive]}>
                        {t.default_days} hari
                      </Text>
                    ) : null}
                  </Pressable>
                );
              })}
            </View>

            <Text style={styles.label}>Tanggal Mulai</Text>
            <TextInput
              style={styles.input}
              value={start}
              onChangeText={setStart}
              placeholder="YYYY-MM-DD"
              placeholderTextColor={colors.textMuted}
            />
            <Text style={styles.label}>Tanggal Selesai</Text>
            <TextInput
              style={styles.input}
              value={end}
              onChangeText={setEnd}
              placeholder="YYYY-MM-DD"
              placeholderTextColor={colors.textMuted}
            />

            {needsExtra ? (
              <>
                <Text style={styles.label}>Hari Tambahan</Text>
                <TextInput
                  style={styles.input}
                  value={extraDays}
                  onChangeText={setExtraDays}
                  keyboardType="numeric"
                  placeholder={`Mis. ${selected === 'maternity' ? '90' : '3 / 5 / 7'}`}
                  placeholderTextColor={colors.textMuted}
                />
                <Text style={styles.hint}>Maksimal {maxExtra} hari tambahan.</Text>
              </>
            ) : null}

            <Text style={styles.label}>Alasan</Text>
            <TextInput
              style={[styles.input, styles.textarea]}
              value={reason}
              onChangeText={setReason}
              multiline
              numberOfLines={3}
              placeholder="Keperluan cuti"
              placeholderTextColor={colors.textMuted}
            />

            <Pressable
              style={[styles.submitBtn, submitting && styles.submitDisabled]}
              onPress={submit}
              disabled={submitting}
            >
              {submitting ? (
                <ActivityIndicator color="#FFFFFF" />
              ) : (
                <Text style={styles.submitText}>Kirim Pengajuan</Text>
              )}
            </Pressable>
          </>
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
  back: { width: 32, alignItems: 'flex-start' },
  title: { fontSize: 20, fontWeight: '800', color: colors.text },
  container: { flex: 1 },
  content: { paddingHorizontal: spacing.lg, paddingBottom: spacing.xl, gap: spacing.sm },
  label: { fontSize: 13, fontWeight: '700', color: colors.text, marginTop: spacing.sm },
  input: {
    backgroundColor: colors.card,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 12,
    paddingHorizontal: spacing.md,
    paddingVertical: 13,
    fontSize: 15,
    color: colors.text,
  },
  textarea: { height: 84, textAlignVertical: 'top' },
  typeWrap: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.sm, marginTop: 2 },
  typeChip: {
    backgroundColor: colors.card,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 12,
    paddingHorizontal: spacing.md,
    paddingVertical: 10,
    alignItems: 'center',
    minWidth: 96,
  },
  typeChipActive: { backgroundColor: colors.primary, borderColor: colors.primary },
  typeChipText: { fontSize: 14, fontWeight: '700', color: colors.text },
  typeChipTextActive: { color: '#FFFFFF' },
  typeChipSub: { fontSize: 11, color: colors.textMuted, marginTop: 2 },
  typeChipSubActive: { color: 'rgba(255,255,255,0.85)' },
  hint: { fontSize: 12, color: colors.textMuted, marginTop: 2 },
  submitBtn: {
    marginTop: spacing.lg,
    backgroundColor: colors.primary,
    borderRadius: 14,
    paddingVertical: 15,
    alignItems: 'center',
  },
  submitDisabled: { opacity: 0.6 },
  submitText: { color: '#FFFFFF', fontSize: 15, fontWeight: '800' },
});
