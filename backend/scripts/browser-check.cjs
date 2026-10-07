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
    const call = (path, body) => page.evaluate(async ({ path, body }) => {
      const response = await fetch(window.apiBase + '/api/v1/auth/' + path, {
        method: path === 'me' ? 'GET' : 'POST', credentials: 'include', cache: 'no-store',
        headers: path === 'me' ? {} : { 'X-CSRF-Protection': '1', ...(body ? { 'Content-Type': 'application/json' } : {}) },
        ...(body ? { body: JSON.stringify(body) } : {}),
      });
      return { status: response.status, body: response.status === 204 ? null : await response.json() };
    }, { path, body });
    const credentials = { email: 'browser@example.com', password: 'browser test password' };
    const registration = await call('register', { name: 'Browser User', ...credentials });
    assert.equal(registration.status, 201);
    assert.deepEqual(Object.keys(registration.body), ['user']);
    assert.deepEqual(Object.keys(registration.body.user).sort(), ['email', 'id', 'name']);
    const monitorCall = (method, path = '', body, csrf = true) => page.evaluate(async ({ method, path, body, csrf }) => {
      const response = await fetch(window.apiBase + '/api/v1/monitors' + path, {
        method, credentials: 'include', cache: 'no-store',
        headers: { ...(method !== 'GET' && csrf ? { 'X-CSRF-Protection': '1' } : {}), ...(body ? { 'Content-Type': 'application/json' } : {}) },
        ...(body ? { body: JSON.stringify(body) } : {}),
      });
      return { status: response.status, body: response.status === 204 ? null : await response.json() };
    }, { method, path, body, csrf });
    const created = await monitorCall('POST', '', { url: 'https://browser.example', interval_seconds: 300 });
    assert.equal(created.status, 201);
    assert.equal(created.body.created_at, created.body.updated_at);
    const monitorPath = '/' + created.body.id;
    const input = { url: 'https://edited.browser.example', interval_seconds: 90 };
    assert.equal((await monitorCall('PUT', monitorPath, input, false)).status, 403);
    assert.equal((await monitorCall('DELETE', monitorPath, undefined, false)).status, 403);
    const updated = await monitorCall('PUT', monitorPath, input);
    assert.equal(updated.status, 200);
    assert.equal(updated.body.id, created.body.id);
    assert.equal(updated.body.url, input.url);
    assert.equal(updated.body.interval_seconds, input.interval_seconds);
    assert.equal(updated.body.created_at, created.body.created_at);
    assert.ok(Date.parse(updated.body.updated_at) > Date.parse(created.body.updated_at));
    const listed = await monitorCall('GET');
    assert.equal(listed.status, 200);
    assert.deepEqual(listed.body.monitors, [updated.body]);
    assert.equal((await monitorCall('DELETE', monitorPath)).status, 204);
    assert.equal((await monitorCall('DELETE', monitorPath)).status, 404);
    assert.deepEqual((await monitorCall('GET')).body.monitors, []);
    const cookies = async () => (await context.cookies(process.env.BROWSER_API_URL + '/api/v1/auth/me')).filter(c => ['access_token', 'refresh_token'].includes(c.name));
    const before = await cookies();
    assert.equal(before.length, 2);
    for (const cookie of before) {
      assert.equal(cookie.httpOnly, true);
      assert.equal(cookie.secure, true);
      assert.equal(cookie.sameSite, 'None');
      assert.equal(cookie.path, cookie.name === 'access_token' ? '/api/v1' : '/api/v1/auth');
    }
    let me = await call('me');
    assert.equal(me.status, 200);
    assert.equal(me.body.id, registration.body.user.id);
    const refresh = await call('refresh');
    assert.equal(refresh.status, 200);
    assert.deepEqual(Object.keys(refresh.body), ['user']);
    const after = await cookies();
    assert.equal(after.length, 2);
    for (const cookie of after) assert.notEqual(cookie.value, before.find(c => c.name === cookie.name).value, 'rotation must replace both cookies');
    // Chromium derives expiry from integer Max-Age and response receipt time.
    assert.ok(Math.abs(after.find(c => c.name === 'refresh_token').expires - before.find(c => c.name === 'refresh_token').expires) < 1, 'refresh must preserve absolute session expiry within cookie precision');
    assert.equal((await call('me')).status, 200);
    assert.equal((await call('logout')).status, 204);
    assert.equal((await cookies()).length, 0);
    assert.equal((await call('me')).status, 401);
    assert.equal((await call('refresh')).status, 401);
    const login = await call('login', credentials);
    assert.equal(login.status, 200);
    assert.deepEqual(Object.keys(login.body), ['user']);
    assert.equal((await call('me')).status, 200);
    assert.equal((await call('logout')).status, 204);
    assert.equal((await cookies()).length, 0);
    console.log('PASS: cross-site HTTPS register, login, cookie /me, refresh of both HttpOnly cookies, logout, cookie removal and monitor create/update/delete with CSRF');
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error.message); process.exitCode = 1; });
