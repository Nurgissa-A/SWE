import { useCallback, useState } from 'react';
import { View, Text, FlatList, Pressable, ActivityIndicator, StyleSheet, Alert } from 'react-native';
import { router, Stack, useFocusEffect } from 'expo-router';
import { getMyEvents, EventItem } from '../../api/mock';
import { loadSession, clearSession } from '../../api/session';
import { handleApiError } from '../../api/errors';
import ErrorView from '../../components/ErrorView';

export default function MyEventsScreen() {
  const [events, setEvents] = useState<EventItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [signedInAs, setSignedInAs] = useState('');

  const load = useCallback(async () => {
    setError('');
    setLoading(true);
    try {
      const session = await loadSession();
      setSignedInAs(session?.user.email ?? '');
      setEvents(await getMyEvents());
    } catch (err) {
      await handleApiError(err, setError);
    } finally {
      setLoading(false);
    }
  }, []);

  useFocusEffect(
    useCallback(() => {
      load();
    }, [load])
  );

  function confirmSignOut() {
    Alert.alert('Sign out?', 'You will need to sign in again to scan tickets.', [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Sign out',
        style: 'destructive',
        onPress: async () => {
          await clearSession();
          router.replace('/');
        },
      },
    ]);
  }

  const header = (
    <Stack.Screen
      options={{
        headerRight: () => (
          <Pressable onPress={confirmSignOut} hitSlop={10}>
            <Text style={styles.signOut}>Sign out</Text>
          </Pressable>
        ),
      }}
    />
  );

  if (loading) {
    return (
      <View style={styles.center}>
        {header}
        <ActivityIndicator size="large" />
      </View>
    );
  }

  if (error) {
    return (
      <>
        {header}
        <ErrorView message={error} onRetry={load} />
      </>
    );
  }

  return (
    <>
      {header}
      <FlatList
        contentInsetAdjustmentBehavior="automatic"
        style={styles.list}
        contentContainerStyle={styles.listContent}
        data={events}
        keyExtractor={(item) => item.id}
        ListHeaderComponent={signedInAs ? <Text style={styles.signedIn}>Signed in as {signedInAs}</Text> : null}
        ListEmptyComponent={<Text style={styles.empty}>You are not assigned to any events yet.</Text>}
        renderItem={({ item }) => (
          <Pressable style={styles.card} onPress={() => router.push(`/events/${item.id}`)}>
            <Text style={styles.title}>{item.title}</Text>
            <Text style={styles.detail}>{item.date}</Text>
            <Text style={styles.detail}>{item.venue}</Text>
          </Pressable>
        )}
      />
    </>
  );
}

const styles = StyleSheet.create({
  center: { flex: 1, justifyContent: 'center', alignItems: 'center' },
  list: { flex: 1, backgroundColor: '#f5f5f5' },
  listContent: { padding: 16 },
  signedIn: { fontSize: 13, color: '#888', marginBottom: 12 },
  empty: { textAlign: 'center', color: '#888', marginTop: 32, fontSize: 16 },
  card: { backgroundColor: '#fff', padding: 16, borderRadius: 12, marginBottom: 12 },
  title: { fontSize: 18, fontWeight: '600', marginBottom: 4 },
  detail: { fontSize: 14, color: '#666' },
  signOut: { color: '#d93025', fontSize: 16, fontWeight: '500' },
});
