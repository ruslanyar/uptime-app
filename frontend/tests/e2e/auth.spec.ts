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
    await page.screenshot({
      animations: 'disabled',
      path: `test-results/register-${screenshot}.png`,
    });
  const historyLength = await page.evaluate(() => history.length);
  await page.getByRole('button', { name: 'Создать аккаунт' }).click();
  await expect(page).toHaveURL(/\/$/);
  expect(await page.evaluate(() => history.length)).toBe(historyLength);
  await expect(
    page.getByRole('heading', { name: 'Здравствуйте, Анна' }),
  ).toBeVisible();
  return email;
}
function apiURL(secure: boolean) {
  return secure
    ? process.env.E2E_HTTPS_API_URL!
    : process.env.NEXT_PUBLIC_API_URL!;
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
  await page.getByRole('button', { name: 'Меню пользователя' }).click();
  await expect(page.getByText(email, { exact: true })).toBeVisible();
  await page.getByRole('menuitem', { name: 'Мой профиль' }).click();
  await expect(page).toHaveURL(/\/account$/);
  await expect(page.getByText(email, { exact: true })).toBeVisible();
  await page.goto('/');
  await page.getByRole('button', { name: 'Меню пользователя' }).click();
  await page.getByRole('menuitem', { name: 'Выйти', exact: true }).click();
  await expect(page).toHaveURL(/\/login$/);
  expect(
    await context.cookies(apiURL(secure) + '/api/v1/auth/me'),
  ).toHaveLength(0);
  await page.getByLabel('Email').fill(email);
  await page.getByLabel('Пароль', { exact: true }).fill(password);
  await page.screenshot({
    animations: 'disabled',
    path: `test-results/login-${info.project.name}.png`,
  });
  await page.getByRole('button', { name: 'Войти', exact: true }).click();
  await expect(page).toHaveURL(/\/$/);
  await page.screenshot({
    animations: 'disabled',
    path: `test-results/dashboard-desktop-${info.project.name}.png`,
  });
  await page.getByRole('button', { name: 'Меню пользователя' }).focus();
  await page.keyboard.press('Enter');
  await expect(page.getByText(email, { exact: true })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByText(email, { exact: true })).not.toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Меню пользователя' }),
  ).toBeFocused();
  await page.setViewportSize({ width: 375, height: 812 });
  await page.getByRole('button', { name: 'Меню пользователя' }).click();
  await expect(page.getByText(email, { exact: true })).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    animations: 'disabled',
    path: `test-results/dashboard-${info.project.name}.png`,
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
  await page.getByRole('button', { name: 'Меню пользователя' }).click();
  await expect(page.getByRole('menuitem', { name: 'Выйти' })).toBeVisible();
  await expect(other.getByRole('button', { name: 'Выйти' })).toBeVisible();
  expect(refreshes).toBe(1);
  const rotated = await context.cookies(apiURL(secure) + '/api/v1/auth/me');
  expect(
    rotated.find((cookie) => cookie.name === 'refresh_token')?.value,
  ).not.toBe(original.find((cookie) => cookie.name === 'refresh_token')?.value);
  await page.getByRole('menuitem', { name: 'Выйти' }).click();
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
    `http://${info.project.name === 'cross-site-https' ? process.env.E2E_HTTPS_HTTP_ADDR : new URL(process.env.NEXT_PUBLIC_API_URL!).host}/api/v1/auth/logout`,
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
  await expect(page).toHaveURL(/\/$/);
  await context.setOffline(false);
  await page.getByRole('button', { name: 'Повторить проверку' }).click();
  await page.getByRole('button', { name: 'Меню пользователя' }).click();
  await expect(page.getByText(email, { exact: true })).toBeVisible();
});
test('duplicate email and incorrect password show distinct errors', async ({
  page,
}) => {
  const email = await register(page);
  await page.getByRole('button', { name: 'Меню пользователя' }).click();
  await page.getByRole('menuitem', { name: 'Выйти' }).click();
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

for (const [path, title, description] of [
  ['/login', 'Войти', 'Рады видеть вас снова в Uptime.'],
  ['/register', 'Создать аккаунт', 'Начните с личного аккаунта в Uptime.'],
]) {
  test(`initial HTML ${path} works without JavaScript`, async ({
    browser,
    baseURL,
  }) => {
    const context = await browser.newContext({
      baseURL,
      javaScriptEnabled: false,
      ignoreHTTPSErrors: true,
    });
    try {
      const page = await context.newPage();
      await page.goto(path);
      await expect(
        page.getByRole('heading', { name: title, exact: true }),
      ).toBeVisible();
      await expect(page.getByText(description, { exact: true })).toBeVisible();
      await expect(page.getByRole('status')).toHaveText('Проверяем сессию…');
      await expect(page.locator('input')).toHaveCount(0);
      await expect(page.getByRole('definition')).toHaveCount(0);
      await expect(
        page.getByRole('link', { name: /Зарегистрироваться|Войти/ }),
      ).toHaveCount(0);
      await page.screenshot({
        animations: 'disabled',
        path: `test-results/initial-${path.slice(1)}-${new URL(baseURL!).protocol.slice(0, -1)}.png`,
      });
    } finally {
      await context.close();
    }
  });
}

test('root and form routes follow current session state', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveURL(/\/login$/);
  await register(page);
  for (const path of ['/', '/login', '/register']) {
    await page.goto(path);
    await expect(page).toHaveURL(/\/$/);
    await expect(
      page.getByRole('heading', { name: 'Здравствуйте, Анна' }),
    ).toBeVisible();
  }
  const historyLength = await page.evaluate(() => history.length);
  await page.getByRole('button', { name: 'Меню пользователя' }).click();
  await page.getByRole('menuitem', { name: 'Выйти', exact: true }).click();
  await expect(page).toHaveURL(/\/login$/);
  expect(await page.evaluate(() => history.length)).toBe(historyLength);
  await page.goto('/account');
  await expect(page).toHaveURL(/\/login$/);
});

test('dark UI stays responsive with long account details', async ({
  page,
}, info) => {
  const name = 'Анна'.repeat(12);
  const email = `long-${Date.now()}-${'a'.repeat(40)}@example.com`;
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto('/register');
  await page.getByLabel('Имя', { exact: true }).fill(name);
  await page.getByLabel('Email', { exact: true }).fill(email);
  await page.getByLabel('Пароль', { exact: true }).fill(password);
  for (const width of [1440, 375]) {
    await page.setViewportSize({ width, height: 1000 });
    await page.screenshot({
      animations: 'disabled',
      path: `test-results/ui-register-${width}-${info.project.name}.png`,
      fullPage: true,
    });
  }
  await page.getByRole('button', { name: 'Создать аккаунт' }).click();
  await expect(page).toHaveURL(/\/$/);
  for (const width of [1440, 375]) {
    await page.setViewportSize({ width, height: 1000 });
    for (const path of ['/', '/account']) {
      await page.goto(path);
      await expect(
        page.getByRole('heading', { name: `Здравствуйте, ${name}` }),
      ).toBeVisible();
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
      expect(
        await page.evaluate(
          () => getComputedStyle(document.body).backgroundColor,
        ),
      ).toBe('rgb(9, 11, 8)');
      await page.screenshot({
        animations: 'disabled',
        path: `test-results/ui-${path === '/' ? 'dashboard' : 'account'}-${width}-${info.project.name}.png`,
        fullPage: true,
      });
      if (path === '/') {
        const trigger = page.getByRole('button', { name: 'Меню пользователя' });
        await trigger.click();
        await expect(page.getByText(email, { exact: true })).toBeVisible();
        const panel = await page.getByRole('menu').boundingBox();
        expect(panel).not.toBeNull();
        expect(panel!.x).toBeGreaterThanOrEqual(0);
        expect(panel!.x + panel!.width).toBeLessThanOrEqual(width);
        await page.screenshot({
          animations: 'disabled',
          path: `test-results/ui-menu-${width}-${info.project.name}.png`,
          fullPage: true,
        });
        await page
          .getByRole('heading', { name: 'Мониторинг скоро появится' })
          .click();
        await expect(page.getByRole('menu')).not.toBeVisible();
      }
    }
  }
  await page.getByRole('button', { name: 'Выйти', exact: true }).click();
  await expect(page).toHaveURL(/\/login$/);
  for (const width of [1440, 375]) {
    await page.setViewportSize({ width, height: 1000 });
    await page.getByLabel('Email', { exact: true }).fill(email);
    await page.getByLabel('Пароль', { exact: true }).fill(password);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      animations: 'disabled',
      path: `test-results/ui-login-${width}-${info.project.name}.png`,
      fullPage: true,
    });
  }
});

