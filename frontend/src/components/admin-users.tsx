'use client';
import { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { redirect } from 'next/navigation';
import { useAuth } from '@/components/auth-provider';
import { Brand, Eyebrow, PageShell, Surface } from '@/components/page-shell';
import { SessionStatus } from '@/components/session-status';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Alert } from '@/components/ui/alert';
import { AuthError } from '@/lib/auth/api';
import {
  AdminAPI,
  adminErrorMessage,
  type AdminUser,
  type UserList,
} from '@/lib/admin/api';

export function AdminUsers() {
  const auth = useAuth();
  if (auth.status === 'anonymous') redirect('/login');
  if (auth.status !== 'authenticated' || !auth.user)
    return (
      <PageShell>
        <SessionStatus />
      </PageShell>
    );
  return (
    <Surface>
      <header className="flex items-center justify-between gap-4 border-b border-border/60 px-6 py-5 sm:px-12">
        <Brand />
        <Button asChild variant="outline">
          <Link href="/">На главную</Link>
        </Button>
      </header>
      <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-10 sm:px-12 sm:py-16">
        <Eyebrow>Администрирование</Eyebrow>
        <h1 className="text-3xl font-medium tracking-tight sm:text-5xl">
          Пользователи
        </h1>
        {auth.user.role === 'admin' ? (
          <UserDirectory key={auth.user.id} userID={auth.user.id} />
        ) : (
          <Alert className="mt-6">Доступ к админке запрещён.</Alert>
        )}
      </main>
    </Surface>
  );
}

