'use client';
import { useState } from 'react';
import Link from 'next/link';
import { redirect } from 'next/navigation';
import { useAuth } from '@/components/auth-provider';
import { Brand, Eyebrow, PageShell, Surface } from '@/components/page-shell';
import {
  Activity,
  ArrowUpRight,
  ChevronDown,
  LogOut,
  UserRound,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
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
import { errorMessage, type User } from '@/lib/auth/api';

export function Dashboard() {
  const auth = useAuth();
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState('');
  // Session clears its user while a mutation runs; retain this screen's user
  // only for the explicitly requested logout and its error state.
  const [logoutUser, setLogoutUser] = useState<User>();
  const user = auth.user ?? (pending || message ? logoutUser : undefined);

  if (auth.status === 'anonymous') redirect('/login');
  if (!user || (auth.status !== 'authenticated' && !pending && !message))
    return (
      <PageShell>
        <SessionStatus />
      </PageShell>
    );

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
              <span
                aria-hidden="true"
                className="grid size-9 shrink-0 place-items-center rounded-full border border-primary/25 bg-primary/10 text-primary"
              >
                {Array.from(user.name.trim())[0]?.toLocaleUpperCase('ru')}
              </span>
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
              <p className="text-xs font-normal text-muted-foreground wrap-anywhere">
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
              disabled={pending}
              onSelect={(event) => {
                event.preventDefault();
                if (!pending) void logout();
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
          <Eyebrow>ОБЗОР</Eyebrow>
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
          <Card className="relative mt-10 items-center overflow-hidden border-border/80 px-6 py-16 text-center sm:mt-14 sm:py-24">
            <div
              aria-hidden="true"
              className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_50%_0%,#d7f54212,transparent_65%)]"
            />
            <div
              aria-hidden="true"
              className="relative grid size-20 place-items-center rounded-2xl border border-primary/30 bg-linear-to-br from-primary/20 to-emerald-500/5 text-primary shadow-[0_0_60px_#d7f54210]"
            >
              <Activity className="size-9" />
            </div>
            <div className="relative mt-2">
              <h2 className="text-2xl font-medium tracking-tight sm:text-3xl">
                Мониторинг скоро появится
              </h2>
              <p className="mx-auto mt-4 max-w-md text-sm leading-relaxed text-muted-foreground">
                Здесь будут ваши мониторы и информация о доступности сервисов.
              </p>
            </div>
            <span className="mt-3 inline-flex items-center gap-2 rounded-full border border-border px-3 py-1.5 font-mono text-[10px] tracking-widest text-primary">
              <ArrowUpRight className="size-3" aria-hidden="true" />В РАЗРАБОТКЕ
            </span>
          </Card>
        </section>
      </main>
    </Surface>
  );
}
