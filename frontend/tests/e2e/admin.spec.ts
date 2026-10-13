import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { resolve } from 'node:path';
import { test, expect, type Page } from '@playwright/test';
import { backendTestEnv } from '../../scripts/test-env.mjs';

const password = 'admin browser password 123';
async function register(page: Page, email: string, name: string) {
  await page.goto('/register');
  await page.getByLabel('Имя', { exact: true }).fill(name);
  await page.getByLabel('Email', { exact: true }).fill(email);
  await page.getByLabel('Пароль', { exact: true }).fill(password);
  await page.getByRole('button', { name: 'Создать аккаунт' }).click();
  await expect(
    page.getByRole('heading', { name: `Здравствуйте, ${name}` }),
  ).toBeVisible();
}

test('admin access, pagination, confirmed deletion and lost response recovery', async ({
  page,
  browser,
}, info) => {
  test.setTimeout(120000);
  const suffix = `${Date.now()}-${info.project.name}`;
  const email = `admin-${suffix}@example.com`;
  const targetEmail = `zzzz-delete-${suffix}@example.com`;
  await register(page, email, 'Администратор');
  await page.getByRole('button', { name: 'Меню пользователя' }).click();
  await expect(
    page.getByRole('menuitem', { name: 'Админка', exact: true }),
  ).toHaveCount(0);
  await page.goto('/admin/users');
  await expect(page.getByText('Доступ к админке запрещён.')).toBeVisible();

  const env = backendTestEnv();
  const database = process.env.E2E_DATABASE_NAME!;
  if (!/^frontend_e2e_[a-f0-9]{16}$/.test(database))
    throw new Error('Unsafe fixture database');
  const databaseURL = new URL(env.TEST_DATABASE_URL!);
  databaseURL.pathname = `/${database}`;
  await promisify(execFile)(
    'go',
    ['run', './cmd/admin', 'grant', '--email', email],
    {
      cwd: resolve('../backend'),
      env: { ...env, DATABASE_URL: databaseURL.href },
    },
  );

  const targetContext = await browser.newContext({ ...info.project.use });
  const target = await targetContext.newPage();
  try {
    await register(target, targetEmail, 'Пользователь для удаления');
    await target.getByRole('button', { name: 'Добавить сайт' }).click();
    await target.getByLabel('URL сайта').fill('https://example.com');
    await target.getByRole('button', { name: 'Создать', exact: true }).click();
    await expect(target.getByText('Сайт добавлен.')).toBeVisible();
    const apiURL = process.env.NEXT_PUBLIC_API_URL!;
    for (let i = 0; i < 25; i++) {
      const response = await page.request.post(
        `${apiURL}/api/v1/auth/register`,
        {
          headers: { 'X-CSRF-Protection': '1' },
          data: {
            name: 'Тестовый пользователь',
            email: `page-${suffix}-${i}@example.com`,
            password,
          },
        },
      );
      expect(response.status()).toBe(201);
    }
    // Fixture registration changes cookies; restore the admin via the real login form.
    await page.goto('/');
    await page.getByRole('button', { name: 'Меню пользователя' }).click();
    await page.getByRole('menuitem', { name: 'Выйти', exact: true }).click();
    await page.getByLabel('Email').fill(email);
    await page.getByLabel('Пароль', { exact: true }).fill(password);
    await page.getByRole('button', { name: 'Войти', exact: true }).click();
    await page.getByRole('button', { name: 'Меню пользователя' }).click();
    await page.getByRole('menuitem', { name: 'Админка', exact: true }).click();
    await expect(page).toHaveURL(/\/admin\/users$/);
    const list = page.getByRole('list', {
      name: 'Зарегистрированные пользователи',
    });
    await expect(list.getByRole('listitem')).toHaveCount(25);
    await expect(
      page.getByRole('button', { name: `Удалить ${email}`, exact: true }),
    ).toHaveCount(0);
    while (
      await page.getByRole('button', { name: 'Далее', exact: true }).isEnabled()
    ) {
      await page.getByRole('button', { name: 'Далее', exact: true }).click();
      await expect(page.getByText('Загружаем пользователей…')).toHaveCount(0);
    }
    const open = page.getByRole('button', {
      name: `Удалить ${targetEmail}`,
      exact: true,
    });
    await open.click();
    const confirmation = page.getByRole('group', {
      name: `Удалить пользователя ${targetEmail}?`,
    });
    await expect(
      confirmation.getByRole('button', { name: 'Отмена' }),
    ).toBeFocused();
    await confirmation.getByRole('button', { name: 'Отмена' }).press('Escape');
    await expect(open).toBeFocused();
    await expect(confirmation).toHaveCount(0);
    await open.click();
    for (const width of [1280, 390]) {
      await page.setViewportSize({ width, height: 900 });
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
      await page.screenshot({
        path: `test-results/admin-delete-${width}-${info.project.name}.png`,
        fullPage: true,
      });
    }
    let deletes = 0;
    await page.route('**/api/v1/admin/users/*', async (route) => {
      if (route.request().method() !== 'DELETE') return route.continue();
      deletes++;
      const upstream = new URL(route.request().url());
      upstream.hostname = '127.0.0.1';
      const response = await route.fetch({ url: upstream.href });
      expect(response.status()).toBe(204);
      await route.abort();
    });
    await confirmation
      .getByRole('button', { name: 'Удалить пользователя', exact: true })
      .click();
    await expect(confirmation).toContainText('Не удалось подтвердить удаление');
    await expect(
      confirmation.getByRole('button', {
        name: 'Удалить пользователя',
        exact: true,
      }),
    ).toBeDisabled();
    expect(deletes).toBe(1);
    await page.unroute('**/api/v1/admin/users/*');
    await page
      .getByRole('button', { name: 'Обновить список', exact: true })
      .click();
    await expect(open).toHaveCount(0);
    await page.reload();
    await expect(open).toHaveCount(0);
    await expect(page.getByText('Загружаем пользователей…')).toHaveCount(0);
    await target.reload();
    await expect(target).toHaveURL(/\/login$/);
    await target.getByLabel('Email').fill(targetEmail);
    await target.getByLabel('Пароль', { exact: true }).fill(password);
    await target.getByRole('button', { name: 'Войти', exact: true }).click();
    await expect(target.locator('main').getByRole('alert')).toContainText(
      'Неверный email или пароль.',
    );
  } finally {
    await targetContext.close();
  }
});
