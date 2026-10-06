'use client';
import { useState } from 'react';
import { redirect } from 'next/navigation';
import { useAuth } from './auth-provider';
import { SessionStatus } from './session-status';
import { Eyebrow, PageHeading } from '@/components/page-shell';
import { Button } from '@/components/ui/button';
import { Alert } from '@/components/ui/alert';
import { errorMessage, type User } from '@/lib/auth/api';

export function Account({ redirectOnly = false }: { redirectOnly?: boolean }) {
  const auth = useAuth();
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState('');
  // Session clears its user while a mutation runs; retain this screen's user
  // only for the explicitly requested logout and its error state.
  const [logoutUser, setLogoutUser] = useState<User>();
  const user = auth.user ?? (pending || message ? logoutUser : undefined);
  if (auth.status === 'anonymous') redirect('/login');
  if (redirectOnly && auth.status === 'authenticated') redirect('/account');
  if (
    redirectOnly ||
    !user ||
    (auth.status !== 'authenticated' && !pending && !message)
  )
    return <SessionStatus />;
  async function logout() {
    if (pending) return;
    setLogoutUser(user);
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
    <>
      <Eyebrow>Личный аккаунт</Eyebrow>
      <PageHeading>Здравствуйте, {user.name}</PageHeading>
      <p className="text-sm text-muted-foreground">Вы вошли в свой аккаунт.</p>
      <dl aria-label="Данные аккаунта" className="my-7 space-y-2 border-y py-6">
        <dt className="font-mono text-xs text-muted-foreground">Имя</dt>
        <dd className="pb-3 text-sm wrap-anywhere last:pb-0">{user.name}</dd>
        <dt className="font-mono text-xs text-muted-foreground">Email</dt>
        <dd className="pb-3 font-mono text-sm wrap-anywhere last:pb-0">
          {user.email}
        </dd>
      </dl>
      {message && (
        <Alert className="mb-4 border-destructive/30 bg-destructive/5 text-destructive">
          <p>{message}</p>
          <Button
            variant="outline"
            className="mt-3"
            onClick={() => {
              setMessage('');
              void auth.session.check(true);
            }}
          >
            Повторить проверку
          </Button>
        </Alert>
      )}
      <Button variant="outline" disabled={pending} onClick={logout}>
        {pending ? 'Выходим…' : 'Выйти'}
      </Button>
    </>
  );
}
