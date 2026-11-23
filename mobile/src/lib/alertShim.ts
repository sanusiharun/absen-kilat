// Web shim for Alert.alert.
//
// react-native-web does not implement Alert: on the web build every
// "Berhasil"/"Gagal" dialog is a silent no-op, which is why applying for leave
// looked like nothing happened. Rather than edit every call site, patch the one
// API once at startup so both old and new screens behave.
//
// Native is untouched: this file does nothing unless Platform.OS === 'web'.
import { Alert, Platform } from 'react-native';

type AlertButton = {
  text?: string;
  style?: 'default' | 'cancel' | 'destructive';
  onPress?: () => void;
};

let installed = false;

export function installWebAlertShim() {
  if (installed || Platform.OS !== 'web' || typeof window === 'undefined') return;
  installed = true;

  const shim = (title?: string, message?: string, buttons?: AlertButton[]) => {
    const body = [title, message].filter(Boolean).join('\n\n');

    // No buttons (or a single one): a plain alert is enough.
    if (!buttons || buttons.length <= 1) {
      window.alert(body);
      buttons?.[0]?.onPress?.();
      return;
    }

    // Two or more buttons: use confirm; run the non-cancel handler on OK.
    const confirmBtn = buttons.find((b) => b.style !== 'cancel') ?? buttons[0];
    const cancelBtn = buttons.find((b) => b.style === 'cancel');
    const ok = window.confirm(body);
    if (ok) confirmBtn?.onPress?.();
    else cancelBtn?.onPress?.();
  };

  (Alert as unknown as { alert: typeof shim }).alert = shim;
}
