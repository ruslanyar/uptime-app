'use client';
import { useEffect, useState } from 'react';
import { Activity, Clock3, Globe, Plus, RefreshCw } from 'lucide-react';
import { useAuth } from '@/components/auth-provider';
import { MonitorForm } from '@/components/monitor-form';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Alert } from '@/components/ui/alert';
import { AuthError } from '@/lib/auth/api';
import {
  MonitorAPI,
  monitorErrorMessage,
  type Monitor,
} from '@/lib/monitors/api';
import { formatInterval, type MonitorInput } from '@/lib/monitors/validation';

export function Monitors({
  userID,
  onRequestState,
}: {
  userID: string;
  onRequestState?: (status: 'pending' | 'failed' | 'idle') => void;
}) {
  const { session } = useAuth();
  const [api] = useState(() => new MonitorAPI());
  const [state, setState] = useState<{
    items: Monitor[];
    loading: boolean;
    error?: unknown;
  }>({ items: [], loading: true });
  const [revision, setRevision] = useState(0);
  const [editing, setEditing] = useState(false);
  const [created, setCreated] = useState(false);
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    async function load() {
      try {
        let items: Monitor[];
        try {
          items = await api.list(controller.signal);
        } catch (error) {
          if (!(error instanceof AuthError) || error.status !== 401)
            throw error;
          await session.check();
          const current = session.snapshot();
          if (
            !active ||
            current.status !== 'authenticated' ||
            current.user?.id !== userID
          )
            throw error;
          items = await api.list(controller.signal);
        }
        if (active) setState({ items, loading: false });
      } catch (error) {
        if (active)
          setState((previous) => ({ ...previous, loading: false, error }));
      }
    }
    void load();
    return () => {
      active = false;
      controller.abort();
    };
  }, [api, session, userID, revision]);

  function reload() {
    setState((previous) => ({ ...previous, loading: true, error: undefined }));
    setRevision((previous) => previous + 1);
  }

  async function create(input: MonitorInput) {
    setCreating(true);
    onRequestState?.('pending');
    let failed = false;
    try {
      await session.check();
      const current = session.snapshot();
      if (current.status !== 'authenticated' || current.user?.id !== userID)
        throw current.error ?? new AuthError('http', 401);
      const item = await api.create(input);
      setState((previous) => ({
        items: [
          item,
          ...previous.items.filter((existing) => existing.id !== item.id),
        ],
        loading: false,
      }));
      setEditing(false);
      setCreated(true);
    } catch (error) {
      failed = true;
      throw error;
    } finally {
      setCreating(false);
      onRequestState?.(
        failed && session.snapshot().status === 'error' ? 'failed' : 'idle',
      );
    }
  }

  return (
    <section aria-labelledby="monitors-title" className="mt-10 sm:mt-14">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h2
            id="monitors-title"
            className="text-2xl font-medium tracking-tight"
          >
            Ваши сайты
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Сайты и интервалы опроса. Проверки доступности скоро появятся.
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            aria-label="Обновить список сайтов"
            disabled={state.loading || creating}
            onClick={reload}
          >
            <RefreshCw aria-hidden="true" className="size-4" />
          </Button>
          <Button
            disabled={editing || state.loading || state.error !== undefined}
            onClick={() => {
              setEditing(true);
              setCreated(false);
            }}
          >
            <Plus aria-hidden="true" className="size-4" />
            Добавить сайт
          </Button>
        </div>
      </div>
      {created && (
        <p role="status" className="mt-4 text-sm text-primary">
          Сайт добавлен.
        </p>
      )}
      {editing && (
        <MonitorForm
          onCreate={create}
          onCancel={() => {
            setEditing(false);
            onRequestState?.('idle');
          }}
          disabled={state.loading}
        />
      )}
      {state.loading && (
        <p role="status" className="mt-8 text-sm text-muted-foreground">
          Загружаем сайты…
        </p>
      )}
      {state.error !== undefined && (
        <Alert className="mt-6 border-destructive/30 bg-destructive/5 text-destructive">
          <p>{monitorErrorMessage(state.error)}</p>
          <Button variant="outline" className="mt-3" onClick={reload}>
            Повторить загрузку
          </Button>
        </Alert>
      )}
      {!state.loading &&
        state.error === undefined &&
        state.items.length === 0 && (
          <Card className="mt-6 items-center px-6 py-14 text-center">
            <div
              aria-hidden="true"
              className="grid size-16 place-items-center rounded-2xl border border-primary/30 bg-primary/10 text-primary"
            >
              <Activity className="size-7" />
            </div>
            <div>
              <h3 className="text-xl font-medium">Пока нет сайтов</h3>
              <p className="mt-3 text-sm text-muted-foreground">
                Добавьте первый сайт и настройте интервал опроса.
              </p>
            </div>
          </Card>
        )}
      {state.items.length > 0 && (
        <ul aria-label="Сайты для мониторинга" className="mt-6 grid gap-4">
          {state.items.map((item) => (
            <li key={item.id}>
              <Card className="gap-4 px-6 sm:flex-row sm:items-center sm:justify-between">
                <div className="flex min-w-0 items-start gap-4">
                  <div
                    aria-hidden="true"
                    className="grid size-10 shrink-0 place-items-center rounded-lg border border-border bg-background text-primary"
                  >
                    <Globe className="size-5" />
                  </div>
                  <div className="min-w-0">
                    <p className="text-sm font-medium wrap-anywhere">
                      {item.url}
                    </p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      Точка мониторинга создана
                    </p>
                  </div>
                </div>
                <p className="flex shrink-0 items-center gap-2 font-mono text-xs text-muted-foreground">
                  <Clock3 aria-hidden="true" className="size-4" />
                  Интервал: {formatInterval(item.interval_seconds)}
                </p>
              </Card>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
