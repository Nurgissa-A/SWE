import { useCallback, useState } from 'react';
import { View, Text, TextInput, FlatList, Pressable, StyleSheet } from 'react-native';
import { router, useLocalSearchParams, useFocusEffect } from 'expo-router';
import { searchAttendees, AttendeeItem } from '../api';
import { handleApiError } from '../api/errors';
import { useI18n } from '../i18n';

const STATUS_LOOK: Record<string, { label: string; color: string }> = {
  valid: { label: 'Not checked in', color: '#666' }, // labels are translated when shown
  checked_in: { label: 'Checked in', color: '#1e8e3e' },
  cancelled: { label: 'Cancelled', color: '#d93025' },
  refunded: { label: 'Refunded', color: '#d93025' },
};

export default function SearchScreen() {
  const { t } = useI18n();
  const { eventId, canUndo } = useLocalSearchParams<{ eventId: string; canUndo?: string }>();
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<AttendeeItem[]>([]);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    setError('');
    try {
      setResults(await searchAttendees(eventId, query));
    } catch (err) {
      await handleApiError(err, setError);
    }
  }, [eventId, query]);

  // Runs when the screen opens, when you come back to it, and when the query changes
  useFocusEffect(
    useCallback(() => {
      load();
    }, [load])
  );

  return (
    <View style={styles.container}>
      <TextInput
        style={styles.input}
        placeholder={t('Name or ticket code')}
        placeholderTextColor="#888"
        autoCapitalize="none"
        autoCorrect={false}
        value={query}
        onChangeText={setQuery}
      />

      {error ? (
        <View style={styles.errorBox}>
          <Text style={styles.errorText}>{error}</Text>
          <Pressable onPress={load}>
            <Text style={styles.retry}>{t('Try again')}</Text>
          </Pressable>
        </View>
      ) : null}

      <FlatList
        data={results}
        keyExtractor={(item) => item.code}
        keyboardShouldPersistTaps="handled"
        ListEmptyComponent={error ? undefined : <Text style={styles.empty}>{t('No attendees found')}</Text>}
        renderItem={({ item }) => {
          const look = STATUS_LOOK[item.status] ?? { label: item.status, color: '#666' };
          return (
            <Pressable
              style={styles.row}
              onPress={() => router.push({ pathname: '/result', params: { eventId, code: item.code, canUndo } })}
            >
              <View style={{ flex: 1 }}>
                <Text style={styles.name}>{item.attendeeName}</Text>
                <Text style={styles.detail}>{item.ticketType} · {item.code}</Text>
              </View>
              <Text style={[styles.status, { color: look.color }]}>{t(look.label)}</Text>
            </Pressable>
          );
        }}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#f5f5f5', padding: 16 },
  input: {
    backgroundColor: '#fff', borderWidth: 1, borderColor: '#ddd', borderRadius: 10,
    padding: 14, fontSize: 16, marginBottom: 12, color: '#000',
  },
  errorBox: { backgroundColor: '#fdecea', padding: 14, borderRadius: 10, marginBottom: 12 },
  errorText: { color: '#d93025', fontSize: 15 },
  retry: { color: '#1a73e8', fontSize: 15, fontWeight: '600', marginTop: 8 },
  empty: { textAlign: 'center', color: '#888', marginTop: 32, fontSize: 16 },
  row: {
    flexDirection: 'row', alignItems: 'center', backgroundColor: '#fff',
    padding: 16, borderRadius: 12, marginBottom: 10,
  },
  name: { fontSize: 17, fontWeight: '600' },
  detail: { fontSize: 13, color: '#666', marginTop: 2 },
  status: { fontSize: 14, fontWeight: '600', marginLeft: 12 },
});
