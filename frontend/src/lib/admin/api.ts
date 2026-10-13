import { apiOrigin, AuthError } from '@/lib/auth/api';

export type AdminUser = {
  id: string;
  name: string;
  email: string;
  role: 'user' | 'admin';
};
export type UserList = {
  users: AdminUser[];
  total: number;
  page: number;
  page_size: number;
};

export class AdminAPI {
  constructor(
    private origin: () => string = () =>
      apiOrigin(process.env.NEXT_PUBLIC_API_URL),
    private request: typeof fetch = (...args) => fetch(...args),
  ) {}

  private async call(
    path: string,
    method: 'GET' | 'DELETE',
    signal?: AbortSignal,
  ) {
    const base = this.origin();
    let response: Response;
    try {
      response = await this.request(`${base}/api/v1/admin/users${path}`, {
        method,
        credentials: 'include',
        cache: 'no-store',
        signal,
        ...(method === 'DELETE'
          ? { headers: { 'X-CSRF-Protection': '1' } }
          : {}),
      });
    } catch {
      throw new AuthError('network');
    }
    if (!response.ok) throw new AuthError('http', response.status);
    return response;
  }

  /** Validates public fields and pagination; aborted reads reject as network errors. */
  async list(page: number, signal?: AbortSignal): Promise<UserList> {
    const response = await this.call(`?page=${page}`, 'GET', signal);
    try {
      const value = await response.json();
      if (
        !value ||
        !Array.isArray(value.users) ||
        !Number.isSafeInteger(value.total) ||
        value.total < 0 ||
        value.page !== page ||
        value.page_size !== 25 ||
        value.users.length > 25
      )
        throw new Error();
      const users: AdminUser[] = value.users.map((user: unknown) => {
        if (!user || typeof user !== 'object') throw new Error();
        const v = user as Record<string, unknown>;
        if (
          typeof v.id !== 'string' ||
          typeof v.name !== 'string' ||
          typeof v.email !== 'string' ||
          (v.role !== 'user' && v.role !== 'admin')
        )
          throw new Error();
        return { id: v.id, name: v.name, email: v.email, role: v.role };
      });
      return { users, total: value.total, page, page_size: 25 };
    } catch {
      throw new AuthError('protocol');
    }
  }

  /** Never retries: network failures can occur after a committed deletion. */
  async delete(id: string): Promise<void> {
    const response = await this.call(`/${encodeURIComponent(id)}`, 'DELETE');
    if (response.status !== 204) throw new AuthError('protocol');
  }
}

export function adminErrorMessage(error: unknown, deleting = false): string {
  if (error instanceof AuthError) {
    if (error.status === 403) return 'Доступ к админке запрещён.';
    if (error.status === 401)
      return 'Сессия истекла. Обновите список для проверки сессии.';
    if (error.status === 404)
      return 'Пользователь уже удалён. Обновите список.';
    if (error.status === 409)
      return 'Аккаунт администратора нельзя удалить. Обновите список.';
    if (error.kind === 'network' || error.kind === 'protocol')
      return deleting
        ? 'Не удалось подтвердить удаление. Обновите список перед повторной попыткой.'
        : 'Не удалось загрузить пользователей. Повторите попытку.';
    if (error.kind === 'config') return 'Не настроен адрес сервера.';
  }
  return 'Не удалось выполнить запрос. Попробуйте позже.';
}
