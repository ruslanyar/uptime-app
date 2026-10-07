import { expect, it, vi } from 'vitest';
import { AuthError } from '@/lib/auth/api';
import { MonitorAPI } from '@/lib/monitors/api';
import { formatInterval, validateMonitor } from '@/lib/monitors/validation';

const item = {
  id: 'monitor-id',
  url: 'https://example.com',
  interval_seconds: 300,
  created_at: '2026-10-07T10:00:00Z',
  updated_at: '2026-10-07T10:00:00Z',
};
it.each([
  ['60', 'seconds', 60],
  ['1440', 'minutes', 86400],
  ['24', 'hours', 86400],
  ['1.5', 'minutes', 90],
  ['0.5', 'hours', 1800],
] as const)('converts %s %s to integer seconds', (amount, unit, seconds) => {
  expect(
    validateMonitor(' https://example.com/path?q=1 ', amount, unit).input,
  ).toEqual({ url: 'https://example.com/path?q=1', interval_seconds: seconds });
});
it.each([
  '',
  'example.com',
  'ftp://example.com',
  'https://user:pass@example.com',
  'https://@example.com',
  'https://example.com/#',
  'https://example.com:0',
  'https://example.com:65536',
  'https://example.com:',
  'https://example.com/%zz',
  'https://example.com/a b',
  'https://example.com/' + 'я'.repeat(1024),
])('rejects invalid URL %s', (url) => {
  expect(validateMonitor(url, '5', 'minutes').errors.url).toBeTruthy();
});
it.each(['', '0', '-1', '59', '86401', 'NaN', 'Infinity', '60.5'])(
  'rejects invalid seconds %s',
  (amount) => {
    expect(
      validateMonitor(item.url, amount, 'seconds').errors.interval,
    ).toBeTruthy();
  },
);
it('formats intervals in the largest exact unit', () => {
  expect([60, 90, 300, 7200].map(formatInterval)).toEqual([
    '1 мин',
    '90 сек',
    '5 мин',
    '2 ч',
  ]);
});
it('creates with cookies and CSRF, and lists without a mutation header', async () => {
  const request = vi
    .fn()
    .mockResolvedValueOnce(new Response(JSON.stringify(item)))
    .mockResolvedValueOnce(new Response(JSON.stringify({ monitors: [item] })));
  const api = new MonitorAPI(() => 'https://api.example.com', request);
  expect(await api.create({ url: item.url, interval_seconds: 300 })).toEqual(
    item,
  );
  expect(request).toHaveBeenNthCalledWith(
    1,
    'https://api.example.com/api/v1/monitors',
    expect.objectContaining({
      method: 'POST',
      credentials: 'include',
      cache: 'no-store',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Protection': '1' },
      body: JSON.stringify({ url: item.url, interval_seconds: 300 }),
    }),
  );
  const signal = new AbortController().signal;
  expect(await api.list(signal)).toEqual([item]);
  expect(request).toHaveBeenNthCalledWith(
    2,
    'https://api.example.com/api/v1/monitors',
    { method: 'GET', credentials: 'include', cache: 'no-store', signal },
  );
});
it.each([400, 401, 409, 500])(
  'preserves HTTP %s and never retries POST',
  async (status) => {
    const request = vi.fn().mockResolvedValue(new Response('{}', { status }));
    await expect(
      new MonitorAPI(() => 'https://api.example.com', request).create(item),
    ).rejects.toMatchObject({ kind: 'http', status });
    expect(request).toHaveBeenCalledTimes(1);
  },
);
it('reports a network error without retrying', async () => {
  const request = vi.fn().mockRejectedValue(new Error('offline'));
  await expect(
    new MonitorAPI(() => 'https://api.example.com', request).create(item),
  ).rejects.toBeInstanceOf(AuthError);
  expect(request).toHaveBeenCalledTimes(1);
});
it.each([
  {},
  { monitors: null },
  { monitors: [null] },
  { monitors: [{ ...item, interval_seconds: 59 }] },
  { monitors: [{ ...item, created_at: 'invalid' }] },
  { monitors: [{ ...item, updated_at: undefined }] },
  { monitors: [{ ...item, updated_at: null }] },
  { monitors: [{ ...item, updated_at: 'invalid' }] },
])('rejects malformed lists', async (body) => {
  const request = vi.fn().mockResolvedValue(new Response(JSON.stringify(body)));
  await expect(
    new MonitorAPI(() => 'https://api.example.com', request).list(),
  ).rejects.toMatchObject({ kind: 'protocol' });
});

it('rejects a creation response without a valid updated_at', async () => {
  const request = vi
    .fn()
    .mockResolvedValue(
      new Response(JSON.stringify({ ...item, updated_at: 'invalid' })),
    );
  await expect(
    new MonitorAPI(() => 'https://api.example.com', request).create(item),
  ).rejects.toMatchObject({ kind: 'protocol' });
  expect(request).toHaveBeenCalledTimes(1);
});

it('updates with cookies and CSRF and preserves the server timestamp', async () => {
  const updated = {
    ...item,
    url: 'https://edited.example.com',
    interval_seconds: 90,
    updated_at: '2026-10-07T12:00:00Z',
  };
  const request = vi
    .fn()
    .mockResolvedValue(new Response(JSON.stringify(updated)));
  const input = {
    url: updated.url,
    interval_seconds: updated.interval_seconds,
  };
  expect(
    await new MonitorAPI(() => 'https://api.example.com', request).update(
      item.id,
      input,
    ),
  ).toEqual(updated);
  expect(request).toHaveBeenCalledExactlyOnceWith(
    'https://api.example.com/api/v1/monitors/monitor-id',
    expect.objectContaining({
      method: 'PUT',
      credentials: 'include',
      cache: 'no-store',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Protection': '1' },
      body: JSON.stringify(input),
    }),
  );
});
it.each([400, 401, 404, 409, 500])(
  'preserves update HTTP %s without retrying',
  async (status) => {
    const request = vi.fn().mockResolvedValue(new Response('{}', { status }));
    await expect(
      new MonitorAPI(() => 'https://api.example.com', request).update(
        item.id,
        item,
      ),
    ).rejects.toMatchObject({ kind: 'http', status });
    expect(request).toHaveBeenCalledTimes(1);
  },
);
it.each([
  { ...item, id: 'other' },
  { ...item, updated_at: 'invalid' },
])('rejects malformed update responses', async (body) => {
  const request = vi.fn().mockResolvedValue(new Response(JSON.stringify(body)));
  await expect(
    new MonitorAPI(() => 'https://api.example.com', request).update(
      item.id,
      item,
    ),
  ).rejects.toMatchObject({ kind: 'protocol' });
  expect(request).toHaveBeenCalledTimes(1);
});
it('does not retry an update with an unknown network result', async () => {
  const request = vi.fn().mockRejectedValue(new Error('offline'));
  await expect(
    new MonitorAPI(() => 'https://api.example.com', request).update(
      item.id,
      item,
    ),
  ).rejects.toMatchObject({ kind: 'network' });
  expect(request).toHaveBeenCalledTimes(1);
});
