import { Stack } from 'expo-router';
import { I18nProvider, useI18n } from '../i18n';

export default function RootLayout() {
  return (
    <I18nProvider>
      <Screens />
    </I18nProvider>
  );
}

function Screens() {
  const { t } = useI18n();
  return (
    <Stack
      screenOptions={{
        headerTransparent: false,
        headerStyle: { backgroundColor: '#fff' },
        headerShadowVisible: true,
        headerBackTitle: t('Back'),
      }}
    >
      <Stack.Screen name="index" options={{ headerShown: false }} />
      <Stack.Screen name="events/index" options={{ title: t('My Events') }} />
      <Stack.Screen name="events/[id]" options={{ title: t('Event') }} />
      <Stack.Screen name="scan" options={{ title: t('Scan ticket') }} />
      <Stack.Screen name="result" options={{ title: t('Result') }} />
      <Stack.Screen name="search" options={{ title: t('Search attendee') }} />
    </Stack>
  );
}
