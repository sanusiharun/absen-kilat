import { Link, useRouter } from 'expo-router';
import { useState } from 'react';
import { Text, View } from 'react-native';

import { authApi } from '@/src/api/endpoints';
import { saveTokens } from '@/src/api/tokenStorage';
import { AuthScreen, Button, ErrorText, Field } from '@/src/components/AuthForm';
import { colors } from '@/src/theme';

export default function RegisterScreen() {
  const router = useRouter();
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function onSubmit() {
    setError('');
    if (!name || !email || !password) {
      setError('Semua kolom wajib diisi');
      return;
    }
    if (password.length < 8) {
      setError('Password minimal 8 karakter');
      return;
    }
    setLoading(true);
    try {
      const user = await authApi.register(email.trim(), password, name.trim());
      // Auto-login setelah register sukses.
      const tokens = await authApi.login(email.trim(), password);
      await saveTokens(tokens.access_token, tokens.refresh_token);
      if (!user.is_email_verified) {
        router.replace('/(auth)/verify-email');
      } else {
        router.replace('/(tabs)');
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Pendaftaran gagal');
    } finally {
      setLoading(false);
    }
  }

  return (
    <AuthScreen title="Daftar" subtitle="Buat akun Absen Kilat baru">
      <Field label="Nama" value={name} onChangeText={setName} autoComplete="name" />
      <Field label="Email" value={email} onChangeText={setEmail} keyboardType="email-address" autoComplete="email" />
      <Field label="Password" value={password} onChangeText={setPassword} secureTextEntry autoComplete="new-password" />
      <ErrorText message={error} />
      <Button title="Daftar" onPress={onSubmit} loading={loading} />
      <View style={{ alignItems: 'center', marginTop: 16 }}>
        <Link href="/(auth)/login" style={{ color: colors.primary, fontWeight: '600' }}>
          Sudah punya akun? Masuk
        </Link>
      </View>
    </AuthScreen>
  );
}
