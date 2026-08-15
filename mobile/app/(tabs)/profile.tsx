import { Ionicons } from '@expo/vector-icons';
import { useRouter } from 'expo-router';
import * as ImagePicker from 'expo-image-picker';
import { useState } from 'react';
import {
  ActivityIndicator,
  Alert,
  Pressable,
  ScrollView,
  StyleSheet,
  Switch,
  Text,
  TextInput,
  View,
} from 'react-native';

import { profileApi } from '@/src/api/endpoints';
import { clearTokens } from '@/src/api/tokenStorage';
import { Button, ErrorText } from '@/src/components/AuthForm';
import { Avatar } from '@/src/components/Avatar';
import { useProfile } from '@/src/hooks/useProfile';
import { colors, spacing } from '@/src/theme';

type Expanded = 'info' | 'password' | null;

export default function ProfileScreen() {
  const router = useRouter();
  const { profile, loading, refresh } = useProfile();
  const [expanded, setExpanded] = useState<Expanded>(null);
  const [name, setName] = useState('');
  const [notif, setNotif] = useState<boolean | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  // Password change fields
  const [currentPw, setCurrentPw] = useState('');
  const [newPw, setNewPw] = useState('');
  const [pwSaving, setPwSaving] = useState(false);
  const [pwMsg, setPwMsg] = useState('');

  if (loading || !profile) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" color={colors.primary} />
      </View>
    );
  }

  async function saveProfile() {
    setSaving(true);
    setError('');
    try {
      await profileApi.update({
        name: name || undefined,
        notification_enabled: notif ?? profile?.notification_enabled ?? true,
      });
      setName('');
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Gagal menyimpan profil');
    } finally {
      setSaving(false);
    }
  }

  async function pickAvatar() {
    const res = await ImagePicker.launchImageLibraryAsync({
      mediaTypes: ImagePicker.MediaTypeOptions.Images,
      quality: 0.7,
    });
    if (res.canceled || !res.assets[0]) return;
    const form = new FormData();
    form.append('avatar', {
      uri: res.assets[0].uri,
      name: `avatar-${Date.now()}.jpg`,
      type: 'image/jpeg',
    } as unknown as Blob);
    try {
      await profileApi.uploadAvatar(form);
      await refresh();
      Alert.alert('Berhasil', 'Foto profil diperbarui.');
    } catch (e) {
      Alert.alert('Gagal', e instanceof Error ? e.message : 'Upload avatar gagal');
    }
  }

  async function changePassword() {
    setPwMsg('');
    if (!currentPw || !newPw) {
      setPwMsg('Isi password saat ini dan password baru');
      return;
    }
    if (newPw.length < 8) {
      setPwMsg('Password baru minimal 8 karakter');
      return;
    }
    setPwSaving(true);
    try {
      await profileApi.changePassword(currentPw, newPw);
      setCurrentPw('');
      setNewPw('');
      setExpanded(null);
      setPwMsg('✅ Password berhasil diganti');
      setTimeout(() => setPwMsg(''), 2500);
    } catch (e) {
      setPwMsg(e instanceof Error ? e.message : 'Gagal ganti password');
    } finally {
      setPwSaving(false);
    }
  }

  async function logout() {
    await clearTokens();
    router.replace('/(auth)/login');
  }

  const showAbout = () => {
    Alert.alert('Tentang Aplikasi', 'Absen Kilat v1.0.0\n\nAplikasi absensi wajah dengan verifikasi lokasi.');
  };

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {/* Header indigo */}
      <View style={styles.header}>
        <View style={styles.headerDeco1} />
        <View style={styles.headerDeco2} />
        <View style={styles.headerContent}>
          <Avatar size={76} />
          <Text style={styles.name}>{profile.name}</Text>
          <Text style={styles.email}>{profile.email}</Text>
          {profile.is_email_verified ? (
            <View style={styles.verifiedBadge}>
              <Ionicons name="checkmark-circle" size={14} color={colors.success} />
              <Text style={styles.verifiedText}>Akun Terverifikasi</Text>
            </View>
          ) : (
            <View style={[styles.verifiedBadge, { backgroundColor: colors.warningSoft }]}>
              <Ionicons name="time-outline" size={14} color={colors.warning} />
              <Text style={[styles.verifiedText, { color: colors.warning }]}>Belum Verifikasi</Text>
            </View>
          )}
        </View>
      </View>

      {/* Menu list */}
      <View style={styles.menuCard}>
        <MenuRow
          icon="person-outline"
          label="Informasi Pribadi"
          onPress={() => setExpanded(expanded === 'info' ? null : 'info')}
        />
        {expanded === 'info' ? (
          <View style={styles.expanded}>
            <Text style={styles.label}>Nama Lengkap</Text>
            <TextInput
              style={styles.input}
              value={name}
              onChangeText={setName}
              placeholder={profile.name}
              placeholderTextColor={colors.textMuted}
            />
            <Text style={styles.label}>Email</Text>
            <Text style={styles.readonly}>{profile.email}</Text>
            <ErrorText message={error} />
            <Button title="Simpan" onPress={saveProfile} loading={saving} />
          </View>
        ) : null}
        <MenuDivider />

        <MenuRow icon="camera-outline" label="Foto Profil" onPress={pickAvatar} />
        <MenuDivider />

        <View style={styles.switchRow}>
          <View style={styles.menuIcon}>
            <Ionicons name="notifications-outline" size={20} color={colors.text} />
          </View>
          <Text style={styles.menuLabel}>Pengaturan Notifikasi</Text>
          <Switch
            value={notif ?? profile.notification_enabled}
            onValueChange={async (v) => {
              setNotif(v);
              try {
                await profileApi.update({ notification_enabled: v });
                await refresh();
              } catch {
                setNotif(null);
              }
            }}
            trackColor={{ true: colors.primary, false: colors.border }}
            thumbColor="#FFFFFF"
          />
        </View>
        <MenuDivider />

        <MenuRow
          icon="key-outline"
          label="Ubah Kata Sandi"
          onPress={() => setExpanded(expanded === 'password' ? null : 'password')}
        />
        {expanded === 'password' ? (
          <View style={styles.expanded}>
            <Text style={styles.label}>Password Saat Ini</Text>
            <TextInput
              style={styles.input}
              value={currentPw}
              onChangeText={setCurrentPw}
              secureTextEntry
              placeholderTextColor={colors.textMuted}
            />
            <Text style={styles.label}>Password Baru</Text>
            <TextInput
              style={styles.input}
              value={newPw}
              onChangeText={setNewPw}
              secureTextEntry
              placeholderTextColor={colors.textMuted}
            />
            {pwMsg ? <Text style={styles.pwMsg}>{pwMsg}</Text> : null}
            <Button title="Ganti Password" onPress={changePassword} loading={pwSaving} variant="ghost" />
          </View>
        ) : null}
        <MenuDivider />

        <MenuRow icon="information-circle-outline" label="Tentang Aplikasi" onPress={showAbout} />
      </View>

      {/* Logout */}
      <Pressable style={styles.logoutBtn} onPress={logout}>
        <Ionicons name="log-out-outline" size={20} color={colors.danger} />
        <Text style={styles.logoutText}>Keluar</Text>
      </Pressable>
      <Text style={styles.version}>Absen Kilat v1.0.0</Text>
    </ScrollView>
  );
}

