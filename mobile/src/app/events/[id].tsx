import { useCallback, useState } from 'react';
import { View, Text, Pressable, ActivityIndicator, StyleSheet } from 'react-native';
import { useLocalSearchParams, Stack, router, useFocusEffect } from 'expo-router';
import { getEvent, EventItem } from '../../api';
import { handleApiError } from '../../api/errors';
import ErrorView from '../../components/ErrorView';
import { useI18n } from '../../i18n';
import { useHeaderText } from '../../components/useHeaderText';

export default function EventDashboardScreen() {
  const { t, locale } = useI18n();
  useHeaderText();
  const { id } = useLocalSearchParams<{ id: string }>();
  const [event, setEvent] = useState<EventItem | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    setError('');
    try {
      setEvent(await getEvent(id));
    } catch (err) {
      await handleApiError(err, setError);
    } finally {
      setLoading(false);
    }
  }, [id, locale]);

  // Reloads every time you come back (so "Checked in" stays up to date)
  useFocusEffect(
    useCallback(() => {
      load();
    }, [load])
  );

  if (loading) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" />
      </View>
    );
  }

  if (error) {
    return <ErrorView message={error} onRetry={load} />;
  }

  if (!event) {
    return (
      <View style={styles.center}>
        <Text>{t('Event not found')}</Text>
      </View>
    );
  }

  // Undo is only offered when the organizer allowed it for this account (fake data: always).
  const canUndo = event.canUndo === false ? '0' : '1';

  return (
    <View style={styles.container}>
      <Stack.Screen options={{ title: event.title }} />

      <Text style={styles.detail}>{event.date}</Text>
      {event.venue ? <Text style={styles.detail}>{event.venue}</Text> : null}

      <View style={styles.stats}>
        <View style={styles.statBox}>
          <Text style={styles.statNumber}>{event.registered ?? '—'}</Text>
          <Text style={styles.statLabel}>{t('Registered')}</Text>
        </View>
        <View style={styles.statBox}>
          <Text style={styles.statNumber}>{event.checkedIn ?? '—'}</Text>
          <Text style={styles.statLabel}>{t('Checked in')}</Text>
        </View>
      </View>

      <Pressable style={styles.button} onPress={() => router.push({ pathname: '/scan', params: { eventId: event.id, canUndo } })}>
        <Text style={styles.buttonText}>{t('Scan ticket')}</Text>
      </Pressable>
      <Pressable
        style={[styles.button, styles.secondary]}
        onPress={() => router.push({ pathname: '/search', params: { eventId: event.id, canUndo } })}
      >
        <Text style={[styles.buttonText, styles.secondaryText]}>{t('Search attendee')}</Text>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  center: { flex: 1, justifyContent: 'center', alignItems: 'center' },
  container: { flex: 1, padding: 24, backgroundColor: '#fff' },
  detail: { fontSize: 16, color: '#666', marginBottom: 4 },
  stats: { flexDirection: 'row', gap: 12, marginVertical: 32 },
  statBox: { flex: 1, backgroundColor: '#f0f4ff', borderRadius: 12, padding: 20, alignItems: 'center' },
  statNumber: { fontSize: 32, fontWeight: '700' },
  statLabel: { fontSize: 14, color: '#666', marginTop: 4 },
  button: { backgroundColor: '#1a73e8', padding: 18, borderRadius: 12, marginBottom: 12 },
  buttonText: { color: '#fff', fontSize: 18, fontWeight: '600', textAlign: 'center' },
  secondary: { backgroundColor: '#fff', borderWidth: 2, borderColor: '#1a73e8' },
  secondaryText: { color: '#1a73e8' },
});
