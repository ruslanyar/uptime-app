import { randomBytes } from 'node:crypto';
import { defineConfig } from '@playwright/test';
// Local servers must bypass inherited CI HTTP proxies.
process.env.NO_PROXY = [
  process.env.NO_PROXY,
  'localhost',
  '127.0.0.1',
  '.auth-client.test',
  '.auth-service.test',
]
  .filter(Boolean)
  .join(',');
process.env.no_proxy = process.env.NO_PROXY;
process.env.FRONTEND_E2E_JWT_SECRET ??= randomBytes(32).toString('hex');
export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 45000,
  forbidOnly: !!process.env.CI,
  use: {
    ignoreHTTPSErrors: true,
    screenshot: 'only-on-failure',
    launchOptions: {
      ...(process.env.BROWSER_EXECUTABLE
        ? { executablePath: process.env.BROWSER_EXECUTABLE }
        : {}),
      args: [
        '--no-proxy-server',
        '--host-resolver-rules=MAP frontend.auth-client.test 127.0.0.1,MAP api.auth-service.test 127.0.0.1',
        '--disable-features=ThirdPartyCookieDeprecation,TrackingProtection3pcd',
      ],
    },
  },
  projects: [
    {
      name: 'localhost',
      testIgnore: '**/third-party.spec.ts',
      use: { browserName: 'chromium', baseURL: 'http://localhost:13000' },
    },
    {
      name: 'cross-site-https',
      use: {
        browserName: 'chromium',
        baseURL: 'https://frontend.auth-client.test:13443',
      },
    },
  ],
  webServer: {
    command: 'node scripts/e2e-server.mjs',
    url: 'http://127.0.0.1:13999',
    timeout: 180000,
    reuseExistingServer: false,
    gracefulShutdown: { signal: 'SIGTERM', timeout: 45000 },
  },
});
