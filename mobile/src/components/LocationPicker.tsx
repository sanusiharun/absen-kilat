// Modal "Absen Dimana?" — pilih lokasi absen dari GET /locations.
// Dipakai di Home (saat tekan CTA "Absen Sekarang") dan di alur absen
// (untuk ganti lokasi). onSelect(null) berarti "tanpa lokasi".
import { Ionicons } from '@expo/vector-icons';
import {
  ActivityIndicator,
  Modal,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { AbsenLocation } from '@/src/api/endpoints';
import { colors, spacing } from '@/src/theme';
import { LocationChoice } from '@/src/utils/absenLocation';

export const LOCATION_TYPE_LABEL: Record<AbsenLocation['type'], string> = {
  office: 'Kantor',
  client: 'Client',
  field: 'Lapangan',
};

export const LOCATION_TYPE_ICON: Record<AbsenLocation['type'], keyof typeof Ionicons.glyphMap> = {
  office: 'business-outline',
  client: 'people-outline',
  field: 'globe-outline',
};

interface LocationPickerProps {
  visible: boolean;
  locations: AbsenLocation[];
  loading?: boolean;
  selectedId?: string | null;
  onSelect: (choice: LocationChoice | null) => void;
  onClose: () => void;
}

export function LocationPicker({
  visible,
  locations,
  loading = false,
  selectedId,
  onSelect,
  onClose,
}: LocationPickerProps) {
  const insets = useSafeAreaInsets();

  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={onClose}>
      <View style={styles.backdrop}>
        <Pressable style={StyleSheet.absoluteFill} onPress={onClose} accessibilityLabel="Tutup" />
        <View style={[styles.sheet, { paddingBottom: Math.max(insets.bottom, spacing.md) }]}>
          <View style={styles.grabber} />
          <Text style={styles.title}>Absen Dimana?</Text>
          <Text style={styles.subtitle}>Pilih lokasi absen — pilihan tersimpan sebagai default.</Text>

          {loading ? (
            <ActivityIndicator color={colors.primary} style={{ marginVertical: spacing.lg }} />
          ) : (
            <ScrollView style={styles.list} bounces={false}>
              {locations.map((loc) => {
                const selected = selectedId === loc.id;
                return (
                  <Pressable
                    key={loc.id}
                    style={[styles.row, selected && styles.rowSelected]}
                    onPress={() =>
                      onSelect({ id: loc.id, name: loc.name, type: loc.type })
                    }
                  >
                    <View style={styles.rowIcon}>
                      <Ionicons name={LOCATION_TYPE_ICON[loc.type]} size={20} color={colors.primary} />
                    </View>
                    <View style={styles.rowInfo}>
                      <Text style={styles.rowName}>{loc.name}</Text>
                      <Text style={styles.rowType}>{LOCATION_TYPE_LABEL[loc.type]}</Text>
                    </View>
                    {selected ? (
                      <Ionicons name="checkmark-circle" size={22} color={colors.primary} />
                    ) : null}
                  </Pressable>
                );
              })}

              <Pressable
                style={styles.row}
                onPress={() => onSelect(null)}
              >
                <View style={[styles.rowIcon, { backgroundColor: colors.bg }]}>
                  <Ionicons name="close-circle-outline" size={20} color={colors.textMuted} />
                </View>
                <View style={styles.rowInfo}>
                  <Text style={styles.rowName}>Tanpa pilihan lokasi</Text>
                  <Text style={styles.rowType}>Pakai lokasi GPS biasa</Text>
                </View>
              </Pressable>
            </ScrollView>
          )}

          <Pressable style={styles.cancelBtn} onPress={onClose}>
            <Text style={styles.cancelText}>Batal</Text>
          </Pressable>
        </View>
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  backdrop: {
    flex: 1,
    backgroundColor: 'rgba(15,23,42,0.45)',
    justifyContent: 'flex-end',
  },
  sheet: {
    backgroundColor: colors.card,
    borderTopLeftRadius: 24,
    borderTopRightRadius: 24,
    paddingHorizontal: spacing.lg,
    paddingTop: spacing.sm,
    maxHeight: '75%',
  },
  grabber: {
    alignSelf: 'center',
    width: 40,
    height: 4,
    borderRadius: 2,
    backgroundColor: colors.border,
    marginBottom: spacing.md,
  },
  title: { fontSize: 18, fontWeight: '800', color: colors.text },
  subtitle: { fontSize: 13, color: colors.textMuted, marginTop: 2, marginBottom: spacing.sm },
  list: { flexGrow: 0 },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    paddingVertical: 12,
    borderBottomWidth: 1,
    borderBottomColor: colors.border,
  },
  rowSelected: { backgroundColor: colors.primarySoft, marginHorizontal: -spacing.sm, paddingHorizontal: spacing.sm, borderRadius: 12 },
  rowIcon: {
    width: 38,
    height: 38,
    borderRadius: 10,
    backgroundColor: colors.primarySoft,
    alignItems: 'center',
    justifyContent: 'center',
  },
  rowInfo: { flex: 1 },
  rowName: { fontSize: 15, fontWeight: '700', color: colors.text },
  rowType: { fontSize: 12, color: colors.textMuted, marginTop: 1 },
  cancelBtn: {
    marginTop: spacing.md,
    backgroundColor: colors.bg,
    borderRadius: 14,
    paddingVertical: 14,
    alignItems: 'center',
    borderWidth: 1,
    borderColor: colors.border,
  },
  cancelText: { color: colors.text, fontSize: 15, fontWeight: '700' },
});
