import { test, expect, type Page } from '@playwright/test';

const password = ' browser password 123 ';
async function register(page: Page) {
  await page.goto('/register');
  await page.getByLabel('Имя', { exact: true }).fill('Анна');
  await page
    .getByLabel('Email', { exact: true })
    .fill(
      `monitors-${Date.now()}-${Math.random().toString(16).slice(2)}@example.com`,
    );
  await page.getByLabel('Пароль', { exact: true }).fill(password);
  await page.getByRole('button', { name: 'Создать аккаунт' }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(
    page.getByRole('heading', { name: 'Пока нет сайтов' }),
  ).toBeVisible();
}
async function add(page: Page, url: string, value: string, unit: string) {
  await page.getByRole('button', { name: 'Добавить сайт' }).click();
  await page.getByLabel('URL сайта').fill(url);
  await page.getByLabel('Интервал опроса').fill(value);
  await page.getByLabel('Единица измерения').selectOption(unit);
  await page.getByRole('button', { name: 'Создать', exact: true }).click();
  await expect(
    page
      .getByRole('list', { name: 'Сайты для мониторинга' })
      .getByText(url, { exact: true }),
  ).toBeVisible();
  await expect(page.getByRole('form', { name: 'Новый сайт' })).toHaveCount(0);
}

test('creates monitors in all units, persists them and rejects duplicates', async ({
  page,
}, info) => {
  await register(page);
  await page.getByRole('button', { name: 'Добавить сайт' }).click();
  await expect(page.getByLabel('URL сайта')).toBeFocused();
  await page.getByRole('button', { name: 'Отмена' }).click();
  await expect(page.getByRole('form')).toHaveCount(0);
  await page.getByRole('button', { name: 'Добавить сайт' }).click();
  await page.getByLabel('URL сайта').fill('invalid');
  await page.getByLabel('Единица измерения').selectOption('seconds');
  await page.getByLabel('Интервал опроса').fill('59');
  await page.getByRole('button', { name: 'Создать', exact: true }).click();
  await expect(page.getByLabel('URL сайта')).toHaveAttribute(
    'aria-invalid',
    'true',
  );
  await expect(page.getByLabel('Интервал опроса')).toHaveAttribute(
    'aria-invalid',
    'true',
  );
  const url = `https://example.com/${'long-path-'.repeat(15)}?source=uptime`;
  await page.getByLabel('URL сайта').fill(` ${url} `);
  await page.getByLabel('Единица измерения').selectOption('minutes');
  await page.getByLabel('Интервал опроса').fill('1.5');
  for (const width of [1440, 375]) {
    await page.setViewportSize({ width, height: 1000 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: `test-results/monitor-form-${width}-${info.project.name}.png`,
      fullPage: true,
    });
  }
  let finish!: () => void;
  const gate = new Promise<void>((resolve) => {
    finish = resolve;
  });
  let posts = 0;
  await page.route('**/api/v1/monitors', async (route) => {
    if (route.request().method() !== 'POST') {
      await route.continue();
      return;
    }
    posts++;
    await gate;
    await route.continue();
  });
  await page.getByRole('button', { name: 'Создать', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Создаём…' })).toBeDisabled();
  await expect(page.getByRole('button', { name: 'Отмена' })).toBeDisabled();
  await expect(
    page.getByRole('button', { name: 'Обновить список сайтов' }),
  ).toBeDisabled();
  finish();
  await expect(page.getByText('Сайт добавлен.')).toBeVisible();
  expect(posts).toBe(1);
  await page.unroute('**/api/v1/monitors');
  await expect(page.getByText(url, { exact: true })).toBeVisible();
  await expect(page.getByText('Интервал: 90 сек')).toBeVisible();
  await add(page, 'https://hours.example.com', '2', 'hours');
  await add(page, 'http://localhost:8080', '60', 'seconds');
  await expect(page.getByText('Интервал: 2 ч')).toBeVisible();
  await expect(page.getByText('Интервал: 1 мин')).toBeVisible();
  await page.reload();
  await expect(
    page
      .getByRole('list', { name: 'Сайты для мониторинга' })
      .getByRole('listitem'),
  ).toHaveCount(3);
  await expect(page.getByText(url, { exact: true })).toBeVisible();
  for (const width of [1440, 375]) {
    await page.setViewportSize({ width, height: 1000 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: `test-results/monitor-list-${width}-${info.project.name}.png`,
      fullPage: true,
    });
  }
  await page.getByRole('button', { name: 'Добавить сайт' }).click();
  await page.getByLabel('URL сайта').fill(url);
  await page.getByRole('button', { name: 'Создать', exact: true }).click();
  await expect(page.locator('main').getByRole('alert')).toContainText(
    'Этот сайт уже добавлен',
  );
  await expect(page.getByLabel('URL сайта')).toHaveValue(url);
  await expect(
    page
      .getByRole('list', { name: 'Сайты для мониторинга' })
      .getByRole('listitem'),
  ).toHaveCount(3);
  await page.getByRole('button', { name: 'Отмена' }).click();
  await page.getByRole('button', { name: 'Меню пользователя' }).click();
  await page.getByRole('menuitem', { name: 'Выйти', exact: true }).click();
  await expect(page).toHaveURL(/\/login$/);
  await register(page);
  await expect(page.getByText(url, { exact: true })).toHaveCount(0);
});

test('recovers a failed list and preserves the form on server failure', async ({
  page,
}) => {
  await register(page);
  await page.route('**/api/v1/monitors', (route) =>
    route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: '{"error":{"code":"internal_error","message":"Internal server error"}}',
    }),
  );
  await page.getByRole('button', { name: 'Обновить список сайтов' }).click();
  await expect(page.locator('main').getByRole('alert')).toBeVisible();
  await page.unroute('**/api/v1/monitors');
  await page.getByRole('button', { name: 'Повторить загрузку' }).click();
  await expect(
    page.getByRole('heading', { name: 'Пока нет сайтов' }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Добавить сайт' }).click();
  await page.getByLabel('URL сайта').fill('https://failed.example.com');
  await page.route('**/api/v1/monitors', (route) =>
    route.fulfill({ status: 500, contentType: 'application/json', body: '{}' }),
  );
  await page.getByRole('button', { name: 'Создать', exact: true }).click();
  await expect(page.locator('main').getByRole('alert')).toBeVisible();
  await expect(page.getByLabel('URL сайта')).toHaveValue(
    'https://failed.example.com',
  );
  await page.unroute('**/api/v1/monitors');
  await page.getByRole('button', { name: 'Создать', exact: true }).click();
  await expect(page.getByText('Сайт добавлен.')).toBeVisible();
});

test('keeps the draft when the session check loses its connection', async ({
  page,
}) => {
  await register(page);
  await page.getByRole('button', { name: 'Добавить сайт' }).click();
  await page.getByLabel('URL сайта').fill('https://draft.example.com');
  await page.route('**/api/v1/auth/me', (route) => route.abort());
  await page.getByRole('button', { name: 'Создать', exact: true }).click();
  await expect(page.locator('main').getByRole('alert')).toBeVisible();
  await expect(page.getByLabel('URL сайта')).toHaveValue(
    'https://draft.example.com',
  );
  await page.unroute('**/api/v1/auth/me');
  await page.getByRole('button', { name: 'Создать', exact: true }).click();
  await expect(page.getByText('Сайт добавлен.')).toBeVisible();
});

test('edits a monitor with keyboard controls, preserves order and persists after reload', async ({
  page,
}, info) => {
  await register(page);
  const original = 'https://original.example.com';
  const other = 'https://other.example.com';
  await add(page, original, '90', 'seconds');
  await add(page, other, '2', 'hours');
  const edit = page.getByRole('button', {
    name: `Редактировать ${original}`,
    exact: true,
  });
  await edit.focus();
  await edit.press('Enter');
  await expect(page.getByLabel('URL сайта')).toBeFocused();
  await expect(page.getByLabel('Интервал опроса')).toHaveValue('90');
  await expect(page.getByLabel('Единица измерения')).toHaveValue('seconds');
  await page.getByLabel('URL сайта').fill('https://discard.example.com');
  await page.getByRole('button', { name: 'Отмена' }).click();
  await expect(edit).toBeFocused();
  await edit.press('Enter');
  await expect(page.getByLabel('URL сайта')).toHaveValue(original);
  const updated = `https://updated.example.com/${'long-path-'.repeat(15)}?source=uptime`;
  await page.getByLabel('URL сайта').fill(updated);
  await page.getByLabel('Единица измерения').selectOption('minutes');
  await page.getByLabel('Интервал опроса').fill('1.5');
  for (const width of [1440, 375]) {
    await page.setViewportSize({ width, height: 1000 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: `test-results/monitor-edit-${width}-${info.project.name}.png`,
      fullPage: true,
    });
  }
  let finish!: () => void;
  const gate = new Promise<void>((resolve) => {
    finish = resolve;
  });
  let puts = 0;
  await page.route('**/api/v1/monitors/*', async (route) => {
    if (route.request().method() === 'PUT') {
      puts++;
      await gate;
    }
    await route.continue();
  });
  const response = page.waitForResponse(
    (response) =>
      response.request().method() === 'PUT' &&
      response.url().includes('/api/v1/monitors/'),
  );
  await page.getByRole('button', { name: 'Сохранить', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Сохраняем…' })).toBeDisabled();
  await expect(page.getByRole('button', { name: 'Отмена' })).toBeDisabled();
  await expect(
    page.getByRole('button', { name: 'Обновить список сайтов', exact: true }),
  ).toBeDisabled();
  await page.getByRole('button', { name: 'Меню пользователя' }).click();
  await expect(
    page.getByRole('menuitem', { name: 'Выйти', exact: true }),
  ).toBeDisabled();
  await page.keyboard.press('Escape');
  finish();
  const saved = await (await response).json();
  expect(saved.url).toBe(updated);
  expect(saved.interval_seconds).toBe(90);
  expect(Date.parse(saved.updated_at)).toBeGreaterThan(
    Date.parse(saved.created_at),
  );
  await expect(page.getByText('Изменения сохранены.')).toBeVisible();
  expect(puts).toBe(1);
  const cards = page
    .getByRole('list', { name: 'Сайты для мониторинга' })
    .getByRole('listitem');
  await expect(cards.nth(0)).toContainText(other);
  await expect(cards.nth(1)).toContainText(updated);
  await expect(
    page.getByRole('form', { name: 'Редактирование сайта' }),
  ).toHaveCount(0);
  await page.unroute('**/api/v1/monitors/*');
  await page.reload();
  await expect(cards.nth(0)).toContainText(other);
  await expect(cards.nth(1)).toContainText(updated);
  for (const width of [1440, 375]) {
    await page.setViewportSize({ width, height: 1000 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: `test-results/monitor-edited-list-${width}-${info.project.name}.png`,
      fullPage: true,
    });
  }
});

test('preserves an edit draft after conflict, server error and a failed session check', async ({
  page,
}) => {
  await register(page);
  await add(page, 'https://first.example.com', '5', 'minutes');
  await add(page, 'https://second.example.com', '2', 'hours');
  await page
    .getByRole('button', {
      name: 'Редактировать https://second.example.com',
      exact: true,
    })
    .click();
  await expect(page.getByLabel('Интервал опроса')).toHaveValue('2');
  await expect(page.getByLabel('Единица измерения')).toHaveValue('hours');
  await page.getByLabel('URL сайта').fill('https://first.example.com');
  await page.getByRole('button', { name: 'Сохранить', exact: true }).click();
  await expect(page.locator('main').getByRole('alert')).toContainText(
    'Этот сайт уже добавлен',
  );
  await expect(page.getByLabel('URL сайта')).toHaveValue(
    'https://first.example.com',
  );
  const draft = 'https://draft.example.com';
  await page.getByLabel('URL сайта').fill(draft);
  await page.route('**/api/v1/monitors/*', (route) =>
    route.fulfill({ status: 500, contentType: 'application/json', body: '{}' }),
  );
  await page.getByRole('button', { name: 'Сохранить', exact: true }).click();
  await expect(page.locator('main').getByRole('alert')).toContainText(
    'Не удалось выполнить запрос',
  );
  await expect(page.getByLabel('URL сайта')).toHaveValue(draft);
  await page.unroute('**/api/v1/monitors/*');
  await page.route('**/api/v1/auth/me', (route) => route.abort());
  await page.getByRole('button', { name: 'Сохранить', exact: true }).click();
  await expect(page.locator('main').getByRole('alert')).toContainText(
    'Не удалось подтвердить сохранение',
  );
  await expect(page.getByLabel('URL сайта')).toHaveValue(draft);
  await page.unroute('**/api/v1/auth/me');
  await page.getByRole('button', { name: 'Сохранить', exact: true }).click();
  await expect(page.getByText('Изменения сохранены.')).toBeVisible();
  await page.reload();
  await expect(
    page
      .getByRole('list', { name: 'Сайты для мониторинга' })
      .getByText(draft, { exact: true }),
  ).toBeVisible();
});

test('confirms deletion, prevents repeat requests and persists the empty state', async ({
  page,
}, info) => {
  await register(page);
  const url = `https://delete.example.com/${'long-path-'.repeat(15)}?source=uptime`;
  await add(page, url, '5', 'minutes');
  const open = page.getByRole('button', {
    name: `Удалить ${url}`,
    exact: true,
  });
  await open.focus();
  await open.press('Enter');
  const confirmation = page.getByRole('group', {
    name: `Удалить сайт ${url}?`,
  });
  await expect(confirmation).toBeVisible();
  await expect(
    confirmation.getByRole('button', { name: 'Отмена' }),
  ).toBeFocused();
  await confirmation.getByRole('button', { name: 'Отмена' }).click();
  await expect(open).toBeFocused();
  await expect(confirmation).toHaveCount(0);
  await page.reload();
  await expect(open).toBeVisible();
  await open.click();
  for (const width of [1440, 640, 375]) {
    await page.setViewportSize({ width, height: 1000 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: `test-results/monitor-delete-${width}-${info.project.name}.png`,
      fullPage: true,
    });
  }
  let finish!: () => void;
  const gate = new Promise<void>((resolve) => {
    finish = resolve;
  });
  let deletes = 0;
  await page.route('**/api/v1/monitors/*', async (route) => {
    if (route.request().method() === 'DELETE') {
      deletes++;
      await gate;
    }
    await route.continue();
  });
  await confirmation
    .getByRole('button', { name: 'Удалить сайт', exact: true })
    .click();
  await expect(
    confirmation.getByRole('button', { name: 'Удаляем…' }),
  ).toBeDisabled();
  await expect(
    confirmation.getByRole('button', { name: 'Отмена' }),
  ).toBeDisabled();
  await expect(
    page.getByRole('button', { name: 'Обновить список сайтов', exact: true }),
  ).toBeDisabled();
  await expect(
    page.getByRole('button', { name: 'Добавить сайт' }),
  ).toBeDisabled();
  await page.getByRole('button', { name: 'Меню пользователя' }).click();
  await expect(
    page.getByRole('menuitem', { name: 'Выйти', exact: true }),
  ).toBeDisabled();
  await page.keyboard.press('Escape');
  finish();
  await expect(page.getByText('Сайт удалён.')).toBeVisible();
  await expect(
    page.getByRole('heading', { name: 'Пока нет сайтов' }),
  ).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Добавить сайт' }),
  ).toBeFocused();
  expect(deletes).toBe(1);
  await page.unroute('**/api/v1/monitors/*');
  await page.reload();
  await expect(
    page.getByRole('heading', { name: 'Пока нет сайтов' }),
  ).toBeVisible();
});

test('retains deletion confirmation after errors and reconciles a missing monitor', async ({
  page,
}) => {
  await register(page);
  const original = 'https://delete-errors.example.com';
  const other = 'https://keep.example.com';
  await add(page, original, '5', 'minutes');
  await add(page, other, '1', 'hours');
  await page
    .getByRole('button', { name: `Удалить ${original}`, exact: true })
    .click();
  const confirmation = page.getByRole('group', {
    name: `Удалить сайт ${original}?`,
  });
  await page.route('**/api/v1/monitors/*', (route) =>
    route.fulfill({ status: 500, contentType: 'application/json', body: '{}' }),
  );
  await confirmation
    .getByRole('button', { name: 'Удалить сайт', exact: true })
    .click();
  await expect(confirmation).toContainText('Не удалось выполнить запрос');
  await page.unroute('**/api/v1/monitors/*');
  await page.route('**/api/v1/auth/me', (route) => route.abort());
  await confirmation
    .getByRole('button', { name: 'Удалить сайт', exact: true })
    .click();
  await expect(confirmation).toContainText('Не удалось подтвердить удаление');
  await page.unroute('**/api/v1/auth/me');
  let deletes = 0;
  await page.route('**/api/v1/monitors/*', async (route) => {
    if (route.request().method() !== 'DELETE') {
      await route.continue();
      return;
    }
    deletes++;
    // Commit the deletion but lose its response, leaving the UI uncertain.
    const target = new URL(route.request().url());
    // Node's request client does not use Chromium's test-domain resolver.
    target.hostname = '127.0.0.1';
    const response = await route.fetch({ url: target.href });
    expect(response.status()).toBe(204);
    await route.abort();
  });
  await confirmation
    .getByRole('button', { name: 'Удалить сайт', exact: true })
    .click();
  await expect(confirmation).toContainText('Не удалось подтвердить удаление');
  expect(deletes).toBe(1);
  await expect(
    page.getByRole('button', { name: `Удалить ${original}`, exact: true }),
  ).toBeVisible();
  await page.unroute('**/api/v1/monitors/*');
  await confirmation
    .getByRole('button', { name: 'Удалить сайт', exact: true })
    .click();
  await expect(confirmation).toContainText('Сайт больше не найден');
  await confirmation
    .getByRole('button', { name: 'Обновить список сайтов', exact: true })
    .click();
  await expect(confirmation).toHaveCount(0);
  await expect(
    page.getByRole('button', { name: `Удалить ${original}`, exact: true }),
  ).toHaveCount(0);
  await expect(
    page
      .getByRole('list', { name: 'Сайты для мониторинга' })
      .getByRole('listitem'),
  ).toHaveCount(1);
  await expect(page.getByText(other, { exact: true })).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Добавить сайт' }),
  ).toBeEnabled();
  await page.reload();
  await expect(
    page
      .getByRole('list', { name: 'Сайты для мониторинга' })
      .getByRole('listitem'),
  ).toHaveCount(1);
  await expect(page.getByText(other, { exact: true })).toBeVisible();
});
