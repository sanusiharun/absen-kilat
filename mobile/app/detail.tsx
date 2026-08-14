import { useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import {
  ActivityIndicator,
  Image,
  Linking,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';

import { attendanceApi, Attendance } from '@/src/api/endpoints';
import { colors, spacing } from '@/src/theme';

export default function DetailScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const [att, setAtt] = useState<Attendance | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!id) return;
    attendanceApi
      .detail(id)
      .then(setAtt)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" color={colors.primary} />
      </View>
    );
  }
  if (!att) {
    return (
      <View style={styles.center}>
        <Text style={styles.muted}>Data absen tidak ditemukan.</Text>
      </View>
    );
  }

  const openMaps = () => {
    if (!att.latitude || !att.longitude) return;
    const url = `https://www.google.com/maps?q=${att.latitude},${att.longitude}`;
    Linking.openURL(url).catch(() => {});
  };

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {att.photo_url ? (
        <Image source={{ uri: att.photo_url }} style={styles.photo} />
      ) : (
        <View style={[styles.photo, styles.noPhoto]}>
          <Text style={styles.muted}>Tidak ada foto</Text>
        </View>
      )}

      <View style={styles.card}>
        <Text style={styles.type}>
          {att.type === 'check_in' ? '⬇️ Absen Masuk' : '⬆️ Absen Keluar'}
        </Text>
        <Text
          style={[
            styles.status,
            { color: att.status === 'on_time' ? colors.success : colors.warning },
          ]}
        >
          {att.status === 'on_time' ? 'Tepat waktu' : 'Terlambat'}
        </Text>
        <Text style={styles.time}>
          🕐 {new Date(att.occurred_at).toLocaleString('id-ID', { dateStyle: 'full', timeStyle: 'short' })}
        </Text>
      </View>

      {att.latitude && att.longitude ? (
        <Pressable style={styles.card} onPress={openMaps}>
          <Text style={styles.cardTitle}>📍 Lokasi</Text>
          <Text style={styles.muted}>
            {att.latitude.toFixed(6)}, {att.longitude.toFixed(6)}
          </Text>
          <Text style={styles.link}>Buka di Google Maps →</Text>
        </Pressable>
      ) : null}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  content: { padding: spacing.lg, gap: spacing.md },
  center: { flex: 1, justifyContent: 'center', alignItems: 'center', backgroundColor: colors.bg },
  photo: { height: 320, borderRadius: 14, backgroundColor: colors.card, resizeMode: 'cover' },
  noPhoto: { alignItems: 'center', justifyContent: 'center' },
  card: {
    backgroundColor: colors.card,
    borderRadius: 14,
    padding: spacing.lg,
    gap: spacing.sm,
    borderWidth: 1,
    borderColor: colors.border,
  },
  cardTitle: { fontSize: 15, fontWeight: '700', color: colors.text },
  type: { fontSize: 18, fontWeight: '800', color: colors.text },
  status: { fontSize: 15, fontWeight: '700' },
  time: { fontSize: 14, color: colors.text },
  muted: { fontSize: 13, color: colors.textMuted },
  link: { color: colors.primary, fontWeight: '600', fontSize: 14 },
});
