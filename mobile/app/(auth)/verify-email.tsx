import { Link, useLocalSearchParams, useRouter } from 'expo-router';
import { useState } from 'react';
import { Text, View } from 'react-native';

import { authApi } from '@/src/api/endpoints';
import { AuthScreen, Button, ErrorText, Field } from '@/src/components/AuthForm';
import { colors } from '@/src/theme';

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
    <AuthScreen title="Verifikasi Email" subtitle="Masukkan token verifikasi yang dikirim ke email kamu (versi pilot: token = ID akun)">
      <Field label="Token" value={token} onChangeText={setToken} autoCapitalize="none" />
      <ErrorText message={error} />
      <Button title="Verifikasi" onPress={onSubmit} loading={loading} />
      <View style={{ alignItems: 'center', marginTop: 16 }}>
        <Link href="/(tabs)" style={{ color: colors.textMuted }}>
          Lewati dulu
        </Link>
      </View>
    </AuthScreen>
  );
}
