import { renderToString } from 'react-dom/server';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { Account } from '@/components/account';
import { AuthForm } from '@/components/auth-form';
import Login from '@/app/login/page';
import Register from '@/app/register/page';
import { AuthError } from '@/lib/auth/api';
import type { SessionState } from '@/lib/auth/session';

const { auth, redirect } = vi.hoisted(() => ({
  auth: {
    status: 'loading',
    user: undefined,
    error: undefined,
    session: { check: vi.fn(), mutate: vi.fn() },
  } as SessionState & {
    session: {
      check: ReturnType<typeof vi.fn>;
      mutate: ReturnType<typeof vi.fn>;
    };
  },
  redirect: vi.fn((path: string) => {
    throw new Error(`redirect:${path}`);
  }),
}));
vi.mock('@/components/auth-provider', () => ({ useAuth: () => auth }));
vi.mock('next/navigation', () => ({ redirect }));

describe('server content and session navigation', () => {
  beforeEach(() => {
    auth.status = 'loading';
    auth.user = undefined;
    auth.error = undefined;
    redirect.mockClear();
  });

  it.each([
    [Login, 'Войти', 'Рады видеть вас снова в Uptime.'],
    [Register, 'Создать аккаунт', 'Начните с личного аккаунта в Uptime.'],
  ])(
    'renders public content before session verification',
    (Page, title, description) => {
      const html = renderToString(<Page />);
      expect(html).toContain(`<h1>${title}</h1>`);
      expect(html).toContain(description);
      expect(html).toContain('Проверяем сессию…');
      expect(html).not.toContain('<input');
      expect(html).not.toContain('footer-link');
      expect(redirect).not.toHaveBeenCalled();
    },
  );

  it.each([Login, Register])(
    'renders the server footer with the anonymous form',
    (Page) => {
      auth.status = 'anonymous';
      const html = renderToString(<Page />);
      expect(html).toContain('<input');
      expect(html).toContain('footer-link');
      expect(redirect).not.toHaveBeenCalled();
    },
  );

  it.each([false, true])(
    'redirects anonymous account/root to login',
    (redirectOnly) => {
      auth.status = 'anonymous';
      expect(() =>
        renderToString(<Account redirectOnly={redirectOnly} />),
      ).toThrow('redirect:/login');
    },
  );

  it.each([false, true])(
    'redirects authenticated forms to account',
    (register) => {
      auth.status = 'authenticated';
      expect(() => renderToString(<AuthForm register={register} />)).toThrow(
        'redirect:/account',
      );
    },
  );

  it('redirects authenticated root and displays authenticated account', () => {
    auth.status = 'authenticated';
    auth.user = { id: '1', name: 'Анна', email: 'anna@example.com' };
    expect(() => renderToString(<Account redirectOnly />)).toThrow(
      'redirect:/account',
    );
    redirect.mockClear();
    expect(renderToString(<Account />)).toContain('anna@example.com');
    expect(redirect).not.toHaveBeenCalled();
  });

  it.each(['loading', 'error'] as const)(
    'does not navigate from %s',
    (status) => {
      auth.status = status;
      auth.error = status === 'error' ? new AuthError('network') : undefined;
      for (const element of [
        <Account key="account" />,
        <Account key="root" redirectOnly />,
        <AuthForm key="login" />,
        <AuthForm key="register" register />,
      ]) {
        const html = renderToString(element);
        expect(html).not.toContain('<input');
        expect(html).not.toContain('profile');
      }
      expect(redirect).not.toHaveBeenCalled();
    },
  );
});
