import { ReactNode } from 'react';
import {
  ActivityIndicator,
  KeyboardAvoidingView,
  Platform,
  Pressable,
  StyleSheet,
  Text,
  TextInput,
  TextInputProps,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { colors, spacing } from '@/src/theme';

interface AuthScreenProps {
  title: string;
  subtitle?: string;
  children: ReactNode;
}

export function AuthScreen({ title, subtitle, children }: AuthScreenProps) {
  return (
    <SafeAreaView style={styles.safe}>
      <KeyboardAvoidingView
        style={styles.flex}
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
      >
        <View style={styles.container}>
          <Text style={styles.brand}>Absen Kilat</Text>
          <Text style={styles.title}>{title}</Text>
          {subtitle ? <Text style={styles.subtitle}>{subtitle}</Text> : null}
          <View style={styles.form}>{children}</View>
        </View>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}

interface FieldProps extends TextInputProps {
  label: string;
}

export function Field({ label, ...props }: FieldProps) {
  return (
    <View style={styles.field}>
      <Text style={styles.label}>{label}</Text>
      <TextInput
        style={styles.input}
        placeholderTextColor={colors.textMuted}
        autoCapitalize="none"
        {...props}
      />
    </View>
  );
}

interface ButtonProps {
  title: string;
  onPress: () => void;
  loading?: boolean;
  variant?: 'primary' | 'ghost' | 'danger';
}

export function Button({ title, onPress, loading, variant = 'primary' }: ButtonProps) {
  const bg =
    variant === 'primary' ? colors.primary : variant === 'danger' ? colors.danger : 'transparent';
  const color =
    variant === 'ghost' ? colors.primary : '#FFFFFF';
  const border = variant === 'ghost' ? { borderWidth: 1, borderColor: colors.primary } : null;

  return (
    <Pressable
      style={[styles.button, { backgroundColor: bg }, border, loading && styles.buttonDisabled]}
      onPress={onPress}
      disabled={loading}
    >
      {loading ? (
        <ActivityIndicator color={color} />
      ) : (
        <Text style={[styles.buttonText, { color }]}>{title}</Text>
      )}
    </Pressable>
  );
}

export function ErrorText({ message }: { message: string }) {
  if (!message) return null;
  return <Text style={styles.error}>{message}</Text>;
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: colors.bg },
  flex: { flex: 1 },
  container: { flex: 1, padding: spacing.lg, justifyContent: 'center' },
  brand: {
    fontSize: 24,
    fontWeight: '800',
    color: colors.primary,
    marginBottom: spacing.xl,
    textAlign: 'center',
  },
  title: { fontSize: 20, fontWeight: '700', color: colors.text },
  subtitle: { fontSize: 14, color: colors.textMuted, marginTop: spacing.xs, marginBottom: spacing.lg },
  form: { marginTop: spacing.lg, gap: spacing.md },
  field: { gap: spacing.xs },
  label: { fontSize: 13, fontWeight: '600', color: colors.text },
  input: {
    backgroundColor: colors.card,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    paddingHorizontal: spacing.md,
    paddingVertical: 12,
    fontSize: 15,
    color: colors.text,
  },
  button: {
    borderRadius: 10,
    paddingVertical: 14,
    alignItems: 'center',
    marginTop: spacing.sm,
  },
  buttonDisabled: { opacity: 0.6 },
  buttonText: { fontSize: 16, fontWeight: '700' },
  error: { color: colors.danger, fontSize: 13, marginTop: spacing.xs },
});
