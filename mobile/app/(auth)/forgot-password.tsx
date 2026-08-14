import { Link, useRouter } from 'expo-router';
import { useState } from 'react';
import { Text, View } from 'react-native';

import { authApi } from '@/src/api/endpoints';
import { AuthScreen, Button, ErrorText, Field } from '@/src/components/AuthForm';
import { colors } from '@/src/theme';

export default function ForgotPasswordScreen() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [error, setError] = useState('');
  const [done, setDone] = useState(false);
  const [loading, setLoading] = useState(false);

  async function onSubmit() {
    setError('');
    if (!email) {
      setError('Email wajib diisi');
      return;
    }
    setLoading(true);
    try {
      await authApi.forgotPassword(email.trim());
      setDone(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Gagal mengirim permintaan');
    } finally {
      setLoading(false);
    }
  }

  if (done) {
    return (
      <AuthScreen title="Cek Email" subtitle="Jika email terdaftar, link reset password sudah dikirim. Hubungi admin untuk token reset di versi pilot.">
        <Button title="Kembali ke Login" onPress={() => router.replace('/(auth)/login')} />
      </AuthScreen>
    );
  }

  return (
    <AuthScreen title="Lupa Password" subtitle="Masukkan email untuk reset password">
      <Field label="Email" value={email} onChangeText={setEmail} keyboardType="email-address" autoComplete="email" />
      <ErrorText message={error} />
      <Button title="Kirim Reset" onPress={onSubmit} loading={loading} />
      <View style={{ alignItems: 'center', marginTop: 16 }}>
        <Link href="/(auth)/login" style={{ color: colors.primary, fontWeight: '600' }}>
          Kembali ke Login
        </Link>
      </View>
    </AuthScreen>
  );
}
