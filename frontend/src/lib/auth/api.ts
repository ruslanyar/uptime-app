export type User = { id: string; name: string; email: string };
export type Credentials = { email: string; password: string; name?: string };

export class AuthError extends Error {
  constructor(
    public kind:
      | 'http'
      | 'network'
      | 'config'
      | 'cookies'
      | 'uncertain'
      | 'unsupported'
      | 'protocol',
    public status = 0,
  ) {
    super(kind);
  }
}

export function apiOrigin(value: string | undefined): string {
  try {
    const url = new URL(value ?? '');
    if (
      !['http:', 'https:'].includes(url.protocol) ||
      url.username ||
      url.password ||
      url.search ||
      url.hash ||
      url.pathname !== '/' ||
      value !== url.origin
    )
      throw new Error();
    return url.origin;
  } catch {
    throw new AuthError('config');
  }
}

function user(value: unknown): User {
  if (!value || typeof value !== 'object') throw new AuthError('protocol');
  const v = value as Record<string, unknown>;
  if (
    typeof v.id !== 'string' ||
    typeof v.name !== 'string' ||
    typeof v.email !== 'string'
  )
    throw new AuthError('protocol');
  return { id: v.id, name: v.name, email: v.email };
}

export class AuthAPI {
  constructor(
    private origin: () => string = () =>
      apiOrigin(process.env.NEXT_PUBLIC_API_URL),
    private request: typeof fetch = (...args) => fetch(...args),
  ) {}
  async call(
    path: 'me' | 'register' | 'login' | 'refresh' | 'logout',
    body?: Credentials,
  ): Promise<User | undefined> {
    const base = this.origin();
    let response: Response;
    try {
      response = await this.request(`${base}/api/v1/auth/${path}`, {
        method: path === 'me' ? 'GET' : 'POST',
        credentials: 'include',
        cache: 'no-store',
        headers:
          path === 'me'
            ? {}
            : {
                'X-CSRF-Protection': '1',
                ...(body ? { 'Content-Type': 'application/json' } : {}),
              },
        ...(body ? { body: JSON.stringify(body) } : {}),
      });
    } catch {
      throw new AuthError('network');
    }
    if (!response.ok) throw new AuthError('http', response.status);
    if (path === 'logout') return;
    try {
      const data = await response.json();
      return user(path === 'me' ? data : data.user);
    } catch {
      throw new AuthError('protocol');
    }
  }
}

export function errorMessage(error: unknown): string {
  if (!(error instanceof AuthError))
    return 'Не удалось выполнить запрос. Попробуйте ещё раз.';
  if (error.kind === 'network')
    return 'Нет связи с сервером. Проверьте подключение и повторите проверку.';
  if (error.kind === 'uncertain')
    return 'Предыдущий запрос не удалось подтвердить. Повторите проверку сессии.';
  if (error.kind === 'config')
    return 'Не настроен адрес сервера. Обратитесь к администратору.';
  if (error.kind === 'unsupported')
    return 'Браузер не поддерживает безопасную работу с сессией. Используйте современный браузер и разрешите хранилище сайта.';
  if (error.kind === 'cookies')
    return 'Не удалось сохранить сессию. Разрешите cookies для этого сайта, включая сторонние cookies, и повторите вход.';
  if (error.status === 400) return 'Проверьте введённые данные.';
  if (error.status === 401) return 'Неверный email или пароль.';
  if (error.status === 403)
    return 'Сервер отклонил запрос. Проверьте настройки доступа к сайту.';
  if (error.status === 409) return 'Аккаунт с таким email уже существует.';
  return 'Сервер временно недоступен. Попробуйте позже.';
}
