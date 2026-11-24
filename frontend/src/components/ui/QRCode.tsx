'use client';

import { useEffect, useState } from 'react';
import QR from 'qrcode';

/** Renders a QR code as an image (black on white so it scans from screens and paper). */
export function QRCode({ value, size = 220, label }: { value: string; size?: number; label: string }) {
  const [src, setSrc] = useState('');
  useEffect(() => {
    let alive = true;
    QR.toDataURL(value, { errorCorrectionLevel: 'H', margin: 2, width: size * 2, color: { dark: '#000000', light: '#ffffff' } })
      .then((url) => { if (alive) setSrc(url); })
      .catch(() => setSrc(''));
    return () => { alive = false; };
  }, [value, size]);
  // eslint-disable-next-line @next/next/no-img-element
  return src ? <img src={src} width={size} height={size} alt={label} className="rounded-xl bg-white" /> : <div style={{ width: size, height: size }} className="animate-pulse rounded-xl bg-secondary" />;
}
