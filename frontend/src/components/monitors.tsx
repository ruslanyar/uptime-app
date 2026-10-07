'use client';
import { useEffect, useRef, useState } from 'react';
import {
  Activity,
  Clock3,
  Globe,
  Pencil,
  Plus,
  RefreshCw,
  Trash2,
} from 'lucide-react';
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
  const [editor, setEditor] = useState<
    | { mode: 'create' }
    | { mode: 'update'; item: Monitor }
    | { mode: 'delete'; item: Monitor }
  >();
  const [message, setMessage] = useState('');
  const [pending, setPending] = useState(false);
  const trigger = useRef<HTMLButtonElement | null>(null);
  const addButton = useRef<HTMLButtonElement | null>(null);
  const cancelDelete = useRef<HTMLButtonElement | null>(null);
  const deleting = useRef(false);
  const [deleteError, setDeleteError] = useState<unknown>();

  useEffect(() => {
    if (pending || state.loading) return;
    if (editor?.mode === 'delete') cancelDelete.current?.focus();
    else if (!editor && trigger.current) {
      if (trigger.current.isConnected) trigger.current.focus();
      else addButton.current?.focus();
    }
  }, [editor, pending, state.loading]);

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
        if (active) {
          setState({ items, loading: false });
          setEditor((current) =>
            current?.mode === 'delete' &&
            !items.some((item) => item.id === current.item.id)
              ? undefined
              : current,
          );
        }
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

  async function save(input: MonitorInput) {
    if (!editor || editor.mode === 'delete') return;
    setPending(true);
    onRequestState?.('pending');
    let failed = false;
    try {
      await session.check();
      const current = session.snapshot();
      if (current.status !== 'authenticated' || current.user?.id !== userID)
        throw current.error ?? new AuthError('http', 401);
      const item =
        editor.mode === 'update'
          ? await api.update(editor.item.id, input)
          : await api.create(input);
      setState((previous) => ({
        items:
          editor.mode === 'update'
            ? previous.items.map((existing) =>
                existing.id === item.id ? item : existing,
              )
            : [
                item,
                ...previous.items.filter((existing) => existing.id !== item.id),
              ],
        loading: false,
      }));
      setEditor(undefined);
      setMessage(
        editor.mode === 'update' ? 'Изменения сохранены.' : 'Сайт добавлен.',
      );
    } catch (error) {
      failed = true;
      throw error;
    } finally {
      setPending(false);
      onRequestState?.(
        failed && session.snapshot().status === 'error' ? 'failed' : 'idle',
      );
    }
  }

  async function remove() {
    if (editor?.mode !== 'delete' || deleting.current || state.loading) return;
    deleting.current = true;
    setPending(true);
    setDeleteError(undefined);
    onRequestState?.('pending');
    let failed = false;
    try {
      await session.check();
      const current = session.snapshot();
      if (current.status !== 'authenticated' || current.user?.id !== userID)
        throw current.error ?? new AuthError('http', 401);
      await api.delete(editor.item.id);
      setState((previous) => ({
        items: previous.items.filter((item) => item.id !== editor.item.id),
        loading: false,
      }));
      setEditor(undefined);
      setMessage('Сайт удалён.');
    } catch (error) {
      failed = true;
      setDeleteError(error);
    } finally {
      deleting.current = false;
      setPending(false);
      onRequestState?.(
        failed && session.snapshot().status === 'error' ? 'failed' : 'idle',
      );
    }
  }

  function cancel() {
    setEditor(undefined);
    setDeleteError(undefined);
    onRequestState?.('idle');
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
            disabled={state.loading || pending}
            onClick={reload}
          >
            <RefreshCw aria-hidden="true" className="size-4" />
          </Button>
          <Button
            ref={addButton}
            disabled={
              !!editor || pending || state.loading || state.error !== undefined
            }
            onClick={(event) => {
              trigger.current = event.currentTarget;
              setEditor({ mode: 'create' });
              setMessage('');
            }}
          >
            <Plus aria-hidden="true" className="size-4" />
            Добавить сайт
          </Button>
        </div>
      </div>
      {message && (
        <p role="status" className="mt-4 text-sm text-primary">
          {message}
        </p>
      )}
      {editor && editor.mode !== 'delete' && (
        <MonitorForm
          key={editor.mode === 'update' ? editor.item.id : 'create'}
          initialValues={editor.mode === 'update' ? editor.item : undefined}
          onSubmit={save}
          onReload={reload}
          onCancel={cancel}
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
              <Card className="gap-4 px-6">
                <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
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
                  <div className="flex flex-wrap items-center gap-4 sm:justify-end lg:shrink-0">
                    <p className="flex items-center gap-2 font-mono text-xs text-muted-foreground">
                      <Clock3 aria-hidden="true" className="size-4" />
                      Интервал: {formatInterval(item.interval_seconds)}
                    </p>
                    <Button
                      variant="outline"
                      size="sm"
                      aria-label={`Редактировать ${item.url}`}
                      disabled={
                        !!editor ||
                        pending ||
                        state.loading ||
                        state.error !== undefined
                      }
                      onClick={(event) => {
                        trigger.current = event.currentTarget;
                        setEditor({ mode: 'update', item });
                        setMessage('');
                      }}
                    >
                      <Pencil aria-hidden="true" className="size-4" />
                      Редактировать
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      aria-label={`Удалить ${item.url}`}
                      disabled={
                        !!editor ||
                        pending ||
                        state.loading ||
                        state.error !== undefined
                      }
                      onClick={(event) => {
                        trigger.current = event.currentTarget;
                        setDeleteError(undefined);
                        setEditor({ mode: 'delete', item });
                        setMessage('');
                      }}
                    >
                      <Trash2 aria-hidden="true" className="size-4" />
                      Удалить
                    </Button>
                  </div>
                </div>
                {editor?.mode === 'delete' && editor.item.id === item.id && (
                  <Alert className="mt-3 border-destructive/30 bg-destructive/5">
                    <div
                      role="group"
                      aria-labelledby={`delete-monitor-${item.id}`}
                    >
                      <p
                        id={`delete-monitor-${item.id}`}
                        className="font-medium wrap-anywhere"
                      >
                        Удалить сайт {editor.item.url}?
                      </p>
                      <p className="mt-2 text-sm text-muted-foreground">
                        Сайт будет удалён из списка. Это действие нельзя
                        отменить.
                      </p>
                      {deleteError !== undefined && (
                        <div
                          role="alert"
                          className="mt-3 text-sm text-destructive"
                        >
                          <p>{monitorErrorMessage(deleteError, 'delete')}</p>
                          {deleteError instanceof AuthError &&
                            (deleteError.status === 404 ||
                              deleteError.kind === 'network' ||
                              deleteError.kind === 'protocol') && (
                              <Button
                                variant="outline"
                                className="mt-3"
                                disabled={pending || state.loading}
                                onClick={reload}
                              >
                                Обновить список сайтов
                              </Button>
                            )}
                        </div>
                      )}
                      <div className="mt-4 flex flex-wrap gap-3">
                        <Button
                          variant="destructive"
                          disabled={pending || state.loading}
                          onClick={() => void remove()}
                        >
                          {pending ? 'Удаляем…' : 'Удалить сайт'}
                        </Button>
                        <Button
                          ref={cancelDelete}
                          variant="outline"
                          disabled={pending || state.loading}
                          onClick={cancel}
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
    </section>
  );
}
