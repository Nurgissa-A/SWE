import { useCallback, useEffect, useState } from 'react';
import { View, Text, Pressable, ActivityIndicator, StyleSheet, Alert } from 'react-native';
import { router, useLocalSearchParams } from 'expo-router';
import { verifyTicket, checkIn, undoCheckIn, VerifyResult } from '../api';
import { handleApiError } from '../api/errors';
import ErrorView from '../components/ErrorView';
import { useI18n } from '../i18n';

const GREEN = '#1e8e3e';
const ORANGE = '#e37400';
const RED = '#d93025';

// Titles are English keys, translated when shown.
const LOOK: Record<VerifyResult['kind'], { color: string; icon: string; title: string }> = {
  valid: { color: GREEN, icon: '✓', title: 'Valid ticket' },
  admitted: { color: GREEN, icon: '✓', title: 'Checked in' },
  already_used: { color: ORANGE, icon: '!', title: 'Already checked in' },
  cancelled: { color: RED, icon: '✕', title: 'Ticket cancelled' },
  refunded: { color: RED, icon: '✕', title: 'Ticket refunded' },
  wrong_event: { color: RED, icon: '✕', title: 'Ticket is for a different event' },
  campaign: { color: RED, icon: '✕', title: 'Promo QR code, not a ticket' },
  not_found: { color: RED, icon: '✕', title: 'Invalid ticket' },
  closed: { color: RED, icon: '✕', title: 'Check-in closed: the event is not published' },
};

export default function ResultScreen() {
  const { t } = useI18n();
  const { eventId, code, canUndo } = useLocalSearchParams<{ eventId: string; code: string; canUndo?: string }>();
  const [result, setResult] = useState<VerifyResult | null>(null);
  const [error, setError] = useState('');
  const [checkedInNow, setCheckedInNow] = useState(false);
  const [busy, setBusy] = useState(false);

  const verify = useCallback(async () => {
    setError('');
    setResult(null);
    try {
      const r = await verifyTicket(eventId, code);
      setResult(r);
      // With the real backend a scan checks the ticket in straight away (one step).
      setCheckedInNow(r.kind === 'admitted');
    } catch (err) {
      await handleApiError(err, setError);
    }
  }, [eventId, code]);

  useEffect(() => {
    verify();
  }, [verify]);

  // Errors while checking in show as a popup, so the result stays visible
  const showPopupError = (message: string) => Alert.alert(t('Could not complete'), message);

  async function handleCheckIn() {
    setBusy(true);
    try {
      await checkIn(eventId, result!);
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
      await undoCheckIn(eventId, result!);
      // Look the ticket up again by its code (never re-send the scanned QR: that would check it in).
      setResult(await verifyTicket(eventId, result!.code));
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
        <Text style={styles.checking}>{t('Checking ticket…')}</Text>
      </View>
    );
  }

  const look = checkedInNow ? LOOK.admitted : LOOK[result.kind];
  const showUndo = canUndo !== '0' && (checkedInNow || result.kind === 'already_used');

  return (
    <View style={styles.container}>
      <View style={[styles.banner, { backgroundColor: look.color }]}>
        <Text style={styles.icon}>{look.icon}</Text>
        <Text style={styles.title}>{t(look.title)}</Text>
      </View>

      <View style={styles.details}>
        {result.attendeeName ? <Text style={styles.name}>{result.attendeeName}</Text> : null}
        {result.ticketType ? <Text style={styles.detail}>{result.ticketType}</Text> : null}
        {result.kind === 'already_used' && !checkedInNow && result.checkedInAt ? (
          <Text style={styles.detail}>{t('Checked in at {time}', { time: result.checkedInAt })}</Text>
        ) : null}
        {result.code ? <Text style={styles.code}>{result.code}</Text> : null}
      </View>

      <View style={styles.actions}>
        {result.kind === 'valid' && !checkedInNow ? (
          <Pressable style={[styles.button, { backgroundColor: GREEN }]} disabled={busy} onPress={handleCheckIn}>
            <Text style={styles.buttonText}>{busy ? t('Checking in…') : t('Check in')}</Text>
          </Pressable>
        ) : null}

        {showUndo ? (
          <Pressable style={[styles.button, styles.outline]} disabled={busy} onPress={handleUndo}>
            <Text style={[styles.buttonText, styles.outlineText]}>{t('Undo check-in')}</Text>
          </Pressable>
        ) : null}

        <Pressable style={styles.button} onPress={() => router.back()}>
          <Text style={styles.buttonText}>{t('Scan next')}</Text>
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
