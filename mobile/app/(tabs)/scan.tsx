import { Ionicons } from '@expo/vector-icons';
import { useRouter } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { colors, spacing } from '@/src/theme';

// Tab "Scan" hanya sebagai pintasan cepat — tombol tab sudah diarahkan ke
// /checkin. Screen ini dipakai sebagai fallback (mis. deep link).
export default function ScanScreen() {
  const router = useRouter();

  return (
    <View style={styles.container}>
      <View style={styles.iconCircle}>
        <Ionicons name="finger-print" size={52} color="#FFFFFF" />
      </View>
      <Text style={styles.title}>Absen Sekarang</Text>
      <Text style={styles.subtitle}>
        Mulai absen masuk atau keluar dengan foto wajah. Pastikan wajah berada di dalam bingkai
        dan izinkan akses lokasi.
      </Text>
      <Pressable style={styles.button} onPress={() => router.push('/checkin')}>
        <Text style={styles.buttonText}>Mulai Absen</Text>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.bg,
    alignItems: 'center',
    justifyContent: 'center',
    padding: spacing.lg,
    gap: spacing.md,
  },
  iconCircle: {
    width: 104,
    height: 104,
    borderRadius: 52,
    backgroundColor: colors.primary,
    alignItems: 'center',
    justifyContent: 'center',
    shadowColor: colors.primaryDark,
    shadowOpacity: 0.35,
    shadowRadius: 14,
    shadowOffset: { width: 0, height: 6 },
    elevation: 8,
  },
  title: { fontSize: 24, fontWeight: '800', color: colors.text },
  subtitle: { fontSize: 14, color: colors.textMuted, textAlign: 'center', lineHeight: 20 },
  button: {
    marginTop: spacing.md,
    backgroundColor: colors.primary,
    borderRadius: 14,
    paddingVertical: 15,
    paddingHorizontal: 40,
  },
  buttonText: { color: '#FFFFFF', fontSize: 16, fontWeight: '700' },
});