function MenuRow({
  icon,
  label,
  onPress,
}: {
  icon: keyof typeof Ionicons.glyphMap;
  label: string;
  onPress: () => void;
}) {
  return (
    <Pressable style={styles.menuRow} onPress={onPress}>
      <View style={styles.menuIcon}>
        <Ionicons name={icon} size={20} color={colors.text} />
      </View>
      <Text style={styles.menuLabel}>{label}</Text>
      <Ionicons name="chevron-forward" size={18} color={colors.textMuted} />
    </Pressable>
  );
}

function MenuDivider() {
  return <View style={styles.menuDivider} />;
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  content: { paddingBottom: spacing.xl },
  center: { flex: 1, justifyContent: 'center', alignItems: 'center', backgroundColor: colors.bg },
  header: {
    backgroundColor: colors.primary,
    borderBottomLeftRadius: 28,
    borderBottomRightRadius: 28,
    paddingHorizontal: spacing.lg,
    paddingTop: spacing.xl,
    paddingBottom: spacing.xl,
    overflow: 'hidden',
  },
  headerDeco1: {
    position: 'absolute',
    top: -40,
    right: -30,
    width: 150,
    height: 150,
    borderRadius: 75,
    backgroundColor: 'rgba(255,255,255,0.12)',
  },
  headerDeco2: {
    position: 'absolute',
    bottom: -50,
    left: -20,
    width: 120,
    height: 120,
    borderRadius: 60,
    backgroundColor: 'rgba(255,255,255,0.08)',
  },
  headerContent: { alignItems: 'center', gap: spacing.xs },
  name: { color: '#FFFFFF', fontSize: 20, fontWeight: '800', marginTop: spacing.sm },
  email: { color: 'rgba(255,255,255,0.85)', fontSize: 13 },
  verifiedBadge: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4,
    backgroundColor: colors.successSoft,
    paddingHorizontal: spacing.sm,
    paddingVertical: 4,
    borderRadius: 999,
    marginTop: spacing.xs,
  },
  verifiedText: { color: colors.success, fontSize: 12, fontWeight: '700' },
  menuCard: {
    marginHorizontal: spacing.lg,
    marginTop: spacing.lg,
    backgroundColor: colors.card,
    borderRadius: 16,
    borderWidth: 1,
    borderColor: colors.border,
    overflow: 'hidden',
  },
  menuRow: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: spacing.md,
    paddingVertical: 15,
    gap: spacing.md,
  },
  menuIcon: {
    width: 36,
    height: 36,
    borderRadius: 10,
    backgroundColor: colors.bg,
    alignItems: 'center',
    justifyContent: 'center',
  },
  menuLabel: { flex: 1, fontSize: 15, fontWeight: '600', color: colors.text },
  menuDivider: { height: 1, backgroundColor: colors.border, marginLeft: 68 },
  switchRow: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: spacing.md,
    paddingVertical: 12,
    gap: spacing.md,
  },
  expanded: {
    paddingHorizontal: spacing.md,
    paddingBottom: spacing.md,
    gap: spacing.xs,
    backgroundColor: colors.bg,
  },
  label: { fontSize: 13, fontWeight: '600', color: colors.text, marginTop: spacing.xs },
  readonly: {
    backgroundColor: colors.card,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 12,
    paddingHorizontal: spacing.md,
    paddingVertical: 13,
    fontSize: 15,
    color: colors.textMuted,
  },
  input: {
    backgroundColor: colors.card,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 12,
    paddingHorizontal: spacing.md,
    paddingVertical: 13,
    fontSize: 15,
    color: colors.text,
  },
  pwMsg: { fontSize: 13, color: colors.text, marginTop: spacing.xs },
  logoutBtn: {
    marginHorizontal: spacing.lg,
    marginTop: spacing.lg,
    backgroundColor: colors.dangerSoft,
    borderRadius: 14,
    paddingVertical: 15,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.sm,
  },
  logoutText: { color: colors.danger, fontSize: 15, fontWeight: '700' },
  version: { textAlign: 'center', color: colors.textMuted, fontSize: 12, marginTop: spacing.md },
});
