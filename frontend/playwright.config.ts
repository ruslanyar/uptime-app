import { backendTestEnv } from './scripts/test-env.mjs';
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
process.env.FRONTEND_E2E_JWT_SECRET = backendTestEnv().JWT_SECRET;
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
      use: { browserName: 'chromium', baseURL: process.env.E2E_FRONTEND_URL },
    },
    {
      name: 'cross-site-https',
      use: {
        browserName: 'chromium',
        baseURL: process.env.E2E_HTTPS_FRONTEND_URL,
      },
    },
  ],
  webServer: {
    command: 'node scripts/e2e-server.mjs',
    url: process.env.E2E_READY_URL,
    timeout: 180000,
    reuseExistingServer: false,
    gracefulShutdown: { signal: 'SIGTERM', timeout: 45000 },
  },
});
