'use client';
import { useRef, useState } from 'react';
import { redirect } from 'next/navigation';
import { useAuth } from './auth-provider';
import { SessionStatus } from './session-status';
import { Avatar } from './avatar';
import { AvatarField } from './avatar-field';
import { Eyebrow, PageHeading } from '@/components/page-shell';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Button } from '@/components/ui/button';
import { Alert } from '@/components/ui/alert';
import { errorMessage, type User } from '@/lib/auth/api';

export function Account({ redirectOnly = false }: { redirectOnly?: boolean }) {
  const auth = useAuth();
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState('');
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState('');
  const [avatar, setAvatar] = useState<File>();
  const [removeAvatar, setRemoveAvatar] = useState(false);
  const [savedAvatar, setSavedAvatar] = useState(false);
  const [nameError, setNameError] = useState('');
  const [saved, setSaved] = useState(false);
  const [saving, setSaving] = useState(false);
  const submitting = useRef(false);
  // Session clears its user while a mutation runs; retain this screen's user
  // while saving a profile or logging out, and when showing a request error.
  const [retainedUser, setRetainedUser] = useState<User>();
  const user =
    auth.user ?? (pending || saving || message ? retainedUser : undefined);
  if (auth.status === 'anonymous') redirect('/login');
  if (redirectOnly && auth.status === 'authenticated') redirect('/account');
  if (
    redirectOnly ||
    !user ||
    (auth.status !== 'authenticated' && !pending && !saving && !message)
  )
    return <SessionStatus />;
  async function save(event: React.SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting.current) return;
    const value = name.trim();
    if (Array.from(value).length < 2 || Array.from(value).length > 50) {
      setNameError('Введите имя длиной от 2 до 50 символов.');
      return;
    }
    submitting.current = true;
    setRetainedUser(user);
    setSaving(true);
    setNameError('');
    setMessage('');
    setSaved(false);
    try {
      await auth.session.mutate('profile', {
        name: value,
        ...(avatar ? { avatar } : {}),
        ...(removeAvatar ? { remove_avatar: true } : {}),
      });
      setSavedAvatar(!!avatar || removeAvatar);
      setAvatar(undefined);
      setRemoveAvatar(false);
      setEditing(false);
      setSaved(true);
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      submitting.current = false;
      setSaving(false);
    }
  }
  async function logout() {
    if (pending || saving) return;
    setRetainedUser(user);
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
      <div className="mt-6">
        <Avatar url={user.avatar_url} />
      </div>
      <dl aria-label="Данные аккаунта" className="my-7 space-y-2 border-y py-6">
        <dt className="font-mono text-xs text-muted-foreground">Имя</dt>
        <dd className="pb-3 text-sm wrap-anywhere last:pb-0">{user.name}</dd>
        <dt className="font-mono text-xs text-muted-foreground">Email</dt>
        <dd className="pb-3 font-mono text-sm wrap-anywhere last:pb-0">
          {user.email}
        </dd>
      </dl>
      {editing ? (
        <form onSubmit={save} noValidate className="mb-7 space-y-4">
          <AvatarField
            url={user.avatar_url}
            file={avatar}
            removed={removeAvatar}
            disabled={saving || pending}
            onChange={(file) => {
              setAvatar(file);
              if (file) setRemoveAvatar(false);
            }}
            onRemove={() => {
              setAvatar(undefined);
              setRemoveAvatar(true);
            }}
          />
          <div className="space-y-2">
            <Label htmlFor="profile-name">Имя</Label>
            <Input
              id="profile-name"
              name="name"
              autoComplete="name"
              value={name}
              onChange={(event) => {
                setName(event.target.value);
                setNameError('');
              }}
              disabled={saving || pending}
              aria-invalid={!!nameError}
              aria-describedby={
                nameError ? 'profile-name-error' : 'profile-name-hint'
              }
            />
            <p id="profile-name-hint" className="text-xs text-muted-foreground">
              От 2 до 50 символов.
            </p>
            {nameError && (
              <p id="profile-name-error" className="text-xs text-destructive">
                {nameError}
              </p>
            )}
          </div>
          <div className="flex flex-wrap gap-3">
            <Button type="submit" disabled={saving || pending}>
              {saving ? 'Сохраняем…' : 'Сохранить'}
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={saving || pending}
              onClick={() => {
                setEditing(false);
                setAvatar(undefined);
                setRemoveAvatar(false);
                setNameError('');
              }}
            >
              Отмена
            </Button>
          </div>
        </form>
      ) : (
        <Button
          className="mb-4"
          disabled={pending || saving}
          onClick={() => {
            setName(user.name);
            setAvatar(undefined);
            setRemoveAvatar(false);
            setEditing(true);
            setNameError('');
            setSaved(false);
          }}
        >
          Редактировать профиль
        </Button>
      )}
      {saved && (
        <p role="status" className="mb-4 text-sm text-primary">
          {savedAvatar ? 'Профиль сохранён.' : 'Имя сохранено.'}
        </p>
      )}
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
      <Button variant="outline" disabled={pending || saving} onClick={logout}>
        {pending ? 'Выходим…' : 'Выйти'}
      </Button>
    </>
  );
}
