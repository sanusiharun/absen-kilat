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

export default function ProfileScreen() {
  const router = useRouter();
  const { profile, loading, refresh } = useProfile();
  const [name, setName] = useState('');
  const [notif, setNotif] = useState(true);
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

  const currentName = name || profile.name;

  async function saveProfile() {
    setSaving(true);
    setError('');
    try {
      await profileApi.update({
        name: name || undefined,
        notification_enabled: notif,
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
    setPwSaving(true);
    try {
      await profileApi.changePassword(currentPw, newPw);
      setCurrentPw('');
      setNewPw('');
      setPwMsg('✅ Password berhasil diganti');
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

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <View style={styles.header}>
        <Avatar size={72} />
        <View style={styles.headerText}>
          <Text style={styles.name}>{profile.name}</Text>
          <Text style={styles.email}>{profile.email}</Text>
        </View>
      </View>

      <View style={styles.card}>
        <Text style={styles.cardTitle}>Profil</Text>
        <Text style={styles.label}>Nama</Text>
        <TextInput
          style={styles.input}
          value={currentName}
          onChangeText={setName}
          placeholder={profile.name}
        />
        <Pressable style={styles.avatarBtn} onPress={pickAvatar}>
          <Text style={styles.avatarBtnText}>📷 Ganti Foto Profil</Text>
        </Pressable>
        <View style={styles.switchRow}>
          <Text style={styles.label}>Notifikasi</Text>
          <Switch
            value={notif}
            onValueChange={setNotif}
            trackColor={{ true: colors.primary }}
          />
        </View>
        <ErrorText message={error} />
        <Button title="Simpan Profil" onPress={saveProfile} loading={saving} />
      </View>

      <View style={styles.card}>
        <Text style={styles.cardTitle}>Ganti Password</Text>
        <Text style={styles.label}>Password Saat Ini</Text>
        <TextInput
          style={styles.input}
          value={currentPw}
          onChangeText={setCurrentPw}
          secureTextEntry
        />
        <Text style={styles.label}>Password Baru</Text>
        <TextInput
          style={styles.input}
          value={newPw}
          onChangeText={setNewPw}
          secureTextEntry
        />
        {pwMsg ? <Text style={styles.pwMsg}>{pwMsg}</Text> : null}
        <Button title="Ganti Password" onPress={changePassword} loading={pwSaving} variant="ghost" />
      </View>

      <Button title="Logout" onPress={logout} variant="danger" />
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  content: { padding: spacing.lg, gap: spacing.md },
  center: { flex: 1, justifyContent: 'center', alignItems: 'center', backgroundColor: colors.bg },
  header: { flexDirection: 'row', alignItems: 'center', gap: spacing.md },
  headerText: { flex: 1 },
  name: { fontSize: 20, fontWeight: '800', color: colors.text },
  email: { fontSize: 13, color: colors.textMuted },
  card: {
    backgroundColor: colors.card,
    borderRadius: 14,
    padding: spacing.lg,
    gap: spacing.sm,
    borderWidth: 1,
    borderColor: colors.border,
  },
  cardTitle: { fontSize: 15, fontWeight: '700', color: colors.text },
  label: { fontSize: 13, fontWeight: '600', color: colors.text },
  input: {
    backgroundColor: colors.bg,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    paddingHorizontal: spacing.md,
    paddingVertical: 10,
    fontSize: 15,
    color: colors.text,
  },
  avatarBtn: { paddingVertical: spacing.sm },
  avatarBtnText: { color: colors.primary, fontWeight: '600' },
  switchRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  pwMsg: { fontSize: 13, color: colors.text },
});