test('menu logout exposes pending state and keeps failures visible', async ({
  page,
}, info) => {
  await register(page);
  let finish!: () => void;
  const gate = new Promise<void>((resolve) => {
    finish = resolve;
  });
  await page.route('**/api/v1/auth/logout', async (route) => {
    await gate;
    await route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: '{}',
    });
  });
  const trigger = page.getByRole('button', { name: 'Меню пользователя' });
  await trigger.focus();
  await page.keyboard.press('Enter');
  await expect(
    page.getByRole('menuitem', { name: 'Мой профиль' }),
  ).toBeFocused();
  await page.keyboard.press('ArrowDown');
  await expect(
    page.getByRole('menuitem', { name: 'Выйти', exact: true }),
  ).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(
    page.getByRole('menuitem', { name: 'Выходим…' }),
  ).toHaveAttribute('aria-disabled', 'true');
  finish();
  await expect(page.locator('main').getByRole('alert')).toBeVisible();
  await expect(
    page.getByRole('menuitem', { name: 'Выйти', exact: true }),
  ).not.toHaveAttribute('aria-disabled', 'true');
  await page.keyboard.press('Escape');
  await expect(trigger).toBeFocused();
  await expect(page.locator('main').getByRole('alert')).toBeVisible();
  await page.screenshot({
    animations: 'disabled',
    path: `test-results/ui-logout-error-${info.project.name}.png`,
    fullPage: true,
  });
});

