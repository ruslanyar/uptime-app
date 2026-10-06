import { renderToString } from 'react-dom/server';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { Dashboard } from '@/components/dashboard';
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
      expect(html).toMatch(new RegExp(`<h1[^>]*>${title}</h1>`));
      expect(html).toContain(description);
      expect(html).toContain('Проверяем сессию…');
      expect(html).not.toContain('<input');
      expect(html).not.toContain('href="/register"');
      expect(html).not.toContain('href="/login"');
      expect(redirect).not.toHaveBeenCalled();
    },
  );

  it.each([Login, Register])(
    'renders the server footer with the anonymous form',
    (Page) => {
      auth.status = 'anonymous';
      const html = renderToString(<Page />);
      expect(html).toContain('<input');
      expect(html).toMatch(/href="\/(login|register)"/);
      expect(redirect).not.toHaveBeenCalled();
    },
  );

  it.each([Account, Dashboard])(
    'redirects anonymous account/root to login',
    (Page) => {
      auth.status = 'anonymous';
      expect(() => renderToString(<Page />)).toThrow('redirect:/login');
    },
  );

  it.each([false, true])(
    'redirects authenticated forms to dashboard',
    (register) => {
      auth.status = 'authenticated';
      expect(() => renderToString(<AuthForm register={register} />)).toThrow(
        'redirect:/',
      );
    },
  );

  it('displays authenticated dashboard and account', () => {
    auth.status = 'authenticated';
    auth.user = { id: '1', name: 'Анна', email: 'anna@example.com' };
    expect(renderToString(<Dashboard />)).toContain('Меню пользователя');
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
        <Dashboard key="root" />,
        <AuthForm key="login" />,
        <AuthForm key="register" register />,
      ]) {
        const html = renderToString(element);
        expect(html).not.toContain('<input');
        expect(html).not.toContain('Данные аккаунта');
      }
      expect(redirect).not.toHaveBeenCalled();
    },
  );
});
