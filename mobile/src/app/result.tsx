import { useCallback, useEffect, useState } from 'react';
import { View, Text, Pressable, ActivityIndicator, StyleSheet, Alert } from 'react-native';
import { router, useLocalSearchParams } from 'expo-router';
import { verifyTicket, checkIn, undoCheckIn, VerifyResult } from '../api/mock';
import { handleApiError } from '../api/errors';
import ErrorView from '../components/ErrorView';

const GREEN = '#1e8e3e';
const ORANGE = '#e37400';
const RED = '#d93025';

const LOOK: Record<VerifyResult['kind'], { color: string; icon: string; title: string }> = {
  valid: { color: GREEN, icon: '✓', title: 'Valid ticket' },
  already_used: { color: ORANGE, icon: '!', title: 'Already checked in' },
  cancelled: { color: RED, icon: '✕', title: 'Ticket cancelled' },
  refunded: { color: RED, icon: '✕', title: 'Ticket refunded' },
  wrong_event: { color: RED, icon: '✕', title: 'Ticket is for a different event' },
  campaign: { color: RED, icon: '✕', title: 'Promo QR code, not a ticket' },
  not_found: { color: RED, icon: '✕', title: 'Invalid ticket' },
};

export default function ResultScreen() {
  const { eventId, code } = useLocalSearchParams<{ eventId: string; code: string }>();
  const [result, setResult] = useState<VerifyResult | null>(null);
  const [error, setError] = useState('');
  const [checkedInNow, setCheckedInNow] = useState(false);
  const [busy, setBusy] = useState(false);

  const verify = useCallback(async () => {
    setError('');
    setResult(null);
    try {
      setResult(await verifyTicket(eventId, code));
    } catch (err) {
      await handleApiError(err, setError);
    }
  }, [eventId, code]);

  useEffect(() => {
    verify();
  }, [verify]);

  // Errors while checking in show as a popup, so the result stays visible
  const showPopupError = (message: string) => Alert.alert('Could not complete', message);

  async function handleCheckIn() {
    setBusy(true);
    try {
      await checkIn(code);
      setCheckedInNow(true);
    } catch (err) {
      await handleApiError(err, showPopupError);
    } finally {
      setBusy(false);
    }
  }

  async function handleUndo() {
    setBusy(true);
    try {
      await undoCheckIn(code);
      setResult(await verifyTicket(eventId, code));
      setCheckedInNow(false);
    } catch (err) {
      await handleApiError(err, showPopupError);
    } finally {
      setBusy(false);
    }
  }

  if (error) {
    return <ErrorView message={error} onRetry={verify} />;
  }

  if (!result) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" />
        <Text style={styles.checking}>Checking ticket…</Text>
      </View>
    );
  }

  const look = checkedInNow ? { color: GREEN, icon: '✓', title: 'Checked in' } : LOOK[result.kind];
  const showUndo = checkedInNow || result.kind === 'already_used';

  return (
    <View style={styles.container}>
      <View style={[styles.banner, { backgroundColor: look.color }]}>
        <Text style={styles.icon}>{look.icon}</Text>
        <Text style={styles.title}>{look.title}</Text>
      </View>

      <View style={styles.details}>
        {result.attendeeName ? <Text style={styles.name}>{result.attendeeName}</Text> : null}
        {result.ticketType ? <Text style={styles.detail}>{result.ticketType}</Text> : null}
        {result.kind === 'already_used' && !checkedInNow && result.checkedInAt ? (
          <Text style={styles.detail}>Checked in at {result.checkedInAt}</Text>
        ) : null}
        <Text style={styles.code}>{result.code}</Text>
      </View>

      <View style={styles.actions}>
        {result.kind === 'valid' && !checkedInNow ? (
          <Pressable style={[styles.button, { backgroundColor: GREEN }]} disabled={busy} onPress={handleCheckIn}>
            <Text style={styles.buttonText}>{busy ? 'Checking in…' : 'Check in'}</Text>
          </Pressable>
        ) : null}

        {showUndo ? (
          <Pressable style={[styles.button, styles.outline]} disabled={busy} onPress={handleUndo}>
            <Text style={[styles.buttonText, styles.outlineText]}>Undo check-in</Text>
          </Pressable>
        ) : null}

        <Pressable style={styles.button} onPress={() => router.back()}>
          <Text style={styles.buttonText}>Scan next</Text>
        </Pressable>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  center: { flex: 1, justifyContent: 'center', alignItems: 'center', backgroundColor: '#fff' },
  checking: { marginTop: 12, fontSize: 16, color: '#666' },
  container: { flex: 1, backgroundColor: '#fff' },
  banner: { alignItems: 'center', paddingVertical: 40, paddingHorizontal: 24 },
  icon: { fontSize: 72, color: '#fff', fontWeight: '700' },
  title: { fontSize: 26, color: '#fff', fontWeight: '700', textAlign: 'center', marginTop: 8 },
  details: { padding: 24, alignItems: 'center' },
  name: { fontSize: 24, fontWeight: '600', marginBottom: 6 },
  detail: { fontSize: 16, color: '#666', marginBottom: 4 },
  code: { fontSize: 13, color: '#999', marginTop: 12, fontFamily: 'Courier' },
  actions: { padding: 24, marginTop: 'auto', gap: 12 },
  button: { backgroundColor: '#1a73e8', padding: 18, borderRadius: 12 },
  buttonText: { color: '#fff', fontSize: 18, fontWeight: '600', textAlign: 'center' },
  outline: { backgroundColor: '#fff', borderWidth: 2, borderColor: '#d93025' },
  outlineText: { color: '#d93025' },
});