test('profile name persists and updates another tab', async ({
  page,
  context,
}, info) => {
  const email = await register(page);
  await page.goto('/account');
  const other = await context.newPage();
  await other.goto('/account');
  await expect(
    other.getByRole('heading', { name: 'Здравствуйте, Анна' }),
  ).toBeVisible();
  await page.bringToFront();
  await page.getByRole('button', { name: 'Редактировать профиль' }).click();
  await page.getByLabel('Имя', { exact: true }).fill('   ');
  await page.getByRole('button', { name: 'Сохранить', exact: true }).click();
  await expect(
    page.getByText('Введите имя длиной от 2 до 50 символов.'),
  ).toBeVisible();
  await page.getByLabel('Имя', { exact: true }).fill('  Новое имя  ');
  await page.getByRole('button', { name: 'Сохранить', exact: true }).click();
  await expect(page.getByRole('status')).toHaveText('Имя сохранено.');
  await expect(
    page.getByRole('heading', { name: 'Здравствуйте, Новое имя' }),
  ).toBeVisible();
  await expect(
    other.getByRole('heading', { name: 'Здравствуйте, Новое имя' }),
  ).toBeVisible();
  await expect(page.getByText(email, { exact: true })).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole('heading', { name: 'Здравствуйте, Новое имя' }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Редактировать профиль' }).click();
  for (const width of [1440, 375]) {
    await page.setViewportSize({ width, height: 1000 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: `test-results/profile-edit-${width}-${info.project.name}.png`,
      fullPage: true,
    });
  }
  await page.getByLabel('Имя', { exact: true }).fill('Отменённое имя');
  await page.getByRole('button', { name: 'Отмена', exact: true }).click();
  await expect(
    page.getByRole('heading', { name: 'Здравствуйте, Новое имя' }),
  ).toBeVisible();
  await page.goto('/');
  await expect(
    page.getByRole('heading', { name: 'Здравствуйте, Новое имя' }),
  ).toBeVisible();
  await other.close();
});
