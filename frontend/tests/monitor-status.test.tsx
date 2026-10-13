import { render, screen } from '@testing-library/react';
import { expect, it } from 'vitest';
import { MonitorStatus } from '@/components/monitor-status';
import type { Monitoring } from '@/lib/monitors/api';

const up: Monitoring = {
  status: 'up',
  last_check: {
    checked_at: '2026-10-07T10:00:00Z',
    http_status: 200,
    response_time_ms: 243,
    error_code: null,
  },
  next_check_at: '2026-10-07T10:05:00Z',
  is_stale: false,
};
it('shows successful headers, timing and a machine-readable timestamp', () => {
  render(<MonitorStatus monitoring={up} />);
  expect(screen.getByText('Доступен')).toBeInTheDocument();
  expect(screen.getByText('HTTP 200')).toBeInTheDocument();
  expect(screen.getByText('243 мс')).toBeInTheDocument();
  expect(document.querySelector('time')).toHaveAttribute(
    'datetime',
    up.last_check!.checked_at,
  );
});
it('shows waiting without inventing a result', () => {
  render(
    <MonitorStatus
      monitoring={{ ...up, status: 'unknown', last_check: null }}
    />,
  );
  expect(screen.getByText('Ожидает проверки')).toBeInTheDocument();
  expect(screen.queryByText('HTTP 200')).not.toBeInTheDocument();
});
it.each([
  ['down', 'timeout', 'Недоступен', 'Превышено время ожидания'],
  ['blocked', 'blocked_address', 'Проверка запрещена', 'Адрес не публичный'],
] as const)(
  'shows %s with a readable reason and no response timing',
  (status, error_code, label, reason) => {
    render(
      <MonitorStatus
        monitoring={{
          ...up,
          status,
          last_check: {
            ...up.last_check!,
            http_status: null,
            response_time_ms: null,
            error_code,
          },
        }}
      />,
    );
    expect(screen.getByText(label)).toBeInTheDocument();
    expect(screen.getByText(reason)).toBeInTheDocument();
    expect(screen.queryByText('243 мс')).not.toBeInTheDocument();
  },
);
it('marks stale results neutral and retains the last known state', () => {
  render(<MonitorStatus monitoring={{ ...up, is_stale: true }} />);
  expect(screen.getByText('Данные устарели')).toBeInTheDocument();
  expect(screen.getByText('Последний результат: доступен')).toBeInTheDocument();
  expect(screen.getByText('HTTP 200').parentElement).toHaveClass(
    'text-muted-foreground',
  );
});
