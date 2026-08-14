import { Link, useRouter } from 'expo-router';
import { useState } from 'react';
import { Text, View } from 'react-native';

import { authApi } from '@/src/api/endpoints';
import { saveTokens } from '@/src/api/tokenStorage';
import { AuthScreen, Button, ErrorText, Field } from '@/src/components/AuthForm';
import { colors } from '@/src/theme';

export default function LoginScreen() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function onSubmit() {
    setError('');
    if (!email || !password) {
      setError('Email dan password wajib diisi');
      return;
    }
    setLoading(true);
    try {
      const tokens = await authApi.login(email.trim(), password);
      await saveTokens(tokens.access_token, tokens.refresh_token);
      router.replace('/(tabs)');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Login gagal');
    } finally {
      setLoading(false);
    }
  }

  return (
    <AuthScreen title="Masuk" subtitle="Absen masuk/keluar dengan cepat">
      <Field label="Email" value={email} onChangeText={setEmail} keyboardType="email-address" autoComplete="email" />
      <Field label="Password" value={password} onChangeText={setPassword} secureTextEntry autoComplete="current-password" />
      <ErrorText message={error} />
      <Button title="Masuk" onPress={onSubmit} loading={loading} />
      <View style={{ alignItems: 'center', gap: 8, marginTop: 16 }}>
        <Link href="/(auth)/register" style={{ color: colors.primary, fontWeight: '600' }}>
          Belum punya akun? Daftar
        </Link>
        <Link href="/(auth)/forgot-password" style={{ color: colors.textMuted }}>
          Lupa password?
        </Link>
      </View>
    </AuthScreen>
  );
}
