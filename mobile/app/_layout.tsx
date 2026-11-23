import {
  PlusJakartaSans_400Regular,
  PlusJakartaSans_500Medium,
  PlusJakartaSans_600SemiBold,
  PlusJakartaSans_700Bold,
  PlusJakartaSans_800ExtraBold,
  useFonts,
} from '@expo-google-fonts/plus-jakarta-sans';
import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { useEffect } from 'react';
import { Text, TextInput } from 'react-native';

import { colors } from '@/src/theme';
import { installWebAlertShim } from '@/src/lib/alertShim';

// react-native-web has no Alert implementation; install the shim before any
// screen mounts so "Berhasil"/"Gagal" dialogs are visible on the web build.
installWebAlertShim();

// Terapkan Plus Jakarta Sans sebagai font default semua Text/TextInput
// setelah font selesai dimuat (fontFamily bawaan RN tidak punya PJS).
function applyDefaultFont() {
  try {
    const FONT = 'PlusJakartaSans_400Regular';
    const setDefault = <T,>(Comp: T) => {
      const C = Comp as { defaultProps?: { style?: unknown } };
      const prev = C.defaultProps?.style;
      C.defaultProps = { ...C.defaultProps, style: [{ fontFamily: FONT }, prev] };
    };
    setDefault(Text);
    setDefault(TextInput);
  } catch {
    // gagal pasang default — biarkan font system
  }
}

export default function RootLayout() {
  const [fontsLoaded] = useFonts({
    PlusJakartaSans_400Regular,
    PlusJakartaSans_500Medium,
    PlusJakartaSans_600SemiBold,
    PlusJakartaSans_700Bold,
    PlusJakartaSans_800ExtraBold,
  });

  useEffect(() => {
    if (fontsLoaded) applyDefaultFont();
  }, [fontsLoaded]);

  if (!fontsLoaded) return null;

  return (
    <>
      <StatusBar style="dark" />
      <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: colors.bg } }}>
        <Stack.Screen name="(auth)" />
        <Stack.Screen name="(tabs)" />
        <Stack.Screen
          name="detail"
          options={{
            headerShown: true,
            title: 'Detail Absen',
            headerTintColor: colors.primary,
            headerTitleStyle: { fontWeight: '700', color: colors.text, fontFamily: 'PlusJakartaSans_700Bold' },
            headerStyle: { backgroundColor: '#FFFFFF' },
            headerShadowVisible: false,
          }}
        />
      </Stack>
    </>
  );
}
