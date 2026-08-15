import { Ionicons } from '@expo/vector-icons';
import { useLocalSearchParams, useRouter } from 'expo-router';
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
import { ApprovalBadge } from '@/src/components/ApprovalBadge';
import { mediaURL } from '@/src/api/config';
import { colors, spacing } from '@/src/theme';

export default function DetailScreen() {
  const router = useRouter();
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

  const isIn = att.type === 'check_in';
  const late = att.status === 'late';
  const timeBig = new Date(att.occurred_at).toLocaleTimeString('id-ID', {
    hour: '2-digit',
    minute: '2-digit',
  });
  const dateFull = new Date(att.occurred_at).toLocaleDateString('id-ID', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  });
  const statusColor = late ? colors.danger : colors.success;
  const cardBg = late ? colors.dangerSoft : '#F0FDF4';
  const statusLabel = late ? 'Terlambat' : 'Tepat Waktu';

  // Sumber absen: tampilkan kecil kalau lewat QR / input manual.
  const sourceLabel = att.source === 'qr' ? 'QR' : att.source === 'manual' ? 'Manual' : null;

  const openMaps = () => {
    if (!att.latitude || !att.longitude) return;
    const url = `https://www.google.com/maps?q=${att.latitude},${att.longitude}`;
    Linking.openURL(url).catch(() => {});
  };

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {/* Kartu status besar */}
      <View style={[styles.statusCard, { backgroundColor: cardBg }]}>
        <View style={styles.statusHeader}>
          <View style={[styles.typeIcon, { backgroundColor: late ? colors.dangerSoft : colors.successSoft }]}>
            <Ionicons
              name={isIn ? 'arrow-down-circle' : 'arrow-up-circle'}
              size={22}
              color={statusColor}
            />
          </View>
          <Text style={styles.typeText}>{isIn ? 'Absen Masuk' : 'Absen Keluar'}</Text>
          <ApprovalBadge status={att.approval_status} />
          <View style={[styles.statusBadge, { backgroundColor: late ? colors.danger : colors.success }]}>
            <Text style={styles.statusBadgeText}>{statusLabel}</Text>
          </View>
        </View>
        <Text style={styles.timeBig}>{timeBig}</Text>
        <Text style={styles.dateText}>{dateFull}</Text>
        {att.device ? <Text style={styles.deviceText}>📱 {att.device}</Text> : null}
        {sourceLabel ? <Text style={styles.deviceText}>Via {sourceLabel}</Text> : null}
      </View>

      {/* Foto selfie */}
      {att.photo_url ? (
        <Image source={{ uri: mediaURL(att.photo_url) }} style={styles.photo} />
      ) : (
        <View style={[styles.photo, styles.noPhoto]}>
          <Text style={styles.muted}>Tidak ada foto</Text>
        </View>
      )}

      {/* Lokasi */}
      {att.latitude && att.longitude ? (
        <View style={styles.card}>
          <View style={styles.cardHeaderRow}>
            <Text style={styles.cardTitle}>📍 Lokasi</Text>
            {att.distance_m != null ? (
              <View style={[styles.distBadge, { backgroundColor: att.distance_m < 100 ? colors.successSoft : colors.warningSoft }]}>
                <Text style={[styles.distBadgeText, { color: att.distance_m < 100 ? colors.success : colors.warning }]}>
                  {att.distance_m} m dari kantor
                </Text>
              </View>
            ) : null}
          </View>
          {att.location_name ? <Text style={styles.locName}>📌 {att.location_name}</Text> : null}
          {att.address ? <Text style={styles.address}>{att.address}</Text> : null}
          <Text style={styles.coords}>
            {att.latitude.toFixed(6)}, {att.longitude.toFixed(6)}
          </Text>
          <Pressable style={styles.mapsBtn} onPress={openMaps}>
            <Ionicons name="map-outline" size={15} color={colors.primary} />
            <Text style={styles.mapsBtnText}>Buka di Google Maps</Text>
          </Pressable>
        </View>
      ) : null}

      {/* Catatan (jika ada) */}
      {att.notes ? (
        <View style={styles.card}>
          <Text style={styles.cardTitle}>📝 Catatan</Text>
          <Text style={styles.notes}>{att.notes}</Text>
        </View>
      ) : null}

      <Pressable style={styles.backBtn} onPress={() => router.back()}>
        <Ionicons name="arrow-back" size={16} color={colors.primary} />
        <Text style={styles.backBtnText}>Kembali</Text>
      </Pressable>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  content: { padding: spacing.lg, gap: spacing.md, paddingBottom: spacing.xl },
  center: { flex: 1, justifyContent: 'center', alignItems: 'center', backgroundColor: colors.bg },
  statusCard: {
    borderRadius: 18,
    padding: spacing.lg,
    alignItems: 'center',
    gap: spacing.xs,
    borderWidth: 1,
    borderColor: colors.border,
  },
  statusHeader: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm },
  typeIcon: {
    width: 38,
    height: 38,
    borderRadius: 12,
    alignItems: 'center',
    justifyContent: 'center',
  },
  typeText: { fontSize: 15, fontWeight: '700', color: colors.text, flex: 1 },
  statusBadge: { paddingHorizontal: spacing.sm, paddingVertical: 4, borderRadius: 999 },
  statusBadgeText: { color: '#FFFFFF', fontSize: 11, fontWeight: '700' },
  timeBig: { fontSize: 44, fontWeight: '800', color: colors.text, marginTop: spacing.xs },
  dateText: { fontSize: 13, color: colors.textMuted },
  deviceText: { fontSize: 12, color: colors.textMuted, marginTop: 2 },
  photo: { height: 240, borderRadius: 16, backgroundColor: colors.card, resizeMode: 'cover' },
  noPhoto: { alignItems: 'center', justifyContent: 'center' },
  card: {
    backgroundColor: colors.card,
    borderRadius: 16,
    padding: spacing.md,
    gap: spacing.xs,
    borderWidth: 1,
    borderColor: colors.border,
  },
  cardHeaderRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  cardTitle: { fontSize: 15, fontWeight: '700', color: colors.text },
  distBadge: { paddingHorizontal: spacing.sm, paddingVertical: 4, borderRadius: 999 },
  distBadgeText: { fontSize: 11, fontWeight: '700' },
  locName: { fontSize: 14, fontWeight: '700', color: colors.text, marginTop: spacing.xs },
  address: { fontSize: 14, color: colors.text, lineHeight: 19, marginTop: spacing.xs },
  coords: { fontSize: 12, color: colors.textMuted },
  mapsBtn: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
    marginTop: spacing.xs,
    alignSelf: 'flex-start',
    backgroundColor: colors.primarySoft,
    paddingHorizontal: spacing.md,
    paddingVertical: 8,
    borderRadius: 10,
  },
  mapsBtnText: { color: colors.primary, fontWeight: '700', fontSize: 13 },
  notes: { fontSize: 14, color: colors.text, lineHeight: 19 },
  backBtn: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 6,
    backgroundColor: colors.primarySoft,
    borderRadius: 14,
    paddingVertical: 14,
    marginTop: spacing.xs,
  },
  backBtnText: { color: colors.primary, fontWeight: '700', fontSize: 15 },
  muted: { color: colors.textMuted, fontSize: 14 },
});
