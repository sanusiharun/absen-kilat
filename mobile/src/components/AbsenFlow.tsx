// AbsenFlow: shared check-in / check-out flow (camera → GPS → confirm → submit).
import { useRouter } from 'expo-router';
import { CameraView, useCameraPermissions } from 'expo-camera';
import * as ImagePicker from 'expo-image-picker';
import * as Location from 'expo-location';
import { useCallback, useEffect, useRef, useState } from 'react';
import {
  ActivityIndicator,
  Image,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';

import { attendanceApi } from '@/src/api/endpoints';
import { Button } from '@/src/components/AuthForm';
import { colors, spacing } from '@/src/theme';

interface AbsenFlowProps {
  type: 'check_in' | 'check_out';
}

interface LocationData {
  lat: number;
  lng: number;
}

export function AbsenFlow({ type }: AbsenFlowProps) {
  const router = useRouter();
  const cameraRef = useRef<CameraView>(null);
  const [permission, requestPermission] = useCameraPermissions();
  const [photoUri, setPhotoUri] = useState<string | null>(null);
  const [location, setLocation] = useState<LocationData | null>(null);
  const [locating, setLocating] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  const isCheckIn = type === 'check_in';
  const title = isCheckIn ? 'Absen Masuk' : 'Absen Keluar';

  // Ambil lokasi saat screen terbuka.
  useEffect(() => {
    (async () => {
      setLocating(true);
      try {
        const { status } = await Location.requestForegroundPermissionsAsync();
        if (status !== 'granted') {
          setError('Izin lokasi ditolak. Absen tetap bisa tanpa lokasi.');
          return;
        }
        const pos = await Location.getCurrentPositionAsync({ accuracy: Location.Accuracy.High });
        setLocation({ lat: pos.coords.latitude, lng: pos.coords.longitude });
      } catch {
        setError('Gagal mengambil lokasi. Coba lagi.');
      } finally {
        setLocating(false);
      }
    })();
  }, []);

  const takePhoto = useCallback(async () => {
    if (!cameraRef.current) return;
    const photo = await cameraRef.current.takePictureAsync({ quality: 0.7 });
    if (photo) setPhotoUri(photo.uri);
  }, []);

  const pickPhoto = useCallback(async () => {
    const res = await ImagePicker.launchImageLibraryAsync({
      mediaTypes: ImagePicker.MediaTypeOptions.Images,
      quality: 0.7,
    });
    if (!res.canceled && res.assets[0]) setPhotoUri(res.assets[0].uri);
  }, []);

  async function submit() {
    if (!photoUri) {
      setError('Foto wajib diambil untuk absen');
      return;
    }
    setSubmitting(true);
    setError('');
    try {
      const form = new FormData();
      form.append('type', type);
      form.append('photo', {
        uri: photoUri,
        name: `absen-${Date.now()}.jpg`,
        type: 'image/jpeg',
      } as unknown as Blob);
      if (location) {
        form.append('lat', String(location.lat));
        form.append('lng', String(location.lng));
      }
      await attendanceApi.checkIn(form);
      router.replace('/(tabs)');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Absen gagal dikirim');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <Text style={styles.title}>{title}</Text>

      {!permission?.granted ? (
        <View style={styles.card}>
          <Text style={styles.muted}>Akses kamera dibutuhkan untuk foto absen.</Text>
          <Button title="Izinkan Kamera" onPress={requestPermission} />
        </View>
      ) : photoUri ? (
        <View style={styles.card}>
          <Image source={{ uri: photoUri }} style={styles.preview} />
          <View style={styles.row}>
            <Button title="Retake" onPress={() => setPhotoUri(null)} variant="ghost" />
            <Button title="Pilih dari Galeri" onPress={pickPhoto} variant="ghost" />
          </View>
        </View>
      ) : (
        <View style={styles.card}>
          <CameraView ref={cameraRef} style={styles.camera} facing="front" />
          <View style={styles.row}>
            <Button title="📷 Ambil Foto" onPress={takePhoto} />
            <Button title="Galeri" onPress={pickPhoto} variant="ghost" />
          </View>
        </View>
      )}

      <View style={styles.card}>
        <Text style={styles.cardTitle}>Lokasi</Text>
        {locating ? (
          <View style={styles.row}>
            <ActivityIndicator color={colors.primary} />
            <Text style={styles.muted}>Mengambil lokasi…</Text>
          </View>
        ) : location ? (
          <Text style={styles.muted}>
            📍 {location.lat.toFixed(5)}, {location.lng.toFixed(5)}
          </Text>
        ) : (
          <Text style={styles.muted}>Lokasi tidak tersedia.</Text>
        )}
      </View>

      {error ? <Text style={styles.error}>{error}</Text> : null}

      <Button
        title={`Kirim ${title}`}
        onPress={submit}
        loading={submitting}
      />
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.bg },
  content: { padding: spacing.lg, gap: spacing.md },
  title: { fontSize: 22, fontWeight: '800', color: colors.text },
  card: {
    backgroundColor: colors.card,
    borderRadius: 14,
    padding: spacing.md,
    gap: spacing.md,
    borderWidth: 1,
    borderColor: colors.border,
  },
  camera: { height: 320, borderRadius: 12, overflow: 'hidden' },
  preview: { height: 320, borderRadius: 12, resizeMode: 'cover' },
  row: { flexDirection: 'row', gap: spacing.sm, alignItems: 'center' },
  cardTitle: { fontSize: 15, fontWeight: '700', color: colors.text },
  muted: { color: colors.textMuted, fontSize: 14 },
  error: { color: colors.danger, fontSize: 13 },
});
