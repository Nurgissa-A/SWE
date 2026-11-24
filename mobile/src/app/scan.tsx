import { useCallback, useRef } from 'react';
import { View, Text, Pressable, StyleSheet } from 'react-native';
import { CameraView, useCameraPermissions } from 'expo-camera';
import { router, useLocalSearchParams, useFocusEffect } from 'expo-router';

// Buttons for testing without a printed QR code
const TEST_CODES = [
  { label: 'Valid', code: 'BF-T-1001' },
  { label: 'Used', code: 'BF-T-1002' },
  { label: 'Refunded', code: 'BF-T-1004' },
  { label: 'Promo QR', code: 'BF-CAMPAIGN-SPRING10' },
  { label: 'Fake', code: 'HELLO-123' },
];

export default function ScanScreen() {
  const { eventId } = useLocalSearchParams<{ eventId: string }>();
  const [permission, requestPermission] = useCameraPermissions();
  const locked = useRef(false); // stops the camera from scanning the same code 10 times

  // When we come back to this screen, allow scanning again
  useFocusEffect(
    useCallback(() => {
      locked.current = false;
    }, [])
  );

  function openResult(code: string) {
    if (locked.current) return;
    locked.current = true;
    router.push({ pathname: '/result', params: { eventId, code } });
  }

  if (!permission) {
    return <View style={styles.center} />;
  }

  if (!permission.granted) {
    return (
      <View style={styles.center}>
        <Text style={styles.message}>Camera access is needed to scan tickets.</Text>
        <Pressable style={styles.button} onPress={requestPermission}>
          <Text style={styles.buttonText}>Allow camera</Text>
        </Pressable>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <CameraView
        style={StyleSheet.absoluteFill}
        facing="back"
        barcodeScannerSettings={{ barcodeTypes: ['qr'] }}
        onBarcodeScanned={({ data }) => openResult(data)}
      />
      <View style={styles.frame} />
      <View style={styles.bottom}>
        <Text style={styles.hint}>Point the camera at the ticket QR code</Text>
        <Text style={styles.testLabel}>Test without a QR (demo data):</Text>
        <View style={styles.testRow}>
          {TEST_CODES.map((t) => (
            <Pressable key={t.code} style={styles.testButton} onPress={() => openResult(t.code)}>
              <Text style={styles.testText}>{t.label}</Text>
            </Pressable>
          ))}
        </View>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  center: { flex: 1, justifyContent: 'center', alignItems: 'center', padding: 24, backgroundColor: '#fff' },
  message: { fontSize: 16, textAlign: 'center', marginBottom: 16 },
  button: { backgroundColor: '#1a73e8', paddingVertical: 14, paddingHorizontal: 24, borderRadius: 10 },
  buttonText: { color: '#fff', fontSize: 16, fontWeight: '600' },
  container: { flex: 1, backgroundColor: '#000' },
  frame: {
    position: 'absolute', top: '20%', alignSelf: 'center',
    width: 250, height: 250, borderWidth: 3, borderColor: '#fff', borderRadius: 16,
  },
  bottom: {
    position: 'absolute', left: 0, right: 0, bottom: 0,
    padding: 20, paddingBottom: 40, backgroundColor: 'rgba(0,0,0,0.65)',
  },
  hint: { color: '#fff', fontSize: 16, textAlign: 'center', marginBottom: 16 },
  testLabel: { color: '#bbb', fontSize: 13, marginBottom: 8 },
  testRow: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
  testButton: { backgroundColor: 'rgba(255,255,255,0.15)', paddingVertical: 8, paddingHorizontal: 12, borderRadius: 8 },
  testText: { color: '#fff', fontSize: 14 },
});
