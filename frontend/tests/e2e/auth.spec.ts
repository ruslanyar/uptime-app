import { createHmac } from 'node:crypto';
import { test, expect, type Page } from '@playwright/test';

const password = ' browser password 123 ';
async function register(page: Page, screenshot?: string) {
  const email = `browser-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`;
  await page.goto('/register');
  await page.getByLabel('Имя', { exact: true }).fill('Анна');
  await page.getByLabel('Email', { exact: true }).fill(email);
  await page.getByLabel('Пароль', { exact: true }).fill(password);
  if (screenshot)
    await page.screenshot({ path: `test-results/register-${screenshot}.png` });
  await page.getByRole('button', { name: 'Создать аккаунт' }).click();
  await expect(page).toHaveURL(/\/account$/);
  await expect(
    page.getByRole('heading', { name: 'Здравствуйте, Анна' }),
  ).toBeVisible();
  return email;
}
function apiURL(secure: boolean) {
  return secure
    ? 'https://api.auth-service.test:18443'
    : 'http://localhost:18080';
}
test('registration, reload, HttpOnly cookies, storage, logout and login', async ({
  page,
  context,
}, info) => {
  const secure = info.project.name === 'cross-site-https';
  const email = await register(page, info.project.name);
  const cookies = await context.cookies(apiURL(secure) + '/api/v1/auth/me');
  expect(cookies).toHaveLength(2);
  for (const cookie of cookies) {
    expect(cookie.httpOnly).toBe(true);
    expect(cookie.secure).toBe(secure);
    expect(cookie.sameSite).toBe(secure ? 'None' : 'Lax');
  }
  expect(cookies.find((cookie) => cookie.name === 'access_token')?.path).toBe(
    '/api/v1',
  );
  expect(cookies.find((cookie) => cookie.name === 'refresh_token')?.path).toBe(
    '/api/v1/auth',
  );
  expect(
    await page.evaluate(() => ({
      cookie: document.cookie,
      local: Object.keys(localStorage),
      session: Object.keys(sessionStorage),
    })),
  ).toEqual({ cookie: '', local: [], session: [] });
  await page.reload();
  await expect(page.getByText(email, { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Выйти', exact: true }).click();
  await expect(page).toHaveURL(/\/login$/);
  expect(
    await context.cookies(apiURL(secure) + '/api/v1/auth/me'),
  ).toHaveLength(0);
  await page.getByLabel('Email').fill(email);
  await page.getByLabel('Пароль', { exact: true }).fill(password);
  await page.screenshot({
    path: `test-results/login-${info.project.name}.png`,
  });
  await page.getByRole('button', { name: 'Войти', exact: true }).click();
  await expect(page).toHaveURL(/\/account$/);
  await page.setViewportSize({ width: 375, height: 812 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: `test-results/account-${info.project.name}.png`,
  });
});
test('simultaneous tabs restore expired access with one refresh and synchronize logout', async ({
  page,
  context,
}, info) => {
  const secure = info.project.name === 'cross-site-https';
  await register(page);
  const original = await context.cookies(apiURL(secure) + '/api/v1/auth/me');
  const other = await context.newPage();
  await other.goto('/account');
  await expect(other.getByRole('button', { name: 'Выйти' })).toBeVisible();
  let refreshes = 0;
  context.on('request', (request) => {
    if (request.url().endsWith('/refresh')) refreshes++;
  });
  // Fixture manipulation happens in Node, never in the application or browser JS.
  const access = original.find((cookie) => cookie.name === 'access_token')!;
  const [header, encoded] = access.value.split('.');
  const claims = JSON.parse(Buffer.from(encoded, 'base64url').toString());
  claims.iat = Math.floor(Date.now() / 1000) - 120;
  claims.exp = Math.floor(Date.now() / 1000) - 60;
  const payload = Buffer.from(JSON.stringify(claims)).toString('base64url');
  const signature = createHmac('sha256', process.env.FRONTEND_E2E_JWT_SECRET!)
    .update(`${header}.${payload}`)
    .digest('base64url');
  await context.addCookies([
    { ...access, value: `${header}.${payload}.${signature}` },
  ]);
  await Promise.all([page.reload(), other.reload()]);
  await expect(page.getByRole('button', { name: 'Выйти' })).toBeVisible();
  await expect(other.getByRole('button', { name: 'Выйти' })).toBeVisible();
  expect(refreshes).toBe(1);
  const rotated = await context.cookies(apiURL(secure) + '/api/v1/auth/me');
  expect(
    rotated.find((cookie) => cookie.name === 'refresh_token')?.value,
  ).not.toBe(original.find((cookie) => cookie.name === 'refresh_token')?.value);
  await page.getByRole('button', { name: 'Выйти' }).click();
  await expect(other).toHaveURL(/\/login$/);
});
test('invalid refresh ends session', async ({ page, context }, info) => {
  const secure = info.project.name === 'cross-site-https';
  await register(page);
  const refresh = (
    await context.cookies(apiURL(secure) + '/api/v1/auth/me')
  ).find((cookie) => cookie.name === 'refresh_token')!;
  await context.clearCookies({ name: 'access_token' });
  await context.addCookies([{ ...refresh, value: 'invalid-refresh' }]);
  await page.reload();
  await expect(page).toHaveURL(/\/login$/);
  expect(
    await context.cookies(apiURL(secure) + '/api/v1/auth/me'),
  ).toHaveLength(0);
});
test('CSRF and origin refusals preserve cookies', async ({
  page,
  context,
}, info) => {
  const api = apiURL(info.project.name === 'cross-site-https');
  await register(page);
  const before = await context.cookies(api + '/api/v1/auth/me');
  const status = await page.evaluate(
    async (api) =>
      (
        await fetch(api + '/api/v1/auth/logout', {
          method: 'POST',
          credentials: 'include',
        })
      ).status,
    api,
  );
  expect(status).toBe(403);
  const refusal = await context.request.post(
    `http://localhost:${info.project.name === 'cross-site-https' ? 18081 : 18080}/api/v1/auth/logout`,
    {
      headers: {
        Origin: 'https://not-allowed.example',
        'X-CSRF-Protection': '1',
      },
    },
  );
  expect(refusal.status()).toBe(403);
  expect(await context.cookies(api + '/api/v1/auth/me')).toEqual(before);
});
test('browser without coordination APIs shows a useful error', async ({
  page,
}) => {
  await page.addInitScript(() =>
    Object.defineProperty(navigator, 'locks', { value: undefined }),
  );
  await page.goto('/login');
  await expect(page.locator('main').getByRole('alert')).toContainText(
    'Браузер не поддерживает',
  );
});
test('network errors preserve the page and allow explicit recovery', async ({
  page,
  context,
}) => {
  const email = await register(page);
  await context.setOffline(true);
  await page.evaluate(() => window.dispatchEvent(new Event('focus')));
  await expect(page.locator('main').getByRole('alert')).toContainText(
    'Нет связи с сервером',
  );
  await expect(page).toHaveURL(/\/account$/);
  await context.setOffline(false);
  await page.getByRole('button', { name: 'Повторить проверку' }).click();
  await expect(page.getByText(email, { exact: true })).toBeVisible();
});
test('duplicate email and incorrect password show distinct errors', async ({
  page,
}) => {
  const email = await register(page);
  await page.getByRole('button', { name: 'Выйти' }).click();
  await expect(page).toHaveURL(/\/login$/);
  await page.getByLabel('Email').fill(email);
  await page.getByLabel('Пароль', { exact: true }).fill('wrong password 12345');
  await page.getByRole('button', { name: 'Войти', exact: true }).click();
  await expect(page.locator('main').getByRole('alert')).toContainText(
    'Неверный email или пароль',
  );
  await page.getByRole('link', { name: 'Зарегистрироваться' }).click();
  await page.getByLabel('Имя', { exact: true }).fill('Анна');
  await page.getByLabel('Email').fill(email);
  await page.getByLabel('Пароль', { exact: true }).fill(password);
  await page.getByRole('button', { name: 'Создать аккаунт' }).click();
  await expect(page.locator('main').getByRole('alert')).toContainText(
    'Аккаунт с таким email уже существует',
  );
});
test('an uncertain refresh is not retried by a new tab until explicit recovery', async ({
  page,
  context,
}, info) => {
  await register(page);
  await context.clearCookies({ name: 'access_token' });
  let refreshes = 0;
  const pattern = '**/api/v1/auth/refresh';
  await context.route(pattern, async (route) => {
    refreshes++;
    await route.abort('failed');
  });
  await page.reload();
  await expect(page.locator('main').getByRole('alert')).toContainText(
    'Нет связи с сервером',
  );
  const other = await context.newPage();
  await other.goto('/account');
  await expect(other.locator('main').getByRole('alert')).toContainText(
    'Предыдущий запрос не удалось подтвердить',
  );
  expect(refreshes).toBe(1);
  expect(
    await other.evaluate(() => localStorage.getItem('uptime-auth-uncertain')),
  ).toBe('1');
  await context.unroute(pattern);
  await other.getByRole('button', { name: 'Повторить проверку' }).click();
  await expect(other.getByRole('button', { name: 'Выйти' })).toBeVisible();
  expect(
    await other.evaluate(() => localStorage.getItem('uptime-auth-uncertain')),
  ).toBeNull();
  expect(
    (
      await context.cookies(
        apiURL(info.project.name === 'cross-site-https') + '/api/v1/auth/me',
      )
    ).length,
  ).toBe(2);
});
