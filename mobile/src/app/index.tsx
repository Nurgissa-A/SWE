import { useEffect, useState } from 'react';
import { View, Text, TextInput, Pressable, ActivityIndicator, StyleSheet } from 'react-native';
import { router } from 'expo-router';
import { login } from '../api/mock';
import { saveSession, loadSession, isStaff } from '../api/session';
import { friendlyMessage } from '../api/errors';

export default function LoginScreen() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [checkingSaved, setCheckingSaved] = useState(true);

  // If already signed in from last time, skip the login screen
  useEffect(() => {
    loadSession().then((session) => {
      if (session && isStaff(session.user)) {
        router.replace('/events');
      } else {
        setCheckingSaved(false);
      }
    });
  }, []);

  async function handleLogin() {
    setError('');
    if (!email.trim() || !password) {
      setError('Please enter your email and password.');
      return;
    }
    setBusy(true);
    try {
      const result = await login(email, password);
      if (!isStaff(result.user)) {
        setError('This app is only for event staff. Attendees can find their tickets on the BiletFlow website.');
        return;
      }
      await saveSession(result);
      router.replace('/events');
    } catch (err) {
      setError(friendlyMessage(err));
    } finally {
      setBusy(false);
    }
  }

  if (checkingSaved) {
    return (
      <View style={[styles.container, { alignItems: 'center' }]}>
        <ActivityIndicator size="large" />
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <Text style={styles.title}>BiletFlow Check-in</Text>
      <Text style={styles.subtitle}>Sign in as event staff</Text>

      <TextInput
        style={styles.input}
        placeholder="Email"
        placeholderTextColor="#888"
        autoCapitalize="none"
        autoCorrect={false}
        keyboardType="email-address"
        value={email}
        onChangeText={setEmail}
      />
      <TextInput
        style={styles.input}
        placeholder="Password"
        placeholderTextColor="#888"
        secureTextEntry
        value={password}
        onChangeText={setPassword}
        onSubmitEditing={handleLogin}
      />

      {error ? <Text style={styles.error}>{error}</Text> : null}

      <Pressable style={[styles.button, busy && styles.buttonDisabled]} onPress={handleLogin} disabled={busy}>
        {busy ? <ActivityIndicator color="#fff" /> : <Text style={styles.buttonText}>Sign in</Text>}
      </Pressable>

      <Text style={styles.demo}>Demo: admin@biletflow.kz / admin123</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, justifyContent: 'center', padding: 24, backgroundColor: '#fff' },
  title: { fontSize: 28, fontWeight: '700', textAlign: 'center' },
  subtitle: { fontSize: 16, color: '#666', textAlign: 'center', marginBottom: 32 },
  input: {
    borderWidth: 1, borderColor: '#ddd', borderRadius: 10,
    padding: 14, fontSize: 16, marginBottom: 12, color: '#000',
  },
  error: { color: '#d93025', fontSize: 15, marginBottom: 12, textAlign: 'center' },
  button: { backgroundColor: '#1a73e8', padding: 16, borderRadius: 10, marginTop: 8, minHeight: 54, justifyContent: 'center' },
  buttonDisabled: { opacity: 0.6 },
  buttonText: { color: '#fff', fontSize: 16, fontWeight: '600', textAlign: 'center' },
  demo: { marginTop: 24, textAlign: 'center', color: '#999', fontSize: 13 },
});
