import { Stack } from 'expo-router';

export default function RootLayout() {
  return (
    <Stack
      screenOptions={{
        headerTransparent: false,
        headerStyle: { backgroundColor: '#fff' },
        headerShadowVisible: true,
      }}
    >
      <Stack.Screen name="index" options={{ headerShown: false }} />
      <Stack.Screen name="events/index" options={{ title: 'My Events' }} />
      <Stack.Screen name="events/[id]" options={{ title: 'Event' }} />
      <Stack.Screen name="scan" options={{ title: 'Scan ticket' }} />
      <Stack.Screen name="result" options={{ title: 'Result' }} />
      <Stack.Screen name="search" options={{ title: 'Search attendee' }} />
    </Stack>
  );
}
