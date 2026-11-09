import type { NextConfig } from "next";

const identityApiUrl = process.env.IDENTITY_API_URL ?? 'http://localhost:8000';
const eventsApiUrl = process.env.EVENTS_API_URL ?? 'http://localhost:8080';

const nextConfig: NextConfig = {
  turbopack: { root: process.cwd() },
  async rewrites() {
    return [
      { source: '/api/auth/:path*', destination: `${identityApiUrl}/api/auth/:path*` },
      { source: '/api/users/:path*', destination: `${identityApiUrl}/api/users/:path*` },
      { source: '/api/organizer-profile', destination: `${identityApiUrl}/api/organizer-profile` },
      { source: '/api/events', destination: `${eventsApiUrl}/api/events` },
      { source: '/api/events/:path*', destination: `${eventsApiUrl}/api/events/:path*` },
    ];
  },
  async headers() {
    return [
      {
        source: '/:path*',
        headers: [
          { key: 'X-Content-Type-Options', value: 'nosniff' },
          { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
          { key: 'Strict-Transport-Security', value: 'max-age=63072000' },
          { key: 'X-Frame-Options', value: 'SAMEORIGIN' },
          { key: 'Permissions-Policy', value: 'camera=(), microphone=(), geolocation=()' },
        ],
      },
    ];
  },
};

export default nextConfig;
