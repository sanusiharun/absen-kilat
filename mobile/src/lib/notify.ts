// Cross-platform user feedback.
//
// react-native-web does NOT implement Alert.alert (it is a silent no-op), so on
// the web build every "berhasil"/"gagal" dialog simply never appeared. These
// helpers keep one call site for both targets: window.alert/confirm on web,
// Alert.alert on native.
import { Alert, Platform } from 'react-native';

export function notify(title: string, message?: string, onDone?: () => void) {
  if (Platform.OS === 'web') {
    const text = message ? `${title}\n\n${message}` : title;
    // window.alert blocks; run the callback right after so navigation still happens.
    if (typeof window !== 'undefined') window.alert(text);
    onDone?.();
    return;
  }
  Alert.alert(title, message, onDone ? [{ text: 'OK', onPress: onDone }] : undefined);
}

export function notifyError(message: string, title = 'Gagal') {
  notify(title, message);
}

export function confirmAction(
  title: string,
  message: string,
  confirmLabel: string,
  onConfirm: () => void,
  destructive = false
) {
  if (Platform.OS === 'web') {
    const ok = typeof window !== 'undefined' ? window.confirm(`${title}\n\n${message}`) : false;
    if (ok) onConfirm();
    return;
  }
  Alert.alert(title, message, [
    { text: 'Batal', style: 'cancel' },
    { text: confirmLabel, style: destructive ? 'destructive' : 'default', onPress: onConfirm },
  ]);
}
