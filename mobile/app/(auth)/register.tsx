import { Ionicons } from '@expo/vector-icons';
import { Link, useRouter } from 'expo-router';
import { useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { authApi } from '@/src/api/endpoints';
import { saveTokens } from '@/src/api/tokenStorage';
import { AuthScreen, Button, ErrorText, Field } from '@/src/components/AuthForm';
import { colors, spacing } from '@/src/theme';

export default function RegisterScreen() {
  const router = useRouter();
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [agreed, setAgreed] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function onSubmit() {
    setError('');
    if (!name || !email || !password || !confirm) {
      setError('Semua kolom wajib diisi');
      return;
    }
    if (password.length < 8) {
      setError('Password minimal 8 karakter');
      return;
    }
    if (password !== confirm) {
      setError('Konfirmasi password tidak cocok');
      return;
    }
    if (!agreed) {
      setError('Anda harus menyetujui Syarat & Ketentuan');
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
    <AuthScreen
      title="Daftar"
      subtitle="Buat akun Absen Kilat baru"
      icon={<Ionicons name="person-add" size={28} color={colors.primary} />}
    >
      <Field
        label="Nama Lengkap"
        value={name}
        onChangeText={setName}
        autoComplete="name"
        placeholder="Nama sesuai identitas"
      />
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
        autoComplete="new-password"
        placeholder="Minimal 8 karakter"
      />
      <Field
        label="Konfirmasi Kata Sandi"
        value={confirm}
        onChangeText={setConfirm}
        secureTextEntry
        autoComplete="new-password"
        placeholder="Ulangi kata sandi"
      />

      <Pressable style={styles.termsRow} onPress={() => setAgreed((a) => !a)}>
        <Ionicons
          name={agreed ? 'checkbox' : 'square-outline'}
          size={22}
          color={agreed ? colors.primary : colors.textMuted}
        />
        <Text style={styles.termsText}>
          Saya menyetujui{' '}
          <Text style={styles.termsLink}>Syarat & Ketentuan</Text> yang berlaku
        </Text>
      </Pressable>

      <ErrorText message={error} />
      <Button title="Daftar" onPress={onSubmit} loading={loading} />

      <Text style={styles.footer}>
        Sudah punya akun?{' '}
        <Link href="/(auth)/login" style={styles.footerLink}>
          Masuk
        </Link>
      </Text>
    </AuthScreen>
  );
}

const styles = StyleSheet.create({
  termsRow: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm, marginTop: spacing.xs },
  termsText: { flex: 1, fontSize: 13, color: colors.textMuted, lineHeight: 18 },
  termsLink: { color: colors.primary, fontWeight: '700' },
  footer: { textAlign: 'center', fontSize: 14, color: colors.textMuted, marginTop: spacing.sm },
  footerLink: { color: colors.primary, fontWeight: '700' },
});
