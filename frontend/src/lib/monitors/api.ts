import { apiOrigin, AuthError } from '@/lib/auth/api';
import { validateMonitor, type MonitorInput } from './validation';

export type Monitor = MonitorInput & {
  id: string;
  created_at: string;
  updated_at: string;
};

function monitor(value: unknown): Monitor {
  if (!value || typeof value !== 'object') throw new AuthError('protocol');
  const v = value as Record<string, unknown>;
  if (
    typeof v.id !== 'string' ||
    typeof v.url !== 'string' ||
    typeof v.interval_seconds !== 'number' ||
    typeof v.created_at !== 'string' ||
    !Number.isFinite(Date.parse(v.created_at)) ||
    typeof v.updated_at !== 'string' ||
    !Number.isFinite(Date.parse(v.updated_at)) ||
    !validateMonitor(v.url, String(v.interval_seconds), 'seconds').input
  )
    throw new AuthError('protocol');
  return {
    id: v.id,
    url: v.url,
    interval_seconds: v.interval_seconds,
    created_at: v.created_at,
    updated_at: v.updated_at,
  };
}

export class MonitorAPI {
  constructor(
    private origin: () => string = () =>
      apiOrigin(process.env.NEXT_PUBLIC_API_URL),
    private request: typeof fetch = (...args) => fetch(...args),
  ) {}

  private async call(
    body?: MonitorInput,
    signal?: AbortSignal,
    method: 'GET' | 'POST' | 'PUT' = body ? 'POST' : 'GET',
    path = '',
  ): Promise<unknown> {
    const base = this.origin();
    let response: Response;
    try {
      response = await this.request(`${base}/api/v1/monitors${path}`, {
        method,
        credentials: 'include',
        cache: 'no-store',
        signal,
        ...(body
          ? {
              headers: {
                'Content-Type': 'application/json',
                'X-CSRF-Protection': '1',
              },
              body: JSON.stringify(body),
            }
          : {}),
      });
    } catch {
      throw new AuthError('network');
    }
    if (!response.ok) throw new AuthError('http', response.status);
    try {
      return await response.json();
    } catch {
      throw new AuthError('protocol');
    }
  }

  async create(input: MonitorInput): Promise<Monitor> {
    return monitor(await this.call(input));
  }

  async update(id: string, input: MonitorInput): Promise<Monitor> {
    const value = monitor(
      await this.call(input, undefined, 'PUT', `/${encodeURIComponent(id)}`),
    );
    if (value.id !== id) throw new AuthError('protocol');
    return value;
  }

  async list(signal?: AbortSignal): Promise<Monitor[]> {
    const value = await this.call(undefined, signal);
    if (
      !value ||
      typeof value !== 'object' ||
      !('monitors' in value) ||
      !Array.isArray(value.monitors)
    )
      throw new AuthError('protocol');
    return value.monitors.map(monitor);
  }
}

export function monitorErrorMessage(
  error: unknown,
  operation: 'list' | 'create' | 'update' = 'list',
): string {
  if (error instanceof AuthError) {
    if (error.status === 409)
      return 'Этот сайт уже добавлен. Проверьте список сайтов.';
    if (error.status === 404)
      return 'Сайт больше не найден. Обновите список сайтов.';
    if (error.status === 400) return 'Проверьте URL и интервал опроса.';
    if (error.status === 401) return 'Сессия истекла. Повторите вход.';
    if (error.kind === 'config') return 'Не настроен адрес сервера.';
    if (error.kind === 'network' || error.kind === 'protocol')
      return operation === 'create'
        ? 'Не удалось подтвердить создание. Обновите список сайтов перед повторной попыткой.'
        : operation === 'update'
          ? 'Не удалось подтвердить сохранение. Обновите список сайтов перед повторной попыткой.'
          : 'Не удалось загрузить сайты. Проверьте подключение и повторите попытку.';
  }
  return 'Не удалось выполнить запрос. Попробуйте ещё раз.';
}
