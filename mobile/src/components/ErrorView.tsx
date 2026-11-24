import { View, Text, Pressable, StyleSheet } from 'react-native';

// Shown on any screen when loading fails (no internet, server down, etc.)
export default function ErrorView({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <View style={styles.container}>
      <Text style={styles.icon}>⚠️</Text>
      <Text style={styles.message}>{message}</Text>
      <Pressable style={styles.button} onPress={onRetry}>
        <Text style={styles.buttonText}>Try again</Text>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, justifyContent: 'center', alignItems: 'center', padding: 24, backgroundColor: '#fff' },
  icon: { fontSize: 48, marginBottom: 12 },
  message: { fontSize: 16, color: '#333', textAlign: 'center', marginBottom: 20 },
  button: { backgroundColor: '#1a73e8', paddingVertical: 14, paddingHorizontal: 28, borderRadius: 10 },
  buttonText: { color: '#fff', fontSize: 16, fontWeight: '600' },
});
