// AbsenFlow: alur absen wajah (scan kamera → konfirmasi → kirim).
// Langkah 1 "scan": kamera dengan overlay bingkai wajah, flash, shutter, flip.
// Langkah 2 "confirm": preview foto, lokasi + reverse geocode, detail, catatan.
import { Ionicons } from '@expo/vector-icons';
import { useRouter } from 'expo-router';
import { CameraView, useCameraPermissions } from 'expo-camera';
import * as ImagePicker from 'expo-image-picker';
import * as Location from 'expo-location';
import { useCallback, useEffect, useRef, useState } from 'react';
import {
  ActivityIndicator,
  Image,
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { attendanceApi, geocodeApi, locationApi, AbsenLocation } from '@/src/api/endpoints';
import { Button } from '@/src/components/AuthForm';
import { LOCATION_TYPE_ICON, LOCATION_TYPE_LABEL, LocationPicker } from '@/src/components/LocationPicker';
import { colors, spacing } from '@/src/theme';
import { notify } from '@/src/lib/notify';
import { appendPhoto } from '@/src/lib/photoForm';
import {
  clearAbsenLocation,
  getSavedAbsenLocation,
  saveAbsenLocation,
  LocationChoice,
} from '@/src/utils/absenLocation';
import { compressImage } from '@/src/utils/image';

interface AbsenFlowProps {
  type: 'check_in' | 'check_out';
  // Lokasi terpilih dari Home (params route). Kalau kosong, baca default tersimpan.
  initialLocationId?: string;
  initialLocationName?: string;
  initialLocationType?: string;
}

interface Coords {
  lat: number;
  lng: number;
  accuracy: number;
}

export function AbsenFlow({ type, initialLocationId, initialLocationName, initialLocationType }: AbsenFlowProps) {
  const router = useRouter();
  const cameraRef = useRef<CameraView>(null);
  const [permission, requestPermission] = useCameraPermissions();

  const [step, setStep] = useState<'scan' | 'confirm'>('scan');
  const [facing, setFacing] = useState<'front' | 'back'>('front');
  const [flash, setFlash] = useState<'off' | 'on'>('off');
  const [photoUri, setPhotoUri] = useState<string | null>(null);

  const [coords, setCoords] = useState<Coords | null>(null);
  const [locating, setLocating] = useState(false);
  const [geocoding, setGeocoding] = useState(false);
  const [address, setAddress] = useState('');

  // FASE B: lokasi absen terpilih ("Absen Dimana") + daftar lokasi.
  const [selectedLocation, setSelectedLocation] = useState<LocationChoice | null>(null);
  const [locations, setLocations] = useState<AbsenLocation[]>([]);
  const [locationsLoading, setLocationsLoading] = useState(false);
  const [pickerVisible, setPickerVisible] = useState(false);

  const [notes, setNotes] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  const isCheckIn = type === 'check_in';
  const title = isCheckIn ? 'Absen Masuk' : 'Absen Keluar';
  const now = new Date();

  // Foto WAJIB kalau lokasi type='field' (dinas lapangan). Kalau office/client,
  // foto opsional — backend tetap bisa menolak kalau di luar radius (pesan
  // error server ditampilkan apa adanya). Tanpa pilihan lokasi: pertahankan
  // perilaku lama (foto wajib) karena radius tidak diketahui.
  const photoRequired = selectedLocation ? selectedLocation.type === 'field' : true;

  // Muat lokasi terpilih (dari params Home / default tersimpan) + daftar lokasi.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      let choice: LocationChoice | null = null;
      if (initialLocationId) {
        choice = {
          id: initialLocationId,
          name: initialLocationName ?? '',
          type: (initialLocationType as AbsenLocation['type'] | undefined) ?? undefined,
        };
      } else {
        choice = await getSavedAbsenLocation();
      }
      if (cancelled) return;
      setSelectedLocation(choice);

      setLocationsLoading(true);
      try {
        const res = await locationApi.list();
        if (!cancelled) setLocations(res.locations);
      } catch {
        // /locations belum tersedia — alur tetap jalan tanpa daftar lokasi
      } finally {
        if (!cancelled) setLocationsLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [initialLocationId, initialLocationName, initialLocationType]);

  // Ambil lokasi + reverse geocode saat screen terbuka.
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
        const c = {
          lat: pos.coords.latitude,
          lng: pos.coords.longitude,
          accuracy: pos.coords.accuracy ?? 999,
        };
        setCoords(c);
        setGeocoding(true);
        try {
          const geo = await geocodeApi.reverse(c.lat, c.lng);
          setAddress(geo.short_address || geo.address);
        } catch {
          // geocode gagal — alamat bisa diketik manual
        } finally {
          setGeocoding(false);
        }
      } catch {
        setError('Gagal mengambil lokasi. Coba lagi.');
      } finally {
        setLocating(false);
      }
    })();
  }, []);

  const takePhoto = useCallback(async () => {
    if (!cameraRef.current) return;
    try {
      const photo = await cameraRef.current.takePictureAsync({ quality: 0.7 });
      if (photo) {
        // Kompres foto sebelum ditampilkan & diupload (hemat kuota).
        setPhotoUri(await compressImage(photo.uri));
        setStep('confirm');
      }
    } catch {
      setError('Gagal mengambil foto. Coba lagi.');
    }
  }, []);

  const pickFromGallery = useCallback(async () => {
    const res = await ImagePicker.launchImageLibraryAsync({
      mediaTypes: ImagePicker.MediaTypeOptions.Images,
      quality: 0.7,
    });
    if (!res.canceled && res.assets[0]) {
      // Kompres foto sebelum diupload.
      setPhotoUri(await compressImage(res.assets[0].uri));
      setStep('confirm');
    }
  }, []);

  const retake = () => {
    setPhotoUri(null);
    setError('');
    setStep('scan');
  };

  // Foto opsional (office/client): lanjut ke konfirmasi tanpa foto.
  const skipPhoto = () => {
    setError('');
    setStep('confirm');
  };

  const onPickLocation = async (choice: LocationChoice | null) => {
    setPickerVisible(false);
    setSelectedLocation(choice);
    if (choice) {
      await saveAbsenLocation(choice);
    } else {
      await clearAbsenLocation();
    }
  };

  async function submit() {
    if (photoRequired && !photoUri) {
      setError('Foto wajib diambil untuk absen');
      return;
    }
    setSubmitting(true);
    setError('');
    try {
      const form = new FormData();
      form.append('type', type);
      if (photoUri) {
        // Web must send a real Blob, native sends {uri,name,type}; if this fails
        // the server would store an attendance with no photo at all.
        try {
          await appendPhoto(form, 'photo', photoUri, `absen-${Date.now()}.jpg`);
        } catch {
          setError('Foto gagal diproses. Coba ambil ulang fotonya.');
          setSubmitting(false);
          return;
        }
      }
      if (selectedLocation) form.append('location_id', selectedLocation.id);
      if (coords) {
        form.append('lat', String(coords.lat));
        form.append('lng', String(coords.lng));
      }
      if (address.trim()) form.append('address', address.trim());
      if (notes.trim()) form.append('notes', notes.trim());
      form.append('device', Platform.OS === 'ios' ? 'iOS' : 'Android');

      await attendanceApi.submit(form);
      notify(
        'Berhasil ✅',
        `Absen ${isCheckIn ? 'masuk' : 'keluar'} tercatat. Terima kasih!`,
        () => router.replace('/(tabs)')
      );
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Absen gagal dikirim');
    } finally {
      setSubmitting(false);
    }
  }

  const accuracyGood = coords ? coords.accuracy < 50 : false;

  // ---------- Langkah 1: Scan wajah ----------
  if (step === 'scan') {
    return (
      <SafeAreaView style={styles.scanSafe} edges={['top', 'bottom']}>
        <View style={styles.scanTopBar}>
          <Pressable onPress={() => router.back()} hitSlop={10} style={styles.backBtn}>
            <Ionicons name="chevron-back" size={26} color="#FFFFFF" />
          </Pressable>
          <Text style={styles.scanTitle}>{title}</Text>
          <View style={styles.backBtn} />
        </View>

        {!permission?.granted ? (
          <View style={styles.permissionWrap}>
            <View style={styles.permissionCard}>
              <Ionicons name="camera-outline" size={40} color={colors.primary} />
              <Text style={styles.permissionTitle}>Akses kamera dibutuhkan</Text>
              <Text style={styles.permissionText}>
                Kami memakai kamera hanya untuk foto absenmu.
              </Text>
              <Button title="Izinkan Kamera" onPress={requestPermission} />
            </View>
          </View>
        ) : (
          <View style={styles.cameraWrap}>
            <CameraView
              ref={cameraRef}
              style={StyleSheet.absoluteFill}
              facing={facing}
              flash={flash}
            />
            {/* Scrim gelap + bingkai wajah (lubang transparan) */}
            <View style={styles.scrim} pointerEvents="none" />
            <View style={styles.faceFrame} pointerEvents="none">
              <View style={[styles.corner, styles.cornerTL]} />
              <View style={[styles.corner, styles.cornerTR]} />
              <View style={[styles.corner, styles.cornerBL]} />
              <View style={[styles.corner, styles.cornerBR]} />
            </View>

            {/* Kontrol: flash | shutter | flip */}
            <View style={styles.controls}>
              <Pressable
                style={styles.controlBtn}
                onPress={() => setFlash((f) => (f === 'off' ? 'on' : 'off'))}
                accessibilityLabel="Flash"
              >
                <Ionicons
                  name={flash === 'on' ? 'flash' : 'flash-off'}
                  size={22}
                  color="#FFFFFF"
                />
              </Pressable>
              <Pressable style={styles.shutter} onPress={takePhoto} accessibilityLabel="Ambil foto">
                <View style={styles.shutterInner} />
              </Pressable>
              <Pressable
                style={styles.controlBtn}
                onPress={() => setFacing((f) => (f === 'front' ? 'back' : 'front'))}
                accessibilityLabel="Balik kamera"
              >
                <Ionicons name="camera-reverse" size={24} color="#FFFFFF" />
              </Pressable>
            </View>

            {photoRequired ? (
              <Text style={styles.hint}>Posisikan wajah di dalam bingkai</Text>
            ) : (
              <Pressable style={styles.skipPhotoBtn} onPress={skipPhoto}>
                <Ionicons name="arrow-forward" size={15} color="#FFFFFF" />
                <Text style={styles.skipPhotoText}>Lanjut Tanpa Foto</Text>
              </Pressable>
            )}
            <Pressable style={styles.galleryLink} onPress={pickFromGallery}>
              <Ionicons name="images-outline" size={16} color="rgba(255,255,255,0.9)" />
              <Text style={styles.galleryText}>Pilih dari galeri</Text>
            </Pressable>
          </View>
        )}
      </SafeAreaView>
    );
  }

  // ---------- Langkah 2: Konfirmasi ----------
  return (
    <SafeAreaView style={styles.confirmSafe} edges={['top', 'bottom']}>
      <ScrollView contentContainerStyle={styles.confirmContent}>
        <View style={styles.confirmTopBar}>
          <Pressable onPress={retake} hitSlop={10} style={styles.backBtnDark}>
            <Ionicons name="chevron-back" size={24} color={colors.text} />
          </Pressable>
          <Text style={styles.confirmTitle}>Konfirmasi Absen</Text>
          <View style={styles.backBtnDark} />
        </View>

        {/* Preview foto + edit */}
        <View style={styles.photoCard}>
          {photoUri ? (
            <Image source={{ uri: photoUri }} style={styles.photo} />
          ) : (
            <View style={[styles.photo, styles.noPhoto]}>
              <Text style={styles.muted}>Foto tidak tersedia</Text>
            </View>
          )}
          <View style={styles.photoActions}>
            <Pressable style={styles.editBtn} onPress={retake}>
              <Ionicons name="camera-reverse-outline" size={16} color={colors.primary} />
              <Text style={styles.editBtnText}>Edit Foto</Text>
            </Pressable>
            <Pressable style={styles.editBtn} onPress={pickFromGallery}>
              <Ionicons name="images-outline" size={16} color={colors.primary} />
              <Text style={styles.editBtnText}>Galeri</Text>
            </Pressable>
          </View>
          {!photoRequired && !photoUri ? (
            <Text style={styles.photoHint}>
              Foto opsional untuk lokasi ini — bisa langsung kirim tanpa foto.
            </Text>
          ) : null}
        </View>

        {/* Lokasi absen terpilih ("Absen Dimana") */}
        <View style={styles.card}>
          <View style={styles.cardHeaderRow}>
            <Text style={styles.cardTitle}>Absen Dimana</Text>
            <Pressable style={styles.changeLocBtn} onPress={() => setPickerVisible(true)} hitSlop={8}>
              <Ionicons name="swap-horizontal" size={14} color={colors.primary} />
              <Text style={styles.changeLocText}>Ganti</Text>
            </Pressable>
          </View>
          {selectedLocation ? (
            <View style={styles.locRow}>
              <View style={styles.locIcon}>
                <Ionicons
                  name={LOCATION_TYPE_ICON[selectedLocation.type ?? 'office']}
                  size={18}
                  color={colors.primary}
                />
              </View>
              <View style={styles.locInfo}>
                <Text style={styles.locName}>{selectedLocation.name}</Text>
                <Text style={styles.locType}>
                  {selectedLocation.type ? LOCATION_TYPE_LABEL[selectedLocation.type] : 'Lokasi'}
                </Text>
              </View>
              {photoRequired ? (
                <View style={styles.photoReqBadge}>
                  <Text style={styles.photoReqText}>Foto wajib</Text>
                </View>
              ) : null}
            </View>
          ) : (
            <Text style={styles.muted}>Tidak dipilih — absen tanpa pilihan lokasi.</Text>
          )}
        </View>

        {/* Lokasi */}
        <View style={styles.card}>
          <View style={styles.cardHeaderRow}>
            <Text style={styles.cardTitle}>Lokasi</Text>
            {coords ? (
              <View style={[styles.accBadge, { backgroundColor: accuracyGood ? colors.successSoft : colors.warningSoft }]}>
                <Ionicons
                  name={accuracyGood ? 'checkmark-circle' : 'alert-circle'}
                  size={12}
                  color={accuracyGood ? colors.success : colors.warning}
                />
                <Text style={[styles.accBadgeText, { color: accuracyGood ? colors.success : colors.warning }]}>
                  {accuracyGood ? 'Akurat' : 'Perkiraan'}
                </Text>
              </View>
            ) : null}
          </View>
          {locating ? (
            <View style={styles.row}>
              <ActivityIndicator color={colors.primary} />
              <Text style={styles.muted}>Mengambil lokasi…</Text>
            </View>
          ) : coords ? (
            <>
              {geocoding ? (
                <View style={styles.row}>
                  <ActivityIndicator size="small" color={colors.primary} />
                  <Text style={styles.muted}>Mencari alamat…</Text>
                </View>
              ) : (
                <View style={styles.addrWrap}>
                  <Ionicons name="location" size={16} color={colors.primary} style={{ marginTop: 14 }} />
                  <TextInput
                    style={styles.addrInput}
                    value={address}
                    onChangeText={setAddress}
                    placeholder="Alamat tidak tersedia — ketik manual"
                    placeholderTextColor={colors.textMuted}
                    multiline
                  />
                </View>
              )}
              <Text style={styles.coords}>
                {coords.lat.toFixed(6)}, {coords.lng.toFixed(6)}
              </Text>
            </>
          ) : (
            <Text style={styles.muted}>Lokasi tidak tersedia.</Text>
          )}
        </View>

        {/* Detail */}
        <View style={styles.card}>
          <Text style={styles.cardTitle}>Detail</Text>
          <DetailRow label="Tipe" value={isCheckIn ? 'Absen Masuk' : 'Absen Keluar'} />
          <DetailRow
            label="Tanggal"
            value={now.toLocaleDateString('id-ID', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })}
          />
          <DetailRow
            label="Waktu"
            value={now.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
          />
        </View>

        {/* Catatan */}
        <View style={styles.card}>
          <Text style={styles.cardTitle}>Catatan (opsional)</Text>
          <TextInput
            style={styles.notesInput}
            value={notes}
            onChangeText={setNotes}
            placeholder="Contoh: kerja lembur, dinas luar…"
            placeholderTextColor={colors.textMuted}
            multiline
          />
        </View>

        {error ? <Text style={styles.error}>{error}</Text> : null}

        <Button title="Kirim Absen" onPress={submit} loading={submitting} />
      </ScrollView>

      <LocationPicker
        visible={pickerVisible}
        locations={locations}
        loading={locationsLoading}
        selectedId={selectedLocation?.id ?? null}
        onSelect={onPickLocation}
        onClose={() => setPickerVisible(false)}
      />
    </SafeAreaView>
  );
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <View style={styles.detailRow}>
      <Text style={styles.detailLabel}>{label}</Text>
      <Text style={styles.detailValue}>{value}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  // ----- scan -----
  scanSafe: { flex: 1, backgroundColor: '#0F172A' },
  scanTopBar: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
  },
  backBtn: { width: 40, alignItems: 'flex-start' },
  scanTitle: { color: '#FFFFFF', fontSize: 17, fontWeight: '800' },
  permissionWrap: { flex: 1, backgroundColor: colors.bg, padding: spacing.lg },
  permissionCard: {
    backgroundColor: colors.card,
    borderRadius: 16,
    padding: spacing.lg,
    alignItems: 'center',
    gap: spacing.sm,
    marginTop: spacing.xl,
    borderWidth: 1,
    borderColor: colors.border,
  },
  permissionTitle: { fontSize: 16, fontWeight: '700', color: colors.text, marginTop: spacing.xs },
  permissionText: { fontSize: 13, color: colors.textMuted, textAlign: 'center', marginBottom: spacing.sm },
  cameraWrap: { flex: 1, alignItems: 'center' },
  scrim: {
    ...StyleSheet.absoluteFillObject,
    backgroundColor: 'rgba(15,23,42,0.45)',
  },
  faceFrame: {
    position: 'absolute',
    top: '20%',
    width: 240,
    height: 300,
    borderWidth: 3,
    borderColor: 'rgba(255,255,255,0.95)',
    borderRadius: 28,
  },
  corner: {
    position: 'absolute',
    width: 26,
    height: 26,
    borderColor: colors.primary,
  },
  cornerTL: { top: -3, left: -3, borderTopWidth: 5, borderLeftWidth: 5, borderTopLeftRadius: 28 },
  cornerTR: { top: -3, right: -3, borderTopWidth: 5, borderRightWidth: 5, borderTopRightRadius: 28 },
  cornerBL: { bottom: -3, left: -3, borderBottomWidth: 5, borderLeftWidth: 5, borderBottomLeftRadius: 28 },
  cornerBR: { bottom: -3, right: -3, borderBottomWidth: 5, borderRightWidth: 5, borderBottomRightRadius: 28 },
  controls: {
    position: 'absolute',
    bottom: 40,
    left: 0,
    right: 0,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-evenly',
  },
  controlBtn: {
    width: 50,
    height: 50,
    borderRadius: 25,
    backgroundColor: 'rgba(255,255,255,0.18)',
    alignItems: 'center',
    justifyContent: 'center',
  },
  shutter: {
    width: 74,
    height: 74,
    borderRadius: 37,
    backgroundColor: 'rgba(255,255,255,0.25)',
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 3,
    borderColor: '#FFFFFF',
  },
  shutterInner: { width: 58, height: 58, borderRadius: 29, backgroundColor: '#FFFFFF' },
  hint: {
    position: 'absolute',
    bottom: 130,
    color: 'rgba(255,255,255,0.95)',
    fontSize: 13,
    fontWeight: '600',
    backgroundColor: 'rgba(15,23,42,0.55)',
    paddingHorizontal: spacing.md,
    paddingVertical: 6,
    borderRadius: 999,
    overflow: 'hidden',
  },
  skipPhotoBtn: {
    position: 'absolute',
    bottom: 130,
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
    backgroundColor: colors.primary,
    paddingHorizontal: spacing.md,
    paddingVertical: 10,
    borderRadius: 999,
    shadowColor: colors.primaryDark,
    shadowOpacity: 0.4,
    shadowRadius: 8,
    shadowOffset: { width: 0, height: 4 },
    elevation: 6,
  },
  skipPhotoText: { color: '#FFFFFF', fontSize: 14, fontWeight: '700' },
  galleryLink: {
    position: 'absolute',
    bottom: 20,
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
  },
  galleryText: { color: 'rgba(255,255,255,0.9)', fontSize: 13, fontWeight: '600' },

  // ----- confirm -----
  confirmSafe: { flex: 1, backgroundColor: colors.bg },
  confirmContent: { padding: spacing.lg, gap: spacing.md, paddingBottom: spacing.xl },
  confirmTopBar: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  backBtnDark: { width: 40, alignItems: 'flex-start' },
  confirmTitle: { fontSize: 18, fontWeight: '800', color: colors.text },
  photoCard: { gap: spacing.sm },
  photo: { height: 230, borderRadius: 16, backgroundColor: colors.card, resizeMode: 'cover' },
  noPhoto: { alignItems: 'center', justifyContent: 'center' },
  photoActions: { flexDirection: 'row', gap: spacing.sm },
  editBtn: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
    backgroundColor: colors.primarySoft,
    paddingHorizontal: spacing.md,
    paddingVertical: 10,
    borderRadius: 10,
  },
  editBtnText: { color: colors.primary, fontWeight: '700', fontSize: 13 },
  photoHint: { fontSize: 12, color: colors.textMuted, fontWeight: '600' },
  changeLocBtn: { flexDirection: 'row', alignItems: 'center', gap: 4, padding: 4 },
  changeLocText: { color: colors.primary, fontWeight: '700', fontSize: 13 },
  locRow: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm, marginTop: spacing.xs },
  locIcon: {
    width: 36,
    height: 36,
    borderRadius: 10,
    backgroundColor: colors.primarySoft,
    alignItems: 'center',
    justifyContent: 'center',
  },
  locInfo: { flex: 1 },
  locName: { fontSize: 15, fontWeight: '700', color: colors.text },
  locType: { fontSize: 12, color: colors.textMuted, marginTop: 1 },
  photoReqBadge: {
    backgroundColor: colors.dangerSoft,
    paddingHorizontal: spacing.sm,
    paddingVertical: 4,
    borderRadius: 999,
  },
  photoReqText: { color: colors.danger, fontSize: 11, fontWeight: '700' },
  card: {
    backgroundColor: colors.card,
    borderRadius: 16,
    padding: spacing.md,
    gap: spacing.xs,
    borderWidth: 1,
    borderColor: colors.border,
  },
  cardHeaderRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  cardTitle: { fontSize: 15, fontWeight: '700', color: colors.text },
  accBadge: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4,
    paddingHorizontal: spacing.sm,
    paddingVertical: 4,
    borderRadius: 999,
  },
  accBadgeText: { fontSize: 11, fontWeight: '700' },
  row: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm },
  addrWrap: { flexDirection: 'row', gap: spacing.sm, alignItems: 'flex-start' },
  addrInput: {
    flex: 1,
    fontSize: 14,
    color: colors.text,
    paddingVertical: 10,
    lineHeight: 19,
  },
  coords: { fontSize: 12, color: colors.textMuted, marginLeft: 24 },
  detailRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    paddingVertical: spacing.xs,
  },
  detailLabel: { fontSize: 13, color: colors.textMuted },
  detailValue: { fontSize: 13, fontWeight: '600', color: colors.text, flexShrink: 1, textAlign: 'right' },
  notesInput: {
    backgroundColor: colors.bg,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 12,
    padding: spacing.md,
    fontSize: 14,
    color: colors.text,
    minHeight: 80,
    textAlignVertical: 'top',
  },
  muted: { color: colors.textMuted, fontSize: 14 },
  error: { color: colors.danger, fontSize: 13, fontWeight: '600' },
});
