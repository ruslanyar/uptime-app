import { test, expect } from '@playwright/test';

const password = ' browser password 123 ';
const apiURL = () => 'https://api.auth-service.test:18443';
test('blocking third-party cookies detects an unsaved session', async ({
  page,
  context,
}) => {
  const cdp = await context.newCDPSession(page);
  await cdp.send('Network.enable');
  await cdp.send('Network.setCookieControls', {
    enableThirdPartyCookieRestriction: true,
    disableThirdPartyCookieMetadata: true,
    disableThirdPartyCookieHeuristics: true,
  });
  await page.goto('/register');
  await page.getByLabel('Имя', { exact: true }).fill('Анна');
  await page.getByLabel('Email').fill(`blocked-${Date.now()}@example.com`);
  await page.getByLabel('Пароль', { exact: true }).fill(password);
  await page.getByRole('button', { name: 'Создать аккаунт' }).click();
  await expect(page.locator('main').getByRole('alert')).toContainText(
    'Не удалось сохранить сессию',
  );
  expect(await context.cookies(apiURL() + '/api/v1/auth/me')).toHaveLength(0);
});
