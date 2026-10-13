import { apiOrigin, AuthError } from '@/lib/auth/api';
import { validateMonitor, type MonitorInput } from './validation';
import { rasterizeIcon } from './rasterize-icon';

export type CheckErrorCode =
  | 'http_status'
  | 'dns'
  | 'connection'
  | 'tls'
  | 'timeout'
  | 'redirect'
  | 'blocked_address'
  | 'network';
export type Monitoring = {
  status: 'unknown' | 'up' | 'down' | 'blocked';
  last_check: {
    checked_at: string;
    http_status: number | null;
    response_time_ms: number | null;
    error_code: CheckErrorCode | null;
  } | null;
  next_check_at: string;
  is_stale: boolean;
};

function monitoring(value: unknown): Monitoring {
  if (!value || typeof value !== 'object') throw new AuthError('protocol');
  const v = value as Record<string, unknown>;
  if (
    typeof v.status !== 'string' ||
    !['unknown', 'up', 'down', 'blocked'].includes(v.status) ||
    typeof v.next_check_at !== 'string' ||
    !Number.isFinite(Date.parse(v.next_check_at)) ||
    typeof v.is_stale !== 'boolean'
  )
    throw new AuthError('protocol');
  if (v.status === 'unknown') {
    if (v.last_check !== null || v.is_stale) throw new AuthError('protocol');
  } else {
    if (!v.last_check || typeof v.last_check !== 'object')
      throw new AuthError('protocol');
    const r = v.last_check as Record<string, unknown>;
    if (
      typeof r.checked_at !== 'string' ||
      !Number.isFinite(Date.parse(r.checked_at))
    )
      throw new AuthError('protocol');
    const hasHTTP =
      typeof r.http_status === 'number' &&
      Number.isInteger(r.http_status) &&
      r.http_status >= 100 &&
      r.http_status <= 599 &&
      typeof r.response_time_ms === 'number' &&
      Number.isSafeInteger(r.response_time_ms) &&
      r.response_time_ms >= 0;
    const noHTTP = r.http_status === null && r.response_time_ms === null;
    const success =
      hasHTTP &&
      (r.http_status as number) >= 200 &&
      (r.http_status as number) <= 299;
    const valid =
      v.status === 'up'
        ? success && r.error_code === null
        : v.status === 'blocked'
          ? noHTTP && r.error_code === 'blocked_address'
          : r.error_code === 'http_status'
            ? hasHTTP && !success
            : noHTTP &&
              [
                'dns',
                'connection',
                'tls',
                'timeout',
                'redirect',
                'network',
              ].includes(typeof r.error_code === 'string' ? r.error_code : '');
    if (!valid) throw new AuthError('protocol');
  }
  return value as Monitoring;
}

export type Monitor = MonitorInput & {
  id: string;
  created_at: string;
  updated_at: string;
  monitoring: Monitoring;
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
    monitoring: monitoring(v.monitoring),
  };
}

/** Requests reject with AuthError; network/protocol failure can follow a committed write. */
export class MonitorAPI {
  constructor(
    private origin: () => string = () =>
      apiOrigin(process.env.NEXT_PUBLIC_API_URL),
    private request: typeof fetch = (...args) => fetch(...args),
  ) {}

  private async call(
    body?: MonitorInput,
    signal?: AbortSignal,
    method: 'GET' | 'POST' | 'PUT' | 'DELETE' = body ? 'POST' : 'GET',
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
        ...(method === 'DELETE'
          ? { headers: { 'X-CSRF-Protection': '1' } }
          : {}),
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
    if (method === 'DELETE') {
      if (response.status !== 204) throw new AuthError('protocol');
      return;
    }
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

  async delete(id: string): Promise<void> {
    await this.call(
      undefined,
      undefined,
      'DELETE',
      `/${encodeURIComponent(id)}`,
    );
  }

  /** Fetch cancellation becomes AuthError('network'); body-read failures become 'protocol'. */
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

  /** Icon discovery is optional: unavailable images and request failures return null. */
  async favicon(url: string, signal?: AbortSignal): Promise<Blob | null> {
    try {
      const response = await this.request(
        `${this.origin()}/api/v1/monitors/favicon?${new URLSearchParams({ url })}`,
        { credentials: 'include', cache: 'no-store', signal },
      );
      if (response.status !== 200) return null;
      const blob = await response.blob();
      if (!blob.size || !blob.type.startsWith('image/')) return null;
      return blob.type === 'image/svg+xml'
        ? await rasterizeIcon(blob, signal)
        : blob;
    } catch {
      return null;
    }
  }
}

export function monitorErrorMessage(
  error: unknown,
  operation: 'list' | 'create' | 'update' | 'delete' = 'list',
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
          : operation === 'delete'
            ? 'Не удалось подтвердить удаление. Обновите список сайтов перед повторной попыткой.'
            : 'Не удалось загрузить сайты. Проверьте подключение и повторите попытку.';
  }
  return 'Не удалось выполнить запрос. Попробуйте ещё раз.';
}
