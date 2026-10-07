'use client';
import { useState } from 'react';
import Link from 'next/link';
import { redirect } from 'next/navigation';
import { useAuth } from '@/components/auth-provider';
import { Brand, Eyebrow, PageShell, Surface } from '@/components/page-shell';
import { ChevronDown, LogOut, UserRound } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Monitors } from '@/components/monitors';
import { Alert } from '@/components/ui/alert';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { SessionStatus } from '@/components/session-status';
import { Avatar } from '@/components/avatar';
import { errorMessage, type User } from '@/lib/auth/api';

export function Dashboard() {
  const auth = useAuth();
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState('');
  // Retain this screen's user for logout and a failed session check during
  // monitor changes, so request errors do not discard the form.
  const [logoutUser, setLogoutUser] = useState<User>();
  const [monitorRequest, setMonitorRequest] = useState<{
    user: User;
    status: 'pending' | 'failed';
  }>();
  const retainMonitor = auth.status === 'error' && monitorRequest;

  const user =
    auth.user ??
    (pending || message
      ? logoutUser
      : retainMonitor
        ? retainMonitor.user
        : undefined);

  if (auth.status === 'anonymous') redirect('/login');
  if (
    !user ||
    (auth.status !== 'authenticated' && !pending && !message && !retainMonitor)
  )
    return (
      <PageShell>
        <SessionStatus />
      </PageShell>
    );

  async function logout() {
    if (pending || monitorRequest?.status === 'pending') return;
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
    <Surface>
      <header className="flex items-center justify-between gap-4 border-b border-border/60 px-6 py-5 sm:px-12">
        <Brand />
        <DropdownMenu modal={false}>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              aria-label="Меню пользователя"
              className="h-auto min-w-0 gap-3 px-2 py-2"
            >
              <Avatar url={user.avatar_url} name={user.name} size="sm" />
              <span className="max-w-25 truncate sm:max-w-45">{user.name}</span>
              <ChevronDown
                aria-hidden="true"
                className="size-4 text-muted-foreground"
              />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            align="end"
            sideOffset={12}
            className="w-72 max-w-[calc(100vw-3rem)] p-2"
          >
            <DropdownMenuLabel className="space-y-1 px-2 py-3">
              <p className="wrap-anywhere">{user.name}</p>
              <p className="font-mono text-xs font-normal text-muted-foreground wrap-anywhere">
                {user.email}
              </p>
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem asChild>
              <Link href="/account">
                <UserRound aria-hidden="true" />
                Мой профиль
              </Link>
            </DropdownMenuItem>
            <DropdownMenuItem
              disabled={pending || monitorRequest?.status === 'pending'}
              onSelect={(event) => {
                event.preventDefault();
                if (!pending && monitorRequest?.status !== 'pending')
                  void logout();
              }}
            >
              <LogOut aria-hidden="true" />
              {pending ? 'Выходим…' : 'Выйти'}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </header>
      <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-10 sm:px-12 sm:py-16">
        <section aria-labelledby="dashboard-title">
          <Eyebrow>Обзор</Eyebrow>
          <h1
            id="dashboard-title"
            className="max-w-3xl text-3xl font-medium tracking-tight wrap-anywhere sm:text-5xl"
          >
            Здравствуйте, {user.name}
          </h1>
          <p className="mt-4 text-sm leading-relaxed text-muted-foreground">
            Добро пожаловать в ваш дашборд Uptime.
          </p>
          {message && (
            <Alert className="mt-6 border-destructive/30 bg-destructive/5 text-destructive">
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
          {(auth.status === 'authenticated' || retainMonitor) && (
            <Monitors
              key={user.id}
              userID={user.id}
              onRequestState={(status) => {
                if (status === 'idle') setMonitorRequest(undefined);
                else setMonitorRequest({ user, status });
              }}
            />
          )}
        </section>
      </main>
    </Surface>
  );
}
