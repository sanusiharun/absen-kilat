import { Ionicons } from '@expo/vector-icons';
import { Link, useRouter } from 'expo-router';
import { useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import { authApi } from '@/src/api/endpoints';
import { AuthScreen, Button, ErrorText, Field } from '@/src/components/AuthForm';
import { colors, spacing } from '@/src/theme';

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
      <AuthScreen
        title="Cek Email"
        subtitle="Jika email terdaftar, link reset password sudah dikirim. Hubungi admin untuk token reset di versi pilot."
        icon={<Ionicons name="checkmark-circle" size={32} color={colors.success} />}
      >
        <Button title="Kembali ke Login" onPress={() => router.replace('/(auth)/login')} />
      </AuthScreen>
    );
  }

  return (
    <AuthScreen
      title="Lupa Kata Sandi"
      subtitle="Masukkan email kamu, kami kirim tautan reset"
      icon={<Ionicons name="mail-outline" size={28} color={colors.primary} />}
    >
      <Field
        label="Email"
        value={email}
        onChangeText={setEmail}
        keyboardType="email-address"
        autoComplete="email"
        placeholder="nama@perusahaan.com"
      />
      <ErrorText message={error} />
      <Button title="Kirim Tautan Reset" onPress={onSubmit} loading={loading} />
      <View style={{ alignItems: 'center', marginTop: spacing.sm }}>
        <Link href="/(auth)/login" style={styles.backLink}>
          Kembali ke login
        </Link>
      </View>
    </AuthScreen>
  );
}

const styles = StyleSheet.create({
  backLink: { color: colors.primary, fontWeight: '600', fontSize: 14 },
});
