import { useLocalSearchParams } from 'expo-router';

import { AbsenFlow } from '@/src/components/AbsenFlow';

export default function CheckOutScreen() {
  const p = useLocalSearchParams<{
    location_id?: string;
    location_name?: string;
    location_type?: string;
  }>();
  return (
    <AbsenFlow
      type="check_out"
      initialLocationId={typeof p.location_id === 'string' ? p.location_id : undefined}
      initialLocationName={typeof p.location_name === 'string' ? p.location_name : undefined}
      initialLocationType={typeof p.location_type === 'string' ? p.location_type : undefined}
    />
  );
}
