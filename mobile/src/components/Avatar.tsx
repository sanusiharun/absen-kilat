import { Image, StyleSheet, Text, View } from 'react-native';

import { mediaURL } from '@/src/api/config';
import { useProfile } from '@/src/hooks/useProfile';
import { colors } from '@/src/theme';

interface AvatarProps {
  size?: number;
}

// Avatar menampilkan foto profil jika ada, fallback inisial nama.
export function Avatar({ size = 40 }: AvatarProps) {
  const { profile } = useProfile();
  const avatarUri = mediaURL(profile?.avatar_url);
  const hasAvatar = !!avatarUri;

  return (
    <View style={[styles.container, { width: size, height: size, borderRadius: size / 2 }]}>
      {hasAvatar ? (
        <Image
          source={{ uri: avatarUri }}
          style={{ width: size, height: size, borderRadius: size / 2 }}
        />
      ) : (
        <View style={[styles.fallback, { width: size, height: size, borderRadius: size / 2 }]}>
          <Text style={styles.initialText}>
            {profile?.name?.charAt(0)?.toUpperCase() ?? '?'}
          </Text>
        </View>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { backgroundColor: colors.primary, overflow: 'hidden' },
  fallback: { alignItems: 'center', justifyContent: 'center', backgroundColor: colors.primary },
  initialText: { color: '#fff', fontSize: 18, fontWeight: '800' },
});