/** Remount for another administrator so no account data survives a user change. */
function UserDirectory({ userID }: { userID: string }) {
  const { session } = useAuth();
  const [api] = useState(() => new AdminAPI());
  const [page, setPage] = useState(1);
  const [revision, setRevision] = useState(0);
  const [state, setState] = useState<{
    data?: UserList;
    loading: boolean;
    error?: unknown;
  }>({ loading: true });
  const [target, setTarget] = useState<AdminUser>();
  const [pending, setPending] = useState(false);
  const [deleteError, setDeleteError] = useState<unknown>();
  const [needsReload, setNeedsReload] = useState(false);
  const [message, setMessage] = useState('');
  const cancelButton = useRef<HTMLButtonElement>(null);
  const refreshButton = useRef<HTMLButtonElement>(null);
  const trigger = useRef<HTMLButtonElement | null>(null);
  const deleting = useRef(false);
  const active = useRef(true);

  useEffect(() => {
    active.current = true;
    return () => {
      active.current = false;
    };
  }, []);
  useEffect(() => {
    if (pending || state.loading) return;
    if (target) cancelButton.current?.focus();
    else if (trigger.current) {
      if (trigger.current.isConnected) trigger.current.focus();
      else refreshButton.current?.focus();
    }
  }, [target, pending, state.loading]);

  useEffect(() => {
    const controller = new AbortController();
    let current = true;
    async function load() {
      try {
        let data: UserList;
        try {
          data = await api.list(page, controller.signal);
        } catch (error) {
          if (!(error instanceof AuthError) || error.status !== 401)
            throw error;
          await session.check();
          const auth = session.snapshot();
          if (
            !current ||
            auth.status !== 'authenticated' ||
            auth.user?.id !== userID ||
            auth.user.role !== 'admin'
          )
            throw error;
          data = await api.list(page, controller.signal);
        }
        if (!current) return;
        if (data.users.length === 0 && page > 1 && data.total > 0) {
          setPage(Math.max(1, Math.ceil(data.total / 25)));
          return;
        }
        setState({ data, loading: false });
        setNeedsReload(false);
      } catch (error) {
        if (current) setState({ loading: false, error });
      }
    }
    void load();
    return () => {
      current = false;
      controller.abort();
    };
  }, [api, session, userID, page, revision]);

  function close() {
    setTarget(undefined);
    setDeleteError(undefined);
  }
  function reload(nextPage = page) {
    close();
    setState({ loading: true });
    setPage(nextPage);
    setRevision((value) => value + 1);
  }

  async function remove() {
    if (!target || deleting.current || needsReload) return;
    deleting.current = true;
    setPending(true);
    setDeleteError(undefined);
    try {
      const auth = session.snapshot();
      if (
        auth.status !== 'authenticated' ||
        auth.user?.id !== userID ||
        auth.user.role !== 'admin'
      )
        throw new AuthError('http', 401);
      await api.delete(target.id);
      if (!active.current) return;
      setMessage('Пользователь удалён.');
      reload();
    } catch (error) {
      if (!active.current) return;
      setDeleteError(error);
      setNeedsReload(true);
      if (error instanceof AuthError && error.status === 403) {
        close();
        setState({ loading: false, error });
      }
    } finally {
      deleting.current = false;
      if (active.current) setPending(false);
    }
  }

  return (
    <section className="mt-8" aria-label="Управление пользователями">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <p className="text-sm text-muted-foreground">
          {state.data
            ? `Всего пользователей: ${state.data.total}`
            : 'Зарегистрированные аккаунты'}
        </p>
        <Button
          ref={refreshButton}
          variant="outline"
          disabled={pending || state.loading}
          onClick={() => reload()}
        >
          Обновить список
        </Button>
      </div>
      {message && (
        <p role="status" className="mt-4 text-sm text-primary">
          {message}
        </p>
      )}
      {state.loading && (
        <p role="status" className="mt-6 text-sm text-muted-foreground">
          Загружаем пользователей…
        </p>
      )}
      {state.error !== undefined && (
        <Alert className="mt-6">
          <p>{adminErrorMessage(state.error)}</p>
        </Alert>
      )}
      {!state.loading && state.data && (
        <>
          {state.data.users.length === 0 ? (
            <Card className="mt-6 p-6">Пользователей пока нет.</Card>
          ) : (
            <ul
              aria-label="Зарегистрированные пользователи"
              className="mt-6 grid gap-4"
            >
              {state.data.users.map((user) => (
                <li key={user.id}>
                  <Card className="gap-4 px-6">
                    <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                      <div className="min-w-0">
                        <p className="font-medium wrap-anywhere">{user.name}</p>
                        <p className="mt-1 font-mono text-xs text-muted-foreground wrap-anywhere">
                          {user.email}
                        </p>
                      </div>
                      <div className="flex shrink-0 flex-wrap items-center gap-4">
                        <span className="font-mono text-xs text-muted-foreground">
                          {user.role === 'admin'
                            ? 'Администратор'
                            : 'Пользователь'}
                        </span>
                        {user.role === 'user' && (
                          <Button
                            variant="outline"
                            size="sm"
                            aria-label={`Удалить ${user.email}`}
                            disabled={pending || !!target || needsReload}
                            onClick={(event) => {
                              trigger.current = event.currentTarget;
                              setTarget(user);
                              setDeleteError(undefined);
                              setMessage('');
                            }}
                          >
                            Удалить
                          </Button>
                        )}
                      </div>
                    </div>
                    {target?.id === user.id && (
                      <Alert className="border-destructive/30 bg-destructive/5">
                        <div
                          role="group"
                          aria-labelledby={`delete-user-${user.id}`}
                          onKeyDown={(event) => {
                            if (event.key === 'Escape' && !pending) close();
                          }}
                        >
                          <p
                            id={`delete-user-${user.id}`}
                            className="font-medium wrap-anywhere"
                          >
                            Удалить пользователя {user.email}?
                          </p>
                          <p className="mt-2 text-sm text-muted-foreground">
                            Аккаунт, все его мониторы и сессии будут удалены
                            безвозвратно.
                          </p>
                          {deleteError !== undefined && (
                            <p
                              role="alert"
                              className="mt-3 text-sm text-destructive"
                            >
                              {adminErrorMessage(deleteError, true)}
                            </p>
                          )}
                          <div className="mt-4 flex flex-wrap gap-3">
                            <Button
                              variant="destructive"
                              disabled={pending || needsReload}
                              onClick={() => void remove()}
                            >
                              {pending ? 'Удаляем…' : 'Удалить пользователя'}
                            </Button>
                            <Button
                              ref={cancelButton}
                              variant="outline"
                              disabled={pending}
                              onClick={close}
                            >
                              Отмена
                            </Button>
                          </div>
                        </div>
                      </Alert>
                    )}
                  </Card>
                </li>
              ))}
            </ul>
          )}
          <nav
            aria-label="Страницы пользователей"
            className="mt-6 flex flex-wrap items-center justify-between gap-4"
          >
            <Button
              variant="outline"
              disabled={pending || !!target || page <= 1}
              onClick={() => reload(page - 1)}
            >
              Назад
            </Button>
            <span className="font-mono text-xs text-muted-foreground">
              Страница {page} из {Math.max(1, Math.ceil(state.data.total / 25))}
            </span>
            <Button
              variant="outline"
              disabled={pending || !!target || page * 25 >= state.data.total}
              onClick={() => reload(page + 1)}
            >
              Далее
            </Button>
          </nav>
        </>
      )}
    </section>
  );
}
