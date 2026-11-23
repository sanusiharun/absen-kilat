// Append an image URI to a FormData body in a way that works on BOTH targets.
//
// Native (iOS/Android): RN's FormData understands `{ uri, name, type }`.
// Web: react-native-web serialises that object to the string "[object Object]",
// so the server sees a text field instead of a file and silently stores the
// attendance WITHOUT a photo. On web we must fetch the URI (blob:/data:/file:)
// and append the real Blob.
import { Platform } from 'react-native';

export async function appendPhoto(form: FormData, field: string, uri: string, filename?: string) {
  const name = filename ?? `photo-${Date.now()}.jpg`;

  if (Platform.OS === 'web') {
    const res = await fetch(uri);
    const blob = await res.blob();
    const ext = (blob.type.split('/')[1] ?? 'jpeg').replace('jpeg', 'jpg');
    form.append(field, blob, name.replace(/\.[a-z]+$/i, `.${ext}`));
    return;
  }

  form.append(field, {
    uri,
    name,
    type: 'image/jpeg',
  } as unknown as Blob);
}
