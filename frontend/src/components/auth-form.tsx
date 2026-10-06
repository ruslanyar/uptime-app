'use client';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useEffect, useRef, useState } from 'react';
import { useAuth } from './auth-provider';
import { SessionStatus } from './session-status';
import { errorMessage, AuthError } from '@/lib/auth/api';
import { validate, type FieldErrors } from '@/lib/auth/validation';

export function AuthForm({ register = false }: { register?: boolean }) {
  const auth = useAuth();
  const router = useRouter();
  const [errors, setErrors] = useState<FieldErrors>({});
  const [message, setMessage] = useState('');
  const [pending, setPending] = useState(false);
  const submitting = useRef(false);
  const [show, setShow] = useState(false);
  useEffect(() => {
    if (auth.status === 'authenticated') router.replace('/account');
  }, [auth.status, router]);
  if (
    (auth.status === 'loading' && !pending) ||
    auth.status === 'authenticated'
  )
    return <SessionStatus />;
  const formError =
    auth.error instanceof AuthError &&
    auth.error.kind === 'http' &&
    [400, 401, 409].includes(auth.error.status);
  if (auth.status === 'error' && !message && !pending && !formError)
    return <SessionStatus />;
  if (
    auth.error instanceof AuthError &&
    ['config', 'unsupported'].includes(auth.error.kind)
  )
    return <SessionStatus />;
  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting.current) return;
    const data = new FormData(event.currentTarget);
    const result = validate(
      {
        email: String(data.get('email') ?? ''),
        password: String(data.get('password') ?? ''),
        ...(register ? { name: String(data.get('name') ?? '') } : {}),
      },
      register,
    );
    setErrors(result.errors);
    setMessage('');
    if (Object.keys(result.errors).length) return;
    submitting.current = true;
    setPending(true);
    try {
      await auth.session.mutate(register ? 'register' : 'login', result.values);
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      submitting.current = false;
      setPending(false);
    }
  }
  return (
    <>
      <p className="eyebrow">ВАШ АККАУНТ</p>
      <h1>{register ? 'Создать аккаунт' : 'Войти'}</h1>
      <p className="muted">
        {register
          ? 'Начните с личного аккаунта в Uptime.'
          : 'Рады видеть вас снова в Uptime.'}
      </p>
      <form onSubmit={submit} noValidate>
        {register && (
          <div className="field">
            <label htmlFor="name">Имя</label>
            <input
              id="name"
              name="name"
              autoComplete="name"
              disabled={pending}
              aria-invalid={!!errors.name}
              aria-describedby={errors.name ? 'name-error' : undefined}
            />
            {errors.name && (
              <p id="name-error" className="field-error">
                {errors.name}
              </p>
            )}
          </div>
        )}
        <div className="field">
          <label htmlFor="email">Email</label>
          <input
            id="email"
            name="email"
            type="email"
            autoComplete="email"
            disabled={pending}
            aria-invalid={!!errors.email}
            aria-describedby={errors.email ? 'email-error' : undefined}
          />
          {errors.email && (
            <p id="email-error" className="field-error">
              {errors.email}
            </p>
          )}
        </div>
        <div className="field">
          <label htmlFor="password">Пароль</label>
          <div className="password">
            <input
              id="password"
              name="password"
              type={show ? 'text' : 'password'}
              autoComplete={register ? 'new-password' : 'current-password'}
              disabled={pending}
              aria-invalid={!!errors.password}
              aria-describedby={
                errors.password ? 'password-error' : 'password-hint'
              }
            />
            <button
              type="button"
              onClick={() => setShow(!show)}
              aria-label={show ? 'Скрыть пароль' : 'Показать пароль'}
            >
              {show ? 'Скрыть' : 'Показать'}
            </button>
          </div>
          <p id="password-hint" className="hint">
            От 15 до 128 символов. Пробелы учитываются.
          </p>
          {errors.password && (
            <p id="password-error" className="field-error">
              {errors.password}
            </p>
          )}
        </div>
        {message && (
          <p className="notice" role="alert">
            {message}
          </p>
        )}
        <button className="primary" disabled={pending} type="submit">
          {pending ? 'Подождите…' : register ? 'Создать аккаунт' : 'Войти'}
        </button>
      </form>
      <p className="footer-link">
        {register ? 'Уже есть аккаунт?' : 'Нет аккаунта?'}{' '}
        <Link href={register ? '/login' : '/register'}>
          {register ? 'Войти' : 'Зарегистрироваться'}
        </Link>
      </p>
    </>
  );
}
