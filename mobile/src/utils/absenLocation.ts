// Penyimpanan pilihan lokasi absen ("Absen Dimana") sebagai default hari berikutnya.
// Key: absen_location_id / absen_location_name / absen_location_type.
import AsyncStorage from '@react-native-async-storage/async-storage';

export type AbsenLocationType = 'office' | 'client' | 'field';

export interface LocationChoice {
  id: string;
  name: string;
  type?: AbsenLocationType;
}

const KEY_ID = 'absen_location_id';
const KEY_NAME = 'absen_location_name';
const KEY_TYPE = 'absen_location_type';

export async function getSavedAbsenLocation(): Promise<LocationChoice | null> {
  try {
    const id = await AsyncStorage.getItem(KEY_ID);
    if (!id) return null;
    const name = (await AsyncStorage.getItem(KEY_NAME)) ?? '';
    const type = (await AsyncStorage.getItem(KEY_TYPE)) as AbsenLocationType | null;
    return { id, name, type: type ?? undefined };
  } catch {
    return null;
  }
}

export async function saveAbsenLocation(choice: LocationChoice): Promise<void> {
  try {
    await AsyncStorage.multiSet([
      [KEY_ID, choice.id],
      [KEY_NAME, choice.name],
      [KEY_TYPE, choice.type ?? ''],
    ]);
  } catch {
    // storage gagal — absen tetap jalan tanpa default
  }
}

export async function clearAbsenLocation(): Promise<void> {
  try {
    await AsyncStorage.multiRemove([KEY_ID, KEY_NAME, KEY_TYPE]);
  } catch {
    // abaikan
  }
}
