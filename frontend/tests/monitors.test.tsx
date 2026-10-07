import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { Monitors } from '@/components/monitors';
import { AuthError } from '@/lib/auth/api';

const { list, create, session, current } = vi.hoisted(() => {
  const current = { status: 'authenticated', user: { id: 'owner' } };
  return {
    list: vi.fn(),
    create: vi.fn(),
    current,
    session: { check: vi.fn(), snapshot: () => current },
  };
});
vi.mock('@/components/auth-provider', () => ({ useAuth: () => ({ session }) }));
vi.mock('@/lib/monitors/api', async (original) => ({
  ...(await original<typeof import('@/lib/monitors/api')>()),
  MonitorAPI: class {
    list = list;
    create = create;
  },
}));
const item = {
  id: 'one',
  url: 'https://example.com',
  interval_seconds: 300,
  created_at: '2026-10-07T10:00:00Z',
  updated_at: '2026-10-07T10:00:00Z',
};
beforeEach(() => {
  list.mockReset().mockResolvedValue([]);
  create.mockReset().mockResolvedValue(item);
  session.check.mockReset().mockResolvedValue(undefined);
  current.status = 'authenticated';
  current.user.id = 'owner';
});
it('loads empty state and displays a created monitor immediately', async () => {
  render(<Monitors userID="owner" />);
  expect(screen.getByText('Загружаем сайты…')).toBeInTheDocument();
  await screen.findByText('Пока нет сайтов');
  fireEvent.click(screen.getByRole('button', { name: 'Добавить сайт' }));
  fireEvent.change(screen.getByLabelText('URL сайта'), {
    target: { value: item.url },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Создать' }));
  await screen.findByText(item.url);
  expect(screen.getByText('Интервал: 5 мин')).toBeInTheDocument();
  expect(screen.getByText('Сайт добавлен.')).toBeInTheDocument();
  expect(screen.queryByRole('form')).not.toBeInTheDocument();
  expect(session.check).toHaveBeenCalledOnce();
});
it('loads saved monitors', async () => {
  list.mockResolvedValue([item]);
  render(<Monitors userID="owner" />);
  await screen.findByText(item.url);
  expect(screen.queryByText('Пока нет сайтов')).not.toBeInTheDocument();
});
it('offers retry after a failed list request', async () => {
  list
    .mockRejectedValueOnce(new AuthError('network'))
    .mockResolvedValueOnce([item]);
  render(<Monitors userID="owner" />);
  await screen.findByRole('alert');
  fireEvent.click(screen.getByRole('button', { name: 'Повторить загрузку' }));
  await screen.findByText(item.url);
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
});
it('checks session and retries GET once on expired access', async () => {
  list
    .mockRejectedValueOnce(new AuthError('http', 401))
    .mockResolvedValueOnce([item]);
  render(<Monitors userID="owner" />);
  await screen.findByText(item.url);
  expect(session.check).toHaveBeenCalledOnce();
  expect(list).toHaveBeenCalledTimes(2);
});
it('does not send POST when the session no longer belongs to the screen user', async () => {
  render(<Monitors userID="owner" />);
  await screen.findByText('Пока нет сайтов');
  fireEvent.click(screen.getByRole('button', { name: 'Добавить сайт' }));
  fireEvent.change(screen.getByLabelText('URL сайта'), {
    target: { value: item.url },
  });
  current.user.id = 'other';
  fireEvent.click(screen.getByRole('button', { name: 'Создать' }));
  await screen.findByRole('alert');
  expect(create).not.toHaveBeenCalled();
});
it('ignores old responses after switching users', async () => {
  let finish!: (items: (typeof item)[]) => void;
  list
    .mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    )
    .mockResolvedValueOnce([]);
  const view = render(<Monitors key="owner" userID="owner" />);
  view.rerender(<Monitors key="other" userID="other" />);
  await screen.findByText('Пока нет сайтов');
  await act(async () => {
    finish([item]);
  });
  expect(screen.queryByText(item.url)).not.toBeInTheDocument();
});
it('blocks list refresh during creation so a late GET cannot remove the new card', async () => {
  let finish!: (value: typeof item) => void;
  create.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  render(<Monitors userID="owner" />);
  await screen.findByText('Пока нет сайтов');
  fireEvent.click(screen.getByRole('button', { name: 'Добавить сайт' }));
  fireEvent.change(screen.getByLabelText('URL сайта'), {
    target: { value: item.url },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Создать' }));
  await waitFor(() => expect(create).toHaveBeenCalledOnce());
  expect(
    screen.getByRole('button', { name: 'Обновить список сайтов' }),
  ).toBeDisabled();
  await act(async () => {
    finish(item);
  });
  await screen.findByText(item.url);
});
