import { Ionicons } from '@expo/vector-icons';
import { Link, useLocalSearchParams, useRouter } from 'expo-router';
import { useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import { authApi } from '@/src/api/endpoints';
import { AuthScreen, Button, ErrorText, Field } from '@/src/components/AuthForm';
import { colors, spacing } from '@/src/theme';

export default function VerifyEmailScreen() {
  const router = useRouter();
  const params = useLocalSearchParams<{ token?: string }>();
  const [token, setToken] = useState(params.token ?? '');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function onSubmit() {
    setError('');
    if (!token) {
      setError('Token wajib diisi');
      return;
    }
    setLoading(true);
    try {
      await authApi.verifyEmail(token.trim());
      router.replace('/(tabs)');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Verifikasi gagal');
    } finally {
      setLoading(false);
    }
  }

  return (
    <AuthScreen
      title="Verifikasi Email"
      subtitle="Masukkan token verifikasi yang dikirim ke email kamu (versi pilot: token = ID akun)"
      icon={<Ionicons name="shield-checkmark-outline" size={28} color={colors.primary} />}
    >
      <Field
        label="Token"
        value={token}
        onChangeText={setToken}
        autoCapitalize="none"
        placeholder="Masukkan token"
      />
      <ErrorText message={error} />
      <Button title="Verifikasi Email" onPress={onSubmit} loading={loading} />
      <View style={{ alignItems: 'center', marginTop: spacing.sm }}>
        <Link href="/(tabs)" style={styles.skip}>
          Lewati dulu
        </Link>
      </View>
    </AuthScreen>
  );
}

const styles = StyleSheet.create({
  skip: { color: colors.textMuted, fontWeight: '600', fontSize: 14 },
});
