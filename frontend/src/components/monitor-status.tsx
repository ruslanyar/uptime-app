import type { CheckErrorCode, Monitoring } from '@/lib/monitors/api';
import { cn } from '@/lib/utils';

const labels = {
  unknown: 'Ожидает проверки',
  up: 'Доступен',
  down: 'Недоступен',
  blocked: 'Проверка запрещена',
};
const errors: Record<CheckErrorCode, string> = {
  http_status: 'Ошибка HTTP',
  dns: 'Не удалось определить адрес сайта',
  connection: 'Не удалось подключиться',
  tls: 'Ошибка защищённого соединения',
  timeout: 'Превышено время ожидания',
  redirect: 'Ошибка перенаправления',
  blocked_address: 'Адрес не публичный',
  network: 'Ошибка сети',
};

/** Presents the latest server check; stale results never use a current success color. */
export function MonitorStatus({ monitoring }: { monitoring: Monitoring }) {
  const { status, last_check: check, is_stale: stale } = monitoring;
  return (
    <div className="mt-2 space-y-1 text-xs">
      <p
        className={cn(
          'flex flex-wrap items-center gap-x-2 gap-y-1 font-medium',
          stale || status === 'unknown' || status === 'blocked'
            ? 'text-muted-foreground'
            : status === 'up'
              ? 'text-emerald-700 dark:text-emerald-400'
              : 'text-destructive',
        )}
      >
        <span className="inline-flex items-center gap-2">
          <span
            aria-hidden="true"
            className="size-2 shrink-0 rounded-full bg-current"
          />
          {stale ? 'Данные устарели' : labels[status]}
        </span>
        {stale && (
          <span>Последний результат: {labels[status].toLowerCase()}</span>
        )}
        {check?.http_status != null && <span>HTTP {check.http_status}</span>}
        {check?.response_time_ms != null && (
          <span>{check.response_time_ms} мс</span>
        )}
      </p>
      {check?.error_code && check.error_code !== 'http_status' && (
        <p className="text-muted-foreground">{errors[check.error_code]}</p>
      )}
      {check && (
        <p className="text-muted-foreground">
          Проверено:{' '}
          <time
            dateTime={check.checked_at}
            title={new Date(check.checked_at).toLocaleString('ru-RU')}
          >
            {new Date(check.checked_at).toLocaleString('ru-RU')}
          </time>
        </p>
      )}
    </div>
  );
}
