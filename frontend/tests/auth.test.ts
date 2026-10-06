import { describe, expect, it, vi } from 'vitest';
import { apiOrigin, AuthAPI, AuthError, errorMessage } from '@/lib/auth/api';
import { validate } from '@/lib/auth/validation';
import { Session } from '@/lib/auth/session';

const user = { id: '1', name: 'Иван', email: 'ivan@example.com' };
const unauthorized = () => Promise.reject(new AuthError('http', 401));
const coordination = () => {
  const lockSpy = vi.fn();
  return {
    lock: <T>(task: () => Promise<T>) => {
      lockSpy();
      return task();
    },
    lockSpy,
    broadcast: vi.fn(),
    isUncertain: vi.fn(() => false),
    setUncertain: vi.fn(),
  };
};
function fixture() {
  const api = new AuthAPI(() => 'http://localhost:8080');
  const call = vi.spyOn(api, 'call');
  const sync = coordination();
  return { call, sync, session: new Session(api, sync) };
}
describe('validation and contract', () => {
  it('counts Unicode code points and preserves password spaces', () => {
    const password = ' ' + '🔐'.repeat(13) + ' ';
    const result = validate(
      {
        name: '  ' + '👩'.repeat(50) + '  ',
        email: ' IVAN@EXAMPLE.COM ',
        password,
      },
      true,
    );
    expect(result.errors).toEqual({});
    expect(result.values.password).toBe(password);
    expect(result.values.email).toBe('ivan@example.com');
    expect(
      validate({ ...result.values, name: '👩'.repeat(51) }, true).errors.name,
    ).toBeTruthy();
    expect(
      validate({ email: 'é'.repeat(125) + '@a.com', password }, false).errors
        .email,
    ).toBeTruthy();
    expect(
      validate({ email: 'a@b.com', password: 'x'.repeat(129) }, false).errors
        .password,
    ).toBeTruthy();
  });
  it('rejects missing and non-origin configuration', () => {
    for (const value of [
      undefined,
      '',
      'https://api.test/path',
      'https://user:pass@api.test',
      'ftp://api.test',
      'https://api.test?q=1',
    ])
      expect(() => apiOrigin(value)).toThrow(AuthError);
    expect(apiOrigin('http://localhost:8080')).toBe('http://localhost:8080');
  });
  it('sends only credentials cookies and CSRF with public response data', async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify({ user, access_token: 'ignored' })),
      );
    const api = new AuthAPI(() => 'http://localhost:8080', fetcher);
    expect(
      await api.call('login', {
        email: user.email,
        password: 'test password 123',
      }),
    ).toEqual(user);
    expect(fetcher.mock.calls[0][1]).toMatchObject({
      credentials: 'include',
      cache: 'no-store',
      method: 'POST',
      headers: { 'X-CSRF-Protection': '1', 'Content-Type': 'application/json' },
    });
    expect(fetcher.mock.calls[0][1].headers).not.toHaveProperty(
      'Authorization',
    );
  });
  it.each([400, 401, 403, 409, 500])('reports HTTP %s', async (status) => {
    const api = new AuthAPI(
      () => 'http://localhost:8080',
      vi.fn().mockResolvedValue(new Response('{}', { status })),
    );
    await expect(api.call('login')).rejects.toMatchObject({ status });
    expect(errorMessage(new AuthError('http', status))).toBeTruthy();
  });
  it('distinguishes network errors and malformed responses', async () => {
    await expect(
      new AuthAPI(
        () => 'http://localhost',
        vi.fn().mockRejectedValue(new TypeError()),
      ).call('me'),
    ).rejects.toMatchObject({ kind: 'network' });
    await expect(
      new AuthAPI(
        () => 'http://localhost',
        vi.fn().mockResolvedValue(new Response('{}')),
      ).call('me'),
    ).rejects.toMatchObject({ kind: 'protocol' });
  });
});
describe('session coordination', () => {
  it('coalesces checks and refreshes once after rechecking under lock', async () => {
    const { call, session, sync } = fixture();
    call
      .mockImplementationOnce(unauthorized)
      .mockImplementationOnce(unauthorized)
      .mockResolvedValueOnce(user)
      .mockResolvedValueOnce(user);
    const one = session.check();
    expect(session.check()).toBe(one);
    await one;
    expect(call.mock.calls.map(([path]) => path)).toEqual([
      'me',
      'me',
      'refresh',
      'me',
    ]);
    expect(sync.lockSpy).toHaveBeenCalledTimes(1);
    expect(session.snapshot().status).toBe('authenticated');
  });
  it('uses a session refreshed by another tab instead of rotating again', async () => {
    const { call, session } = fixture();
    call.mockImplementationOnce(unauthorized).mockResolvedValueOnce(user);
    await session.check();
    expect(call.mock.calls.map(([path]) => path)).toEqual(['me', 'me']);
  });
  it('ends an invalid refresh session without retries', async () => {
    const { call, session } = fixture();
    call.mockImplementation(unauthorized);
    await session.check();
    expect(session.snapshot().status).toBe('anonymous');
    expect(call).toHaveBeenCalledTimes(3);
  });
  it('keeps network failures as errors and never automatically retries uncertain refresh', async () => {
    const { call, session } = fixture();
    call.mockImplementation((path) =>
      path === 'refresh'
        ? Promise.reject(new AuthError('network'))
        : unauthorized(),
    );
    await session.check();
    await session.check();
    expect(session.snapshot().status).toBe('error');
    expect(call.mock.calls.filter(([path]) => path === 'refresh')).toHaveLength(
      1,
    );
    await session.check(true);
    expect(call.mock.calls.filter(([path]) => path === 'refresh')).toHaveLength(
      2,
    );
  });
  it('ignores an old response after logout', async () => {
    const { call, session } = fixture();
    let resolve!: (value: typeof user) => void;
    call.mockReturnValueOnce(
      new Promise((done) => {
        resolve = done;
      }),
    );
    const checking = session.check();
    session.receive('logout');
    resolve(user);
    await checking;
    expect(session.snapshot().status).toBe('anonymous');
  });
  it('ignores an in-flight me result after a local logout', async () => {
    const { call, session } = fixture();
    let resolve!: (value: typeof user) => void;
    call
      .mockReturnValueOnce(
        new Promise((done) => {
          resolve = done;
        }),
      )
      .mockResolvedValueOnce(undefined);
    const checking = session.check();
    await session.mutate('logout');
    resolve(user);
    await checking;
    expect(session.snapshot().status).toBe('anonymous');
  });
  it('does not refresh on a failed me network request', async () => {
    const { call, session, sync } = fixture();
    call.mockRejectedValue(new AuthError('network'));
    await session.check();
    expect(session.snapshot().status).toBe('error');
    expect(call).toHaveBeenCalledTimes(1);
    expect(sync.lockSpy).not.toHaveBeenCalled();
  });
  it('does not retry an uncertain POST recorded by another tab', async () => {
    const { call, session, sync } = fixture();
    sync.isUncertain.mockReturnValue(true);
    call.mockImplementation(unauthorized);
    await session.check();
    expect(session.snapshot().status).toBe('error');
    expect(call.mock.calls.map(([path]) => path)).toEqual(['me', 'me']);
  });
  it('clears the shared failure flag on explicit recovery of a valid session', async () => {
    const { call, session, sync } = fixture();
    sync.isUncertain.mockReturnValue(true);
    call.mockResolvedValue(user);
    await session.check(true);
    expect(session.snapshot().status).toBe('authenticated');
    expect(sync.setUncertain).toHaveBeenCalledWith(false);
  });
  it('detects blocked cookies without resubmitting login', async () => {
    const { call, session } = fixture();
    call.mockResolvedValueOnce(user).mockImplementationOnce(unauthorized);
    await expect(
      session.mutate('login', {
        email: user.email,
        password: 'test password 123',
      }),
    ).rejects.toMatchObject({ kind: 'cookies' });
    expect(call.mock.calls.map(([path]) => path)).toEqual(['login', 'me']);
  });
  it('refreshes expired access before saving and broadcasts the updated profile', async () => {
    const { call, session, sync } = fixture();
    const updated = { ...user, name: 'New' };
    call
      .mockImplementationOnce(unauthorized)
      .mockImplementationOnce(unauthorized)
      .mockResolvedValueOnce(user)
      .mockResolvedValueOnce(user)
      .mockResolvedValueOnce(updated)
      .mockResolvedValueOnce(updated);
    await session.mutate('profile', { name: 'New' });
    expect(call.mock.calls.map(([path]) => path)).toEqual([
      'me',
      'me',
      'refresh',
      'me',
      'profile',
      'me',
    ]);
    expect(session.snapshot().user).toEqual(updated);
    expect(sync.broadcast).toHaveBeenCalledWith('changed');
  });
  it('does not retry a profile write after a network failure', async () => {
    const { call, session, sync } = fixture();
    call
      .mockResolvedValueOnce(user)
      .mockRejectedValueOnce(new AuthError('network'));
    await expect(
      session.mutate('profile', { name: 'New' }),
    ).rejects.toMatchObject({ kind: 'network' });
    expect(call.mock.calls.map(([path]) => path)).toEqual(['me', 'profile']);
    expect(sync.broadcast).toHaveBeenCalledWith('uncertain');
  });
  it('preserves an error when logout fails', async () => {
    const { call, session, sync } = fixture();
    call.mockRejectedValue(new AuthError('http', 500));
    await expect(session.mutate('logout')).rejects.toMatchObject({
      status: 500,
    });
    expect(session.snapshot().status).toBe('error');
    expect(sync.broadcast).not.toHaveBeenCalledWith('logout');
  });
});

it('sends avatars as multipart without setting the boundary header and retains avatar URLs', async () => {
  const avatar_url =
    '/api/v1/avatars/00000000-0000-4000-8000-000000000001.webp';
  const fetcher = vi
    .fn()
    .mockResolvedValue(
      new Response(JSON.stringify({ user: { ...user, avatar_url } })),
    );
  const api = new AuthAPI(() => 'http://localhost:8080', fetcher);
  const avatar = new File(['webp'], 'avatar.webp', { type: 'image/webp' });
  expect(await api.call('profile', { name: 'Иван', avatar })).toEqual({
    ...user,
    avatar_url,
  });
  const request = fetcher.mock.calls[0][1];
  expect(request.headers).toEqual({ 'X-CSRF-Protection': '1' });
  expect(request.credentials).toBe('include');
  expect(request.body).toBeInstanceOf(FormData);
  expect(request.body.get('name')).toBe('Иван');
  expect(request.body.get('avatar')).toEqual(avatar);
});
