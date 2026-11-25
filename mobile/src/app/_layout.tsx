import { Stack } from 'expo-router';
import { I18nProvider } from '../i18n';

export default function RootLayout() {
  return (
    <I18nProvider>
      <Screens />
    </I18nProvider>
  );
}

// Titles and the Back label are set by each screen (useHeaderText) so they follow the chosen language.
function Screens() {
  return (
    <Stack
      screenOptions={{
        headerTransparent: false,
        headerStyle: { backgroundColor: '#fff' },
        headerShadowVisible: true,
      }}
    >
      <Stack.Screen name="index" options={{ headerShown: false }} />
      <Stack.Screen name="events/index" />
      <Stack.Screen name="events/[id]" />
      <Stack.Screen name="scan" />
      <Stack.Screen name="result" />
      <Stack.Screen name="search" />
    </Stack>
  );
}
