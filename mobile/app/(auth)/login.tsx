import { Ionicons } from '@expo/vector-icons';
import { Link, useRouter } from 'expo-router';
import { useState } from 'react';
import { Alert, Pressable, StyleSheet, Text, View } from 'react-native';

import { authApi } from '@/src/api/endpoints';
import { saveTokens } from '@/src/api/tokenStorage';
import { AuthScreen, Button, ErrorText, Field } from '@/src/components/AuthForm';
import { colors, spacing } from '@/src/theme';

export default function LoginScreen() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function onSubmit() {
    setError('');
    if (!email || !password) {
      setError('Email dan kata sandi wajib diisi');
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
    <AuthScreen
      title="Masuk"
      subtitle="Absen masuk/keluar dengan cepat"
      icon={<Ionicons name="lock-closed" size={28} color={colors.primary} />}
    >
      <Field
        label="Email"
        value={email}
        onChangeText={setEmail}
        keyboardType="email-address"
        autoComplete="email"
        placeholder="nama@perusahaan.com"
      />
      <Field
        label="Kata Sandi"
        value={password}
        onChangeText={setPassword}
        secureTextEntry
        autoComplete="current-password"
        placeholder="••••••••"
      />
      <Link href="/(auth)/forgot-password" style={styles.forgot}>
        Lupa kata sandi?
      </Link>
      <ErrorText message={error} />
      <Button title="Masuk" onPress={onSubmit} loading={loading} />

      <View style={styles.orRow}>
        <View style={styles.orLine} />
        <Text style={styles.orText}>atau</Text>
        <View style={styles.orLine} />
      </View>

      <Pressable
        style={styles.googleBtn}
        onPress={() => Alert.alert('Belum Tersedia', 'Masuk dengan Google belum aktif di versi ini.')}
      >
        <Text style={styles.googleLogo}>G</Text>
        <Text style={styles.googleText}>Masuk dengan Google</Text>
      </Pressable>

      <Text style={styles.footer}>
        Belum punya akun?{' '}
        <Link href="/(auth)/register" style={styles.footerLink}>
          Daftar di sini
        </Link>
      </Text>
    </AuthScreen>
  );
}

const styles = StyleSheet.create({
  forgot: {
    alignSelf: 'flex-end',
    color: colors.primary,
    fontWeight: '600',
    fontSize: 13,
    marginTop: -spacing.xs,
  },
  orRow: { flexDirection: 'row', alignItems: 'center', gap: spacing.md, marginTop: spacing.xs },
  orLine: { flex: 1, height: 1, backgroundColor: colors.border },
  orText: { fontSize: 13, color: colors.textMuted },
  googleBtn: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.sm,
    backgroundColor: colors.card,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 12,
    paddingVertical: 14,
  },
  googleLogo: {
    fontSize: 18,
    fontWeight: '800',
    color: '#4285F4',
    backgroundColor: '#FFFFFF',
    width: 22,
    height: 22,
    textAlign: 'center',
    lineHeight: 22,
    overflow: 'hidden',
  },
  googleText: { fontSize: 15, fontWeight: '600', color: colors.text },
  footer: { textAlign: 'center', fontSize: 14, color: colors.textMuted, marginTop: spacing.sm },
  footerLink: { color: colors.primary, fontWeight: '700' },
});
