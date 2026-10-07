import { apiOrigin, AuthError } from '@/lib/auth/api';
import { validateMonitor, type MonitorInput } from './validation';

export type Monitor = MonitorInput & { id: string; created_at: string };

function monitor(value: unknown): Monitor {
  if (!value || typeof value !== 'object') throw new AuthError('protocol');
  const v = value as Record<string, unknown>;
  if (
    typeof v.id !== 'string' ||
    typeof v.url !== 'string' ||
    typeof v.interval_seconds !== 'number' ||
    typeof v.created_at !== 'string' ||
    !Number.isFinite(Date.parse(v.created_at)) ||
    !validateMonitor(v.url, String(v.interval_seconds), 'seconds').input
  )
    throw new AuthError('protocol');
  return {
    id: v.id,
    url: v.url,
    interval_seconds: v.interval_seconds,
    created_at: v.created_at,
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
  ): Promise<unknown> {
    const base = this.origin();
    let response: Response;
    try {
      response = await this.request(`${base}/api/v1/monitors`, {
        method: body ? 'POST' : 'GET',
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

export function monitorErrorMessage(error: unknown, creating = false): string {
  if (error instanceof AuthError) {
    if (error.status === 409)
      return 'Этот сайт уже добавлен. Проверьте список сайтов.';
    if (error.status === 400) return 'Проверьте URL и интервал опроса.';
    if (error.status === 401) return 'Сессия истекла. Повторите вход.';
    if (error.kind === 'config') return 'Не настроен адрес сервера.';
    if (error.kind === 'network' || error.kind === 'protocol')
      return creating
        ? 'Не удалось подтвердить создание. Обновите список сайтов перед повторной попыткой.'
        : 'Не удалось загрузить сайты. Проверьте подключение и повторите попытку.';
  }
  return 'Не удалось выполнить запрос. Попробуйте ещё раз.';
}
