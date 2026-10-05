// Invoked by TestBrowserCrossSite; no tokens or credentials are printed.
const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
(async () => {
  const browser = await chromium.launch({
    ...(process.env.BROWSER_EXECUTABLE ? { executablePath: process.env.BROWSER_EXECUTABLE } : {}),
    headless: true,
    args: [
      '--no-proxy-server',
      '--host-resolver-rules=MAP frontend.auth-client.test 127.0.0.1,MAP api.auth-service.test 127.0.0.1',
      '--disable-features=ThirdPartyCookieDeprecation,TrackingProtection3pcd',
    ],
  });
  try {
    const context = await browser.newContext({ ignoreHTTPSErrors: true });
    const page = await context.newPage();
    await page.goto(process.env.BROWSER_FRONTEND_URL);
    const result = await page.evaluate(async () => {
      const post = async (path, body) => {
        const response = await fetch(window.apiBase + '/api/v1/auth/' + path, {
          method: 'POST', credentials: 'include',
          headers: { 'X-CSRF-Protection': '1', ...(body ? { 'Content-Type': 'application/json' } : {}) },
          ...(body ? { body: JSON.stringify(body) } : {}),
        });
        return { status: response.status, body: response.status === 204 ? null : await response.json() };
      };
      const registration = await post('register', {
        name: 'Browser User', email: 'browser@example.com', password: 'browser test password',
      });
      const refresh = await post('refresh');
      const logout = await post('logout');
      const afterLogout = await post('refresh');
      const me = await fetch(window.apiBase + '/api/v1/auth/me', {
        headers: { Authorization: 'Bearer ' + registration.body.access_token },
      });
      return {
        registration: registration.status, refresh: refresh.status,
        logout: logout.status, afterLogout: afterLogout.status, me: me.status,
        sameUser: registration.body.user?.id === refresh.body.user?.id,
      };
    });
    assert.deepEqual(result, { registration: 201, refresh: 200, logout: 204, afterLogout: 401, me: 200, sameUser: true });
    const cookies = await context.cookies(process.env.BROWSER_API_URL);
    assert.equal(cookies.filter(c => c.name === 'refresh_token').length, 0);
    console.log('PASS: cross-site HTTPS register, refresh, logout, cookie removal, access JWT after logout');
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error.message); process.exitCode = 1; });
