import { Ionicons } from '@expo/vector-icons';
import { Link, useRouter } from 'expo-router';
import { useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import { authApi } from '@/src/api/endpoints';
import { AuthScreen, Button, ErrorText, Field } from '@/src/components/AuthForm';
import { colors, spacing } from '@/src/theme';

export default function ResetPasswordScreen() {
  const router = useRouter();
  const [token, setToken] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function onSubmit() {
    setError('');
    if (!token || !password) {
      setError('Token dan password baru wajib diisi');
      return;
    }
    if (password.length < 8) {
      setError('Password minimal 8 karakter');
      return;
    }
    setLoading(true);
    try {
      await authApi.resetPassword(token.trim(), password);
      router.replace('/(auth)/login');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Reset gagal');
    } finally {
      setLoading(false);
    }
  }

  return (
    <AuthScreen
      title="Reset Password"
      subtitle="Masukkan token reset dan password baru"
      icon={<Ionicons name="key-outline" size={28} color={colors.primary} />}
    >
      <Field label="Token" value={token} onChangeText={setToken} autoCapitalize="none" placeholder="Token dari email" />
      <Field
        label="Password Baru"
        value={password}
        onChangeText={setPassword}
        secureTextEntry
        autoComplete="new-password"
        placeholder="Minimal 8 karakter"
      />
      <ErrorText message={error} />
      <Button title="Reset Password" onPress={onSubmit} loading={loading} />
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
