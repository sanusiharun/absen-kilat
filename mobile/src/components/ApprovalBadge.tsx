// Badge kecil status persetujuan absen (approval_status).
// pending → "Menunggu" (oranye), approved → "Disetujui" (hijau), rejected → "Ditolak" (merah).
import { StyleSheet, Text, View } from 'react-native';

import { colors } from '@/src/theme';

type ApprovalStatus = 'pending' | 'approved' | 'rejected';

const CONFIG: Record<ApprovalStatus, { label: string; bg: string; color: string }> = {
  pending: { label: 'Menunggu', bg: colors.warningSoft, color: colors.warning },
  approved: { label: 'Disetujui', bg: colors.successSoft, color: colors.success },
  rejected: { label: 'Ditolak', bg: colors.dangerSoft, color: colors.danger },
};

export function ApprovalBadge({ status }: { status?: ApprovalStatus | null }) {
  if (!status) return null;
  const cfg = CONFIG[status] ?? CONFIG.pending;
  return (
    <View style={[styles.badge, { backgroundColor: cfg.bg }]}>
      <Text style={[styles.text, { color: cfg.color }]}>{cfg.label}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  badge: { paddingHorizontal: 8, paddingVertical: 4, borderRadius: 999, alignSelf: 'flex-start' },
  text: { fontSize: 11, fontWeight: '700' },
});
