import { defineConfig, devices } from '@playwright/test';

/**
 * End-to-end tests against a running local stack with the demo data loaded
 * (scripts/init_local.py, the three services, then scripts/seed_demo.py). See README.md.
 * They create real orders and check-ins, so run them against a development database only.
 */
const WEB_URL = process.env.WEB_URL ?? 'http://localhost:3000';
const DOOR_URL = process.env.DOOR_APP_URL ?? 'http://localhost:3200';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  workers: 1, // the tests share one seeded database
  retries: 0,
  timeout: 60_000,
  reporter: [['list'], ['html', { open: 'never' }]],
  globalSetup: './global-setup.ts',
  use: { trace: 'retain-on-failure', screenshot: 'only-on-failure' },
  projects: [
    {
      name: 'web',
      testDir: './tests/web',
      use: { ...devices['Desktop Chrome'], baseURL: WEB_URL, viewport: { width: 1280, height: 900 } },
    },
    {
      name: 'door-app',
      testDir: './tests/door',
      use: {
        ...devices['iPhone 13'],
        browserName: 'chromium',
        baseURL: DOOR_URL,
        permissions: ['camera'],
        launchOptions: {
          // A fake camera that shows a real ticket QR (written by global-setup.ts).
          args: ['--use-fake-ui-for-media-stream', '--use-fake-device-for-media-stream', '--use-file-for-fake-video-capture=.cache/ticket.y4m'],
        },
      },
    },
  ],
  webServer: {
    // Serves the exported check-in app (cd mobile && npm run export:web) and forwards /api to the web app.
    command: 'node serve-door-app.mjs',
    url: `${DOOR_URL}/`,
    reuseExistingServer: true,
    env: { WEB_URL, PORT: new URL(DOOR_URL).port },
  },
});
