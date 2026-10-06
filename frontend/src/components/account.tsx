'use client';
import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from './auth-provider';
import { SessionStatus } from './session-status';
import { errorMessage } from '@/lib/auth/api';

export function Account({ redirectOnly = false }: { redirectOnly?: boolean }) {
  const auth = useAuth();
  const router = useRouter();
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState('');
  useEffect(() => {
    if (auth.status === 'anonymous') router.replace('/login');
    else if (redirectOnly && auth.status === 'authenticated')
      router.replace('/account');
  }, [auth.status, redirectOnly, router]);
  if (redirectOnly || auth.status !== 'authenticated' || !auth.user)
    return <SessionStatus />;
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
    <>
      <p className="eyebrow">ЛИЧНЫЙ АККАУНТ</p>
      <h1>Здравствуйте, {auth.user.name}</h1>
      <p className="muted">Вы вошли в свой аккаунт.</p>
      <dl className="profile">
        <dt>Имя</dt>
        <dd>{auth.user.name}</dd>
        <dt>Email</dt>
        <dd>{auth.user.email}</dd>
      </dl>
      {message && (
        <p role="alert" className="notice">
          {message}
        </p>
      )}
      <button className="secondary" disabled={pending} onClick={logout}>
        {pending ? 'Выходим…' : 'Выйти'}
      </button>
    </>
  );
}
