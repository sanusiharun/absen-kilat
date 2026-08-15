// Utilitas gambar: kompresi foto sebelum upload agar hemat kuota & cepat.
import * as ImageManipulator from 'expo-image-manipulator';

// Kompres foto: resize lebar maksimal 720px + kualitas JPEG 0.6.
// Jika gagal, kembalikan URI asli (upload tetap jalan).
export async function compressImage(uri: string): Promise<string> {
  try {
    const result = await ImageManipulator.manipulateAsync(
      uri,
      [{ resize: { width: 720 } }],
      { compress: 0.6, format: ImageManipulator.SaveFormat.JPEG }
    );
    return result.uri;
  } catch {
    return uri;
  }
}
