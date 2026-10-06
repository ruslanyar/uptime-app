'use client';
import { useRef, useState } from 'react';
import Link from 'next/link';
import { redirect } from 'next/navigation';
import { useAuth } from '@/components/auth-provider';
import { PageShell } from '@/components/page-shell';
import { SessionStatus } from '@/components/session-status';
import { errorMessage } from '@/lib/auth/api';

export function Dashboard() {
  const auth = useAuth();
  const menu = useRef<HTMLDetailsElement>(null);
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState('');

  if (auth.status === 'anonymous') redirect('/login');
  if (auth.status !== 'authenticated' || !auth.user)
    return (
      <PageShell>
        <SessionStatus />
      </PageShell>
    );

  async function logout() {
    setPending(true);
    setMessage('');
    try {
      await auth.session.mutate('logout');
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="shell dashboard-shell">
      <header className="dashboard-header">
        <Link className="brand" href="/" aria-label="Uptime — главная">
          <span aria-hidden="true" />
          uptime
        </Link>
        <details
          className="user-menu"
          ref={menu}
          onKeyDown={(event) => {
            if (event.key === 'Escape' && menu.current) {
              menu.current.open = false;
              menu.current.querySelector('summary')?.focus();
            }
          }}
          onBlur={(event) => {
            if (!event.currentTarget.contains(event.relatedTarget))
              event.currentTarget.open = false;
          }}
        >
          <summary aria-label="Меню пользователя">
            <span className="user-avatar" aria-hidden="true">
              {Array.from(auth.user.name.trim())[0]?.toLocaleUpperCase('ru')}
            </span>
            <span className="user-name">{auth.user.name}</span>
            <span className="user-chevron" aria-hidden="true">
              ⌄
            </span>
          </summary>
          <div className="user-menu-panel">
            <p className="user-menu-name">{auth.user.name}</p>
            <p className="user-menu-email">{auth.user.email}</p>
            <nav aria-label="Аккаунт пользователя">
              <Link href="/account">Мой профиль</Link>
              <button type="button" disabled={pending} onClick={logout}>
                {pending ? 'Выходим…' : 'Выйти'}
              </button>
            </nav>
          </div>
        </details>
      </header>
      <main className="dashboard-main">
        <section
          className="dashboard-content"
          aria-labelledby="dashboard-title"
        >
          <p className="eyebrow">ОБЗОР</p>
          <h1 id="dashboard-title">Здравствуйте, {auth.user.name}</h1>
          <p className="muted">Добро пожаловать в ваш дашборд Uptime.</p>
          {message && (
            <p role="alert" className="notice">
              {message}
            </p>
          )}
          <div className="dashboard-empty">
            <span className="dashboard-empty-icon" aria-hidden="true">
              ◷
            </span>
            <h2>Мониторинг скоро появится</h2>
            <p className="muted">
              Здесь будут ваши мониторы и информация о доступности сервисов.
            </p>
          </div>
        </section>
      </main>
    </div>
  );
}
